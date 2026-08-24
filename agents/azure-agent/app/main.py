"""Azure Agent microservice - FastAPI app with A2A endpoints."""

from __future__ import annotations
from dotenv import load_dotenv
load_dotenv()

import logging
import os

import mlflow
from fastapi import FastAPI

from app.a2a.models import AgentCapabilities, AgentCard, AgentSkill
from app.a2a.server import create_a2a_router
from app.agent import AzureAgent
from app.telemetry import setup_telemetry

logger = logging.getLogger(__name__)

PORT = int(os.getenv("PORT", "8095"))
AGENT_URL = os.getenv("AGENT_URL", f"http://localhost:{PORT}")

# MLflow tracing: same experiment across all 6 RootCauseway services so a
# stage-by-stage view of the incident pipeline is browsable in one place.
mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://mlflow.rootcauseway.svc.cluster.local:5000"))
mlflow.set_experiment("rootcauseway-incident-pipeline")

agent = AzureAgent()

card = AgentCard(
    name="RootCauseway Azure Agent",
    description=(
        "Collects Azure-native evidence (Activity Log, Key Vault "
        "properties/RBAC, NSG rules, network-only reachability) directly "
        "from Azure's management APIs, without needing kubectl access "
        "into the target cluster."
    ),
    url=AGENT_URL,
    version="0.1.0",
    capabilities=AgentCapabilities(),
    skills=[
        AgentSkill(
            id="azure-aks-activity-log",
            name="AKS Activity Log",
            description=(
                "Pulls Azure Monitor Activity Log entries for an AKS cluster's "
                "control plane (node pool scaling, upgrades, API server "
                "operations) -- events kubectl run from inside the cluster "
                "can never see. Reads software_context.cloud_resources "
                "{subscription_id, resource_group, aks_cluster}."
            ),
        ),
        AgentSkill(
            id="azure-keyvault-diagnostics",
            name="Key Vault Diagnostics",
            description=(
                "Reads Key Vault properties and current RBAC role assignments. "
                "NOT a secret-access audit trail (that needs a Log Analytics "
                "workspace, not provisioned). Reads "
                "software_context.cloud_resources {subscription_id, "
                "resource_group, key_vault}."
            ),
        ),
        AgentSkill(
            id="azure-network-diagnostics",
            name="Network Diagnostics",
            description=(
                "NSG rule dump for the resource's network security group, plus "
                "a raw TCP reachability check (timeout/refused/reset/DNS "
                "classification) against the software's database host:port -- "
                "network-only, no credential stored. Reads "
                "software_context.cloud_resources {nsg} and "
                "software_context.databases {host, port}."
            ),
        ),
    ],
)

app = FastAPI(title="RootCauseway Azure Agent", version="0.1.0")
setup_telemetry(app)
app.include_router(create_a2a_router(card, agent.handle_task))


@app.get("/health")
async def health():
    return {"status": "ok", "agent": "azure-agent"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
