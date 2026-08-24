"""Orchestrator: the brain of the RootCauseway system.

Receives incidents, uses LLM to decide which A2A agents to call,
dispatches tasks, collects results, and updates the incident.

Supports skills-aware orchestration and JIT credential management.
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
import uuid
from datetime import datetime, timezone
from typing import Any, Awaitable

import mlflow
from mlflow.entities import SpanType

from app.a2a.client import A2AClient
from app.a2a.models import (
    AgentCard,
    Artifact,
    DataPart,
    Message,
    Role,
    Task,
    TaskStatus,
)
from app.credentials.jit_provider import JITCredentialProvider
from app.loop.inner_loop import InnerLoop
from app.loop.outer_loop import OuterLoop
from app.models.api import CreateAgentRunRequest, IncidentEventCreate, UpdateAgentRunRequest
from app.observability.metrics import record_swallowed_error
from app.orchestrator.context_builder import ContextBuilder
from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)

# Skill IDs (as used by the orchestrator/A2A cards, e.g. "evidence-collection")
# don't line up with the backend's `agent_runs.agent_type` CHECK constraint
# (backend/migrations/002_incident_cockpit.up.sql), which predates the current
# skill naming. A naive `skill_id.replace("-", "_")` only happens to work for
# "triage" — everything else needs an explicit mapping.
_AGENT_TYPE_BY_SKILL: dict[str, str] = {
    "triage": "triage",
    "evidence-collection": "evidence_analysis",
    "rca": "rca_generator",
    "postmortem": "postmortem_generator",
    # k8s-agent's AgentCard skills (agents/k8s-agent/app/main.py) -- all 4
    # were unmapped until the rootcauseway_swallowed_errors_total metric caught
    # "incident-analysis" 500ing live in production and surfaced that the
    # other 3 have the exact same latent bug, just never picked by the
    # LLM's skill selection yet. k8s-agent's own a2a_agents row uses
    # agent_type="debug", so that's the natural CHECK-constraint-valid
    # value for anything it does.
    "k8s-debug": "debug",
    "k8s-logs": "debug",
    "k8s-diagnostics": "debug",
    "incident-analysis": "debug",
}

SKILLS_SELECTION_PROMPT = """\
You are the RootCauseway Orchestrator. Given an alert and software context, decide which \
specialized skills to invoke and in what order.

## Alert
{alert_json}

## Software Context
{software_context_json}

## Available Skills
{skills_json}

## Instructions
Analyze the alert and context. Decide which skills to invoke, in what order. \
Each skill is linked to one or more agents that can execute it. Each entry under \
"Formatted" below states what that skill "requires" (e.g. "requires: kubernetes_cluster \
access", or "requires: none") -- copy that skill's declared resource type(s) verbatim \
into "required_resource_types" for any skill you select; don't guess or invent one from \
the description. JIT credentials for those resource types are provisioned automatically \
before the skill is called.

Output ONLY valid JSON with this schema:

{{
  "severity_assessment": "critical|high|medium|low",
  "reasoning": "brief explanation of your decision",
  "skill_calls": [
    {{
      "skill_id": "<skill id>",
      "agent_id": "<preferred agent id>",
      "agent_url": "<agent url>",
      "priority": 1,
      "input_summary": "what to tell this agent",
      "required_resource_types": ["kubernetes_cluster"]
    }}
  ]
}}

Always include triage first. For critical/high/medium severity, include all relevant \
skills AND the "rca" skill specifically — it is the ONLY skill whose output becomes \
this incident's actual root cause record (RCI + RCA with 5 Whys); every other \
diagnostic skill (including any skill whose description also mentions "root cause \
analysis", e.g. a Kubernetes-focused one) only adds supporting evidence and does NOT \
replace it. If you are inclined to skip "rca" because another skill already looks \
like it covers root cause analysis, include "rca" anyway — that other skill's output \
becomes an input to it, not a substitute for it. \
For low severity, only triage and evidence. \
NEVER include postmortem — it runs automatically when the incident is resolved.

If the Software Context's "cloud_provider" names a cloud (e.g. "azure", "aws", "gcp"), \
also include that cloud's own diagnostic skill(s) if one is available in Available \
Skills (e.g. an "azure-*" skill for cloud_provider "azure"). A cluster-debug skill \
like "k8s-debug" only has RBAC access to the cluster IT is deployed in — if the \
incident's software runs in a DIFFERENT cluster (common for cloud-hosted software), \
that skill's Kubernetes evidence will come back empty even though it still runs \
without erroring. The matching cloud-native skill queries that cloud's own APIs \
instead and does not have this limitation.

{similar_incidents_section}
"""


class OrchestratorDecision:
    """Records the orchestrator's agent/skill selection decision."""

    def __init__(
        self,
        severity_assessment: str,
        reasoning: str,
        agent_calls: list[dict[str, Any]],
    ):
        self.severity_assessment = severity_assessment
        self.reasoning = reasoning
        self.agent_calls = agent_calls

    def to_dict(self) -> dict[str, Any]:
        return {
            "severity_assessment": self.severity_assessment,
            "reasoning": self.reasoning,
            "agent_calls": self.agent_calls,
        }


