"""RCA Agent: generates RCI (investigation), RCA (root cause analysis), and hypothesis."""

from __future__ import annotations

import json
import logging
import uuid
from typing import Any

import httpx
import mlflow
from mlflow.entities import SpanType

from app.a2a.client import A2APeerClient
from app.a2a.models import Artifact, DataPart, Message, Role, Task, TaskStatus
from app.observability.metrics import record_swallowed_error

logger = logging.getLogger(__name__)

RCA_PROMPT = """\
You are an expert root cause analyst applying the blameless-postmortem \
methodology used by Google SRE and adopted by most mature incident-response \
practices. Given an alert, triage result, and evidence, generate a \
comprehensive Root Cause Investigation (RCI), Root Cause Analysis (RCA), and \
hypothesis.

## Alert
{alert_json}

## Triage Result
{triage_json}

## Evidence
{evidence_json}

## Software Context
{software_context_json}
{skill_instructions_section}
## Rules
- Ground every claim in the evidence above. Do not invent metrics, log \
lines, or facts that are not present in the input.
- If the evidence is insufficient for a confident conclusion, say so \
explicitly in the relevant summary field and lower "confidence" accordingly \
-- do not guess.
- Confidence calibration: 0.8-1.0 = evidence directly and unambiguously \
supports the conclusion; 0.4-0.7 = plausible but circumstantial, or the \
evidence supports more than one explanation; 0.0-0.3 = little to no \
supporting evidence, largely speculative.
- Write in blameless language: describe what the system, process, or \
decision did -- never who made a mistake. Do not name or blame an \
individual person.
- "root_cause_summary" is the underlying systemic condition that made the \
incident possible (a missing safeguard, an untested assumption, an \
undersized resource) -- not just the proximate event that triggered it. \
Capture the proximate trigger as the first entry of "five_whys" or in \
"contributing_factors" instead.

Respond with ONLY valid JSON containing three sections:
{{
  "rci": {{
    "investigation_summary": "detailed investigation narrative",
    "impact_assessment": "business and technical impact",
    "affected_services": ["list of affected services"],
    "affected_users_estimate": "estimated user impact",
    "detection_method": "how the issue was detected (e.g. alert, customer report, synthetic monitoring)",
    "detection_time": "ISO 8601 timestamp of when the issue was first detected -- best estimate from the alert/evidence timestamps above, or null if it cannot be determined",
    "timeline": [
      {{"time": "timestamp or relative time", "event": "what happened"}}
    ]
  }},
  "rca": {{
    "root_cause_summary": "the underlying systemic root cause, per the Rules above",
    "root_cause_category": "infrastructure|code|config|dependency|human|external",
    "contributing_factors": ["factor1", "factor2"],
    "five_whys": [
      {{"why": "question", "answer": "answer, evidence-grounded"}}
    ],
    "confidence": 0.0-1.0
  }},
  "hypothesis": {{
    "root_cause": "primary hypothesis for root cause",
    "confidence": 0.0-1.0,
    "recommended_actions": ["action1", "action2"],
    "mitigation_steps": ["step1", "step2"],
    "verification_steps": ["how to verify this hypothesis"]
  }}
}}
"""


def _skill_instructions_section(input_data: dict[str, Any]) -> str:
    """Renders a skill's custom prompt_template (Skill.prompt_template in
    the Go model, threaded through by the orchestrator as
    input_data["skill_prompt_template"]) as an *additional* instructions
    section injected into the prompt -- not a full replacement of it, so
    the strict JSON output schema below (which the rest of the pipeline's
    parsing/persistence depends on) still gets followed. Empty string when
    the dispatched skill has no custom template, e.g. the built-in "rca"
    skill (agent-card-sourced, never has one) or a custom skill nobody
    filled the field in for."""
    template = input_data.get("skill_prompt_template")
    if not template:
        return ""
    return f"\n## Additional Skill-Specific Instructions\n{template}\n"


