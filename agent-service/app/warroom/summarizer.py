"""LLM-based summarization of war room meeting transcripts.

Given the raw transcript captured from a Teams meeting (or the
MockTeamsClient's canned transcript in dev/test), calls an OpenAI-compatible
chat completions endpoint (same settings pattern as the rest of
agent-service: openai_api_base / openai_api_key / llm_model) and parses the
response into a structured WarRoomSummary.
"""

from __future__ import annotations

import json
import logging
import re

import httpx
from pydantic import BaseModel, Field

from app.config.settings import get_settings

logger = logging.getLogger(__name__)


class WarRoomActionItem(BaseModel):
    description: str
    owner_hint: str | None = None


class WarRoomSummary(BaseModel):
    executive_summary: str
    key_points: list[str] = Field(default_factory=list)
    action_items: list[WarRoomActionItem] = Field(default_factory=list)


SUMMARY_PROMPT_TEMPLATE = """You are an SRE assistant summarizing an incident \
war room meeting transcript. Read the transcript below and extract:

1. executive_summary: a 2-4 sentence summary of what happened and how it was resolved.
2. key_points: a list of the most important factual points raised during the meeting.
3. action_items: a list of concrete follow-up actions mentioned, each with a
   "description" and an optional "owner_hint" (the name of whoever volunteered
   or was assigned to it, if mentioned; omit or leave blank if unclear).

Respond with ONLY a single JSON object matching this shape, no prose before or after:
{{
  "executive_summary": "...",
  "key_points": ["...", "..."],
  "action_items": [{{"description": "...", "owner_hint": "..."}}]
}}

Transcript:
---
{transcript}
---
"""


def build_prompt(transcript: str) -> str:
    return SUMMARY_PROMPT_TEMPLATE.format(transcript=transcript)


def _extract_json_object(text: str) -> dict:
    """Extract the first top-level JSON object from an LLM response.

    Models occasionally wrap JSON in code fences or add stray prose; this
    finds the first `{...}` span and parses it, raising ValueError if none
    is found or it doesn't parse.
    """
    match = re.search(r"\{.*\}", text, re.DOTALL)
    if not match:
        raise ValueError(f"no JSON object found in LLM response: {text[:200]!r}")
    return json.loads(match.group(0))


class WarRoomSummarizer:
    """Summarizes war room transcripts via an OpenAI-compatible chat endpoint."""

    def __init__(self, http_client: httpx.AsyncClient | None = None):
        self._client = http_client
        self._owns_client = http_client is None

    async def _get_client(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = httpx.AsyncClient(timeout=120.0)
        return self._client

    async def close(self) -> None:
        if self._owns_client and self._client is not None:
            await self._client.aclose()

    async def summarize(self, transcript: str) -> WarRoomSummary:
        """Summarize a raw meeting transcript into structured output."""
        if not transcript or not transcript.strip():
            return WarRoomSummary(executive_summary="No transcript available to summarize.")

        settings = get_settings()
        client = await self._get_client()

        prompt = build_prompt(transcript)
        resp = await client.post(
            f"{settings.openai_api_base}/chat/completions",
            headers={"Authorization": f"Bearer {settings.openai_api_key}"},
            json={
                "model": settings.llm_model,
                "messages": [{"role": "user", "content": prompt}],
                "temperature": 0.1,
            },
        )
        resp.raise_for_status()
        content = resp.json()["choices"][0]["message"]["content"]

        try:
            data = _extract_json_object(content)
            return WarRoomSummary.model_validate(data)
        except (ValueError, json.JSONDecodeError) as exc:
            logger.warning("Failed to parse war room summary from LLM output: %s", exc)
            return WarRoomSummary(executive_summary=content.strip()[:2000])
