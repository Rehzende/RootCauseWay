-- War Room: Microsoft Teams meetings spun up from an incident, with
-- post-meeting transcript/attendance capture and LLM summarization.
CREATE TABLE war_room_meetings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    provider VARCHAR(20) NOT NULL DEFAULT 'teams' CHECK (provider IN ('teams')),
    external_meeting_id VARCHAR(500) NOT NULL,
    join_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'active', 'ended', 'summarized')),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    raw_transcript TEXT,
    attendance JSONB,
    summary JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_war_room_meetings_org ON war_room_meetings(org_id);
CREATE INDEX idx_war_room_meetings_incident ON war_room_meetings(incident_id);
CREATE INDEX idx_war_room_meetings_status ON war_room_meetings(status);
