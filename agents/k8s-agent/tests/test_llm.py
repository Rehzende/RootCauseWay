"""Coverage for analyze_incident's happy path, LLM-call failure, and
malformed-JSON-response fallback -- no test existed for this agent at all
before the platform audit."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.llm import analyze_incident
from app.observability_metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL


def _mock_response(content: str, usage: dict | None = None, model: str | None = None) -> MagicMock:
    resp = MagicMock()
    resp.raise_for_status = MagicMock()
    body: dict = {"choices": [{"message": {"content": content}}]}
    if usage is not None:
        body["usage"] = usage
    if model is not None:
        body["model"] = model
    resp.json.return_value = body
    return resp


@pytest.mark.asyncio
async def test_analyze_incident_happy_path():
    llm_json = json.dumps({
        "summary": "pool exhaustion causing high latency",
        "severity": "critical",
        "root_causes": [{"cause": "connection pool exhausted", "confidence": "high", "evidence": ["p95=30s"]}],
        "recommended_actions": ["increase pool size"],
    })
    with patch("httpx.AsyncClient") as MockClient:
        client_instance = MockClient.return_value.__aenter__.return_value
        client_instance.post = AsyncMock(return_value=_mock_response(llm_json))

        result = await analyze_incident(
            evidence={"active_alerts": []}, api_base="http://lm-studio:1234/v1",
            api_key="x", model="m",
        )

    assert result["severity"] == "critical"
    assert result["root_causes"][0]["confidence"] == "high"
    # No usage/model in the mocked response -- must fall back to the
    # configured model and zeroed counts, not crash on missing keys.
    assert result["llm_usage"] == {"model": "m", "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}


@pytest.mark.asyncio
async def test_analyze_incident_captures_real_usage_from_api_response():
    llm_json = json.dumps({"summary": "x", "severity": "warning", "root_causes": [], "recommended_actions": []})
    usage = {"prompt_tokens": 900, "completion_tokens": 120, "total_tokens": 1020}
    with patch("httpx.AsyncClient") as MockClient:
        client_instance = MockClient.return_value.__aenter__.return_value
        client_instance.post = AsyncMock(
            return_value=_mock_response(llm_json, usage=usage, model="qwen2.5-coder-14b-instruct")
        )

        result = await analyze_incident(
            evidence={"active_alerts": []}, api_base="http://lm-studio:1234/v1",
            api_key="x", model="m",
        )

    assert result["llm_usage"] == {
        "model": "qwen2.5-coder-14b-instruct",
        "prompt_tokens": 900,
        "completion_tokens": 120,
        "total_tokens": 1020,
    }


@pytest.mark.asyncio
async def test_analyze_incident_llm_unreachable_records_metric_and_returns_error():
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="k8s_agent", error_type="llm_call_failed"
    )._value.get()

    with patch("httpx.AsyncClient") as MockClient:
        client_instance = MockClient.return_value.__aenter__.return_value
        client_instance.post = AsyncMock(side_effect=RuntimeError("connection refused"))

        result = await analyze_incident(
            evidence={}, api_base="http://lm-studio:1234/v1", api_key="x", model="m",
        )

    assert "error" in result
    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="k8s_agent", error_type="llm_call_failed"
    )._value.get()
    assert after == before + 1


@pytest.mark.asyncio
async def test_analyze_incident_malformed_json_returns_error_with_raw_response():
    with patch("httpx.AsyncClient") as MockClient:
        client_instance = MockClient.return_value.__aenter__.return_value
        client_instance.post = AsyncMock(return_value=_mock_response("not json at all"))

        result = await analyze_incident(
            evidence={}, api_base="http://lm-studio:1234/v1", api_key="x", model="m",
        )

    assert "error" in result
    assert result["raw_response"] == "not json at all"
