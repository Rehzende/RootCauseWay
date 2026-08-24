"""Triage Agent: assesses alert severity, categorizes issues, identifies affected components."""

from __future__ import annotations

import json
import logging
from typing import Any

import httpx
import mlflow
from mlflow.entities import SpanType

from app.observability.metrics import record_swallowed_error

from app.a2a.models import (
    Artifact,
    DataPart,
    Message,
    Task,
    TaskStatus,
)

logger = logging.getLogger(__name__)

TRIAGE_PROMPT = """\
You are an expert incident triage analyst. Given an alert and software context, \
assess the severity, categorize the issue, identify affected components, and \
provide a concise summary.

## Alert
{alert_json}

## Software Context
{software_context_json}
{skill_instructions_section}
## Rules
- Ground the assessment in the alert and software context above; do not \
invent components or symptoms not present in the input.
- Confidence calibration: 0.8-1.0 = the alert unambiguously identifies the \
severity and category; 0.4-0.7 = plausible but the alert is ambiguous or \
incomplete; 0.0-0.3 = too little information to assess confidently.

Respond with ONLY valid JSON:
{{
  "severity_assessment": "critical|high|medium|low",
  "category": "string (e.g. infrastructure, application, database, network, security)",
  "affected_components": ["list", "of", "components"],
  "summary": "concise summary of the issue",
  "confidence": 0.0-1.0
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


class TriageAgent:
    def __init__(self, api_base: str, api_key: str, model: str):
        self._api_base = api_base
        self._api_key = api_key
        self._model = model

    @mlflow.trace(span_type=SpanType.AGENT, name="triage_agent.handle_task")
    async def handle_task(self, task_id: str, message: Message) -> Task:
        """Process a triage task and return results as artifacts."""
        input_data = self._extract_data(message)
        alert = input_data.get("alert", {})
        software_context = input_data.get("software_context", {})
        llm_config = input_data.get("llm_config") or {}

        prompt = TRIAGE_PROMPT.format(
            alert_json=json.dumps(alert, indent=2, default=str),
            software_context_json=json.dumps(software_context, indent=2, default=str),
            skill_instructions_section=_skill_instructions_section(input_data),
        )

        llm_usage: dict[str, Any] = {}
        try:
            result, llm_usage = await self._call_llm(prompt, llm_config)
            parsed = self._parse_json(result)
        except Exception:
            record_swallowed_error("triage_agent", "llm_call_failed")
            logger.exception("Triage LLM call failed")
            parsed = {
                "severity_assessment": alert.get("severity", "medium"),
                "category": "unknown",
                "affected_components": [],
                "summary": f"Triage failed for alert: {alert.get('title', 'unknown')}",
                "confidence": 0.0,
            }

        return Task(
            id=task_id,
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(
                    name="triage_result",
                    description="Alert triage assessment",
                    parts=[DataPart(data=parsed)],
                ),
                # Real token/model usage from the API response's `usage`
                # field (not the chars/4 heuristic the orchestrator used to
                # fabricate) -- merged flat into agent_result by the
                # orchestrator, same as every other artifact here.
                Artifact(
                    name="llm_usage",
                    description="LLM token usage for this call",
                    parts=[DataPart(data=llm_usage)],
                ),
            ],
        )

    @mlflow.trace(span_type=SpanType.LLM, name="triage_agent.call_llm")
    async def _call_llm(self, prompt: str, llm_config: dict[str, Any] | None = None) -> tuple[str, dict[str, Any]]:
        """llm_config (from the orchestrator's per-task input_data, see
        Orchestrator.handle_incident) overrides this instance's own
        api_base/api_key/model/temperature for this call only -- lets the
        LLM & Tokens settings UI's org-default/per-agent overrides take
        effect live, without redeploying this pod's env vars."""
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