class Orchestrator:
    """Hub-and-spoke orchestrator that dispatches to A2A agents."""

    def __init__(
        self,
        backend_client: BackendClient,
        a2a_client: A2AClient,
        llm_call: Any,  # async callable(prompt) -> tuple[str, dict]  -- (content, llm_usage)
        jit_provider: JITCredentialProvider | None = None,
    ):
        self._backend = backend_client
        self._a2a = a2a_client
        self._llm_call = llm_call
        self._context_builder = ContextBuilder()
        self._jit = jit_provider or JITCredentialProvider(backend_client)
        self._inner_loop = InnerLoop()
        self._outer_loop = OuterLoop()
        # Populated at the end of handle_incident() with per-stage/total
        # wall-clock durations (ms). See the parallelization note in
        # handle_incident for what's concurrent vs. sequential.
        self.last_pipeline_timings: dict[str, Any] = {}

    @mlflow.trace(span_type=SpanType.CHAIN, name="orchestrator.handle_incident")
    async def handle_incident(
        self,
        incident_id: uuid.UUID,
        alert: dict[str, Any],
        software_id: str,
        org_id: uuid.UUID,
        extra_concurrent_task: Awaitable[Any] | None = None,
    ) -> dict[str, Any]:
        """Main entry point: orchestrate the full incident investigation pipeline.

        `extra_concurrent_task`: optional awaitable folded into the initial
        asyncio.gather() alongside context building. This is the integration
        point for running evidence/snapshot-style work that doesn't depend on
        software_context or similar_incidents concurrently with them instead
        of sequentially before them -- see the parallelization note below.
        """
        pipeline_start = time.monotonic()
        stage_timings: dict[str, Any] = {}

        # 1 + 1b. Build software context and search for similar past incidents
        # concurrently (asyncio.gather). These two lookups are independent:
        # context building reads the software catalog by software_id;
        # similar-incident search queries the knowledge base by
        # alert/software_id/org_id. Neither depends on the other's output, so
        # running them concurrently saves roughly min(t_context, t_similar)
        # of wall-clock time per run instead of paying t_context + t_similar,
        # with no change to what either stage returns.
        #
        # NOTE on the originally-requested "evidence vs hypothesis" and
        # "evidence vs snapshot-collector" parallelization: this codebase's
        # live pipeline is NOT the fixed 6-stage
        # triage->evidence->hypothesis->rci->rca->postmortem sequence that
        # app/agents/crew_factory.py builds (that CrewAI Process.sequential
        # Crew is never invoked from this orchestrator or from
        # app/workers/alert_worker.py -- it's dead code). The live pipeline
        # instead dispatches an LLM-selected, dynamically-ordered list of
        # skill calls (see `decision.agent_calls` in the loop below), each
        # built from `accumulated_context` that includes every prior call's
        # result. hypothesis_agent.py's own prompt ("Based on the following
        # evidence, generate a root cause hypothesis...") and
        # crew_factory.py's explicit `hypothesis_task.context = [triage_task,
        # evidence_task]` both confirm hypothesis genuinely consumes evidence
        # output -- so fan-out parallelizing those two would change agent
        # output semantics (hypothesis would run on stale/absent evidence),
        # which the task called out as unsafe. Not done.
        #
        # The evidence-collection-agent-call vs. SnapshotCollector
        # parallelization is genuinely safe (SnapshotCollector.collect_snapshots
        # doesn't depend on any agent call's output, only on
        # incident_id/software_id/alert/org_id), but that call site lives in
        # app/workers/alert_worker.py (AlertWorker._handle_alert_received),
        # which is out of this task's file territory and awaited *before*
        # this method is even called. `extra_concurrent_task` above is the
        # ready-made hook: alert_worker.py's owner can pass
        # `collector.collect_snapshots(...)` in as `extra_concurrent_task`
        # instead of awaiting it beforehand, and it will run concurrently
        # with context building here.
        _t0 = time.monotonic()
        gather_tasks: list[Awaitable[Any]] = [
            self._context_builder.build_context(software_id, self._backend, org_id),
            self._outer_loop.find_similar_incidents(
                self._backend, org_id, software_id, alert,
            ),
        ]
        if extra_concurrent_task is not None:
            gather_tasks.append(extra_concurrent_task)

        gather_results = await asyncio.gather(*gather_tasks, return_exceptions=True)

        software_context = gather_results[0]
        if isinstance(software_context, BaseException):
            logger.warning("build_context raised during concurrent gather: %s", software_context)
            software_context = {"software": {"id": software_id, "name": "unknown"}}

        similar_incidents = gather_results[1]
        if isinstance(similar_incidents, BaseException):
            logger.warning("find_similar_incidents raised during concurrent gather: %s", similar_incidents)
            similar_incidents = []

        if extra_concurrent_task is not None and len(gather_results) > 2:
            extra_result = gather_results[2]
            if isinstance(extra_result, BaseException):
                logger.warning("extra_concurrent_task raised during concurrent gather: %s", extra_result)

        stage_timings["context_and_similar_incidents_ms"] = int((time.monotonic() - _t0) * 1000)

        self._similar_context = await self._outer_loop.build_few_shot_context(
            similar_incidents,
        )

        # 2. Discover available skills (with linked agents)
        available_skills = await self._discover_skills(org_id)
        # Fall back to agent-based discovery if no skills endpoint
        if not available_skills:
            available_agents = await self._discover_agents(org_id)
            if not available_agents:
                available_agents = self._default_agents()
            available_skills = self._skills_from_agents(available_agents)

        logger.info(
            "Available skills: %d → %s",
            len(available_skills),
            [s.get("id") for s in available_skills],
        )

        # 3. Use LLM to decide which skills to invoke
        decision = await self._analyze_and_select_skills(
            alert, software_context, available_skills
        )
        logger.info(
            "Decision: %d calls → %s",
            len(decision.agent_calls),
            [(c.get("skill_id") or c.get("agent_id")) for c in decision.agent_calls],
        )

        # 4. Log decision
        try:
            await self._backend.create_orchestrator_decision(
                incident_id, {
                    "decision_type": "agent_selection",
                    "reasoning": decision.reasoning,
                    "selected_agents": [c.get("skill_id", c.get("agent_id")) for c in decision.agent_calls],
                    "context_used": {"severity_assessment": decision.severity_assessment},
                    "confidence": 0.9 if decision.reasoning != "LLM unavailable, using severity-based default pipeline" else 0.5,
                }
            )
        except Exception:
            logger.warning("Failed to persist orchestrator decision")

        # 5. Dispatch tasks sequentially, passing prior results forward
        results: dict[str, Any] = {}
        accumulated_context = {
            "alert": alert,
            "software_context": software_context,
        }
        lease_ids: list[uuid.UUID] = []
        previous_run_id: str | None = None

        from app.config.settings import get_settings
        _settings = get_settings()
        _cheap_model = _settings.llm_model

        # Org-level default LLM provider/model, set via the LLM & Tokens
        # settings UI (backend/migrations/023_llm_settings.up.sql). Falls
        # back to this pod's own env-var defaults (_settings.openai_*) if
        # the org never configured one -- an org's default_llm_base_url is
        # "" until they explicitly save a setting, and every field here is
        # a plain string/reference, never a raw secret in transit beyond
        # what already crosses this internal, unauthenticated-by-JWT trust
        # boundary for platform-hosted agents.
        try:
            _org_settings = await self._backend.get_organization_settings(org_id)
        except Exception:
            _org_settings = {}
        _org_llm_base = _org_settings.get("default_llm_base_url") or _settings.openai_api_base
        _org_llm_model = _org_settings.get("default_llm_model") or _cheap_model
        _org_llm_key_ref = _org_settings.get("default_llm_api_key_ref") or ""

        for call in decision.agent_calls:
            agent_url = call["agent_url"]
            skill_id = call.get("skill_id", "default")
            agent_id = call.get("agent_id", "unknown")
            input_summary = call.get("input_summary", "")
            required_resource_types = call.get("required_resource_types", [])

            # HITL gate: if this call is the postmortem stage and the org has
            # pipeline_hitl_gate_enabled, pause here instead of dispatching.
            # In practice the LLM prompt and _default_skill_decision() both
            # exclude postmortem from decision.agent_calls (it normally runs
            # via the incident.resolved event -> see run_postmortem_only /
            # run_postmortem_stage below), but this check makes the gate
            # correct regardless of how postmortem ends up in the call list.
            if "postmortem" in skill_id.lower():
                gate_result = await self._maybe_gate_postmortem(incident_id, org_id)
                if gate_result is not None:
                    results[skill_id] = gate_result
                    continue

            task_id = str(uuid.uuid4())

            # Timeline: mark this skill's dispatch as starting. agent_run_started
            # is generic and always fires; the semantic type (triage_started,
            # rca_started, ...) only exists for skills the timeline UI has a
            # dedicated icon for -- see _semantic_events_for_skill.
            _sem_started, _sem_completed = self._semantic_events_for_skill(skill_id)
            await self._emit_incident_event(
                incident_id, org_id, "agent_run_started", {"skill_id": skill_id, "agent_id": agent_id},
            )
            if _sem_started:
                await self._emit_incident_event(incident_id, org_id, _sem_started, {"skill_id": skill_id})

            # 5a. Request JIT credentials for required resources
            credentials: dict[str, Any] = {}
            for resource_type in required_resource_types:
                try:
                    agent_uuid = uuid.UUID(agent_id) if self._is_uuid(agent_id) else uuid.uuid4()
                    lease = await self._jit.request_credentials(
                        incident_id=incident_id,
                        agent_id=agent_uuid,
                        skill_id=skill_id,
                        software_id=software_id,
                        resource_type=resource_type,
                        org_id=org_id,
                        ttl_seconds=900,
                        reason=f"Incident {incident_id}: {input_summary}",
                    )
                    if lease:
                        credentials[resource_type] = lease.credential_data
                        lease_ids.append(lease.id)
                except Exception:
                    logger.warning(
                        "Failed to get JIT credentials for %s/%s",
                        skill_id, resource_type,
                    )

            # Determine hosting type and LLM credential routing
            agent_hosting_type = call.get("hosting_type", "managed")
            agent_llm_provider = call.get("llm_provider", "platform")
            agent_managed_config = call.get("managed_config") or {}

            # Build input message with all accumulated context + credentials
            input_data = {
                **accumulated_context,
                "task_instruction": input_summary,
                "skill_id": skill_id,
            }
            if credentials:
                input_data["credentials"] = credentials
            if call.get("skill_prompt_template"):
                input_data["skill_prompt_template"] = call["skill_prompt_template"]

            # For managed agents using the platform LLM, resolve the
            # effective model/base_url from (highest to lowest precedence):
            # 1. this agent's own managed_config.model/temperature override
            #    (set per-agent via the LLM & Tokens settings UI)
            # 2. the org's default_llm_* settings (same UI, org-wide)
            # 3. this pod's own env-var defaults (_settings.openai_*)
            # Every one of these still resolves to the platform's own
            # LM Studio/OpenRouter/etc credentials -- BYOA agents (below)
            # never receive a platform key at all, unchanged from before.
            if agent_hosting_type == "managed" and agent_llm_provider == "platform":
                llm_config: dict[str, Any] = {
                    # NOTE: default_llm_api_key_ref is stored and used here
                    # as the literal API key value, not resolved through the
                    # JIT credential vault (unlike a2a_agents.llm_api_key_ref
                    # for BYOA agents) -- wiring org-level LLM keys through
                    # the vault is a follow-up, not done in this pass. It's
                    # named "_ref" for consistency with that eventual state,
                    # and to avoid ever logging it as though it were public.
                    "api_key": _org_llm_key_ref or _settings.openai_api_key,
                    "api_base": _org_llm_base,
                    "model": agent_managed_config.get("model") or _org_llm_model,
                    "provider": "platform",
                }
                if agent_managed_config.get("temperature") is not None:
                    llm_config["temperature"] = agent_managed_config["temperature"]
                input_data["llm_config"] = llm_config
            # For BYOA agents, don't pass LLM keys — agent uses its own
            # JIT credentials for resource access are always passed regardless of hosting type

            message = Message(
                role=Role.USER,
                parts=[DataPart(data=input_data)],
            )

            # Create agent_run for tracking in the cockpit DAG
            import time as _time
            run_id = None
            start_mono = _time.monotonic()
            try:
                run_resp = await self._backend.create_agent_run(
                    incident_id,
                    CreateAgentRunRequest(
                        incident_id=incident_id,
                        agent_name=skill_id,
                        # "custom" (not skill_id.replace("-", "_")) is the
                        # fallback for anything not in the map -- found
                        # live validating the Skills fix: a user-created
                        # skill's skill_id is a UUID (e.g.
                        # "4ac73e02-5cdb-..."), and replace("-","_") on a
                        # UUID produces something nowhere close to the
                        # agent_runs.agent_type CHECK constraint's allowed
                        # values, 500ing create_agent_run every time. Every
                        # custom skill's dispatch still succeeded (this
                        # only breaks the *tracking* row), but silently --
                        # the actual analysis was untracked in the
                        # incident's DAG/cockpit view.
                        agent_type=_AGENT_TYPE_BY_SKILL.get(skill_id, "custom"),
                        parent_run_id=uuid.UUID(previous_run_id) if previous_run_id else None,
                        input_data={"skill_id": skill_id, "agent_url": agent_url, "model": call.get("model", "")},
                    ),
                )
                run_id = run_resp.get("id")
                logger.info("Created agent_run %s for %s (parent: %s)", run_id, skill_id, previous_run_id)
                previous_run_id = run_id
            except Exception:
                record_swallowed_error("orchestrator", "agent_run_create_failed")
                logger.warning("Failed to create agent_run for %s", skill_id)

            try:
                # Dispatch via A2A
                result_task = await self._a2a.send_task(agent_url, task_id, message)

                # Extract artifacts. Keyed by skill_id (e.g. "rca",
                # "postmortem"), not agent_id: _persist_results and the
                # outer-loop knowledge extraction below both look results up
                # by skill name (results.get("rca")/.get("postmortem")).
                # Before _discover_agents' endpoint_url fix, agent_id
                # happened to equal skill_id because the _default_agents()
                # fallback path was always taken (its "id" field is the
                # literal skill name); real registry discovery uses actual
                # agent UUIDs, which silently broke RCI/RCA/postmortem
                # persistence once discovery started succeeding.
                agent_result = self._extract_result(result_task)
                results[skill_id] = agent_result
                accumulated_context[agent_id] = agent_result
                # Flatten RCA artifacts into top-level context for downstream agents
                if isinstance(agent_result, dict):
                    for key in ("rci", "rca", "hypothesis", "triage_result", "evidence_result"):
                        if key in agent_result:
                            accumulated_context[key] = agent_result[key]

                # Store agent output as evidence on the incident
                try:
                    from app.models.api import IncidentEvidenceCreate
                    evidence_title = f"{skill_id} analysis result"
                    evidence_type = "agent_output"
                    await self._backend.add_incident_evidence(
                        incident_id,
                        IncidentEvidenceCreate(
                            type=evidence_type,
                            title=evidence_title,
                            content=agent_result if isinstance(agent_result, dict) else {"raw": str(agent_result)},
                            source=f"agent:{skill_id}",
                        ),
                        org_id,
                    )
                except Exception:
                    logger.debug("Failed to store evidence for %s", skill_id)

                # Update agent_run as completed
                if run_id:
                    duration_ms = int((_time.monotonic() - start_mono) * 1000)
                    try:
                        # Real usage from the agent's own LLM call (see each
                        # A2A agent's "llm_usage" artifact), not a fabricated
                        # label/heuristic. Falls back to the configured
                        # model + a chars/4 estimate only if the agent
                        # couldn't report real usage (e.g. it errored before
                        # calling the LLM at all).
                        _llm_usage = agent_result.get("llm_usage") if isinstance(agent_result, dict) else None
                        if _llm_usage and _llm_usage.get("total_tokens"):
                            _model = _llm_usage.get("model") or _cheap_model
                            _est_tokens = _llm_usage["total_tokens"]
                        else:
                            _model = _cheap_model
                            _output_str = json.dumps(agent_result) if isinstance(agent_result, dict) else str(agent_result)
                            _est_tokens = len(_output_str) // 4 + 500  # +500 for input prompt, rough fallback only

                        await self._backend.update_agent_run(
                            incident_id,
                            uuid.UUID(run_id),
                            UpdateAgentRunRequest(
                                status="completed",
                                output_data=agent_result,
                                duration_ms=duration_ms,
                                model_used=_model,
                                tokens_used=_est_tokens,
                                completed_at=datetime.now(timezone.utc),
                            ),
                        )
                        logger.info("Updated agent_run %s → completed (%dms, model=%s, ~%d tokens)", run_id, duration_ms, _model, _est_tokens)
                    except Exception:
                        logger.warning("Failed to update agent_run %s", run_id)
                    stage_timings[skill_id] = {"duration_ms": duration_ms, "status": "completed"}

                # Timeline: this skill's dispatch succeeded. rca_completed/
                # postmortem_completed/hypothesis_generated are NOT emitted
                # here -- those fire from _persist_results once the
                # artifacts are actually confirmed persisted, since a
                # successful A2A call doesn't guarantee the backend write
                # that follows it also succeeds.
                await self._emit_incident_event(
                    incident_id, org_id, "agent_run_completed", {"skill_id": skill_id},
                )
                for _ct in _sem_completed:
                    await self._emit_incident_event(incident_id, org_id, _ct, {"skill_id": skill_id})

            except Exception:
                record_swallowed_error("orchestrator", "agent_dispatch_failed")
                logger.exception(
                    "Failed to dispatch task to %s (%s)",
                    agent_id,
                    agent_url,
                )
                results[skill_id] = {"error": "agent_unavailable"}
                _fail_duration_ms = int((_time.monotonic() - start_mono) * 1000)
                stage_timings[skill_id] = {"duration_ms": _fail_duration_ms, "status": "failed"}
                await self._emit_incident_event(
                    incident_id, org_id, "agent_run_failed", {"skill_id": skill_id, "error": "agent_unavailable"},
                )
                # Mark agent_run as failed
                if run_id:
                    try:
                        await self._backend.update_agent_run(
                            incident_id,
                            uuid.UUID(run_id),
                            UpdateAgentRunRequest(
                                status="failed",
                                error_message="agent_unavailable",
                                duration_ms=_fail_duration_ms,
                                completed_at=datetime.now(timezone.utc),
                            ),
                        )
                    except Exception:
                        pass

            finally:
                # 5b. Revoke JIT credentials for this task
                for lid in lease_ids:
                    try:
                        await self._jit.revoke_credentials(lid)
                    except Exception:
                        logger.warning("Failed to revoke lease %s", lid)
                lease_ids.clear()

        # 6. Inner loop: refine if RCA confidence is low
        results = await self._inner_loop.evaluate_and_refine(
            self, incident_id, results,
        )
        self._record_inner_loop_stats(incident_id)

        # 7. Persist final results
        await self._persist_results(incident_id, results, org_id)
        await self._persist_mlflow_trace_link(incident_id, org_id)

        # 8. Outer loop: extract and store knowledge for future incidents
        rca_data = results.get("rca", {})
        if isinstance(rca_data, dict):
            rca_inner = rca_data.get("rca", rca_data)
        else:
            rca_inner = {}
        postmortem_data = results.get("postmortem", {})
        if isinstance(postmortem_data, dict) and "error" not in postmortem_data:
            pm_inner = postmortem_data.get("postmortem", postmortem_data)
        else:
            pm_inner = None

        if rca_inner and isinstance(rca_inner, dict):
            await self._outer_loop.extract_and_store_knowledge(
                self._backend, incident_id, org_id, software_id,
                rca_inner, pm_inner,
            )

        # Record total pipeline wall-clock time alongside the per-stage
        # timings collected above. Exposed as an instance attribute (not
        # folded into `results`) so callers that iterate results.items()
        # expecting agent-id -> agent-result pairs (e.g. alert_worker.py)
        # aren't affected; tests/introspection can read
        # orchestrator.last_pipeline_timings after a run.
        total_duration_ms = int((time.monotonic() - pipeline_start) * 1000)
        stage_timings["total_pipeline_ms"] = total_duration_ms
        self.last_pipeline_timings = stage_timings
        logger.info(
            "Pipeline for incident %s completed in %dms (stages: %s)",
            incident_id, total_duration_ms, stage_timings,
        )

        return results

    async def _discover_skills(self, org_id: uuid.UUID) -> list[dict[str, Any]]:
        """Discover available skills with linked agents from backend.

        A skill with no agent linked (`agents` empty/missing) can't
        actually be dispatched -- `_analyze_and_select_skills`'s agent_url
        backfill has nothing to resolve it to, so the LLM either
        hallucinates a URL or leaves it blank, and dispatch fails. Worse:
        before this filter existed, returning *any* non-empty skill list
        here -- even one agent-less skill, e.g. right after a user creates
        a custom skill via the UI but before linking it to an agent --
        permanently bypassed the working agent-card-discovery fallback
        below for the rest of the org's incidents, since that fallback
        only runs when this method returns empty. Dropping agent-less
        skills here means the fallback still kicks in until every
        discovered skill actually has somewhere to go.
        """
        try:
            skills = await self._backend.list_skills(org_id)
        except Exception:
            logger.warning("Failed to list skills, falling back to agent discovery")
            return []
        # enabled defaults True for callers/tests that omit the field (the
        # Go List query doesn't filter by it -- disabling a skill must not
        # remove it from GET /skills, since the admin UI needs to keep
        # showing/re-enabling it) -- only an explicit `enabled: false`
        # excludes a skill here. Found live: the enable/disable toggle
        # already persisted correctly, but had zero effect on dispatch --
        # a "disabled" skill kept being selected by the LLM and dispatched
        # exactly like an enabled one.
        usable = [s for s in skills if s.get("agents") and s.get("enabled", True)]
        dropped = len(skills) - len(usable)
        if dropped:
            logger.warning(
                "Dropped %d skill(s) with no linked agent or disabled (unusable for dispatch): %s",
                dropped, [s.get("id") for s in skills if not (s.get("agents") and s.get("enabled", True))],
            )
        return usable

    async def _discover_agents(self, org_id: uuid.UUID) -> list[dict[str, Any]]:
        """Discover available A2A agents from backend or config."""
        try:
            agents_data = await self._backend.list_a2a_agents(org_id)
            agents = []
            for agent_data in agents_data:
                try:
                    # The a2a_agents table/API calls this field "endpoint_url"
                    # (see backend/internal/models/models.go); it was never
                    # "url", so this always resolved to "" and every agent
                    # failed discovery (UnsupportedProtocol from a relative
                    # "/.well-known/agent.json" request), silently forcing
                    # the _default_agents()/.env fallback path on every run.
                    url = agent_data.get("endpoint_url", "")
                    card = await self._a2a.discover(url)
                    agents.append({
                        "id": agent_data.get("id", card.name),
                        "url": url,
                        "card": card.model_dump(by_alias=True),
                        "hosting_type": agent_data.get("hosting_type", "managed"),
                        "llm_provider": agent_data.get("llm_provider", "platform"),
                        # Per-agent LLM override (model/temperature), set via
                        # the LLM & Tokens settings UI's per-agent form. Reuses
                        # this existing free-form JSONB column rather than
                        # adding dedicated a2a_agents columns -- see migration
                        # 023_llm_settings.up.sql.
                        "managed_config": agent_data.get("managed_config") or {},
                    })
                except Exception:
                    logger.warning("Failed to discover agent at %s", agent_data.get("endpoint_url"))
            return agents
        except Exception:
            logger.warning("Failed to list A2A agents, using defaults")
            return self._default_agents()

    def _default_agents(self) -> list[dict[str, Any]]:
        """Fallback default agent list when discovery fails."""
        from app.config.settings import get_settings

        settings = get_settings()
        agent_urls = {
            "triage": settings.a2a_triage_agent_url,
            "evidence": settings.a2a_evidence_agent_url,
            "rca": settings.a2a_rca_agent_url,
            "postmortem": settings.a2a_postmortem_agent_url,
        }
        defaults = []
        for name, skill_id, desc in [
            ("triage", "triage", "Alert triage and severity assessment"),
            ("evidence", "evidence-collection", "Evidence collection and analysis"),
            ("rca", "rca", "Root cause investigation and analysis"),
            ("postmortem", "postmortem", "Blameless postmortem generation"),
        ]:
            url = agent_urls[name]
            defaults.append({
                "id": name,
                "url": url,
                "card": {
                    "name": f"RootCauseway {name.title()} Agent",
                    "url": url,
                    "version": "0.1.0",
                    "skills": [{"id": skill_id, "name": desc, "description": desc}],
                },
            })
        return defaults

    @staticmethod
    def _skills_from_agents(agents: list[dict[str, Any]]) -> list[dict[str, Any]]:
        """Convert agent-based discovery to skills-based format."""
        skills = []
        for agent in agents:
            card = agent.get("card", {})
            for skill in card.get("skills", []):
                skills.append({
                    "id": skill.get("id", "default"),
                    "name": skill.get("name", ""),
                    "description": skill.get("description", ""),
                    "required_resource_types": skill.get("required_resource_types", []),
                    "agents": [{
                        "id": agent["id"],
                        "url": agent["url"],
                        "name": card.get("name", ""),
                        "hosting_type": agent.get("hosting_type", "managed"),
                        "llm_provider": agent.get("llm_provider", "platform"),
                        "managed_config": agent.get("managed_config") or {},
                    }],
                })
        return skills

    @mlflow.trace(span_type=SpanType.CHAIN, name="orchestrator.select_skills")
    async def _analyze_and_select_skills(
        self,
        alert: dict[str, Any],
        software_context: dict[str, Any],
        available_skills: list[dict[str, Any]],
    ) -> OrchestratorDecision:
        """Use LLM to determine which skills to invoke and in what order."""

        # Build skills display for the prompt
        skills_display = []
        for s in available_skills:
            agent_names = ", ".join(
                a.get("name", a.get("id", "unknown"))
                for a in s.get("agents", [])
            )
            resource_types = s.get("required_resource_types", [])
            requires = f" - requires: {', '.join(resource_types)} access" if resource_types else " - requires: none"
            skills_display.append(
                f"- {s['id']} (agents: {agent_names}){requires}: {s.get('description', '')}"
            )

        similar_section = getattr(self, "_similar_context", "") or ""
        prompt = SKILLS_SELECTION_PROMPT.format(
            alert_json=json.dumps(alert, indent=2, default=str),
            software_context_json=json.dumps(software_context, indent=2, default=str),
            skills_json=json.dumps(available_skills, indent=2, default=str)
            + "\n\nFormatted:\n" + "\n".join(skills_display),
            similar_incidents_section=similar_section,
        )

        try:
            llm_output, skill_selection_usage = await self._llm_call(prompt)
            parsed = self._parse_json(llm_output)
            if skill_selection_usage:
                logger.info(
                    "Skill-selection LLM call: model=%s tokens=%s",
                    skill_selection_usage.get("model"),
                    skill_selection_usage.get("total_tokens"),
                )
        except Exception:
            logger.warning("LLM skill selection failed, using default pipeline")
            parsed = self._default_skill_decision(alert, available_skills)

        # Normalize: support both "skill_calls" and "agent_calls" keys
        calls = parsed.get("skill_calls", parsed.get("agent_calls", []))

        # Fallback: if LLM returned 0 calls, use default pipeline
        if not calls:
            logger.warning("LLM returned 0 agent calls, falling back to default pipeline")
            parsed = self._default_skill_decision(alert, available_skills)
            calls = parsed.get("skill_calls", parsed.get("agent_calls", []))

        # Safety net for a live-found gap: "rca" is the only skill whose
        # output gets promoted into the incident's structured RCI/RCA
        # fields (see _persist_results / the skill_id == "rca" check
        # elsewhere) -- every other skill's output, however good, only
        # ever lands as raw evidence. The prompt above now says this
        # explicitly, but this platform runs on a small/fast local model
        # chosen for latency, and it doesn't reliably follow that
        # instruction -- confirmed live: for medium+ severity it
        # repeatedly picked k8s-agent's "incident-analysis" (whose own
        # description also says "structured root-cause analysis") and
        # skipped "rca" entirely, so real incidents kept ending up with no
        # RCI/RCA at all. Force it in rather than only asking nicely.
        severity = parsed.get("severity_assessment", "medium")
        has_rca_call = any(c.get("skill_id") == "rca" for c in calls)
        rca_skill = next((s for s in available_skills if s["id"] == "rca"), None)
        if severity != "low" and not has_rca_call and rca_skill:
            logger.info("Forcing 'rca' skill call: LLM omitted it for severity=%s", severity)
            calls.append({
                "skill_id": "rca",
                "priority": len(calls) + 1,
                "input_summary": "Root cause analysis (added: not selected by skill-selection LLM)",
                "required_resource_types": rca_skill.get("required_resource_types", []),
            })

        # Ensure each call has agent_url resolved from skills data, and
        # always attach hosting_type/llm_provider/managed_config by looking
        # up the matching skill's agent -- unconditionally, not just when
        # agent_url was missing. This used to live inside the "agent_url
        # missing" branch, so any call the LLM (or _default_skill_decision)
        # already gave a concrete agent_url to silently never got its
        # managed_config (per-agent LLM override) attached at all.
        for call in calls:
            skill_id = call.get("skill_id", "")
            matching_skill = next(
                (s for s in available_skills if s["id"] == skill_id), None
            )
            if matching_skill and matching_skill.get("agents"):
                agent = matching_skill["agents"][0]
                call.setdefault("agent_url", agent.get("url", ""))
                call.setdefault("agent_id", agent.get("id", ""))
                call.setdefault("hosting_type", agent.get("hosting_type", "managed"))
                call.setdefault("llm_provider", agent.get("llm_provider", "platform"))
                call.setdefault("managed_config", agent.get("managed_config") or {})
            # skill.prompt_template lives on the skill itself, not its
            # agent -- previously captured on create/edit but never read
            # anywhere downstream (found live: a user filled it in, it had
            # zero effect on the actual LLM call). Threaded into
            # input_data below and consumed by each agent's own
            # _call_llm/prompt-building as an *additional* instructions
            # section, not a full prompt replacement -- replacing the
            # whole prompt would risk breaking the strict JSON output
            # schema every agent's response parsing depends on.
            if matching_skill and matching_skill.get("prompt_template"):
                call.setdefault("skill_prompt_template", matching_skill["prompt_template"])

        return OrchestratorDecision(
            severity_assessment=parsed.get("severity_assessment", "medium"),
            reasoning=parsed.get("reasoning", "default pipeline"),
            agent_calls=calls,
        )

    def _default_skill_decision(
        self, alert: dict[str, Any], skills: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Fallback decision when LLM fails."""
        severity = alert.get("severity", "medium")
        logger.info("Default skill decision: severity=%s, skills=%d", severity, len(skills))
        for s in skills:
            logger.info("  skill: %s, agents: %s", s.get("id"), [a.get("url") for a in s.get("agents", [])])
        calls = []
        priority = 1

        # Map skill IDs to pipeline order
        pipeline_order = ["triage", "evidence-collection", "evidence", "rca"]
        ordered_skills = sorted(
            skills,
            key=lambda s: next(
                (i for i, p in enumerate(pipeline_order) if p in s.get("id", "")),
                len(pipeline_order),
            ),
        )

        for skill in ordered_skills:
            skill_id = skill.get("id", "default")
            if severity in ("low",) and skill_id not in ("triage", "evidence-collection", "evidence"):
                continue
            if "postmortem" in skill_id:
                continue

            agents = skill.get("agents", [])
            agent = agents[0] if agents else {}

            calls.append({
                "skill_id": skill_id,
                "agent_id": agent.get("id", "unknown"),
                "agent_url": agent.get("url", ""),
                "priority": priority,
                "input_summary": f"Process alert using skill {skill_id}",
                "required_resource_types": skill.get("required_resource_types", []),
            })
            priority += 1

        return {
            "severity_assessment": severity,
            "reasoning": "LLM unavailable, using severity-based default pipeline",
            "skill_calls": calls,
        }

    def _extract_result(self, task: Task) -> dict[str, Any]:
        """Extract structured data from task artifacts."""
        result: dict[str, Any] = {}
        for artifact in task.artifacts:
            for part in artifact.parts:
                if hasattr(part, "data"):
                    result[artifact.name] = part.data
                elif hasattr(part, "text"):
                    result[artifact.name] = part.text
        if not result and task.artifacts:
            result = {"raw": [a.model_dump() for a in task.artifacts]}
        return result

    def _record_inner_loop_stats(self, incident_id: uuid.UUID) -> None:
        """Attach this run's inner-loop stats (did it need refinement, how
        many iterations, confidence before/after) as attributes on the
        active MLflow span, so they become queryable training labels for
        a future "will this incident need refinement" meta-model instead
        of only ever existing as a log line. Same get_current_active_span()
        pattern as _persist_mlflow_trace_link -- see that docstring for why
        it's the correct scoping under concurrent incidents."""
        stats = self._inner_loop.last_run_stats
        if not stats:
            return
        span = mlflow.get_current_active_span()
        if span is None:
            return
        try:
            span.set_attributes({f"inner_loop.{k}": v for k, v in stats.items()})
        except Exception:
            record_swallowed_error("orchestrator", "inner_loop_stats_span_attr_failed")
            logger.warning("Failed to record inner-loop stats on MLflow span for incident %s", incident_id)

    async def _persist_mlflow_trace_link(self, incident_id: uuid.UUID, org_id: uuid.UUID) -> None:
        """Save this pipeline run's MLflow trace as evidence on the
        incident, so the frontend can link straight to it instead of the
        two systems staying totally unaware of each other (see the
        platform audit backlog). get_current_active_span() reads from
        mlflow's context-var-based tracking, correctly scoped to this
        async call even under concurrent incidents on the same event loop
        -- unlike get_last_active_trace_id(thread_local=True), which would
        be wrong here since asyncio tasks share one OS thread."""
        span = mlflow.get_current_active_span()
        if span is None:
            return
        try:
            from app.config.settings import get_settings
            from app.models.api import IncidentEvidenceCreate

            settings = get_settings()
            experiment = mlflow.get_experiment_by_name(settings.mlflow_experiment_name)
            experiment_id = experiment.experiment_id if experiment else "0"
            # Deliberately links to the experiment's traces tab, not a
            # selectedTraceId=... deep link to this exact trace: the
            # client-side trace_id from get_current_active_span() doesn't
            # match the ID the server ends up storing it under (verified
            # live -- looks like server-side re-assignment on ingestion,
            # not just an async-flush timing issue), so a deep link would
            # silently 404 rather than fail loud. The traces tab, sorted by
            # recency, still gets a human to the right trace in one click.
            await self._backend.add_incident_evidence(
                incident_id,
                IncidentEvidenceCreate(
                    type="trace",
                    title="MLflow pipeline trace",
                    content={
                        "trace_id": span.trace_id,
                        "url": f"{settings.mlflow_public_url}/#/experiments/{experiment_id}/traces",
                    },
                    source="mlflow",
                ),
                org_id,
            )
        except Exception:
            record_swallowed_error("orchestrator", "mlflow_trace_link_persist_failed")
            logger.exception("Failed to persist MLflow trace link for incident %s", incident_id)

    async def _emit_incident_event(
        self,
        incident_id: uuid.UUID,
        org_id: uuid.UUID,
        event_type: str,
        data: dict[str, Any] | None = None,
    ) -> None:
        """Best-effort incident_events timeline write. Never raises -- a
        timeline entry is a nice-to-have observability signal, not something
        that should ever abort or degrade the actual investigation. Mirrors
        the existing best-effort pattern used for `correlated_alert` and
        `war_room_created` elsewhere in this codebase.
        """
        try:
            await self._backend.add_incident_event(
                incident_id, IncidentEventCreate(type=event_type, data=data), org_id,
            )
        except Exception:
            record_swallowed_error("orchestrator", "incident_event_write_failed")
            logger.warning("Failed to write incident event %r for %s", event_type, incident_id)

    @staticmethod
    def _semantic_events_for_skill(skill_id: str) -> tuple[str | None, list[str]]:
        """Map a skill_id to the semantic incident_events types (beyond the
        generic agent_run_started/completed always emitted) it should also
        produce -- e.g. "triage" also gets a triage_started/triage_completed
        pair the timeline UI has a dedicated icon/color for. Skills with no
        dedicated semantic type (k8s-debug, azure-*, incident-analysis, a
        user-created custom skill, ...) get (None, []) and rely on the
        generic agent_run_* events alone.
        """
        s = skill_id.lower()
        if s == "triage":
            return "triage_started", ["triage_completed"]
        if "evidence" in s:
            return None, ["evidence_collected"]
        if s == "rca":
            return "rca_started", []  # rca_completed/hypothesis_generated/rci_completed
            # are emitted by _persist_results once the artifacts are
            # actually confirmed persisted, not just because the A2A call
            # returned -- see _persist_results below.
        if "postmortem" in s:
            return "postmortem_started", []  # postmortem_completed likewise
            # emitted by _persist_results on confirmed persist.
        return None, []

    async def _persist_results(
        self, incident_id: uuid.UUID, results: dict[str, Any], org_id: uuid.UUID
    ) -> None:
        """Store RCI, RCA, and postmortem results via the backend."""
        logger.info("Persisting results for incident %s, keys: %s", incident_id, list(results.keys()))
        for k, v in results.items():
            sub_keys = list(v.keys()) if isinstance(v, dict) else "not-dict"
            logger.info("  result[%s] sub-keys: %s", k, sub_keys)

        # RCA agent returns artifacts named "rci", "rca", "hypothesis"
        rca_result = results.get("rca", {})
        if isinstance(rca_result, dict) and "error" not in rca_result:
            if rca_result.get("hypothesis"):
                await self._emit_incident_event(incident_id, org_id, "hypothesis_generated")

            rci_data = rca_result.get("rci", {})
            if rci_data and isinstance(rci_data, dict):
                rci_data = self._sanitize_rci(rci_data)
                try:
                    await self._backend.create_rci(incident_id, rci_data)
                    logger.info("Persisted RCI for incident %s", incident_id)
                    await self._emit_incident_event(incident_id, org_id, "rci_completed")
                except Exception:
                    record_swallowed_error("orchestrator", "rci_persist_failed")
                    logger.exception("Failed to persist RCI for incident %s", incident_id)

            rca_data = rca_result.get("rca", {})
            if rca_data and isinstance(rca_data, dict):
                rca_data = self._sanitize_rca(rca_data)
                try:
                    await self._backend.create_rca(incident_id, rca_data)
                    logger.info("Persisted RCA for incident %s", incident_id)
                    await self._emit_incident_event(incident_id, org_id, "rca_completed")
                except Exception:
                    record_swallowed_error("orchestrator", "rca_persist_failed")
                    logger.exception("Failed to persist RCA for incident %s", incident_id)

        pm_result = results.get("postmortem", {})
        if isinstance(pm_result, dict) and "error" not in pm_result:
            pm_data = pm_result.get("postmortem", pm_result.get("postmortem_result", pm_result))
            if pm_data and isinstance(pm_data, dict):
                pm_data = self._sanitize_postmortem(pm_data)
                try:
                    await self._backend.create_postmortem(incident_id, pm_data)
                    logger.info("Persisted postmortem for incident %s", incident_id)
                    await self._emit_incident_event(incident_id, org_id, "postmortem_completed")
                except Exception:
                    record_swallowed_error("orchestrator", "postmortem_persist_failed")
                    logger.exception("Failed to persist postmortem for incident %s", incident_id)

    # -- field sanitizers --------------------------------------------------

    @staticmethod
    def _sanitize_rci(data: dict[str, Any]) -> dict[str, Any]:
        _ALLOWED = {
            "investigation_summary", "impact_assessment", "affected_services",
            "affected_users_estimate", "detection_method", "detection_time",
            "evidence_ids",
        }
        out = {k: v for k, v in data.items() if k in _ALLOWED}
        # coerce affected_users_estimate to int
        raw = out.get("affected_users_estimate")
        if raw is not None and not isinstance(raw, int):
            try:
                out["affected_users_estimate"] = int(
                    "".join(c for c in str(raw) if c.isdigit()) or "0"
                )
            except (ValueError, TypeError):
                out["affected_users_estimate"] = 0
        # detection_time is Go *time.Time (RFC3339) -- the LLM sometimes
        # returns null, a non-ISO string, or omits the field entirely. Drop
        # it in all of those cases rather than send an explicit null or
        # something Go's JSON binding would 400 on; the field is already
        # a nullable pointer on the Go side, so simply omitting it (instead
        # of forwarding a JSON null) is the correct "unknown" encoding.
        raw_dt = out.get("detection_time")
        if raw_dt is None:
            out.pop("detection_time", None)
        else:
            try:
                datetime.fromisoformat(str(raw_dt).replace("Z", "+00:00"))
            except (ValueError, TypeError):
                out.pop("detection_time", None)
        return out

    @staticmethod
    def _sanitize_rca(data: dict[str, Any]) -> dict[str, Any]:
        _ALLOWED = {
            "root_cause_summary", "root_cause_category", "contributing_factors",
            "five_whys", "confidence", "evidence_ids",
        }
        out = {k: v for k, v in data.items() if k in _ALLOWED}
        # coerce confidence to float 0-1
        raw = out.get("confidence")
        if raw is not None:
            try:
                val = float(raw)
                out["confidence"] = max(0.0, min(1.0, val))
            except (ValueError, TypeError):
                out["confidence"] = 0.0
        return out

    @staticmethod
    def _sanitize_postmortem(data: dict[str, Any]) -> dict[str, Any]:
        _ALLOWED = {
            "title", "executive_summary", "incident_timeline_narrative",
            "root_cause_detail", "impact_detail", "lessons_learned",
            "action_items", "what_went_well", "what_went_wrong",
            "prevention_measures",
        }
        return {k: v for k, v in data.items() if k in _ALLOWED}

    async def refine_rca(
        self, incident_id: uuid.UUID, refinement_input: dict[str, Any]
    ) -> dict[str, Any] | None:
        """Re-run RCA with additional context for refinement."""
        try:
            prompt = (
                "You are refining a root cause analysis. The previous analysis had "
                "low confidence. Please review the previous results and provide a "
                "more thorough analysis.\n\n"
                f"Previous results: {json.dumps(refinement_input, indent=2, default=str)}"
            )
            llm_output, _refine_usage = await self._llm_call(prompt)
            return self._parse_json(llm_output)
        except Exception:
            logger.warning("RCA refinement failed for incident %s", incident_id)
            return None

    async def dispatch_single_skill(
        self,
        incident_id: uuid.UUID,
        skill_id: str,
        agent_url: str,
        input_data: dict[str, Any],
        credentials: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Dispatch a single skill execution (used by runbook executor)."""
        task_id = str(uuid.uuid4())
        message_data = {"task_instruction": input_data, "skill_id": skill_id}
        if credentials:
            message_data["credentials"] = credentials

        message = Message(
            role=Role.USER,
            parts=[DataPart(data=message_data)],
        )
        result_task = await self._a2a.send_task(agent_url, task_id, message)
        return self._extract_result(result_task)

    # -- HITL (human-in-the-loop) postmortem gate -------------------------
    #
    # Today, postmortem is actually triggered by AlertWorker._handle_incident_resolved
    # (app/workers/alert_worker.py), which dispatches directly to the postmortem
    # A2A agent rather than going through handle_incident()'s dynamic skill
    # loop (the LLM prompt explicitly tells it never to select postmortem;
    # _default_skill_decision() excludes it too). alert_worker.py is out of
    # this task's file territory, so the three methods below are the reusable
    # building blocks for the gate + postmortem dispatch: `run_postmortem_stage`
    # is a drop-in replacement for alert_worker's inline A2A-dispatch-then-persist
    # logic (its owner can call it instead of duplicating gate-check logic),
    # and `run_postmortem_only` is the narrow resume entry point used by
    # resume_listener.py once a human has already approved.

    async def _maybe_gate_postmortem(
        self, incident_id: uuid.UUID, org_id: uuid.UUID,
    ) -> dict[str, Any] | None:
        """Check the org's HITL gate before running the postmortem stage.

        Returns a "paused" result dict (after marking the incident as
        awaiting approval) if the org has pipeline_hitl_gate_enabled set, or
        None if the postmortem stage should proceed normally (gate disabled,
        or the settings lookup itself failed -- fail open so a backend
        hiccup doesn't silently strand incidents without a postmortem).
        """
        try:
            settings = await self._backend.get_organization_settings(org_id)
            # Defensive: only trust a well-formed dict response. Anything
            # else (malformed backend response, unexpected client stub in
            # tests, etc.) fails open rather than risking a truthy-by-default
            # object silently gating every postmortem.
            gate_enabled = (
                bool(settings.get("pipeline_hitl_gate_enabled", False))
                if isinstance(settings, dict)
                else False
            )
        except Exception:
            logger.warning(
                "Failed to fetch org settings for incident %s HITL gate check; "
                "proceeding without gating", incident_id,
            )
            return None

        if not gate_enabled:
            return None

        try:
            await self._backend.mark_awaiting_approval(incident_id, "postmortem")
        except Exception:
            logger.exception(
                "Failed to mark incident %s as awaiting approval", incident_id,
            )

        logger.info(
            "Pipeline for incident %s paused before postmortem: awaiting human approval",
            incident_id,
        )
        return {
            "status": "paused",
            "awaiting_approval_stage": "postmortem",
            "message": "Pipeline paused, awaiting human approval before postmortem",
        }

    async def _build_postmortem_context(self, incident_id: uuid.UUID, org_id: uuid.UUID) -> dict[str, Any]:
        """Gather alert/RCI/RCA/software context for the postmortem agent,
        shaped to match what PostmortemAgent.handle_task actually reads
        (alert/rci/rca/software_context/evidence).

        Found live: this used to return {"incident": ..., "rci": ...,
        "rca": ...} -- a shape PostmortemAgent.handle_task never reads
        ("incident" isn't one of the keys it looks for; it wants "alert").
        Worse, `_dispatch_postmortem` below sent it as a TextPart, and
        handle_task's `_extract_data` only reads `DataPart.data` -- so
        every postmortem generated via this path (the incident.resolved
        trigger, and any HITL-gate resume) ran with a fully empty
        `input_data` regardless of what context was built here. Every
        field in the prompt (alert/triage/evidence/rci/rca/software) was
        empty `{}`, on every model, the whole time this code path existed.
        """
        incident = await self._backend.get_incident(incident_id, org_id)
        alert = {
            "title": incident.get("title", ""),
            "description": incident.get("description", ""),
            "severity": incident.get("severity", "medium"),
        }

        software_context: dict[str, Any] = {}
        software_id = incident.get("software_id")
        if software_id:
            try:
                software_context = await self._backend.get_software(software_id, org_id)
            except Exception:
                logger.debug("Failed to fetch software context for incident %s postmortem", incident_id)

        rca = None
        try:
            rca = await self._backend.get_rca(incident_id)
        except Exception:
            logger.debug("No RCA available for incident %s postmortem context", incident_id)

        rci = None
        try:
            rci = await self._backend.get_rci(incident_id)
        except Exception:
            logger.debug("No RCI available for incident %s postmortem context", incident_id)

        return {
            "alert": alert,
            "rci": rci,
            "rca": rca,
            "software_context": software_context,
            "evidence": incident.get("evidence") or [],
        }

    async def _dispatch_postmortem(
        self, incident_id: uuid.UUID, context: dict[str, Any], org_id: uuid.UUID,
    ) -> dict[str, Any]:
        """Low-level: dispatch the postmortem A2A agent and persist its
        result. No gate check here -- callers (`run_postmortem_stage`,
        `run_postmortem_only`) are responsible for gating.

        Sends `context` as a DataPart, matching every other agent dispatch
        in this file (the main handle_incident loop, dispatch_single_skill)
        -- see _build_postmortem_context's docstring for why this used to
        be a TextPart, which PostmortemAgent.handle_task can't read at all.
        """
        from app.config.settings import get_settings

        settings = get_settings()
        postmortem_url = settings.a2a_postmortem_agent_url

        await self._emit_incident_event(incident_id, org_id, "postmortem_started")

        message = Message(
            role=Role.USER,
            parts=[DataPart(data=context)],
        )
        task_id = str(uuid.uuid4())
        result_task = await self._a2a.send_task(postmortem_url, task_id, message)
        result = self._extract_result(result_task)

        # postmortem_completed is emitted by _persist_results itself, once
        # the postmortem is actually confirmed persisted -- not here, where
        # we only know the A2A call returned.
        await self._persist_results(incident_id, {"postmortem": result}, org_id)
        return result

    # This is the entry point the incident.resolved/closed trigger uses
    # (alert_worker.py -> AlertWorker._handle_incident_resolved), a
    # separate call path from handle_incident's own trace above -- without
    # this decorator a postmortem generated by closing an incident would be
    # invisible in MLflow even though the initial triage/evidence/rca
    # pipeline traced fine.
    @mlflow.trace(span_type=SpanType.CHAIN, name="orchestrator.run_postmortem_stage")
    async def run_postmortem_stage(
        self,
        incident_id: uuid.UUID,
        org_id: uuid.UUID,
        context: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Run the postmortem stage, respecting the HITL gate.

        Intended as the single entry point for triggering postmortem
        (whether from the incident.resolved event or elsewhere), so the gate
        check always happens. If the org's gate is enabled, marks the
        incident awaiting approval and returns immediately without calling
        the postmortem agent; otherwise dispatches and persists postmortem
        results synchronously.
        """
        gated = await self._maybe_gate_postmortem(incident_id, org_id)
        if gated is not None:
            return gated

        if context is None:
            context = await self._build_postmortem_context(incident_id, org_id)
        result = await self._dispatch_postmortem(incident_id, context, org_id)
        return {"postmortem": result, "status": "completed"}

    @mlflow.trace(span_type=SpanType.CHAIN, name="orchestrator.run_postmortem_only")
    async def run_postmortem_only(
        self,
        incident_id: uuid.UUID,
        org_id: uuid.UUID,
        context: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Resume a HITL-gated pipeline run by executing just the postmortem
        stage, reusing `_dispatch_postmortem`/`_build_postmortem_context`
        rather than duplicating A2A-dispatch-and-persist logic.

        Called by resume_listener.py after it receives a
        pipeline.stage_approved event -- approval has already happened by
        the time this runs, so (unlike `run_postmortem_stage`) it does NOT
        re-check the HITL gate.
        """
        if context is None:
            context = await self._build_postmortem_context(incident_id, org_id)
        result = await self._dispatch_postmortem(incident_id, context, org_id)
        logger.info("Resumed and completed postmortem stage for incident %s", incident_id)
        return {"postmortem": result, "status": "completed"}

    @staticmethod
    def _is_uuid(value: str) -> bool:
        try:
            uuid.UUID(value)
            return True
        except (ValueError, AttributeError):
            return False

    @staticmethod
    def _parse_json(text: str) -> dict[str, Any]:
        """Parse JSON from LLM output, handling markdown code blocks."""
        try:
            return json.loads(text)
        except (json.JSONDecodeError, TypeError):
            pass
        if "```json" in str(text):
            start = text.index("```json") + 7
            end = text.index("```", start)
            return json.loads(text[start:end].strip())
        if "```" in str(text):
            start = text.index("```") + 3
            end = text.index("```", start)
            return json.loads(text[start:end].strip())
        return {}
