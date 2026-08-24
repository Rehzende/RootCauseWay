"""Regression coverage for the Knowledge Base read/write field-name
mismatch a platform audit found -- pinned separately from
test_outer_loop.py so a future accidental revert to the old (wrong) field
names fails loudly and specifically here, rather than only showing up as
"N/A" text a human has to notice in a broader assertion.

Ground truth for every field name below is
backend/internal/models/features.go's KnowledgeBaseEntry/
CreateKnowledgeBaseRequest JSON tags -- not this file's own assumptions.
"""

from __future__ import annotations

from unittest.mock import AsyncMock
from uuid import uuid4

import pytest

from app.loop.outer_loop import OuterLoop
from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL

# The exact set of keys CreateKnowledgeBaseRequest (Go) will actually bind.
# Anything outside this set is silently dropped by Gin -- not an error, but
# dead weight that misleads whoever reads the Python code into thinking
# it's persisted.
_ALLOWED_CREATE_KEYS = {
    "incident_id", "software_id", "category", "error_pattern",
    "root_cause_summary", "resolution_summary", "lessons_learned",
    "action_items", "tags",
}


@pytest.fixture
def outer_loop():
    return OuterLoop()


@pytest.mark.asyncio
async def test_build_few_shot_context_uses_real_backend_field_names(outer_loop):
    """A real /internal/knowledge-base/search response entry -- exact shape
    of backend/internal/models/features.go's KnowledgeBaseEntry, no
    "title"/"alert_pattern"/"root_cause"/"resolution" keys exist there."""
    entry = {
        "id": str(uuid4()),
        "category": "database",
        "error_pattern": "ConnectionPoolExhausted",
        "root_cause_summary": "Pool size too small for peak load",
        "resolution_summary": "Increased max_connections from 20 to 100",
        "human_validated": True,
        "confidence": 0.85,
    }

    result = await outer_loop.build_few_shot_context([entry])

    assert "ConnectionPoolExhausted" in result
    assert "Pool size too small for peak load" in result
    assert "Increased max_connections from 20 to 100" in result
    assert "N/A" not in result


@pytest.mark.asyncio
async def test_build_few_shot_context_falls_back_gracefully_on_missing_fields(outer_loop):
    """A legitimately sparse entry (no error_pattern/resolution_summary)
    must still render, with N/A only for the fields actually absent --
    not crash, not silently drop the entry."""
    result = await outer_loop.build_few_shot_context([{"root_cause_summary": "leak"}])

    assert "leak" in result
    assert "N/A" in result  # for the missing resolution/category


@pytest.mark.asyncio
async def test_extract_and_store_knowledge_sends_only_fields_the_backend_binds():
    backend = AsyncMock()
    backend.create_knowledge_entry = AsyncMock(return_value={"id": str(uuid4())})
    outer_loop = OuterLoop()

    await outer_loop.extract_and_store_knowledge(
        backend, uuid4(), uuid4(), "sw-1",
        rca_data={
            "root_cause_summary": "Connection pool exhausted under load",
            "root_cause_category": "infrastructure",
        },
        postmortem_data={
            "title": "Postmortem: Checkout Outage",
            "prevention_measures": ["Add pool size alert", "Load test before release"],
            "lessons_learned": ["Pool size was never load-tested"],
            "action_items": [{"description": "Add alert", "owner": "Platform", "priority": "P1"}],
        },
    )

    backend.create_knowledge_entry.assert_awaited_once()
    _org_id, payload = backend.create_knowledge_entry.call_args[0]

    assert set(payload.keys()) <= _ALLOWED_CREATE_KEYS
    assert payload["root_cause_summary"] == "Connection pool exhausted under load"
    assert payload["category"] == "infrastructure"
    assert payload["resolution_summary"] == "Add pool size alert; Load test before release"
    assert payload["lessons_learned"] == ["Pool size was never load-tested"]
    assert payload["action_items"] == [{"description": "Add alert", "owner": "Platform", "priority": "P1"}]
    assert "title" not in payload  # the field doesn't exist on the backend struct


@pytest.mark.asyncio
async def test_extract_and_store_knowledge_skips_when_root_cause_summary_missing():
    """root_cause_summary is binding:"required" on the Go side -- sending
    a request without it is a guaranteed 400. Skip client-side instead of
    burning a doomed HTTP call every time RCA data is incomplete."""
    backend = AsyncMock()
    outer_loop = OuterLoop()

    await outer_loop.extract_and_store_knowledge(
        backend, uuid4(), uuid4(), "sw-1",
        rca_data={"root_cause_category": "infrastructure"},  # no root_cause_summary
        postmortem_data=None,
    )

    backend.create_knowledge_entry.assert_not_awaited()


@pytest.mark.asyncio
async def test_extract_and_store_knowledge_records_swallowed_error_on_failure():
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="outer_loop", error_type="knowledge_entry_create_failed"
    )._value.get()

    backend = AsyncMock()
    backend.create_knowledge_entry = AsyncMock(side_effect=RuntimeError("backend down"))
    outer_loop = OuterLoop()

    await outer_loop.extract_and_store_knowledge(
        backend, uuid4(), uuid4(), "sw-1",
        rca_data={"root_cause_summary": "leak"},
        postmortem_data=None,
    )

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="outer_loop", error_type="knowledge_entry_create_failed"
    )._value.get()
    assert after == before + 1
