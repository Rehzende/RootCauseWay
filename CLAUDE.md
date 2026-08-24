# RootCauseway -- Root Cause Analysis Intelligence

## Structure
- backend/ -- Go API (Gin), port 8080
- agent-service/ -- Python orchestrator (FastAPI), port 8081
- agents/ -- A2A agent microservices (triage 8090, evidence 8091, rca 8092, postmortem 8093)
- frontend/ -- React (Vite + TailwindCSS), port 3000 (dev) / 80 (prod)
- contracts/ -- OpenAPI, Redis events, DB schema, A2A protocol
- scripts/ -- Migration runner, seed data

## Dev Commands
```
make up              # Start all services (dev docker compose)
make down            # Stop all services
make test            # Run all tests
make dev-backend     # Go API on :8080
make dev-agent       # Python orchestrator on :8081
make dev-frontend    # React on :3000
make ci              # Full CI checks locally (lint + test)
```

## Production
```
cp .env.prod.example .env.prod   # Fill in real values
make prod-build                  # Build all images
make prod-up                     # Start production stack
make prod-logs                   # Tail logs
```

## Database
PostgreSQL 17, migrations in backend/migrations/
```
make db-up           # Start postgres + redis
make db-migrate      # Apply pending migrations
make db-rollback     # Rollback last migration
make db-status       # Show migration status
```

## Testing
```
make test-backend    # go test ./... -v -count=1
make test-agent      # pytest tests/ -v
make test-frontend   # vitest run
make lint            # golangci-lint + eslint
```

## Architecture
The system uses an event-driven architecture with Redis as the message broker
(pub/sub dual-written to a Redis Stream; agent-service's AlertWorker consumes
the stream via consumer group).

Backend (Go) exposes the REST API and owns Postgres. agent-service (Python)
is the orchestrator: on `alert.received` it uses an LLM to dynamically decide
which agent skills to call and in what order (NOT a fixed pipeline stage
sequence). It dispatches to specialized A2A agent microservices, accumulates each
call's result as context for the next, and persists RCI/RCA/postmortem via
the backend's internal API.

Five agent microservices, each a separate deployment speaking the Google A2A
protocol (JSON-RPC, `tasks/send`): `triage-agent` (8090), `evidence-agent`
(8091), `rca-agent` (8092), `postmortem-agent` (8093), `k8s-agent` (8094,
also reachable standalone via Alertmanager webhook, independent of the
orchestrator -- see agents/k8s-agent/app/main.py). As of the A2A mesh work,
`rca-agent`, `evidence-agent` and `postmortem-agent` also hold a lightweight
A2A client of their own (`app/a2a/client.py`, distinct from
agent-service's full-featured one) so they can fetch supplementary data
directly from a peer agent when the orchestrator didn't include it --
bounded, non-recursive, deterministic triggers only (missing evidence/rca,
or a k8s-labeled alert), never LLM-decided fan-out.

All 6 Python services emit MLflow traces (`@mlflow.trace`) to a self-hosted
MLflow tracking server (mlflow-server/, one shared "rootcauseway-incident-pipeline"
experiment) for GenAI observability -- separate from the
Prometheus/Loki/Tempo/Grafana stack, which covers infra-level metrics/logs/
traces instead. agent-service also exposes `rootcauseway_swallowed_errors_total`
(component, error_type) so a `logger.exception(...)`-and-continue site
becomes a real Prometheus alert (`RootCausewayAgentServiceSwallowedError`) instead
of a log line nobody's watching.

Frontend is a React SPA served via nginx with API reverse proxy.
