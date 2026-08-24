"""Minimal CRUD app used as the real Azure-lab workload for RootCauseway's
validation exercise -- see project_backlog.md ("Projeto grande em
planejamento: validação em Azure real"). Its only job is to be a genuine
AKS + Postgres consumer with real traffic and real Prometheus metrics, so
the fault-injection scenarios (network/AKS/storage/Key Vault/Postgres)
have something actually observable to break.

Same instrumentation pattern as RootCauseway's own Python services
(prometheus-fastapi-instrumentator -> /metrics), per CLAUDE.md.
"""
from __future__ import annotations

import os
import logging

import asyncpg
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from prometheus_fastapi_instrumentator import Instrumentator

logger = logging.getLogger("demo-app")
logging.basicConfig(level=logging.INFO)

app = FastAPI(title="RootCauseway Azure Lab Demo App")
Instrumentator().instrument(app).expose(app)  # -> GET /metrics

pool: asyncpg.Pool | None = None


class Item(BaseModel):
    name: str
    description: str = ""


def _dsn() -> str:
    host = os.environ["DB_HOST"]
    user = os.environ["DB_USER"]
    password = os.environ["DB_PASSWORD"]
    dbname = os.environ.get("DB_NAME", "postgres")
    return f"postgresql://{user}:{password}@{host}:5432/{dbname}?sslmode=require"


@app.on_event("startup")
async def startup() -> None:
    global pool
    pool = await asyncpg.create_pool(dsn=_dsn(), min_size=1, max_size=5, command_timeout=10)
    async with pool.acquire() as conn:
        await conn.execute(
            """
            CREATE TABLE IF NOT EXISTS items (
                id SERIAL PRIMARY KEY,
                name TEXT NOT NULL,
                description TEXT NOT NULL DEFAULT '',
                created_at TIMESTAMPTZ NOT NULL DEFAULT now()
            )
            """
        )
    logger.info("startup complete, db pool ready")


@app.on_event("shutdown")
async def shutdown() -> None:
    if pool is not None:
        await pool.close()


@app.get("/healthz")
async def healthz():
    """Liveness: process is up. Doesn't touch the DB on purpose, so it
    stays accurate even during a deliberate DB-outage fault scenario."""
    return {"status": "ok"}


@app.get("/readyz")
async def readyz():
    """Readiness: process AND database are both reachable. This is the
    one that should flip during the Postgres/network fault scenarios."""
    if pool is None:
        raise HTTPException(status_code=503, detail="db pool not initialized")
    try:
        async with pool.acquire() as conn:
            await conn.fetchval("SELECT 1")
    except Exception as e:
        raise HTTPException(status_code=503, detail=f"db unreachable: {e}")
    return {"status": "ok"}


@app.get("/items")
async def list_items():
    async with pool.acquire() as conn:
        rows = await conn.fetch("SELECT id, name, description, created_at FROM items ORDER BY id DESC LIMIT 50")
    return [dict(r) for r in rows]


@app.post("/items", status_code=201)
async def create_item(item: Item):
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "INSERT INTO items (name, description) VALUES ($1, $2) RETURNING id, name, description, created_at",
            item.name, item.description,
        )
    return dict(row)
