
"""Postmortem Agent microservice - FastAPI app with A2A endpoints."""

from __future__ import annotations
from dotenv import load_dotenv
load_dotenv()

import os

import mlflow
from fastapi import FastAPI

from app.a2a.models import AgentCapabilities, AgentCard, AgentSkill
from app.a2a.server import create_a2a_router
from app.observability.metrics import metrics_app
from app.agent import PostmortemAgent

PORT = int(os.getenv("PORT", "8093"))
API_BASE = os.getenv("OPENAI_API_BASE", "http://127.0.0.1:1234/v1")
API_KEY = os.getenv("OPENAI_API_KEY", "lm-studio")
MODEL = os.getenv("LLM_MODEL", "qwen/qwen2.5-coder-14b")
AGENT_URL = os.getenv("AGENT_URL", f"http://localhost:{PORT}")

# A2A mesh peer: see rca-agent/app/main.py for the same pattern.
RCA_AGENT_URL = os.getenv("RCA_AGENT_URL", "")

# MLflow tracing: same experiment across all 6 RootCauseway services so a
# stage-by-stage view of the incident pipeline is browsable in one
# place. See mlflow-server/ for the tracking server itself.
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow.rootcauseway.svc.cluster.local:5000"))
mlflow.set_experiment("rootcauseway-incident-pipeline")

agent = PostmortemAgent(api_base=API_BASE, api_key=API_KEY, model=MODEL, rca_agent_url=RCA_AGENT_URL)

card = AgentCard(
    name="RootCauseway Postmortem Agent",
    description="Generates blameless postmortems from full incident context",
    url=AGENT_URL,
    version="0.1.0",
    capabilities=AgentCapabilities(),
    skills=[
        AgentSkill(
            id="postmortem",
            name="Postmortem Generation",
            description="Generate a blameless postmortem with timeline, lessons learned, and action items",
        )
    ],
)

app = FastAPI(title="RootCauseway Postmortem Agent", version="0.1.0")
app.include_router(create_a2a_router(card, agent.handle_task))
app.mount("/metrics", metrics_app)


@app.get("/health")
async def health():
    return {"status": "ok", "agent": "postmortem"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
