"""FastAPI application entry point."""

from __future__ import annotations

import asyncio
import logging
from contextlib import asynccontextmanager

import mlflow
import redis.asyncio as redis
from fastapi import FastAPI

from app.config.settings import get_settings
from app.observability.metrics import StreamQueueDepthUpdater, metrics_app
from app.orchestrator.resume_listener import ResumeListener
from app.services.backend_client import BackendClient
from app.services.event_publisher import EventPublisher
from app.warroom.consumer import WarRoomConsumer
from app.workers.alert_worker import AlertWorker

logger = logging.getLogger(__name__)

worker_instance: AlertWorker | None = None
worker_task: asyncio.Task | None = None
warroom_consumer_instance: WarRoomConsumer | None = None
warroom_consumer_task: asyncio.Task | None = None
queue_depth_updater: StreamQueueDepthUpdater | None = None
resume_listener_instance: ResumeListener | None = None
resume_listener_task: asyncio.Task | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global worker_instance, worker_task, warroom_consumer_instance, warroom_consumer_task, queue_depth_updater
    global resume_listener_instance, resume_listener_task

    settings = get_settings()
    logging.basicConfig(level=settings.log_level.upper())
    mlflow.set_tracking_uri(settings.mlflow_tracking_uri)
    mlflow.set_experiment(settings.mlflow_experiment_name)

    redis_client = redis.from_url(settings.redis_url)
    backend_client = BackendClient(settings.backend_api_url)
    event_publisher = EventPublisher(redis_client)

    worker_instance = AlertWorker(redis_client, backend_client, event_publisher)
    worker_task = asyncio.create_task(worker_instance.start())
    logger.info("Alert worker started as background task")

    warroom_consumer_instance = WarRoomConsumer(redis_client, backend_client)
    warroom_consumer_task = asyncio.create_task(warroom_consumer_instance.start())
    logger.info("War room consumer started as background task")

    queue_depth_updater = StreamQueueDepthUpdater(settings.redis_url, settings.event_stream_name)
    await queue_depth_updater.start()

    # Pipeline HITL gate: resumes postmortem for incidents once a human
    # approves via POST /incidents/{id}/approve-stage (pipeline.stage_approved
    # event). Reuses worker_instance's already-wired Orchestrator (same
    # backend_client/a2a_client/llm_call/JIT provider) rather than
    # constructing a second one, so this and AlertWorker always act on
    # consistent orchestrator state.
    resume_listener_instance = ResumeListener(redis_client, worker_instance._orchestrator)
    resume_listener_task = asyncio.create_task(resume_listener_instance.start())
    logger.info("Pipeline gate resume listener started as background task")

    yield

    if queue_depth_updater:
        await queue_depth_updater.stop()
    if worker_instance:
        await worker_instance.stop()
    if worker_task:
        worker_task.cancel()
        try:
            await worker_task
        except asyncio.CancelledError:
            pass
    if warroom_consumer_instance:
        await warroom_consumer_instance.stop()
    if warroom_consumer_task:
        warroom_consumer_task.cancel()
        try:
            await warroom_consumer_task
        except asyncio.CancelledError:
            pass
    if resume_listener_instance:
        await resume_listener_instance.stop()
    if resume_listener_task:
        resume_listener_task.cancel()
        try:
            await resume_listener_task
        except asyncio.CancelledError:
            pass
    await backend_client.close()
    await redis_client.aclose()
    logger.info("Shutdown complete")


app = FastAPI(title="RootCauseway Agent Service", version="0.1.0", lifespan=lifespan)

app.mount("/metrics", metrics_app)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/status")
async def status():
    return {
        "worker_running": worker_instance._running if worker_instance else False,
        "warroom_consumer_running": warroom_consumer_instance._running if warroom_consumer_instance else False,
        "resume_listener_running": resume_listener_instance._running if resume_listener_instance else False,
        "features": {
            "loop_engineering": True,
            "correlation": True,
            "notifications": True,
            "runbooks": True,
            "war_room": True,
        },
    }
