package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WarRoomMeeting mirrors the war_room_meetings table. A war room is a
// Microsoft Teams meeting spun up directly from an incident so on-call
// responders can collaborate; once it ends, RootCauseway captures the transcript,
// attendance report, and an LLM-generated summary.
type WarRoomMeeting struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	IncidentID        uuid.UUID       `json:"incident_id"`
	Provider          string          `json:"provider"`
	ExternalMeetingID string          `json:"external_meeting_id"`
	JoinURL           string          `json:"join_url"`
	Status            string          `json:"status"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	EndedAt           *time.Time      `json:"ended_at,omitempty"`
	RawTranscript     *string         `json:"raw_transcript,omitempty"`
	Attendance        json.RawMessage `json:"attendance,omitempty"`
	Summary           json.RawMessage `json:"summary,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// WarRoomAttendee is one entry of the WarRoomMeeting.Attendance JSON array.
type WarRoomAttendee struct {
	Name      string     `json:"name,omitempty"`
	Email     string     `json:"email,omitempty"`
	JoinTime  *time.Time `json:"join_time,omitempty"`
	LeaveTime *time.Time `json:"leave_time,omitempty"`
}

// WarRoomSummary is the shape stored in WarRoomMeeting.Summary, produced by
// the agent-service LLM summarizer from the raw transcript.
type WarRoomSummary struct {
	ExecutiveSummary string              `json:"executive_summary"`
	KeyPoints        []string            `json:"key_points"`
	ActionItems      []WarRoomActionItem `json:"action_items"`
}

// WarRoomActionItem is a single follow-up action extracted from the
// meeting transcript.
type WarRoomActionItem struct {
	Description string `json:"description"`
	OwnerHint   string `json:"owner_hint,omitempty"`
}

// CreateWarRoomRequest is currently empty: the meeting subject is derived
// from the incident. Kept as a named type so the handler contract is
// stable if fields (e.g. custom subject) are added later.
type CreateWarRoomRequest struct {
	Subject string `json:"subject,omitempty"`
}

// AttachWarRoomSummaryRequest is the internal request body agent-service
// posts back once it has summarized the transcript.
type AttachWarRoomSummaryRequest struct {
	Summary      WarRoomSummary    `json:"summary" binding:"required"`
	Participants []WarRoomAttendee `json:"participants"`
}

// WarRoomMeetingEndedPayload is published on the warroom.meeting.ended
// event when a war room is manually (or, in the future, webhook-)
// marked ended.
type WarRoomMeetingEndedPayload struct {
	MeetingID         uuid.UUID `json:"meeting_id"`
	IncidentID        uuid.UUID `json:"incident_id"`
	ExternalMeetingID string    `json:"external_meeting_id"`
}
