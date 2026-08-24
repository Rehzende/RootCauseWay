"""Postmortem Agent: generates blameless postmortem from full incident context."""

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

POSTMORTEM_PROMPT = """\
You are an expert at writing blameless postmortems, following the format \
used by Google SRE and adopted by most mature incident-response practices \
(PagerDuty, Atlassian). Given the full incident context including alert, \
triage, evidence, RCI, and RCA, generate a comprehensive blameless postmortem.

## Alert
{alert_json}

## Triage
{triage_json}

## Evidence
{evidence_json}

## Root Cause Investigation
{rci_json}

## Root Cause Analysis
{rca_json}

## Software Context
{software_context_json}
{skill_instructions_section}
## Rules
- Ground every claim in the context above. Do not invent facts not present \
in the input.
- Write in blameless language throughout: describe what the system or \
process did, never who made a mistake. Action item owners are teams or \
roles, never a person's name.
- "what_went_wrong" describes gaps in systems, tooling, or process -- not \
individual performance.
- Each action item must be a concrete, verifiable change (a code change, a \
new alert, a runbook, a load test) -- not a vague intention like "be more \
careful" or "improve monitoring" without specifics.
- Compute "due_date" for each action item as an ISO 8601 date (YYYY-MM-DD), \
offset from the alert's own timestamp above by its priority: P0 = 3 days, \
P1 = 2 weeks, P2 = 1 month, P3 = 1 quarter.

Respond with ONLY valid JSON:
{{
  "title": "Postmortem: concise incident title",
  "executive_summary": "2-3 sentence summary for leadership",
  "incident_timeline": [
    {{"time": "timestamp or relative", "event": "what happened", "actor": "system or process, never a person's name"}}
  ],
  "incident_timeline_narrative": "the same timeline written as flowing chronological prose, from detection to resolution",
  "root_cause_detail": "detailed root cause explanation, consistent with the RCA above",
  "impact_detail": "detailed impact on users and business",
  "lessons_learned": ["lesson1", "lesson2"],
  "what_went_well": ["thing1", "thing2"],
  "what_went_wrong": ["thing1", "thing2"],
  "action_items": [
    {{
      "description": "concrete, verifiable action",
      "owner": "team or role, never a person's name",
      "priority": "P0|P1|P2|P3",
      "due_date": "YYYY-MM-DD, per the Rules above",
      "completed": false
    }}
  ],
  "prevention_measures": ["measure1", "measure2"]
}}
"""


def _skill_instructions_section(input_data: dict[str, Any]) -> str:
    """See rca_agent.py's identical helper for the full rationale --
    renders a linked skill's custom prompt_template as an additional
    instructions section, never a replacement of the strict JSON schema
    below."""
    template = input_data.get("skill_prompt_template")
    if not template:
        return ""
    return f"\n## Additional Skill-Specific Instructions\n{template}\n"


class PostmortemAgent:
    def __init__(self, api_base: str, api_key: str, model: str, rca_agent_url: str = ""):
        self._api_base = api_base
        self._api_key = api_key
        self._model = model
        # A2A mesh: see rca_agent.RCAAgent for the same pattern/rationale.
        self._rca_agent_url = rca_agent_url
        self._peer = A2APeerClient()

    @mlflow.trace(span_type=SpanType.AGENT, name="postmortem_agent.handle_task")
    async def handle_task(self, task_id: str, message: Message) -> Task:
        input_data = self._extract_data(message)
        alert = input_data.get("alert", {})
        triage = input_data.get("triage", input_data.get("triage_result", {}))
        evidence = input_data.get("evidence", input_data.get("evidence_result", {}))
        rci = input_data.get("rci", {})
        rca = input_data.get("rca", {})
        software_context = input_data.get("software_context", {})
        llm_config = input_data.get("llm_config") or {}

        if not rca and self._rca_agent_url:
            rci, rca = await self._fetch_rca(alert, triage, evidence, software_context, rci)

        prompt = POSTMORTEM_PROMPT.format(
            alert_json=json.dumps(alert, indent=2, default=str),
            triage_json=json.dumps(triage, indent=2, default=str),
            evidence_json=json.dumps(evidence, indent=2, default=str),
            rci_json=json.dumps(rci, indent=2, default=str),
            rca_json=json.dumps(rca, indent=2, default=str),
            software_context_json=json.dumps(software_context, indent=2, default=str),
            skill_instructions_section=_skill_instructions_section(input_data),
        )

        llm_usage: dict[str, Any] = {}
        try:
            result, llm_usage = await self._call_llm(prompt, llm_config)
            parsed = self._parse_json(result)
        except Exception:
            record_swallowed_error("postmortem_agent", "llm_call_failed")
            logger.exception("Postmortem LLM call failed")
            parsed = {
                "title": "Postmortem: Generation Failed",
                "executive_summary": "Postmortem generation failed",
                "incident_timeline": [],
                "lessons_learned": [],
                "action_items": [],
                "what_went_well": [],
                "what_went_wrong": [],
                "prevention_measures": [],
            }

        return Task(
            id=task_id,
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(
                    name="postmortem",
                    description="Blameless postmortem document",
                    parts=[DataPart(data=parsed)],
                ),
                Artifact(
                    name="llm_usage",
                    description="LLM token usage for this call",
                    parts=[DataPart(data=llm_usage)],
                ),
            ],
        )

    @mlflow.trace(span_type=SpanType.AGENT, name="postmortem_agent.fetch_rca_from_peer")
    async def _fetch_rca(
        self,
        alert: dict[str, Any],
        triage: dict[str, Any],
        evidence: dict[str, Any],
        software_context: dict[str, Any],
        existing_rci: dict[str, Any],
    ) -> tuple[dict[str, Any], dict[str, Any]]:
        """A2A mesh call: the orchestrator didn't include an RCA in this
        task's input (e.g. postmortem got triggered standalone by the
        incident.resolved event without a prior rca stage), so generate one
        via rca-agent rather than writing a postmortem with an unknown root
        cause. Returns (rci, rca) -- keeps existing_rci if rca-agent's call
        fails, since a partial RCI is still better than none."""
        try:
            task = await self._peer.send_task(
                self._rca_agent_url,
                str(uuid.uuid4()),
                Message(role=Role.USER, parts=[DataPart(data={
                    "alert": alert, "triage": triage, "evidence": evidence,
                    "software_context": software_context,
                })]),
            )
            result = A2APeerClient.extract_result(task)
            return result.get("rci", existing_rci), result.get("rca", {})
        except Exception:
            record_swallowed_error("postmortem_agent", "mesh_call_rca_agent_failed")
            logger.exception("Mesh call to rca-agent failed, proceeding without a fresh RCA")
            return existing_rci, {}

    @mlflow.trace(span_type=SpanType.LLM, name="postmortem_agent.call_llm")
    async def _call_llm(self, prompt: str, llm_config: dict[str, Any] | None = None) -> tuple[str, dict[str, Any]]:
        """See triage_agent.TriageAgent._call_llm for the llm_config override rationale."""
        cfg = llm_config or {}
        api_base = cfg.get("api_base") or self._api_base
        api_key = cfg.get("api_key") or self._api_key
        model = cfg.get("model") or self._model
        temperature = cfg.get("temperature", 0.3)
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
