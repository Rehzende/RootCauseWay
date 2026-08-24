"""LLM-backed root-cause analysis. Talks to any OpenAI-compatible chat
completions endpoint (LM Studio, OpenRouter, etc.) via OPENAI_API_BASE."""

from __future__ import annotations

import json
import logging
import re
from typing import Any

import httpx
import mlflow
from mlflow.entities import SpanType

from app.observability_metrics import record_swallowed_error

logger = logging.getLogger(__name__)

SYSTEM_PROMPT = """\
Você é um engenheiro de SRE sênior fazendo root cause analysis (RCA) de um \
incidente em produção. Você recebe evidências correlacionadas: alertas ativos, \
métricas (golden signals), logs de erro recentes, traces com erro e o estado \
atual do Kubernetes para o serviço afetado.

Responda SOMENTE com um objeto JSON válido, sem markdown, sem texto fora do \
JSON, no formato exato:
{
  "summary": "resumo de 1-2 frases do que está acontecendo, em linguagem simples",
  "severity": "critical" | "warning" | "info",
  "root_causes": [
    {"cause": "descrição da causa provável", "confidence": "high" | "medium" | "low", "evidence": ["evidência 1", "evidência 2"]}
  ],
  "recommended_actions": ["ação recomendada 1", "ação recomendada 2"]
}

Ordene root_causes da mais provável para a menos provável. Cite evidência \
concreta (valores de métrica, trecho de log, nome de trace) — não invente \
causas sem respaldo nos dados fornecidos. Se a evidência for insuficiente \
para uma conclusão confiante, diga isso em "summary" e marque confidence "low".\
"""


def _build_user_prompt(evidence: dict[str, Any]) -> str:
    return "Evidências do incidente:\n\n```json\n" + json.dumps(evidence, indent=2, default=str, ensure_ascii=False) + "\n```"


def _extract_json(text: str) -> dict[str, Any]:
    """Best-effort JSON extraction — strips ```json fences if the model added them anyway."""
    stripped = text.strip()
    match = re.search(r"```(?:json)?\s*(\{.*\})\s*```", stripped, re.DOTALL)
    candidate = match.group(1) if match else stripped
    # Fall back to the first {...} block if there's leading/trailing prose.
    if not candidate.lstrip().startswith("{"):
        brace_match = re.search(r"\{.*\}", candidate, re.DOTALL)
        if brace_match:
            candidate = brace_match.group(0)
    return json.loads(candidate)


@mlflow.trace(span_type=SpanType.LLM, name="k8s_agent.analyze_incident")
async def analyze_incident(
    evidence: dict[str, Any],
    api_base: str,
    api_key: str,
    model: str,
    timeout: float = 150.0,
) -> dict[str, Any]:
    """Send correlated evidence to the LLM and return a structured RCA.

    On any failure (LLM unreachable, malformed response, etc.) returns a dict
    with an "error" key and the raw evidence still attached, so callers can
    fall back to just showing the raw data instead of losing the whole task.
    """
    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": _build_user_prompt(evidence)},
        ],
        "temperature": 0.2,
        "max_tokens": 1200,
    }
    headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}

    try:
        async with httpx.AsyncClient() as client:
            r = await client.post(f"{api_base}/chat/completions", json=payload, headers=headers, timeout=timeout)
            r.raise_for_status()
            data = r.json()
            content = data["choices"][0]["message"]["content"]
    except Exception as exc:
        record_swallowed_error("k8s_agent", "llm_call_failed")
        logger.exception("LLM call failed")
        return {"error": f"LLM call failed: {exc}"}

    usage = data.get("usage") or {}
    llm_usage = {
        "model": data.get("model") or model,
        "prompt_tokens": usage.get("prompt_tokens", 0),
        "completion_tokens": usage.get("completion_tokens", 0),
        "total_tokens": usage.get("total_tokens", 0),
    }

    try:
        result = _extract_json(content)
    except Exception as exc:
        logger.warning("LLM returned non-JSON output: %s", exc)
        return {"error": f"LLM returned non-JSON output: {exc}", "raw_response": content, "llm_usage": llm_usage}

    # Real token/model usage -- k8s-agent's standalone webhook path doesn't
    # persist to agent_runs (no orchestrator involved), so this is currently
    # only visible via the MLflow span attributes and this dict, not the
    # Analytics cost dashboard. Still captured for parity with the other
    # 4 agents and for future wiring.
    result["llm_usage"] = llm_usage
    return result
