"""Tests for NotificationDispatcher."""

import json

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import uuid4

from app.notifications.dispatcher import NotificationDispatcher


@pytest.fixture
def dispatcher():
    d = NotificationDispatcher()
    return d


class TestNotificationDispatcher:
    def test_format_incident_message_created(self, dispatcher):
        msg = dispatcher._format_incident_message(
            {"incident_id": "abc", "severity": "critical"},
            "incident_created",
        )
        assert msg["title"] == "New Incident Created"
        assert "critical" in msg["body"]
        assert msg["color"] == "FF0000"

    def test_format_incident_message_rca(self, dispatcher):
        msg = dispatcher._format_incident_message(
            {"incident_id": "abc", "severity": "medium", "root_cause": "OOM"},
            "rca_completed",
        )
        assert msg["title"] == "Root Cause Analysis Completed"
        assert "OOM" in msg["body"]

    def test_match_channels_no_policies(self, dispatcher):
        channels = [
            {"id": "ch1", "type": "slack", "enabled": True},
            {"id": "ch2", "type": "teams", "enabled": False},
        ]
        matched = dispatcher._match_channels(channels, [], "incident_created", "high")
        assert len(matched) == 1
        assert matched[0]["id"] == "ch1"

    def test_match_channels_with_policy(self, dispatcher):
        channels = [
            {"id": "ch1", "type": "slack", "enabled": True},
            {"id": "ch2", "type": "teams", "enabled": True},
        ]
        policies = [
            {
                "event_types": ["incident_created"],
                "severities": ["critical", "high"],
                "channel_ids": ["ch1"],
            },
        ]
        matched = dispatcher._match_channels(channels, policies, "incident_created", "high")
        assert len(matched) == 1
        assert matched[0]["id"] == "ch1"

    def test_match_channels_severity_mismatch(self, dispatcher):
        channels = [{"id": "ch1", "type": "slack", "enabled": True}]
        policies = [
            {"event_types": ["incident_created"], "severities": ["critical"], "channel_ids": ["ch1"]},
        ]
        matched = dispatcher._match_channels(channels, policies, "incident_created", "low")
        assert len(matched) == 0

    @pytest.mark.asyncio
    async def test_notify_sends_to_matching_channels(self, dispatcher):
        backend = AsyncMock()
        backend.list_notification_channels = AsyncMock(return_value=[
            {"id": "ch1", "type": "webhook", "webhook_url": "http://example.com/hook", "enabled": True},
        ])
        backend.list_escalation_policies = AsyncMock(return_value=[])
        backend.create_notification_log = AsyncMock(return_value={})

        with patch.object(dispatcher, "send_webhook", new_callable=AsyncMock) as mock_send:
            await dispatcher.notify(
                backend, uuid4(), uuid4(), "incident_created",
                {"severity": "high", "incident_id": "test"},
            )
            mock_send.assert_called_once()
            backend.create_notification_log.assert_called_once()

    @pytest.mark.asyncio
    async def test_notify_channels_fetch_failure(self, dispatcher):
        backend = AsyncMock()
        backend.list_notification_channels = AsyncMock(side_effect=Exception("fail"))

        # Should not raise
        await dispatcher.notify(backend, uuid4(), uuid4(), "incident_created", {})

    # --- Interactive elements (Acknowledge / View RCA / Resolve) ---

    @pytest.mark.asyncio
    async def test_send_slack_new_incident_includes_actions_block(self, dispatcher):
        dispatcher._client.post = AsyncMock(return_value=MagicMock(raise_for_status=MagicMock()))
        incident_id = str(uuid4())
        channel_id = str(uuid4())

        await dispatcher.send_slack(
            "http://hooks.slack.com/x",
            {"title": "New Incident", "body": "...", "incident_id": incident_id},
            channel_id=channel_id,
            event_type="incident_created",
        )

        sent = dispatcher._client.post.call_args.kwargs["json"]
        blocks = sent["blocks"]
        actions_blocks = [b for b in blocks if b["type"] == "actions"]
        assert len(actions_blocks) == 1

        elements = actions_blocks[0]["elements"]
        assert len(elements) == 3
        action_ids = {el["action_id"] for el in elements}
        assert action_ids == {"rootcauseway_acknowledge", "rootcauseway_view_rca", "rootcauseway_resolve"}

        for el in elements:
            value = json.loads(el["value"])
            assert value["incident_id"] == incident_id
            assert value["channel_id"] == channel_id
            assert value["action"] in {"acknowledge", "view_rca", "resolve"}

    @pytest.mark.asyncio
    async def test_send_slack_non_incident_created_has_no_actions_block(self, dispatcher):
        dispatcher._client.post = AsyncMock(return_value=MagicMock(raise_for_status=MagicMock()))

        await dispatcher.send_slack(
            "http://hooks.slack.com/x",
            {"title": "RCA done", "body": "...", "incident_id": str(uuid4())},
            channel_id=str(uuid4()),
            event_type="rca_completed",
        )

        sent = dispatcher._client.post.call_args.kwargs["json"]
        assert all(b["type"] != "actions" for b in sent["blocks"])

    @pytest.mark.asyncio
    async def test_send_slack_without_channel_id_has_no_actions_block(self, dispatcher):
        dispatcher._client.post = AsyncMock(return_value=MagicMock(raise_for_status=MagicMock()))

        await dispatcher.send_slack(
            "http://hooks.slack.com/x",
            {"title": "New Incident", "body": "...", "incident_id": str(uuid4())},
            channel_id=None,
            event_type="incident_created",
        )

        sent = dispatcher._client.post.call_args.kwargs["json"]
        assert all(b["type"] != "actions" for b in sent["blocks"])

    @pytest.mark.asyncio
    async def test_send_teams_new_incident_sends_adaptive_card_with_actions(self, dispatcher):
        dispatcher._client.post = AsyncMock(return_value=MagicMock(raise_for_status=MagicMock()))
        incident_id = str(uuid4())
        channel_id = str(uuid4())

        await dispatcher.send_teams(
            "http://outlook.office.com/x",
            {"title": "New Incident", "body": "...", "incident_id": incident_id},
            channel_id=channel_id,
            event_type="incident_created",
        )

        sent = dispatcher._client.post.call_args.kwargs["json"]
        assert sent["type"] == "message"
        attachment = sent["attachments"][0]
        assert attachment["contentType"] == "application/vnd.microsoft.card.adaptive"

        card = attachment["content"]
        assert card["type"] == "AdaptiveCard"
        actions = card["actions"]
        assert len(actions) == 3
        assert all(a["type"] == "Action.Submit" for a in actions)

        data_by_title = {a["title"]: a["data"] for a in actions}
        assert set(data_by_title.keys()) == {"Acknowledge", "View RCA", "Resolve"}
        for data in data_by_title.values():
            assert data["incident_id"] == incident_id
            assert data["channel_id"] == channel_id

    @pytest.mark.asyncio
    async def test_send_teams_non_incident_created_uses_legacy_messagecard(self, dispatcher):
        dispatcher._client.post = AsyncMock(return_value=MagicMock(raise_for_status=MagicMock()))

        await dispatcher.send_teams(
            "http://outlook.office.com/x",
            {"title": "RCA done", "body": "...", "incident_id": str(uuid4()), "color": "00CC00"},
            channel_id=str(uuid4()),
            event_type="rca_completed",
        )

        sent = dispatcher._client.post.call_args.kwargs["json"]
        assert sent["@type"] == "MessageCard"
        assert "attachments" not in sent

    @pytest.mark.asyncio
    async def test_notify_passes_channel_id_and_event_type_to_slack(self, dispatcher):
        backend = AsyncMock()
        channel_id = str(uuid4())
        backend.list_notification_channels = AsyncMock(return_value=[
            {"id": channel_id, "type": "slack", "webhook_url": "http://hooks.slack.com/x", "enabled": True},
        ])
        backend.list_escalation_policies = AsyncMock(return_value=[])
        backend.create_notification_log = AsyncMock(return_value={})

        with patch.object(dispatcher, "send_slack", new_callable=AsyncMock) as mock_send:
            await dispatcher.notify(
                backend, uuid4(), uuid4(), "incident_created",
                {"severity": "high", "incident_id": "test"},
            )
            mock_send.assert_called_once()
            _, kwargs = mock_send.call_args
            assert kwargs["channel_id"] == channel_id
            assert kwargs["event_type"] == "incident_created"
