"""Correlation engine: checks if incoming alerts should be correlated with existing incidents."""

from __future__ import annotations

import logging
from typing import Any
from uuid import UUID

from app.models.api import IncidentEventCreate
from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)


class CorrelationEngine:
    """Checks if incoming alerts should be correlated with existing incidents.

    Correlation is attempted in this order (first match wins):

    1. Fingerprint dedup -- a literally-repeated alert (identical `fingerprint`)
       that already has a recent incident short-circuits everything else below;
       it's treated as a duplicate, not a fresh correlation decision.
    2. Same-service correlation -- existing behavior: correlation rules + an
       open incident on the exact same software_id within the time window.
    3. Dependency-graph cascade -- if the alert's service didn't correlate
       directly, check open incidents on services it depends on (upstream) or
       that depend on it (downstream), per the software catalog's
       `dependencies` field. This catches cascading failures (e.g. a database
       outage that trips alerts on every dependent service) so they land on
       one incident instead of one-per-service.

    Whenever a dedup or dependency-cascade match is found, the match is also
    recorded on the incident timeline (via `add_incident_event`) with a
    `matched_on` payload describing which relationship/fingerprint matched, so
    it's distinguishable from a plain same-service correlation.
    """

    DEFAULT_TIME_WINDOW = 300  # 5 minutes
    DEFAULT_DEDUP_WINDOW = 900  # 15 minutes; see settings.correlation_dedup_window_seconds

    async def check_correlation(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        software_id: str,
        alert: dict[str, Any],
        time_window_seconds: int = DEFAULT_TIME_WINDOW,
        dedup_window_seconds: int = DEFAULT_DEDUP_WINDOW,
        exclude_incident_id: UUID | None = None,
    ) -> UUID | None:
        """Check if alert correlates with an existing open incident.

        Returns the existing incident_id to correlate against, or None if a new
        incident should be created.

        `exclude_incident_id`: the incident IngestAlert (Go) already created
        for THIS exact alert instance, before alert.received was even
        published. Found live: without excluding it, every brand-new
        incident is trivially "an open incident on this software_id within
        the window" (itself) and self-correlates -- the pipeline silently
        never runs for it. Callers processing a fresh alert.received event
        should always pass payload.incident_id here.
        """
        dedup_match = await self._check_fingerprint_dedup(
            backend_client, org_id, alert, dedup_window_seconds, exclude_incident_id,
        )
        if dedup_match:
            return dedup_match

        same_service_match = await self._check_same_service(
            backend_client, org_id, software_id, alert, time_window_seconds, exclude_incident_id,
        )
        if same_service_match:
            return same_service_match

        return await self._check_dependency_cascade(
            backend_client, org_id, software_id, alert, time_window_seconds,
        )

    async def _check_fingerprint_dedup(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        alert: dict[str, Any],
        dedup_window_seconds: int,
        exclude_incident_id: UUID | None = None,
    ) -> UUID | None:
        """Short-circuit correlation entirely when this exact alert (by
        fingerprint) already has a recent incident -- it's a duplicate firing,
        not a new correlation decision."""
        fingerprint = alert.get("fingerprint")
        if not fingerprint:
            return None

        try:
            result = await backend_client.find_incident_by_fingerprint(
                org_id=org_id, fingerprint=fingerprint, window_seconds=dedup_window_seconds,
                exclude_incident_id=exclude_incident_id,
            )
        except Exception:
            logger.warning("Fingerprint dedup lookup failed, continuing with correlation")
            return None

        if not result or not result.get("id"):
            return None

        incident_id = UUID(result["id"])
        logger.info(
            "Alert deduplicated against existing incident %s (fingerprint: %s)",
            incident_id, fingerprint,
        )
        await self._record_match(
            backend_client, org_id, incident_id, alert,
            match_type="fingerprint_dedup",
            matched_on={"fingerprint": fingerprint},
        )
        return incident_id

    async def _check_same_service(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        software_id: str,
        alert: dict[str, Any],
        time_window_seconds: int,
        exclude_incident_id: UUID | None = None,
    ) -> UUID | None:
        """Original correlation path: correlation rules + open incident on the
        exact same software_id within the time window."""
        try:
            rules = await backend_client.list_correlation_rules(org_id)
        except Exception:
            logger.warning("Failed to fetch correlation rules, skipping correlation")
            return None

        # Use org-specific time window if configured in rules
        effective_window = time_window_seconds
        for rule in rules:
            if rule.get("time_window_seconds"):
                effective_window = rule["time_window_seconds"]
                break

        try:
            result = await backend_client.check_correlation(
                org_id=org_id,
                software_id=software_id,
                alert=alert,
                time_window_seconds=effective_window,
                exclude_incident_id=exclude_incident_id,
            )
        except Exception:
            logger.warning("Correlation check failed, treating as new incident")
            return None

        if result and result.get("incident_id"):
            incident_id = UUID(result["incident_id"])
            logger.info(
                "Alert correlated with existing incident %s (rule: %s)",
                incident_id,
                result.get("rule_id", "default"),
            )
            return incident_id

        return None

    async def _check_dependency_cascade(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        software_id: str,
        alert: dict[str, Any],
        time_window_seconds: int,
    ) -> UUID | None:
        """Check open incidents on services upstream (dependencies) or
        downstream (dependents) of software_id, per the software catalog's
        dependency graph -- catches cascading failures across related services."""
        try:
            graph = await backend_client.get_software_dependency_graph(software_id)
        except Exception:
            logger.debug(
                "Failed to fetch dependency graph for %s, skipping cascade correlation", software_id,
            )
            return None

        if not graph:
            return None

        relation_by_id: dict[str, str] = {}
        related_ids: list[str] = []
        for entry in graph.get("upstream") or []:
            sid = entry.get("software_id")
            if sid:
                relation_by_id[sid] = "upstream_dependency"
                related_ids.append(sid)
        for entry in graph.get("downstream") or []:
            sid = entry.get("software_id")
            if sid:
                relation_by_id.setdefault(sid, "downstream_dependent")
                related_ids.append(sid)

        if not related_ids:
            return None

        try:
            incidents = await backend_client.list_open_incidents_by_software(
                org_id=org_id, software_ids=related_ids, time_window_seconds=time_window_seconds,
            )
        except Exception:
            logger.warning("Dependency-graph correlation check failed, treating as new incident")
            return None

        if not incidents:
            return None

        match = incidents[0]
        incident_id = UUID(match["id"])
        relation = relation_by_id.get(str(match.get("software_id")), "dependency")
        logger.info(
            "Alert cascaded to existing incident %s via %s (software_id: %s)",
            incident_id, relation, software_id,
        )
        await self._record_match(
            backend_client, org_id, incident_id, alert,
            match_type="dependency_cascade",
            matched_on={"relationship": relation, "software_id": software_id},
        )
        return incident_id

    async def _record_match(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        incident_id: UUID,
        alert: dict[str, Any],
        match_type: str,
        matched_on: dict[str, Any],
    ) -> None:
        """Record a dedup/cascade correlation match on the incident timeline so
        it's distinguishable from a plain same-service correlation."""
        try:
            await backend_client.add_incident_event(
                incident_id,
                IncidentEventCreate(
                    type="correlated_alert",
                    data={"match_type": match_type, "matched_on": matched_on, "alert": alert},
                ),
                org_id,
            )
        except Exception:
            logger.warning("Failed to record correlation match event on incident %s", incident_id)
