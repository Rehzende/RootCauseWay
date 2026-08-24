"""Outer loop: learns from past incidents and provides few-shot context."""

from __future__ import annotations

import logging
from typing import Any
from uuid import UUID

from app.observability.metrics import record_swallowed_error
from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)


class OuterLoop:
    """Learns from past incidents and provides few-shot context."""

    async def find_similar_incidents(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        software_id: str,
        alert: dict[str, Any],
        category: str | None = None,
    ) -> list[dict[str, Any]]:
        """Search knowledge base for similar past incidents."""
        try:
            query = self._build_search_query(alert, category)
            entries = await backend_client.search_knowledge_base(
                org_id=org_id,
                software_id=software_id,
                query=query,
                limit=5,
            )
            return entries
        except Exception:
            logger.warning("Failed to search knowledge base for similar incidents")
            return []

    async def build_few_shot_context(
        self, similar_entries: list[dict[str, Any]]
    ) -> str:
        """Format past incidents as few-shot examples for the LLM prompt.

        Field names here must match backend/internal/models/features.go's
        KnowledgeBaseEntry JSON tags exactly -- they previously didn't
        (read "title"/"alert_pattern"/"root_cause"/"resolution", real
        payload has no "title" at all and uses "error_pattern"/
        "root_cause_summary"/"resolution_summary"), so every real search
        hit rendered as a wall of "N/A" and the actual substantive text
        never reached this prompt.
        """
        if not similar_entries:
            return ""

        sections = []
        for i, entry in enumerate(similar_entries, 1):
            title = entry.get("error_pattern") or f"Past Incident {i}"
            section = (
                f"### Example {i}: {title}\n"
                f"**Root Cause:** {entry.get('root_cause_summary', 'N/A')}\n"
                f"**Category:** {entry.get('category', 'N/A')}\n"
                f"**Resolution:** {entry.get('resolution_summary', 'N/A')}\n"
                f"**Confidence:** {entry.get('confidence', 'N/A')}\n"
            )
            if entry.get("human_validated"):
                section += "**Status:** Human-validated\n"
            sections.append(section)

        return "## Similar Past Incidents\n\n" + "\n".join(sections)

    async def extract_and_store_knowledge(
        self,
        backend_client: BackendClient,
        incident_id: UUID,
        org_id: UUID,
        software_id: str,
        rca_data: dict[str, Any],
        postmortem_data: dict[str, Any] | None = None,
    ) -> None:
        """After incident resolution, extract knowledge and store in KB.

        Field names must match CreateKnowledgeBaseRequest's JSON tags
        exactly (backend/internal/models/features.go) -- they previously
        didn't ("title"/"root_cause"/"resolution" aren't fields on that
        struct at all; the real name is "root_cause_summary", which is
        also `binding:"required"`), so this call 400'd on every single
        invocation since it was written. Silently: the exception was only
        ever logger.warning'd, never counted by
        rootcauseway_swallowed_errors_total, so it never became an alert either.
        """
        root_cause_summary = rca_data.get("root_cause_summary", "")
        if not root_cause_summary:
            # binding:"required" on the Go side would 400 this anyway --
            # skip the doomed request instead of spending it.
            logger.info(
                "Skipping knowledge base write for incident %s: no root_cause_summary in rca_data",
                incident_id,
            )
            return
        try:
            prevention_measures = postmortem_data.get("prevention_measures", []) if postmortem_data else []
            knowledge_entry = {
                "incident_id": str(incident_id),
                "software_id": software_id,
                "category": rca_data.get("root_cause_category", "unknown"),
                # No "title" field exists on the backend entry -- the RCA's
                # own five_whys/contributing_factors don't describe the
                # triggering alert either, so root_cause_summary itself
                # (truncated) is the most honest available proxy for
                # "what pattern should match this in the future".
                "error_pattern": root_cause_summary[:500],
                "root_cause_summary": root_cause_summary,
                "resolution_summary": "; ".join(prevention_measures) if prevention_measures else "",
                "lessons_learned": (postmortem_data.get("lessons_learned", []) if postmortem_data else []),
                "action_items": (postmortem_data.get("action_items", []) if postmortem_data else []),
                "tags": [rca_data["root_cause_category"]] if rca_data.get("root_cause_category") else [],
            }
            await backend_client.create_knowledge_entry(org_id, knowledge_entry)
            logger.info("Stored knowledge entry for incident %s", incident_id)
        except Exception:
            record_swallowed_error("outer_loop", "knowledge_entry_create_failed")
            logger.warning("Failed to store knowledge entry for incident %s", incident_id)

    def _build_search_query(
        self, alert: dict[str, Any], category: str | None
    ) -> str:
        """Build a search query from alert data."""
        parts = []
        if alert.get("alert_name"):
            parts.append(alert["alert_name"])
        if alert.get("service"):
            parts.append(alert["service"])
        if alert.get("summary"):
            parts.append(alert["summary"])
        if category:
            parts.append(category)
        return " ".join(parts) if parts else "incident"
