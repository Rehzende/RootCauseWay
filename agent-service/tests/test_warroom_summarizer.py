"""Tests for WarRoomSummarizer: prompt structure + parsed LLM output."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from app.config.settings import get_settings
from app.warroom.summarizer import WarRoomSummarizer, WarRoomSummary, build_prompt


@pytest.fixture
def transcript():
    return (
        "Alice: kicking off the war room for the checkout outage.\n"
        "Bob: 502s spiking on the payments gateway since 14:02 UTC.\n"
        "Alice: rolling back the last deploy.\n"
        "Bob: rollback resolved it. I'll add a canary check before next deploy."
    )


@pytest.fixture
def llm_response_payload():
    return {
        "executive_summary": "Checkout outage caused by a bad deploy; resolved via rollback.",
        "key_points": [
            "502s spiked on payments gateway at 14:02 UTC",
            "Rollback of the last deploy resolved the issue",
        ],
        "action_items": [
            {"description": "Add a canary check before deploys", "owner_hint": "Bob"},
        ],
    }


class TestBuildPrompt:
    def test_prompt_includes_transcript_and_instructions(self, transcript):
        prompt = build_prompt(transcript)

        assert transcript in prompt
        assert "executive_summary" in prompt
        assert "key_points" in prompt
        assert "action_items" in prompt
        assert "owner_hint" in prompt


class TestSummarize:
    @respx.mock
    async def test_calls_llm_and_parses_structured_output(self, transcript, llm_response_payload):
        settings = get_settings()
        route = respx.post(f"{settings.openai_api_base}/chat/completions").mock(
            return_value=httpx.Response(
                200,
                json={
                    "choices": [
                        {"message": {"content": json.dumps(llm_response_payload)}},
                    ],
                },
            )
        )

        summarizer = WarRoomSummarizer()
        result = await summarizer.summarize(transcript)
        await summarizer.close()

        assert route.called
        request = route.calls.last.request
        body = json.loads(request.content)
        assert body["model"] == settings.llm_model
        assert transcript in body["messages"][0]["content"]
        assert request.headers["Authorization"] == f"Bearer {settings.openai_api_key}"

        assert isinstance(result, WarRoomSummary)
        assert result.executive_summary == llm_response_payload["executive_summary"]
        assert result.key_points == llm_response_payload["key_points"]
        assert len(result.action_items) == 1
        assert result.action_items[0].description == "Add a canary check before deploys"
        assert result.action_items[0].owner_hint == "Bob"

    @respx.mock
    async def test_handles_prose_wrapped_json(self, transcript, llm_response_payload):
        settings = get_settings()
        wrapped = f"Sure, here is the summary:\n```json\n{json.dumps(llm_response_payload)}\n```"
        respx.post(f"{settings.openai_api_base}/chat/completions").mock(
            return_value=httpx.Response(
                200, json={"choices": [{"message": {"content": wrapped}}]},
            )
        )

        summarizer = WarRoomSummarizer()
        result = await summarizer.summarize(transcript)
        await summarizer.close()

        assert result.executive_summary == llm_response_payload["executive_summary"]

    @respx.mock
    async def test_falls_back_to_raw_text_on_unparseable_response(self, transcript):
        settings = get_settings()
        respx.post(f"{settings.openai_api_base}/chat/completions").mock(
            return_value=httpx.Response(
                200, json={"choices": [{"message": {"content": "not json at all"}}]},
            )
        )

        summarizer = WarRoomSummarizer()
        result = await summarizer.summarize(transcript)
        await summarizer.close()

        assert isinstance(result, WarRoomSummary)
        assert result.executive_summary == "not json at all"
        assert result.key_points == []
        assert result.action_items == []

    async def test_empty_transcript_short_circuits_without_llm_call(self):
        summarizer = WarRoomSummarizer()
        result = await summarizer.summarize("")
        await summarizer.close()

        assert "No transcript" in result.executive_summary
