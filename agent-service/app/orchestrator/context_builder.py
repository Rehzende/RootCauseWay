"""Builds rich context for the orchestrator from the software catalog."""

from __future__ import annotations

import logging
from typing import Any
from uuid import UUID

from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)


class ContextBuilder:
    """Fetches software catalog data and structures it for LLM consumption."""

    async def build_context(self, software_id: str, backend_client: BackendClient, org_id: UUID) -> dict[str, Any]:
        """Fetch the software entry with all enriched fields and return structured context.

        Found live: every key read here except "dependencies" didn't match
        any field the Go backend's SoftwareEntry actually returns (it's
        repository_url/pipeline_url/database_info/stakeholders/sre_team/
        architects -- not repository/ci_cd_pipeline/databases/team, and
        there's no "observability" or "sla" field on this model at all).
        Every one of those `.get(key, default)` calls silently fell through
        to its empty default, every time, for every incident, since this
        method was written -- filling in a software's cloud resources,
        databases, repo, pipeline, or team via the catalog UI had zero
        effect on RCA/postmortem quality no matter how thoroughly a user
        populated it. `infra_details` (a real field) was never read at all.
        """
        try:
            software = await backend_client.get_software(software_id, org_id)
        except Exception:
            logger.warning("Failed to fetch software %s, using minimal context", software_id)
            software = {"id": software_id}

        return {
            "software": {
                "id": software.get("id", software_id),
                "name": software.get("name", "unknown"),
                "description": software.get("description", ""),
                "status": software.get("status", ""),
                "tags": software.get("tags") or [],
                # criticality/type: business-impact tier and entry kind, so
                # the LLM can weigh e.g. a "critical" service's incident more
                # heavily than a "low"-tier internal job's.
                "criticality": software.get("criticality", "medium"),
                "type": software.get("type", "service"),
            },
            "repository_url": software.get("repository_url", ""),
            "pipeline_url": software.get("pipeline_url", ""),
            "runbook_url": software.get("runbook_url", ""),
            "dashboard_url": software.get("dashboard_url", ""),
            "cloud_provider": software.get("cloud_provider", ""),
            "cloud_resources": software.get("cloud_resources") or [],
            "databases": software.get("database_info") or [],
            "infra_details": software.get("infra_details") or {},
            "dependencies": software.get("dependencies") or [],
            "team": {
                "stakeholders": software.get("stakeholders") or [],
                "sre_team": software.get("sre_team") or [],
                "architects": software.get("architects") or [],
            },
        }
