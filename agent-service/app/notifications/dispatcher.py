"""Notification dispatcher: sends notifications through configured channels."""

from __future__ import annotations

import json
import logging
from typing import Any
from uuid import UUID

import httpx

from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)

# Event types that represent a *new* incident. Interactive action buttons
# (Acknowledge / View RCA / Resolve) are only attached to these -- follow-up
# notifications (rca_completed, postmortem_ready, ...) are informational and
# don't need the full action set again.
NEW_INCIDENT_EVENT_TYPES = {"incident_created"}


class NotificationDispatcher:
    """Sends notifications through configured channels."""

    def __init__(self, http_client: httpx.AsyncClient | None = None):
        self._client = http_client or httpx.AsyncClient(timeout=15.0)

    async def close(self) -> None:
        await self._client.aclose()

    async def notify(
        self,
        backend_client: BackendClient,
        org_id: UUID,
        incident_id: UUID,
        event_type: str,
        data: dict[str, Any],
    ) -> None:
        """Send notifications through all matching channels.

        1. Fetch notification channels for org
        2. Fetch escalation policies matching this event/severity
        3. Send to each matching channel
        4. Log each notification attempt
        """
        try:
            channels = await backend_client.list_notification_channels(org_id)
        except Exception:
            logger.warning("Failed to fetch notification channels for org %s", org_id)
            return

        try:
            policies = await backend_client.list_escalation_policies(org_id)
        except Exception:
            logger.warning("Failed to fetch escalation policies for org %s", org_id)
            policies = []

        severity = data.get("severity", "medium")
        matching_channels = self._match_channels(channels, policies, event_type, severity)

        message = self._format_incident_message(data, event_type)

        for channel in matching_channels:
            channel_type = channel.get("type", "webhook")
            channel_url = channel.get("webhook_url") or channel.get("url", "")
            success = False
            error_msg = None

            try:
                if channel_type == "slack":
                    await self.send_slack(
                        channel_url, message,
                        channel_id=channel.get("id"), event_type=event_type,
                    )
                elif channel_type == "teams":
                    await self.send_teams(
                        channel_url, message,
                        channel_id=channel.get("id"), event_type=event_type,
                    )
                elif channel_type == "pagerduty":
                    routing_key = channel.get("routing_key", "")
                    await self.send_pagerduty(routing_key, message)
                else:
                    await self.send_webhook(channel_url, message)
                success = True
            except Exception as exc:
                error_msg = str(exc)
                logger.warning(
                    "Failed to send %s notification to channel %s: %s",
                    channel_type, channel.get("id"), exc,
                )

            # Log notification attempt
            try:
                await backend_client.create_notification_log(
                    org_id=org_id,
                    data={
                        "incident_id": str(incident_id),
                        "channel_id": str(channel.get("id", "")),
                        "channel_type": channel_type,
                        "event_type": event_type,
                        "success": success,
                        "error": error_msg,
                    },
                )
            except Exception:
                logger.warning("Failed to log notification attempt")

    async def send_slack(
        self,
        webhook_url: str,
        payload: dict[str, Any],
        channel_id: Any = None,
        event_type: str | None = None,
    ) -> None:
        """Send Slack notification via webhook.

        For new-incident notifications, attaches a Block Kit `actions`
        block with Acknowledge / View RCA / Resolve buttons. Each button's
        `value` carries {"incident_id", "channel_id", "action"} as JSON --
        clicking it POSTs that value back to
        POST /api/v1/webhooks/slack/interactive, which verifies the
        request's X-Slack-Signature against this channel's configured
        signing secret before dispatching the action.
        """
        blocks: list[dict[str, Any]] = [
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": f"*{payload.get('title', 'Incident Update')}*\n{payload.get('body', '')}",
                },
            },
        ]

        incident_id = payload.get("incident_id")
        if channel_id and incident_id and event_type in NEW_INCIDENT_EVENT_TYPES:
            blocks.append(self._build_slack_actions_block(incident_id, channel_id))

        slack_payload = {
            "text": payload.get("title", "RootCauseway Notification"),
            "blocks": blocks,
        }
        resp = await self._client.post(webhook_url, json=slack_payload)
        resp.raise_for_status()

    def _build_slack_actions_block(self, incident_id: Any, channel_id: Any) -> dict[str, Any]:
        """Build the Block Kit `actions` block with the three interactive
        buttons, each carrying incident_id/channel_id/action as its value.
        """

        def button(text: str, action: str, style: str | None = None) -> dict[str, Any]:
            value = json.dumps({
                "incident_id": str(incident_id),
                "channel_id": str(channel_id),
                "action": action,
            })
            btn: dict[str, Any] = {
                "type": "button",
                "text": {"type": "plain_text", "text": text},
                "action_id": f"rootcauseway_{action}",
                "value": value,
            }
            if style:
                btn["style"] = style
            return btn

        return {
            "type": "actions",
            "elements": [
                button("Acknowledge", "acknowledge", style="primary"),
                button("View RCA", "view_rca"),
                button("Resolve", "resolve", style="danger"),
            ],
        }

    async def send_teams(
        self,
        webhook_url: str,
        payload: dict[str, Any],
        channel_id: Any = None,
        event_type: str | None = None,
    ) -> None:
        """Send Teams notification via webhook.

        For new-incident notifications, sends an Adaptive Card (instead of
        the legacy MessageCard) with `Action.Submit` buttons for
        Acknowledge / View RCA / Resolve. Each action's `data` carries
        {"incident_id", "channel_id", "action"} -- when a bot/workflow
        configured against this channel forwards the resulting activity to
        POST /api/v1/webhooks/teams/interactive, that value is used to
        dispatch the action (see notification_interactive_handlers.go).
        """
        incident_id = payload.get("incident_id")
        if channel_id and incident_id and event_type in NEW_INCIDENT_EVENT_TYPES:
            teams_payload = {
                "type": "message",
                "attachments": [
                    {
                        "contentType": "application/vnd.microsoft.card.adaptive",
                        "content": self._build_teams_adaptive_card(payload, incident_id, channel_id),
                    },
                ],
            }
        else:
            teams_payload = {
                "@type": "MessageCard",
                "summary": payload.get("title", "RootCauseway Notification"),
                "themeColor": payload.get("color", "FF0000"),
                "title": payload.get("title", "Incident Update"),
                "sections": [
                    {
                        "activityTitle": payload.get("title", ""),
                        "text": payload.get("body", ""),
                    },
                ],
            }
        resp = await self._client.post(webhook_url, json=teams_payload)
        resp.raise_for_status()

    def _build_teams_adaptive_card(
        self, payload: dict[str, Any], incident_id: Any, channel_id: Any
    ) -> dict[str, Any]:
        """Build an Adaptive Card with Action.Submit buttons for the three
        interactive actions, mirroring the Slack Block Kit actions block.
        """

        def action(title: str, action_name: str) -> dict[str, Any]:
            return {
                "type": "Action.Submit",
                "title": title,
                "data": {
                    "incident_id": str(incident_id),
                    "channel_id": str(channel_id),
                    "action": action_name,
                },
            }

        return {
            "type": "AdaptiveCard",
            "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
            "version": "1.4",
            "body": [
                {
                    "type": "TextBlock",
                    "text": payload.get("title", "Incident Update"),
                    "weight": "Bolder",
                    "size": "Medium",
                },
                {
                    "type": "TextBlock",
                    "text": payload.get("body", ""),
                    "wrap": True,
                },
            ],
            "actions": [
                action("Acknowledge", "acknowledge"),
                action("View RCA", "view_rca"),
                action("Resolve", "resolve"),
            ],
        }

    async def send_pagerduty(self, routing_key: str, payload: dict[str, Any]) -> None:
        """Trigger PagerDuty incident."""
        pd_payload = {
            "routing_key": routing_key,
            "event_action": "trigger",
            "payload": {
                "summary": payload.get("title", "RootCauseway Incident"),
                "severity": payload.get("severity", "warning"),
                "source": "rootcauseway-agent-service",
                "custom_details": payload,
            },
        }
        resp = await self._client.post(
            "https://events.pagerduty.com/v2/enqueue",
            json=pd_payload,
        )
        resp.raise_for_status()

    async def send_webhook(self, url: str, payload: dict[str, Any]) -> None:
        """Send generic webhook notification."""
        resp = await self._client.post(url, json=payload)
        resp.raise_for_status()

    def _format_incident_message(
        self, data: dict[str, Any], event_type: str
    ) -> dict[str, Any]:
        """Format a human-readable incident notification."""
        titles = {
            "incident_created": "New Incident Created",
            "rca_completed": "Root Cause Analysis Completed",
            "postmortem_ready": "Postmortem Report Ready",
            "runbook_started": "Runbook Execution Started",
            "runbook_completed": "Runbook Execution Completed",
            "escalation": "Incident Escalated",
        }
        title = titles.get(event_type, f"Incident Update: {event_type}")

        severity = data.get("severity", "unknown")
        incident_id = data.get("incident_id", "unknown")

        body_parts = [f"Incident: {incident_id}", f"Severity: {severity}"]
        if data.get("root_cause"):
            body_parts.append(f"Root Cause: {data['root_cause']}")
        if data.get("summary"):
            body_parts.append(f"Summary: {data['summary']}")

        color_map = {"critical": "FF0000", "high": "FF6600", "medium": "FFCC00", "low": "00CC00"}

        return {
            "title": title,
            "body": "\n".join(body_parts),
            "severity": severity,
            "event_type": event_type,
            "incident_id": str(incident_id),
            "color": color_map.get(severity, "CCCCCC"),
        }

    def _match_channels(
        self,
        channels: list[dict[str, Any]],
        policies: list[dict[str, Any]],
        event_type: str,
        severity: str,
    ) -> list[dict[str, Any]]:
        """Filter channels that match escalation policies for this event."""
        if not policies:
            # No policies configured: send to all enabled channels
            return [c for c in channels if c.get("enabled", True)]

        matching_channel_ids = set()
        for policy in policies:
            policy_events = policy.get("event_types", [])
            policy_severities = policy.get("severities", [])

            if policy_events and event_type not in policy_events:
                continue
            if policy_severities and severity not in policy_severities:
                continue

            for cid in policy.get("channel_ids", []):
                matching_channel_ids.add(str(cid))

        if not matching_channel_ids:
            return []

        return [
            c for c in channels
            if str(c.get("id", "")) in matching_channel_ids and c.get("enabled", True)
        ]
