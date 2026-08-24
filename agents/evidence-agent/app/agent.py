"""Evidence Agent: analyzes what evidence to collect based on alert and software context."""

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

EVIDENCE_PROMPT = """\
You are an expert incident evidence analyst. Given an alert, triage result, and \
software context, determine what evidence should be collected and analyzed.

## Alert
{alert_json}

## Triage Result
{triage_json}

## Software Context
{software_context_json}

Based on the software's cloud resources, databases, observability stack, and \
deployment pipeline, determine:
{skill_instructions_section}
## Rules
- Only recommend data sources and log queries that are plausible given the \
software context above (its actual observability stack, databases, cloud \
resources). Do not invent tools or systems that aren't implied by the context.
- "relevance" and "rationale" must explain why this specific evidence \
matters for *this* alert, not a generic justification that could apply to \
any incident.

Respond with ONLY valid JSON:
{{
  "evidence_findings": [
    {{
      "type": "log|metric|trace|config|deployment|database",
      "title": "description of evidence",
      "source": "where to find it",
      "relevance": "why this matters",
      "priority": "high|medium|low"
    }}
  ],
  "recommended_data_sources": ["list of systems to check"],
  "log_queries": [
    {{
      "source": "system name",
      "query": "suggested query or search pattern",
      "rationale": "why this query"
    }}
  ],
  "summary": "overall evidence collection strategy"
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


class EvidenceAgent:
    def __init__(self, api_base: str, api_key: str, model: str, k8s_agent_url: str = ""):
        self._api_base = api_base
        self._api_key = api_key
        self._model = model
        # A2A mesh: see rca_agent.RCAAgent for the same pattern/rationale.
        self._k8s_agent_url = k8s_agent_url
        self._peer = A2APeerClient()

    @mlflow.trace(span_type=SpanType.AGENT, name="evidence_agent.handle_task")
    async def handle_task(self, task_id: str, message: Message) -> Task:
        input_data = self._extract_data(message)
        alert = input_data.get("alert", {})
        triage = input_data.get("triage", input_data.get("triage_result", {}))
        software_context = input_data.get("software_context", {})
        llm_config = input_data.get("llm_config") or {}

        prompt = EVIDENCE_PROMPT.format(
            alert_json=json.dumps(alert, indent=2, default=str),
            triage_json=json.dumps(triage, indent=2, default=str),
            software_context_json=json.dumps(software_context, indent=2, default=str),
            skill_instructions_section=_skill_instructions_section(input_data),
        )

        llm_usage: dict[str, Any] = {}
        try:
            result, llm_usage = await self._call_llm(prompt, llm_config)
            parsed = self._parse_json(result)
        except Exception:
            record_swallowed_error("evidence_agent", "llm_call_failed")
            logger.exception("Evidence LLM call failed")
            parsed = {
                "evidence_findings": [],
                "recommended_data_sources": [],
                "log_queries": [],
                "summary": "Evidence collection failed",
            }

        namespace = alert.get("labels", {}).get("namespace") if isinstance(alert.get("labels"), dict) else None
        if namespace and self._k8s_agent_url:
            k8s_diag = await self._fetch_k8s_diagnostics(namespace, alert.get("service", ""))
            if k8s_diag:
                parsed["k8s_diagnostics"] = k8s_diag

        return Task(
            id=task_id,
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(
                    name="evidence_result",
                    description="Evidence collection analysis",
                    parts=[DataPart(data=parsed)],
                ),
                Artifact(
                    name="llm_usage",
                    description="LLM token usage for this call",
                    parts=[DataPart(data=llm_usage)],
                ),
            ],
        )

    @mlflow.trace(span_type=SpanType.TOOL, name="evidence_agent.fetch_k8s_diagnostics")
    async def _fetch_k8s_diagnostics(self, namespace: str, service: str) -> dict[str, Any]:
        """A2A mesh call: enrich this agent's evidence-collection strategy
        with live cluster diagnostics for a k8s-related alert. Omits
        `service` from the payload so k8s-agent skips its own (redundant)
        LLM RCA and returns just the raw k8s_cluster_data artifact."""
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
            record_swallowed_error("evidence_agent", "mesh_call_k8s_agent_failed")
            logger.exception("Mesh call to k8s-agent failed, proceeding without cluster diagnostics")
            return {}

    @mlflow.trace(span_type=SpanType.LLM, name="evidence_agent.call_llm")
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