class RCAAgent:
    def __init__(
        self,
        api_base: str,
        api_key: str,
        model: str,
        evidence_agent_url: str = "",
        k8s_agent_url: str = "",
    ):
        self._api_base = api_base
        self._api_key = api_key
        self._model = model
        # A2A mesh: peers this agent can call directly for data its own
        # input is missing, rather than only ever working off whatever the
        # orchestrator pre-fetched. Empty string disables that link (e.g.
        # in tests, or a deployment that doesn't wire the peer URL).
        self._evidence_agent_url = evidence_agent_url
        self._k8s_agent_url = k8s_agent_url
        self._peer = A2APeerClient()

    @mlflow.trace(span_type=SpanType.AGENT, name="rca_agent.handle_task")
    async def handle_task(self, task_id: str, message: Message) -> Task:
        input_data = self._extract_data(message)
        alert = input_data.get("alert", {})
        triage = input_data.get("triage", input_data.get("triage_result", {}))
        evidence = input_data.get("evidence", input_data.get("evidence_result", {}))
        software_context = input_data.get("software_context", {})
        llm_config = input_data.get("llm_config") or {}

        if not evidence and self._evidence_agent_url:
            evidence = await self._fetch_evidence(alert, triage, software_context)

        namespace = alert.get("labels", {}).get("namespace") if isinstance(alert.get("labels"), dict) else None
        if namespace and self._k8s_agent_url and not evidence.get("k8s_diagnostics"):
            k8s_diag = await self._fetch_k8s_diagnostics(namespace, alert.get("service", ""))
            if k8s_diag:
                evidence = {**evidence, "k8s_diagnostics": k8s_diag}

        prompt = RCA_PROMPT.format(
            alert_json=json.dumps(alert, indent=2, default=str),
            triage_json=json.dumps(triage, indent=2, default=str),
            evidence_json=json.dumps(evidence, indent=2, default=str),
            software_context_json=json.dumps(software_context, indent=2, default=str),
            skill_instructions_section=_skill_instructions_section(input_data),
        )

        llm_usage: dict[str, Any] = {}
        try:
            result, llm_usage = await self._call_llm(prompt, llm_config)
            parsed = self._parse_json(result)
        except Exception:
            record_swallowed_error("rca_agent", "llm_call_failed")
            logger.exception("RCA LLM call failed")
            parsed = {
                "rci": {"investigation_summary": "RCA failed", "impact_assessment": "unknown", "affected_services": []},
                "rca": {"root_cause_summary": "unknown", "root_cause_category": "unknown", "contributing_factors": [], "five_whys": [], "confidence": 0.0},
                "hypothesis": {"root_cause": "unknown", "confidence": 0.0, "recommended_actions": [], "mitigation_steps": []},
            }

        return Task(
            id=task_id,
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="rci", description="Root Cause Investigation", parts=[DataPart(data=parsed.get("rci", {}))]),
                Artifact(name="rca", description="Root Cause Analysis", parts=[DataPart(data=parsed.get("rca", {}))]),
                Artifact(name="hypothesis", description="Root cause hypothesis", parts=[DataPart(data=parsed.get("hypothesis", {}))]),
                Artifact(name="llm_usage", description="LLM token usage for this call", parts=[DataPart(data=llm_usage)]),
            ],
        )

    @mlflow.trace(span_type=SpanType.AGENT, name="rca_agent.fetch_evidence_from_peer")
    async def _fetch_evidence(
        self, alert: dict[str, Any], triage: dict[str, Any], software_context: dict[str, Any]
    ) -> dict[str, Any]:
        """A2A mesh call: the orchestrator didn't include evidence in this
        task's input (e.g. it decided to call rca directly), so ask
        evidence-agent for it rather than running RCA blind."""
        try:
            task = await self._peer.send_task(
                self._evidence_agent_url,
                str(uuid.uuid4()),
                Message(role=Role.USER, parts=[DataPart(data={
                    "alert": alert, "triage": triage, "software_context": software_context,
                })]),
            )
            result = A2APeerClient.extract_result(task)
            return result.get("evidence_result", {})
        except Exception:
            record_swallowed_error("rca_agent", "mesh_call_evidence_agent_failed")
            logger.exception("Mesh call to evidence-agent failed, proceeding without evidence")
            return {}

    @mlflow.trace(span_type=SpanType.TOOL, name="rca_agent.fetch_k8s_diagnostics")
    async def _fetch_k8s_diagnostics(self, namespace: str, service: str) -> dict[str, Any]:
        """A2A mesh call: enrich RCA with live cluster diagnostics for a
        k8s-related alert. Deliberately omits `service` from the payload --
        k8s-agent only runs its own (redundant, costly) LLM RCA when
        `service` is present; we just want the raw k8s_cluster_data
        artifact (pod status, events, describe output)."""
        try:
            params: dict[str, Any] = {"namespace": namespace}
            if service:
                params["labels"] = {"app": service}
            task = await self._peer.send_task(
                self._k8s_agent_url,
                str(uuid.uuid4()),
                Message(role=Role.USER, parts=[DataPart(data=params)]),
            )
            result = A2APeerClient.extract_result(task)
            return result.get("k8s_cluster_data", {})
        except Exception:
            record_swallowed_error("rca_agent", "mesh_call_k8s_agent_failed")
            logger.exception("Mesh call to k8s-agent failed, proceeding without cluster diagnostics")
            return {}

    @mlflow.trace(span_type=SpanType.LLM, name="rca_agent.call_llm")
    async def _call_llm(self, prompt: str, llm_config: dict[str, Any] | None = None) -> tuple[str, dict[str, Any]]:
        """See triage_agent.TriageAgent._call_llm for the llm_config override rationale."""
        cfg = llm_config or {}
        api_base = cfg.get("api_base") or self._api_base
        api_key = cfg.get("api_key") or self._api_key
        model = cfg.get("model") or self._model
        temperature = cfg.get("temperature", 0.2)
        async with httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(
                f"{api_base}/chat/completions",
                headers={"Authorization": f"Bearer {api_key}"},
                json={
                    "model": model,
                    "messages": [{"role": "user", "content": prompt}],
                    "temperature": temperature,
                },
            )
            resp.raise_for_status()
            data = resp.json()
            usage = data.get("usage") or {}
            llm_usage = {
                "model": data.get("model") or model,
                "prompt_tokens": usage.get("prompt_tokens", 0),
                "completion_tokens": usage.get("completion_tokens", 0),
                "total_tokens": usage.get("total_tokens", 0),
            }
            return data["choices"][0]["message"]["content"], llm_usage

    @staticmethod
    def _extract_data(message: Message) -> dict[str, Any]:
        for part in message.parts:
            if hasattr(part, "data"):
                return part.data
        return {}

    @staticmethod
    def _parse_json(text: str) -> dict[str, Any]:
        try:
            return json.loads(text)
        except (json.JSONDecodeError, TypeError):
            pass
        for marker in ("```json", "```"):
            if marker in str(text):
                start = text.index(marker) + len(marker)
                end = text.index("```", start)
                return json.loads(text[start:end].strip())
        return {}
