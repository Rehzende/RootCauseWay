"""Redis Streams consumer worker that processes alert events via A2A orchestrator.

Consumes the durable ``rootcauseway:events`` stream with a consumer group (XREADGROUP),
acks only after successful processing, retries failures with exponential
backoff, dead-letters exhausted messages to ``rootcauseway:events:dlq``, and reclaims
stale pending entries from crashed consumers on startup (XAUTOCLAIM).
See contracts/events/redis-events.yaml.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import socket
import uuid
from datetime import datetime, timezone
from typing import Any

import httpx
import mlflow
import redis.asyncio as redis
from mlflow.entities import SpanType
from redis.exceptions import ResponseError

from app.a2a.client import A2AClient
from app.config.settings import get_settings
from app.correlation.engine import CorrelationEngine
from app.evidence.snapshot_collector import SnapshotCollector
from app.models.api import IncidentEventCreate
from app.models.events import (
    AgentStatusPayload,
    AlertReceivedPayload,
)
from app.notifications.dispatcher import NotificationDispatcher
from app.observability.metrics import record_stale_event_skipped, record_swallowed_error
from app.orchestrator.orchestrator import Orchestrator
from app.runbooks.executor import RunbookExecutor
from app.services.backend_client import BackendClient
from app.services.event_publisher import EventPublisher

logger = logging.getLogger(__name__)


@mlflow.trace(span_type=SpanType.LLM, name="orchestrator.skill_selection_llm")
async def _llm_call(prompt: str) -> tuple[str, dict[str, Any]]:
    """Call the LLM via OpenAI-compatible API (LiteLLM / LM Studio / vLLM).

    Returns (content, llm_usage) -- llm_usage carries the real
    model/prompt_tokens/completion_tokens/total_tokens reported by the API
    response's own `usage` field, same shape as every A2A agent's
    `_call_llm`. Callers that don't care can discard the second value.
    """
    settings = get_settings()
    async with httpx.AsyncClient(timeout=120.0) as client:
        resp = await client.post(
            f"{settings.openai_api_base}/chat/completions",
            headers={"Authorization": f"Bearer {settings.openai_api_key}"},
            json={
                "model": settings.llm_model,
                "messages": [{"role": "user", "content": prompt}],
                "temperature": 0.2,
            },
        )
        resp.raise_for_status()
        data = resp.json()
        usage = data.get("usage") or {}
        llm_usage = {
            "model": data.get("model") or settings.llm_model,
            "prompt_tokens": usage.get("prompt_tokens", 0),
            "completion_tokens": usage.get("completion_tokens", 0),
            "total_tokens": usage.get("total_tokens", 0),
        }
        return data["choices"][0]["message"]["content"], llm_usage


def _as_str(value: Any) -> str:
    """Decode redis bytes responses to str (client may not use decode_responses)."""
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return str(value)


class AlertWorker:
    # Event types this worker processes; others on the stream are acked and skipped.
    HANDLED_EVENT_TYPES = frozenset({
        "alert.received", "evidence.uploaded", "incident.resolved", "runbook.execution.started",
    })

    def __init__(
        self,
        redis_client: redis.Redis,
        backend_client: BackendClient,
        event_publisher: EventPublisher,
        consumer_name: str | None = None,
    ):
        self._redis = redis_client
        self._backend = backend_client
        self._publisher = event_publisher
        self._running = False
        self._a2a_client = A2AClient()
        self._orchestrator = Orchestrator(
            backend_client=backend_client,
            a2a_client=self._a2a_client,
            llm_call=_llm_call,
        )
        self._correlation = CorrelationEngine()
        self._notifier = NotificationDispatcher()
        self._runbook_executor = RunbookExecutor()

        settings = get_settings()
        self._stream = settings.event_stream_name
        self._group = settings.event_consumer_group
        self._dlq_stream = settings.event_dlq_stream
        self._dlq_maxlen = settings.event_dlq_maxlen
        self._max_retries = settings.event_max_retries
        self._backoff_base = settings.event_retry_backoff_base
        self._autoclaim_idle_ms = settings.event_autoclaim_idle_ms
        # Unique per instance so multiple replicas can share the consumer group.
        self._consumer = consumer_name or f"{socket.gethostname()}-{os.getpid()}"

    async def start(self) -> None:
        """Consume events from the durable Redis Stream via a consumer group."""
        self._running = True
        await self._ensure_group()

        # Recover messages left pending by crashed/restarted consumers.
        try:
            claimed = await self._claim_stale_messages()
            if claimed:
                logger.info("Recovered %d stale pending stream entries", claimed)
        except Exception:
            logger.exception("Failed to autoclaim stale pending entries on startup")

        logger.info(
            "AlertWorker started: stream=%s group=%s consumer=%s",
            self._stream, self._group, self._consumer,
        )

        while self._running:
            try:
                await self._consume_once()
            except asyncio.CancelledError:
                raise
            except Exception:
                logger.exception("Error reading from event stream")
                await asyncio.sleep(1.0)

    async def stop(self) -> None:
        self._running = False
        await self._a2a_client.close()
        await self._notifier.close()

    # ---- Redis Streams transport ----

    async def _ensure_group(self) -> None:
        """Create the consumer group (and stream) if they don't exist yet."""
        try:
            await self._redis.xgroup_create(self._stream, self._group, id="0", mkstream=True)
            logger.info("Created consumer group %s on stream %s", self._group, self._stream)
        except ResponseError as exc:
            if "BUSYGROUP" not in str(exc):
                raise

    async def _consume_once(self) -> int:
        """Read one batch of new messages for this consumer. Returns count processed."""
        response = await self._redis.xreadgroup(
            groupname=self._group,
            consumername=self._consumer,
            streams={self._stream: ">"},
            count=10,
            block=1000,
        )
        if not response:
            return 0

        processed = 0
        for _stream_name, entries in response:
            for entry_id, fields in entries:
                await self._process_entry(entry_id, fields)
                processed += 1
        return processed

    async def _claim_stale_messages(self) -> int:
        """XAUTOCLAIM pending entries idle longer than the configured threshold."""
        start_id = "0-0"
        claimed = 0
        while True:
            result = await self._redis.xautoclaim(
                self._stream,
                self._group,
                self._consumer,
                min_idle_time=self._autoclaim_idle_ms,
                start_id=start_id,
                count=100,
            )
            next_id, entries = result[0], result[1]
            for entry_id, fields in entries:
                if not fields:  # entry trimmed/deleted from the stream
                    await self._redis.xack(self._stream, self._group, entry_id)
                    continue
                await self._process_entry(entry_id, fields)
                claimed += 1
            if not entries or _as_str(next_id) == "0-0":
                break
            start_id = next_id
        return claimed

    async def _process_entry(self, entry_id: Any, fields: dict[Any, Any]) -> bool:
        """Process one stream entry with retries; ack on success, DLQ on exhaustion.

        Returns True if the entry was processed successfully, False if it was
        dead-lettered.
        """
        decoded = {_as_str(k): _as_str(v) for k, v in fields.items()}
        payload_json = decoded.get("payload", "")
        event_type = decoded.get("event_type", "")

        # Skip event types this worker doesn't handle (e.g. agent.status echoes).
        if event_type and event_type not in self.HANDLED_EVENT_TYPES:
            await self._redis.xack(self._stream, self._group, entry_id)
            return True

        last_error: Exception | None = None
        for attempt in range(self._max_retries + 1):
            try:
                await self._handle_message({"type": "stream", "data": payload_json})
                await self._redis.xack(self._stream, self._group, entry_id)
                return True
            except Exception as exc:
                last_error = exc
                logger.exception(
                    "Error processing stream entry %s (attempt %d/%d)",
                    _as_str(entry_id), attempt + 1, self._max_retries + 1,
                )
                if attempt < self._max_retries:
                    await asyncio.sleep(self._backoff_base * (2 ** attempt))

        await self._send_to_dlq(entry_id, decoded, last_error)
        # Ack the original so it doesn't stay pending forever; the DLQ copy
        # is the durable record for manual inspection/replay.
        await self._redis.xack(self._stream, self._group, entry_id)
        return False

    async def _send_to_dlq(self, entry_id: Any, fields: dict[str, str], error: Exception | None) -> None:
        """Copy an exhausted message to the dead-letter stream with error metadata."""
        dlq_fields = {
            **fields,
            "error": str(error) if error else "unknown",
            "error_type": type(error).__name__ if error else "unknown",
            "failed_at": datetime.now(timezone.utc).isoformat(),
            "original_entry_id": _as_str(entry_id),
            "consumer": self._consumer,
            "retries": str(self._max_retries),
        }
        try:
            await self._redis.xadd(
                self._dlq_stream, dlq_fields, maxlen=self._dlq_maxlen, approximate=True,
            )
            logger.error(
                "Stream entry %s dead-lettered to %s after %d retries",
                _as_str(entry_id), self._dlq_stream, self._max_retries,
            )
        except Exception:
            logger.exception("Failed to write entry %s to DLQ %s", _as_str(entry_id), self._dlq_stream)

    def _is_stale(self, data: dict[str, Any]) -> bool:
        """True if `data["timestamp"]` (set by the Go backend at publish
        time) is older than stale_event_threshold_seconds. Missing/
        unparseable timestamps are treated as NOT stale -- fail open, since
        skipping is a lossy operation and a malformed timestamp shouldn't
        silently drop a real event."""
        threshold = get_settings().stale_event_threshold_seconds
        if threshold <= 0:
            return False
        raw = data.get("timestamp")
        if not raw:
            return False
        try:
            published_at = datetime.fromisoformat(raw.replace("Z", "+00:00"))
        except (ValueError, AttributeError):
            return False
        age_seconds = (datetime.now(timezone.utc) - published_at).total_seconds()
        return age_seconds > threshold

    async def _handle_message(self, message: dict[str, Any]) -> None:
        data = json.loads(message["data"])
        event_type = data.get("event_type", "alert.received")

        if self._is_stale(data):
            record_stale_event_skipped(event_type)
            logger.warning(
                "Skipping stale %s event (event_id=%s, published %s) -- backlog "
                "older than stale_event_threshold_seconds, not worth burning LLM "
                "capacity on a likely-superseded event",
                event_type, data.get("event_id"), data.get("timestamp"),
            )
            return

        if event_type == "evidence.uploaded":
            await self._handle_evidence_uploaded(data)
            return
        if event_type == "incident.resolved":
            await self._handle_incident_resolved(data)
            return
        if event_type == "runbook.execution.started":
            await self._handle_runbook_execution_started(data)
            return

        org_id = uuid.UUID(data["org_id"])
        payload = AlertReceivedPayload.model_validate(data["payload"])

        alert_dict = payload.normalized_alert.model_dump(mode="json")

        # Check correlation: does this alert belong to an existing incident?
        # exclude_incident_id=payload.incident_id -- IngestAlert (Go) already
        # created and committed this incident before publishing this exact
        # alert.received event, so without excluding it here every brand-new
        # incident self-correlates against itself and the pipeline below
        # never runs. Found live: see CorrelationEngine.check_correlation's
        # docstring for the full story.
        existing_incident = await self._correlation.check_correlation(
            backend_client=self._backend,
            org_id=org_id,
            software_id=str(payload.software_id),
            alert=alert_dict,
            exclude_incident_id=payload.incident_id,
        )

        if existing_incident:
            # Add alert as event to existing incident
            logger.info(
                "Alert correlated with existing incident %s", existing_incident,
            )
            try:
                await self._backend.add_incident_event(
                    existing_incident,
                    IncidentEventCreate(
                        type="correlated_alert",
                        data={"alert": alert_dict, "original_incident_id": str(payload.incident_id)},
                    ),
                    org_id,
                )
            except Exception:
                logger.warning("Failed to add correlated event to incident %s", existing_incident)
            return

        agent_id = uuid.uuid4()

        # Publish agent.status(started)
        await self._publisher.publish_agent_status(
            org_id,
            AgentStatusPayload(
                incident_id=payload.incident_id,
                agent_id=agent_id,
                agent_name="a2a-orchestrator",
                status="started",
            ),
        )

        try:
            # Collect evidence snapshots from observability sources
            collector = SnapshotCollector(self._backend)
            snapshot_evidence = await collector.collect_snapshots(
                incident_id=payload.incident_id,
                software_id=str(payload.software_id),
                alert=alert_dict,
                org_id=org_id,
            )
            if snapshot_evidence:
                logger.info(
                    "Collected %d evidence snapshots for incident %s",
                    len(snapshot_evidence), payload.incident_id,
                )

            results = await self._orchestrator.handle_incident(
                incident_id=payload.incident_id,
                alert=alert_dict,
                software_id=str(payload.software_id),
                org_id=org_id,
            )

            logger.info(
                "Orchestrator completed for incident %s: %d agent results",
                payload.incident_id,
                len(results),
            )

            await self._publisher.publish_agent_status(
                org_id,
                AgentStatusPayload(
                    incident_id=payload.incident_id,
                    agent_id=agent_id,
                    agent_name="a2a-orchestrator",
                    status="completed",
                ),
            )

            # Send notifications
            await self._notifier.notify(
                self._backend, org_id, payload.incident_id,
                "incident_created",
                {"incident_id": str(payload.incident_id), "severity": alert_dict.get("severity", "medium")},
            )

            # If RCA completed, send RCA notification
            rca_result = results.get("rca", {})
            if isinstance(rca_result, dict) and "error" not in rca_result:
                await self._notifier.notify(
                    self._backend, org_id, payload.incident_id,
                    "rca_completed",
                    {
                        "incident_id": str(payload.incident_id),
                        "severity": alert_dict.get("severity", "medium"),
                        "root_cause": rca_result.get("rca", {}).get("root_cause_summary", ""),
                    },
                )

        except Exception as exc:
            logger.exception("Orchestrator failed for incident %s", payload.incident_id)
            await self._publisher.publish_agent_status(
                org_id,
                AgentStatusPayload(
                    incident_id=payload.incident_id,
                    agent_id=agent_id,
                    agent_name="a2a-orchestrator",
                    status="failed",
                    message=str(exc),
                ),
            )

    async def _handle_evidence_uploaded(self, data: dict[str, Any]) -> None:
        """Re-run evidence analysis and RCA when new evidence is uploaded by a user."""
        org_id = uuid.UUID(data["org_id"])
        payload = data.get("payload", {})
        incident_id = uuid.UUID(payload["incident_id"])
        evidence_title = payload.get("title", "unknown")
        evidence_type = payload.get("type", "manual")

        logger.info(
            "New evidence uploaded for incident %s: %s (%s)",
            incident_id, evidence_title, evidence_type,
        )

        # Fetch the incident to get software_id and build context
        try:
            # Get all current evidence for this incident
            incident = await self._backend.get_incident(incident_id, org_id)
            software_id = str(incident.get("software_id", ""))
            # "service" must be the software's real slug/name (e.g.
            # "pulse-backend"), matching the Prometheus `service` label a
            # k8s-agent skill queries by -- not the raw software_id UUID.
            # Found live: a manual re-analysis's synthetic alert used to set
            # service=software_id, so k8s-agent's Prometheus/Loki/Tempo
            # queries for this path matched nothing (zero metrics, zero
            # events, zero logs) and produced a near-empty, low-confidence
            # RCA. The original alert.received path doesn't have this bug --
            # it carries the real alert labels -- this only affected
            # re-analysis triggered by an evidence upload.
            service = software_id
            try:
                software = await self._backend.get_software(software_id, org_id)
                service = software.get("slug") or software.get("name") or software_id
            except Exception:
                logger.warning(
                    "Failed to fetch software %s for re-analysis alert context, "
                    "falling back to software_id as service label", software_id,
                )
            alert_dict = {
                "title": incident.get("title", ""),
                "description": incident.get("description", ""),
                "severity": incident.get("severity", "medium"),
                "service": service,
                "re_analysis_trigger": "evidence_uploaded",
                "new_evidence": {"title": evidence_title, "type": evidence_type},
            }

            # Re-run only evidence-collection and RCA (not triage/postmortem)
            results = await self._orchestrator.handle_incident(
                incident_id=incident_id,
                alert=alert_dict,
                software_id=software_id,
                org_id=org_id,
            )

            logger.info(
                "Re-analysis completed for incident %s after evidence upload: %d results",
                incident_id, len(results),
            )
        except Exception:
            logger.exception("Failed to re-analyze incident %s after evidence upload", incident_id)

    async def _handle_runbook_execution_started(self, data: dict[str, Any]) -> None:
        """Drive a runbook execution's automated steps forward.

        FeaturesHandler.ExecuteRunbook (Go) already published this event on
        every runbook run -- nothing ever consumed it, so an "automated"
        step type was purely cosmetic: every step, automated or not,
        required a human to click "Mark Complete". RunbookExecutor picks up
        from wherever the execution currently is and runs forward until it
        hits a manual/approval step (still needs a human) or finishes.
        """
        org_id = uuid.UUID(data["org_id"])
        payload = data.get("payload", {})
        execution_id = uuid.UUID(payload["execution_id"])

        try:
            execution = await self._backend.get_runbook_execution(execution_id, org_id)
            incident_id_raw = execution.get("incident_id")
            incident_id = uuid.UUID(incident_id_raw) if incident_id_raw else uuid.uuid4()

            final = await self._runbook_executor.run_automated_steps(
                backend_client=self._backend,
                jit_provider=self._orchestrator._jit,
                orchestrator=self._orchestrator,
                org_id=org_id,
                execution_id=execution_id,
                incident_id=incident_id,
            )
            logger.info(
                "Runbook execution %s automation pass finished: status=%s current_step=%s",
                execution_id, final.get("status"), final.get("current_step"),
            )
        except Exception:
            record_swallowed_error("alert_worker", "runbook_automation_failed")
            logger.exception("Runbook automation failed for execution %s", execution_id)

    async def _handle_incident_resolved(self, data: dict[str, Any]) -> None:
        """Run postmortem agent when incident is resolved."""
        org_id = uuid.UUID(data["org_id"])
        payload = data.get("payload", {})
        incident_id = uuid.UUID(payload["incident_id"])

        logger.info("Incident %s resolved — triggering postmortem agent", incident_id)

        try:
            incident = await self._backend.get_incident(incident_id, org_id)
            software_id = str(incident.get("software_id", ""))

            # rca is still fetched here (not just inside
            # _build_postmortem_context below) because it's reused after
            # the postmortem completes, for the knowledge-base extraction
            # step further down.
            rca = None
            try:
                rca = await self._backend.get_rca(incident_id)
            except Exception:
                pass

            from app.models.api import CreateAgentRunRequest
            run_resp = await self._backend.create_agent_run(
                incident_id,
                CreateAgentRunRequest(
                    incident_id=incident_id,
                    agent_name="postmortem",
                    # agent_runs.agent_type has a CHECK constraint allowing
                    # only {triage, evidence_analysis, hypothesis,
                    # rci_generator, rca_generator, postmortem_generator,
                    # debug, custom} (migration 002) -- plain "postmortem"
                    # 500s. Mirrors _AGENT_TYPE_BY_SKILL in orchestrator.py.
                    agent_type="postmortem_generator",
                    input_data={"trigger": "incident_resolved"},
                ),
            )
            run_id = run_resp.get("id")

            import time as _time
            start_mono = _time.monotonic()

            # context=None: let run_postmortem_stage build it via
            # Orchestrator._build_postmortem_context, the single source of
            # truth for this shape. This function used to build its own
            # copy here -- {"incident": ..., "rci": ..., "rca": ...} -- a
            # shape PostmortemAgent.handle_task never reads (it wants
            # "alert", not "incident") and which was sent as a TextPart the
            # agent's own parser can't read at all (see
            # _build_postmortem_context's docstring). Every postmortem
            # generated by resolving an incident ran with empty context,
            # regardless of model.
            #
            # run_postmortem_stage is the single gated entry point: it checks
            # the org's HITL gate (pipeline_hitl_gate_enabled) before
            # dispatching, marks the incident awaiting approval and returns
            # early if gated, otherwise dispatches the postmortem agent and
            # persists RCI/RCA/postmortem results itself.
            stage_result = await self._orchestrator.run_postmortem_stage(
                incident_id, org_id,
            )

            duration_ms = int((_time.monotonic() - start_mono) * 1000)

            if stage_result.get("status") == "paused":
                if run_id:
                    from app.models.api import UpdateAgentRunRequest
                    # agent_runs.status only allows pending/running/completed/
                    # failed/skipped (see migration 002) -- "skipped" is the
                    # closest fit for "gated, not executed"; the actual
                    # paused/awaiting-approval state is preserved in output_data.
                    await self._backend.update_agent_run(
                        incident_id, uuid.UUID(run_id),
                        UpdateAgentRunRequest(
                            status="skipped",
                            output_data=stage_result,
                            duration_ms=duration_ms,
                        ),
                    )
                logger.info(
                    "Postmortem for incident %s paused: awaiting human approval",
                    incident_id,
                )
                return

            result = stage_result.get("postmortem", {})

            if run_id:
                from app.models.api import UpdateAgentRunRequest
                await self._backend.update_agent_run(
                    incident_id, uuid.UUID(run_id),
                    UpdateAgentRunRequest(
                        status="completed",
                        output_data=result,
                        duration_ms=duration_ms,
                        model_used="anthropic/claude-sonnet-4-6",
                    ),
                )

            logger.info("Postmortem completed for incident %s (%dms)", incident_id, duration_ms)

            # Extract lessons to knowledge base (outer loop)
            try:
                from app.loop.outer_loop import OuterLoop
                outer_loop = OuterLoop()
                rca_data = rca.get("rca", {}) if isinstance(rca, dict) else {}
                postmortem_data = result if isinstance(result, dict) else None
                await outer_loop.extract_and_store_knowledge(
                    backend_client=self._backend,
                    incident_id=incident_id,
                    org_id=org_id,
                    software_id=software_id,
                    rca_data=rca_data,
                    postmortem_data=postmortem_data,
                )
            except Exception as e:
                logger.error("Failed to extract lessons for incident %s: %s", incident_id, e)

            # Dispatch resolved notification
            try:
                await self._notifier.notify(
                    self._backend, org_id, incident_id,
                    "incident.resolved",
                    {"incident_id": str(incident_id), "severity": incident.get("severity", "medium")},
                )
            except Exception as e:
                logger.error("Failed to dispatch resolved notification for incident %s: %s", incident_id, e)

        except Exception:
            # This is the exact class of bug found repeatedly this session
            # (missing incident_id, wrong agent_type, LLM ReadTimeout under
            # load) -- it silently meant "closing an incident never
            # produces a postmortem" with zero visibility until someone
            # went looking. record_swallowed_error + the
            # RootCausewayAgentServiceSwallowedError alert rule make that loud.
            record_swallowed_error("alert_worker", "postmortem_generation_failed")
            logger.exception("Failed to generate postmortem for incident %s", incident_id)
