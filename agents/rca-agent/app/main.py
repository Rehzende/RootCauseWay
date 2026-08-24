
"""RCA Agent microservice - FastAPI app with A2A endpoints."""

from __future__ import annotations
from dotenv import load_dotenv
load_dotenv()

import os

import mlflow
from fastapi import FastAPI

from app.a2a.models import AgentCapabilities, AgentCard, AgentSkill
from app.a2a.server import create_a2a_router
from app.observability.metrics import metrics_app
from app.agent import RCAAgent

PORT = int(os.getenv("PORT", "8092"))
API_BASE = os.getenv("OPENAI_API_BASE", "http://127.0.0.1:1234/v1")
API_KEY = os.getenv("OPENAI_API_KEY", "lm-studio")
MODEL = os.getenv("LLM_MODEL", "qwen/qwen2.5-coder-14b")
AGENT_URL = os.getenv("AGENT_URL", f"http://localhost:{PORT}")

# A2A mesh peers: URLs of the other agents this one can call directly for
# supplementary data (see agent.py). Empty string disables the link (the
# constructor treats "" as "peer not configured").
EVIDENCE_AGENT_URL = os.getenv("EVIDENCE_AGENT_URL", "")
K8S_AGENT_URL = os.getenv("K8S_AGENT_URL", "")

# MLflow tracing: same experiment across all 6 RootCauseway services so a
# stage-by-stage view of the incident pipeline is browsable in one
# place. See mlflow-server/ for the tracking server itself.
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow.rootcauseway.svc.cluster.local:5000"))
mlflow.set_experiment("rootcauseway-incident-pipeline")

agent = RCAAgent(
    api_base=API_BASE, api_key=API_KEY, model=MODEL,
    evidence_agent_url=EVIDENCE_AGENT_URL, k8s_agent_url=K8S_AGENT_URL,
)

card = AgentCard(
    name="RootCauseway RCA Agent",
    description="Performs Root Cause Investigation, Root Cause Analysis, and hypothesis generation",
    url=AGENT_URL,
    version="0.1.0",
    capabilities=AgentCapabilities(),
    skills=[
        AgentSkill(
            id="rca",
            name="Root Cause Analysis",
            description="Generate RCI, RCA with 5 Whys, and root cause hypothesis from alert, triage, and evidence",
        )
    ],
)

app = FastAPI(title="RootCauseway RCA Agent", version="0.1.0")
app.include_router(create_a2a_router(card, agent.handle_task))
app.mount("/metrics", metrics_app)


@app.get("/health")
async def health():
    return {"status": "ok", "agent": "rca"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
