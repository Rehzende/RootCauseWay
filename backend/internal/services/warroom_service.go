package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// WarRoomRepository defines the DB operations for war room meetings.
type WarRoomRepository interface {
	Create(ctx context.Context, m *models.WarRoomMeeting) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.WarRoomMeeting, error)
	GetLatestByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error)
	Update(ctx context.Context, m *models.WarRoomMeeting) error
}

// WarRoomIncidentReader is the minimal incident-read surface the war room
// service needs (org id + title, for scoping and the meeting subject).
// Kept narrow so this package doesn't need the full IncidentServiceInterface.
type WarRoomIncidentReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error)
}

// WarRoomIncidentEventAdder lets the war room service add a best-effort
// timeline event to the incident without depending on the full
// IncidentService. Optional: when nil, timeline events are skipped.
type WarRoomIncidentEventAdder interface {
	AddEvent(ctx context.Context, incidentID uuid.UUID, actor string, req models.CreateEventRequest) (*models.IncidentEvent, error)
}

// TeamsClientResolver builds a teams.TeamsClient for a given org, based on
// that org's own configured Teams integration settings -- see
// NewTeamsClientResolver, which is what production wires in. Replaces a
// single client fixed at process boot (previously teams.NewClientFromEnv(),
// one Azure tenant for the whole deployment) with a per-org, per-call
// resolution, so changing an org's Teams credentials via the Integrations
// settings UI takes effect immediately, no backend redeploy needed.
type TeamsClientResolver func(ctx context.Context, orgID uuid.UUID) (teams.TeamsClient, error)

// WarRoomService owns the war room meeting lifecycle: creating Teams
// meetings, persisting meeting state, and (on manual end) fetching the
// transcript/attendance report and publishing warroom.meeting.ended for
// agent-service to summarize.
type WarRoomService struct {
	repo         WarRoomRepository
	resolveTeams TeamsClientResolver
	incidents    WarRoomIncidentReader
	events       WarRoomIncidentEventAdder // optional
	publisher    EventPublisher            // optional
}

func NewWarRoomService(repo WarRoomRepository, resolveTeams TeamsClientResolver, incidents WarRoomIncidentReader) *WarRoomService {
	return &WarRoomService{repo: repo, resolveTeams: resolveTeams, incidents: incidents}
}

// SetIncidentEventAdder wires an optional timeline-event sink (typically
// *IncidentService). When unset, war room lifecycle events simply aren't
// added to the incident timeline.
func (s *WarRoomService) SetIncidentEventAdder(a WarRoomIncidentEventAdder) {
	s.events = a
}

// SetEventPublisher wires the optional Redis event publisher used to
// announce warroom.meeting.ended.
func (s *WarRoomService) SetEventPublisher(p EventPublisher) {
	s.publisher = p
}

// CreateWarRoom creates a Teams meeting for the given incident and
// persists it as a new war_room_meetings row.
func (s *WarRoomService) CreateWarRoom(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	incident, err := s.incidents.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}

	client, err := s.resolveTeams(ctx, incident.OrgID)
	if err != nil {
		return nil, fmt.Errorf("resolve teams client: %w", err)
	}

	subject := fmt.Sprintf("War Room: %s", incident.Title)
	externalID, joinURL, err := client.CreateMeeting(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("create teams meeting: %w", err)
	}

	now := time.Now()
	meeting := &models.WarRoomMeeting{
		ID:                uuid.New(),
		OrgID:             incident.OrgID,
		IncidentID:        incidentID,
		Provider:          "teams",
		ExternalMeetingID: externalID,
		JoinURL:           joinURL,
		Status:            "scheduled",
		StartedAt:         &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.Create(ctx, meeting); err != nil {
		return nil, fmt.Errorf("persist war room meeting: %w", err)
	}

	if s.events != nil {
		data, _ := json.Marshal(map[string]string{
			"meeting_id": meeting.ID.String(),
			"join_url":   joinURL,
		})
		if _, err := s.events.AddEvent(ctx, incidentID, "system", models.CreateEventRequest{
			Type: "war_room_created",
			Data: data,
		}); err != nil {
			slog.Warn("failed to add war_room_created timeline event", "incident_id", incidentID, "error", err)
		}
	}

	return meeting, nil
}

