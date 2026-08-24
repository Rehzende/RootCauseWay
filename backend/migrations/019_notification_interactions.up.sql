-- Notification Interactions: audit trail for actions taken from inside a
-- Slack message or Teams Adaptive Card (acknowledge / resolve / view_rca),
-- posted back to POST /api/v1/webhooks/{slack,teams}/interactive.
CREATE TABLE notification_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id UUID REFERENCES notification_channels(id) ON DELETE SET NULL,
    channel_type VARCHAR(20) NOT NULL CHECK (channel_type IN ('slack', 'teams')),
    action VARCHAR(30) NOT NULL CHECK (action IN ('acknowledge', 'resolve', 'view_rca')),
    actor VARCHAR(255),
    request_token VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'error')),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_interactions_incident ON notification_interactions(incident_id);
CREATE INDEX idx_notification_interactions_channel ON notification_interactions(channel_id);
CREATE INDEX idx_notification_interactions_org ON notification_interactions(org_id);

-- Best-effort de-dupe: Slack/Teams can retry delivery of the same
-- interaction (e.g. Slack retries on slow HTTP 200s). request_token is the
-- provider's own per-click identifier (Slack action_ts, Teams activity id)
-- when present, so replays land as a unique-violation instead of a second
-- audit row.
CREATE UNIQUE INDEX idx_notification_interactions_dedupe
    ON notification_interactions(channel_id, request_token)
    WHERE request_token IS NOT NULL AND channel_id IS NOT NULL;
