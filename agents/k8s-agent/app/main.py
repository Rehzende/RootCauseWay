
"""K8s Debug Agent microservice - FastAPI app with A2A endpoints."""

from __future__ import annotations
from dotenv import load_dotenv
load_dotenv()

import json
import logging
import os
from datetime import datetime, timezone

import mlflow

from app.observability_metrics import record_swallowed_error
from fastapi import BackgroundTasks, FastAPI, Request

from app.a2a.models import AgentCapabilities, AgentCard, AgentSkill
from app.a2a.server import create_a2a_router
from app.agent import K8sDebugAgent
from app.telemetry import setup_telemetry

logger = logging.getLogger(__name__)

PORT = int(os.getenv("PORT", "8094"))
API_BASE = os.getenv("OPENAI_API_BASE", "http://127.0.0.1:1234/v1")
API_KEY = os.getenv("OPENAI_API_KEY", "lm-studio")
MODEL = os.getenv("LLM_MODEL", "qwen/qwen2.5-coder-14b")
AGENT_URL = os.getenv("AGENT_URL", f"http://localhost:{PORT}")

# MLflow tracing: same experiment across all 6 RootCauseway services so a
# stage-by-stage view of the incident pipeline is browsable in one place.
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow.rootcauseway.svc.cluster.local:5000"))
mlflow.set_experiment("rootcauseway-incident-pipeline")

# Prometheus/k8s service name -> OTel `service.name` — they don't always match
# (e.g. job "pulse-backend" traces as "pulso-backend"). Override/extend via
# SERVICE_OTEL_MAP='{"job-name": "otel-service-name"}'.
SERVICE_OTEL_MAP = {"pulse-backend": "pulso-backend"}
try:
    SERVICE_OTEL_MAP.update(json.loads(os.getenv("SERVICE_OTEL_MAP", "{}")))
except json.JSONDecodeError:
    logger.warning("SERVICE_OTEL_MAP is not valid JSON, ignoring")

agent = K8sDebugAgent(api_base=API_BASE, api_key=API_KEY, model=MODEL)

card = AgentCard(
    name="RootCauseway K8s Debug Agent",
    description="Collects Kubernetes cluster diagnostics, analyzes pod/node status, and recommends remediation",
    url=AGENT_URL,
    version="0.1.0",
    capabilities=AgentCapabilities(),
    skills=[
        AgentSkill(
            id="k8s-debug",
            name="K8s Debug",
            description="Debug Kubernetes issues by collecting pod status, logs, and events",
        ),
        AgentSkill(
            id="k8s-logs",
            name="K8s Log Analysis",
            description="Analyze Kubernetes pod logs for error patterns",
        ),
        AgentSkill(
            id="k8s-diagnostics",
            name="K8s Diagnostics",
            description="Run comprehensive Kubernetes cluster diagnostics",
        ),
        AgentSkill(
            id="incident-analysis",
            name="Incident Root Cause Analysis",
            description=(
                "Given a service name, correlates Kubernetes state with Prometheus metrics, "
                "Loki logs, Tempo traces and active Alertmanager alerts, then asks an LLM for "
                "a structured root-cause analysis. Pass {namespace, service, otel_service?} "
                "in the task message to trigger it."
            ),
        ),
    ],
)

app = FastAPI(title="RootCauseway K8s Debug Agent", version="0.1.0")
setup_telemetry(app)
app.include_router(create_a2a_router(card, agent.handle_task))


@app.get("/health")
async def health():
    return {"status": "ok", "agent": "k8s-debug"}


# In-memory ring buffer of recent auto-triggered analyses — a lightweight
# substitute for a UI: there isn't one deployed for this agent, so `GET
# /incidents` is the way to see what the webhook has analyzed without going
# through the A2A API by hand. Not persisted across restarts.
MAX_INCIDENTS = 30
recent_incidents: list[dict] = []


async def _run_and_record_analysis(namespace: str, service: str, alertname: str) -> None:
    otel_service = SERVICE_OTEL_MAP.get(service, service)
    logger.info(f"[webhook] Alert {alertname!r} firing for service={service!r} — running RCA")
    try:
        rca = await agent.analyze_service(namespace=namespace, service=service, otel_service=otel_service)
    except Exception as exc:
        record_swallowed_error("k8s_agent", "webhook_rca_failed")
        logger.exception(f"[webhook] RCA failed for service={service!r}")
        rca = {"error": str(exc)}

    entry = {
        "triggered_at": datetime.now(timezone.utc).isoformat(),
        "alertname": alertname,
        "namespace": namespace,
        "service": service,
        "analysis": rca,
    }
    recent_incidents.insert(0, entry)
    del recent_incidents[MAX_INCIDENTS:]

    # Also goes to stdout -> Loki, so it's queryable there even without /incidents.
    logger.info(f"[webhook] RCA result for {service}: {json.dumps(rca, default=str, ensure_ascii=False)}")


@app.post("/webhook/alertmanager")
async def alertmanager_webhook(request: Request, background_tasks: BackgroundTasks):
    """Alertmanager webhook receiver — for each *firing* alert that carries a
    `service` label, kicks off a root-cause analysis in the background (so
    Alertmanager gets its 200 back immediately instead of waiting on the LLM)
    and records the result for later retrieval via GET /incidents.

    Configure Alertmanager's receiver with:
      webhook_configs:
        - url: http://rootcauseway-k8s-agent.rootcauseway.svc.cluster.local:8094/webhook/alertmanager
    """
    payload = await request.json()
    triggered = []
    for a in payload.get("alerts", []):
        if a.get("status") != "firing":
            continue
        labels = a.get("labels", {})
        service = labels.get("service")
        if not service:
            continue
        namespace = labels.get("namespace", "default")
        alertname = labels.get("alertname", "unknown")
        background_tasks.add_task(_run_and_record_analysis, namespace, service, alertname)
        triggered.append({"service": service, "namespace": namespace, "alertname": alertname})

    return {"received_alerts": len(payload.get("alerts", [])), "analyses_triggered": triggered}


@app.get("/incidents")
async def list_incidents():
    """Recent auto-triggered analyses, most recent first."""
    return {"count": len(recent_incidents), "incidents": recent_incidents}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
