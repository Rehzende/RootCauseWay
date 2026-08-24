"""Tests for InnerLoop."""

import pytest
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

from app.loop.inner_loop import InnerLoop


@pytest.fixture
def inner_loop():
    return InnerLoop()


@pytest.fixture
def mock_orchestrator():
    orch = AsyncMock()
    orch.refine_rca = AsyncMock(return_value={"rca": {"rca": {"confidence": 0.9, "root_cause_summary": "refined"}}})
    return orch


class TestInnerLoop:
    @pytest.mark.asyncio
    async def test_high_confidence_skips_refinement(self, inner_loop, mock_orchestrator):
        results = {"rca": {"rca": {"confidence": 0.85, "root_cause_summary": "known cause"}}}
        refined = await inner_loop.evaluate_and_refine(mock_orchestrator, uuid4(), results)
        assert refined == results
        mock_orchestrator.refine_rca.assert_not_called()
        assert inner_loop.last_run_stats == {
            "initial_confidence": 0.85,
            "final_confidence": 0.85,
            "iterations_run": 0,
            "refined": False,
        }

    @pytest.mark.asyncio
    async def test_low_confidence_triggers_refinement(self, inner_loop, mock_orchestrator):
        results = {"rca": {"rca": {"confidence": 0.3, "root_cause_summary": "uncertain"}}}
        refined = await inner_loop.evaluate_and_refine(mock_orchestrator, uuid4(), results)
        mock_orchestrator.refine_rca.assert_called()
        # After refinement, confidence should be merged from refined result
        assert refined["rca"]["rca"]["confidence"] == 0.9
        assert inner_loop.last_run_stats == {
            "initial_confidence": 0.3,
            "final_confidence": 0.9,
            "iterations_run": 1,
            "refined": True,
        }

    @pytest.mark.asyncio
    async def test_max_iterations_limit(self, inner_loop):
        # Orchestrator always returns low confidence
        orch = AsyncMock()
        orch.refine_rca = AsyncMock(return_value={"rca": {"rca": {"confidence": 0.2}}})

        results = {"rca": {"rca": {"confidence": 0.1}}}
        refined = await inner_loop.evaluate_and_refine(orch, uuid4(), results, max_iterations=2)
        assert orch.refine_rca.call_count == 2
        assert inner_loop.last_run_stats == {
            "initial_confidence": 0.1,
            "final_confidence": 0.2,
            "iterations_run": 2,
            "refined": True,
        }

    @pytest.mark.asyncio
    async def test_extract_confidence_direct(self, inner_loop):
        assert inner_loop._extract_confidence({"rca": {"confidence": 0.5}}) == 0.5

    @pytest.mark.asyncio
    async def test_extract_confidence_nested(self, inner_loop):
        assert inner_loop._extract_confidence({"rca": {"rca": {"confidence": 0.8}}}) == 0.8

    @pytest.mark.asyncio
    async def test_extract_confidence_missing(self, inner_loop):
        # No RCA data at all -- nothing to refine, returns 1.0
        assert inner_loop._extract_confidence({}) == 1.0

    @pytest.mark.asyncio
    async def test_extract_confidence_error_rca(self, inner_loop):
        # RCA with error -- nothing to refine
        assert inner_loop._extract_confidence({"rca": {"error": "agent_unavailable"}}) == 1.0
