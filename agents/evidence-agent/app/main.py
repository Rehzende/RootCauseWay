
"""Evidence Agent microservice - FastAPI app with A2A endpoints."""

from __future__ import annotations
from dotenv import load_dotenv
load_dotenv()

import os

import mlflow
from fastapi import FastAPI

from app.a2a.models import AgentCapabilities, AgentCard, AgentSkill
from app.a2a.server import create_a2a_router
from app.observability.metrics import metrics_app
from app.agent import EvidenceAgent

PORT = int(os.getenv("PORT", "8091"))
API_BASE = os.getenv("OPENAI_API_BASE", "http://127.0.0.1:1234/v1")
API_KEY = os.getenv("OPENAI_API_KEY", "lm-studio")
MODEL = os.getenv("LLM_MODEL", "qwen/qwen2.5-coder-14b")
AGENT_URL = os.getenv("AGENT_URL", f"http://localhost:{PORT}")

# A2A mesh peer: see rca-agent/app/main.py for the same pattern.
K8S_AGENT_URL = os.getenv("K8S_AGENT_URL", "")

# MLflow tracing: same experiment across all 6 RootCauseway services so a
# stage-by-stage view of the incident pipeline is browsable in one
# place. See mlflow-server/ for the tracking server itself.
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow.rootcauseway.svc.cluster.local:5000"))
mlflow.set_experiment("rootcauseway-incident-pipeline")

agent = EvidenceAgent(api_base=API_BASE, api_key=API_KEY, model=MODEL, k8s_agent_url=K8S_AGENT_URL)

card = AgentCard(
    name="RootCauseway Evidence Agent",
    description="Analyzes what evidence to collect and recommends data sources and queries",
    url=AGENT_URL,
    version="0.1.0",
    capabilities=AgentCapabilities(),
    skills=[
        AgentSkill(
            id="evidence-collection",
            name="Evidence Collection",
            description="Analyze alert context to determine evidence to collect, data sources, and log queries",
        )
    ],
)

app = FastAPI(title="RootCauseway Evidence Agent", version="0.1.0")
app.include_router(create_a2a_router(card, agent.handle_task))
app.mount("/metrics", metrics_app)


@app.get("/health")
async def health():
    return {"status": "ok", "agent": "evidence"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
