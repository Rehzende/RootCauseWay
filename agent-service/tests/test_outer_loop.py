"""Tests for OuterLoop.

Field names in these fixtures deliberately mirror
backend/internal/models/features.go's KnowledgeBaseEntry JSON tags exactly
(error_pattern, root_cause_summary, resolution_summary -- no "title" field
exists at all). A platform audit found the previous version of this file
used made-up field names ("title", "alert_pattern", "root_cause",
"resolution") that happened to match what build_few_shot_context/
extract_and_store_knowledge read at the time -- which meant the tests
passed while the real, deployed backend/agent-service integration was
completely broken end to end (every real search hit rendered as "N/A",
every write attempt 400'd). See test_outer_loop_knowledge_base_shape.py
for the regression coverage pinning the real shape explicitly.
"""

import pytest
from unittest.mock import AsyncMock
from uuid import uuid4

from app.loop.outer_loop import OuterLoop


@pytest.fixture
def outer_loop():
    return OuterLoop()


@pytest.fixture
def mock_backend():
    backend = AsyncMock()
    backend.search_knowledge_base = AsyncMock(return_value=[
        {
            "error_pattern": "HighMemoryUsage",
            "root_cause_summary": "Memory leak in connection pool",
            "category": "memory",
            "resolution_summary": "Restart and patch",
            "confidence": 0.95,
            "human_validated": True,
        },
    ])
    backend.create_knowledge_entry = AsyncMock(return_value={"id": str(uuid4())})
    return backend


class TestOuterLoop:
    @pytest.mark.asyncio
    async def test_find_similar_incidents(self, outer_loop, mock_backend):
        results = await outer_loop.find_similar_incidents(
            mock_backend, uuid4(), "svc-1",
            {"alert_name": "HighCPU", "service": "api"},
        )
        assert len(results) == 1
        assert results[0]["error_pattern"] == "HighMemoryUsage"
        mock_backend.search_knowledge_base.assert_called_once()

    @pytest.mark.asyncio
    async def test_find_similar_incidents_failure(self, outer_loop):
        backend = AsyncMock()
        backend.search_knowledge_base = AsyncMock(side_effect=Exception("fail"))
        results = await outer_loop.find_similar_incidents(
            backend, uuid4(), "svc-1", {},
        )
        assert results == []

    @pytest.mark.asyncio
    async def test_build_few_shot_context_empty(self, outer_loop):
        result = await outer_loop.build_few_shot_context([])
        assert result == ""

    @pytest.mark.asyncio
    async def test_build_few_shot_context_with_entries(self, outer_loop):
        entries = [
            {
                "error_pattern": "HighMem",
                "root_cause_summary": "Leak",
                "category": "memory",
                "resolution_summary": "Patch",
                "confidence": 0.9,
                "human_validated": True,
            },
        ]
        result = await outer_loop.build_few_shot_context(entries)
        assert "## Similar Past Incidents" in result
        assert "HighMem" in result
        assert "Leak" in result
        assert "Patch" in result
        assert "Human-validated" in result
        assert "N/A" not in result

    @pytest.mark.asyncio
    async def test_extract_and_store_knowledge(self, outer_loop, mock_backend):
        await outer_loop.extract_and_store_knowledge(
            mock_backend, uuid4(), uuid4(), "svc-1",
            {"root_cause_summary": "leak", "root_cause_category": "memory", "confidence": 0.9},
            {"title": "Postmortem", "prevention_measures": ["fix"], "lessons_learned": ["check"]},
        )
        mock_backend.create_knowledge_entry.assert_called_once()
        call_data = mock_backend.create_knowledge_entry.call_args[0][1]
        assert call_data["root_cause_summary"] == "leak"
        assert call_data["category"] == "memory"
