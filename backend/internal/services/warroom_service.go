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

// WarRoomSoftwareReader is the minimal software-catalog read surface used
// to invite the affected software's stakeholders/SRE team to the war room
// meeting. Optional: when nil, meetings are created with no invited
// attendees (organizer only), same as before this existed.
type WarRoomSoftwareReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error)
}

// person mirrors the frontend's Person shape (name/email, at minimum) that
// SoftwareEntry.Stakeholders/SreTeam are stored as -- see SoftwarePage.tsx's
// PersonListEditor. Unmarshaled locally rather than in models.SoftwareEntry
// itself, which deliberately keeps those two fields as raw JSON (the
// software catalog API passes them through opaquely; this is the first
// caller that needs to actually read them structured).
type person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
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
	software     WarRoomSoftwareReader     // optional
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

// SetSoftwareReader wires the optional software-catalog reader used to
// invite the affected software's stakeholders/SRE team to the meeting.
// When unset, meetings are created with no invited attendees.
func (s *WarRoomService) SetSoftwareReader(r WarRoomSoftwareReader) {
	s.software = r
}

// meetingAttendees resolves the affected software's stakeholders + SRE team
// into a deduplicated attendee list for the Teams invite. Best-effort: a
// software lookup failure or a software with no stakeholders/sre_team set
// just means no attendees get invited, never a reason to fail meeting
// creation itself.
func (s *WarRoomService) meetingAttendees(ctx context.Context, softwareID uuid.UUID) []teams.Attendee {
	if s.software == nil {
		return nil
	}
	sw, err := s.software.GetByID(ctx, softwareID)
	if err != nil {
		slog.Warn("failed to load software for war room attendee list", "software_id", softwareID, "error", err)
		return nil
	}

	seen := make(map[string]bool)
	var attendees []teams.Attendee
	for _, raw := range [][]byte{sw.Stakeholders, sw.SreTeam} {
		if len(raw) == 0 {
			continue
		}
		var people []person
		if err := json.Unmarshal(raw, &people); err != nil {
			continue // malformed/legacy shape -- skip rather than fail the whole meeting
		}
		for _, p := range people {
			if p.Email == "" || seen[p.Email] {
				continue
			}
			seen[p.Email] = true
			attendees = append(attendees, teams.Attendee{Name: p.Name, Email: p.Email})
		}
	}
	return attendees
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
	attendees := s.meetingAttendees(ctx, incident.SoftwareID)
	externalID, joinURL, err := client.CreateMeeting(ctx, subject, attendees)
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

	// Publishes warroom.meeting.created so agent-service's WarRoomConsumer
	// can notify configured channels (Slack/Teams/webhook/PagerDuty) with
	// the join link. Before this, the link only ever reached whoever opened
	// the incident in RootCauseWay itself -- nobody got paged/pinged.
	if s.publisher != nil {
		envelope := models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "warroom.meeting.created",
			OrgID:     incident.OrgID,
			Timestamp: now,
			Payload: models.WarRoomMeetingCreatedPayload{
				MeetingID:  meeting.ID,
				IncidentID: incidentID,
				JoinURL:    joinURL,
				Severity:   incident.Severity,
			},
		}
		channel := fmt.Sprintf("rootcauseway:%s:warroom.meeting.created", incident.OrgID.String())
		if err := s.publisher.Publish(ctx, channel, envelope); err != nil {
			slog.Error("failed to publish warroom.meeting.created event", "meeting_id", meeting.ID, "error", err)
		}
	}

	return meeting, nil
}

// GetByIncident returns the most recent war room meeting for an incident.
func (s *WarRoomService) GetByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	meeting, err := s.repo.GetLatestByIncident(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	if meeting != nil && meeting.Status == "scheduled" {
		s.maybeMarkActive(ctx, meeting)
	}
	return meeting, nil
}

// maybeMarkActive checks, best-effort, whether anyone has actually joined a
// still-"scheduled" meeting yet, flipping it to "active" the first time
// they have -- a real, if approximate, signal instead of the always-false
// decorative "active" WarRoomStatus that existed on the frontend with
// nothing on the backend ever setting it.
//
// Deliberately cheap: piggybacks on GetByIncident, which the frontend's
// useWarRoom hook already polls every 10s while a meeting isn't
// "summarized" -- no dedicated background poller, and nowhere near the
// Calls API integration a genuine live roster ("who's in the call right
// now") would need (separate, much bigger project -- see backlog). One-way
// only: never un-marks a meeting active again.
//
// Caveat, not yet confirmed against a real ongoing meeting: Microsoft's
// attendanceReports endpoint (GraphTeamsClient.GetAttendanceReport) is
// documented primarily as a post-meeting artifact; whether it returns
// partial data while a meeting is still genuinely in progress varies and
// hasn't been live-tested here. Worst case if it never does is exactly
// today's behavior (status stays "scheduled" until EndWarRoom marks it
// "ended") -- this is a pure incremental improvement, not a regression
// risk, but verify it against a real live meeting before relying on it.
func (s *WarRoomService) maybeMarkActive(ctx context.Context, meeting *models.WarRoomMeeting) {
	client, err := s.resolveTeams(ctx, meeting.OrgID)
	if err != nil {
		return
	}
	attendees, err := client.GetAttendanceReport(ctx, meeting.ExternalMeetingID)
	if err != nil || len(attendees) == 0 {
		return
	}

	meeting.Status = "active"
	meeting.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, meeting); err != nil {
		slog.Warn("failed to mark war room active", "meeting_id", meeting.ID, "error", err)
		return
	}
	slog.Info("war room meeting marked active", "meeting_id", meeting.ID, "attendee_count", len(attendees))
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