// GetByIncident returns the most recent war room meeting for an incident.
func (s *WarRoomService) GetByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	return s.repo.GetLatestByIncident(ctx, incidentID)
}

// GetByID returns a war room meeting by its own ID. Used by the internal
// endpoint agent-service polls to fetch the raw transcript for summarization.
func (s *WarRoomService) GetByID(ctx context.Context, meetingID uuid.UUID) (*models.WarRoomMeeting, error) {
	return s.repo.GetByID(ctx, meetingID)
}

// EndWarRoom marks a meeting ended, best-effort fetches its transcript
// and attendance report from the Teams provider, persists them, and
// publishes warroom.meeting.ended so agent-service can summarize it.
//
// v1 limitation: there is no real Graph subscription webhook receiver in
// this environment, so meeting-end detection is triggered manually via
// this method (invoked from POST /warroom/:meetingId/end). The method is
// intentionally the single entry point for "meeting ended" handling so a
// future Graph change-notification webhook can call it directly once a
// real subscription is configured against a real tenant.
func (s *WarRoomService) EndWarRoom(ctx context.Context, meetingID uuid.UUID) (*models.WarRoomMeeting, error) {
	meeting, err := s.repo.GetByID(ctx, meetingID)
	if err != nil {
		return nil, fmt.Errorf("get war room meeting: %w", err)
	}

	now := time.Now()
	meeting.Status = "ended"
	meeting.EndedAt = &now
	meeting.UpdatedAt = now

	client, err := s.resolveTeams(ctx, meeting.OrgID)
	if err != nil {
		// Best-effort, same as the transcript/attendance fetches below:
		// the meeting still gets marked ended either way, just without
		// transcript/attendance data attached.
		slog.Warn("failed to resolve teams client for war room end", "meeting_id", meetingID, "error", err)
	} else {
		if transcript, err := client.GetTranscript(ctx, meeting.ExternalMeetingID); err != nil {
			slog.Warn("failed to fetch war room transcript", "meeting_id", meetingID, "error", err)
		} else {
			meeting.RawTranscript = &transcript
		}

		if attendees, err := client.GetAttendanceReport(ctx, meeting.ExternalMeetingID); err != nil {
			slog.Warn("failed to fetch war room attendance report", "meeting_id", meetingID, "error", err)
		} else if data, err := json.Marshal(attendees); err == nil {
			meeting.Attendance = data
		}
	}

	if err := s.repo.Update(ctx, meeting); err != nil {
		return nil, fmt.Errorf("update war room meeting: %w", err)
	}

	if s.publisher != nil {
		envelope := models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "warroom.meeting.ended",
			OrgID:     meeting.OrgID,
			Timestamp: now,
			Payload: models.WarRoomMeetingEndedPayload{
				MeetingID:         meeting.ID,
				IncidentID:        meeting.IncidentID,
				ExternalMeetingID: meeting.ExternalMeetingID,
			},
		}
		channel := fmt.Sprintf("rootcauseway:%s:warroom.meeting.ended", meeting.OrgID.String())
		if err := s.publisher.Publish(ctx, channel, envelope); err != nil {
			slog.Error("failed to publish warroom.meeting.ended event", "meeting_id", meetingID, "error", err)
		}
	}

	return meeting, nil
}

// AttachSummary is called by agent-service (via the internal endpoint)
// once it has summarized the transcript, writing back the structured
// summary and the participant list.
func (s *WarRoomService) AttachSummary(ctx context.Context, meetingID uuid.UUID, summary models.WarRoomSummary, participants []models.WarRoomAttendee) (*models.WarRoomMeeting, error) {
	meeting, err := s.repo.GetByID(ctx, meetingID)
	if err != nil {
		return nil, fmt.Errorf("get war room meeting: %w", err)
	}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return nil, fmt.Errorf("marshal summary: %w", err)
	}
	meeting.Summary = summaryJSON

	if len(participants) > 0 {
		if data, err := json.Marshal(participants); err == nil {
			meeting.Attendance = data
		}
	}

	meeting.Status = "summarized"
	meeting.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, meeting); err != nil {
		return nil, fmt.Errorf("update war room meeting: %w", err)
	}
	return meeting, nil
}
