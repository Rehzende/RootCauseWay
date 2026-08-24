//go:build integration

package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

func createTestIncidentForWarRoom(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	swID := createTestSoftware(t, pool, orgID)
	incidentID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO incidents (id, org_id, software_id, title, severity, status) VALUES ($1,$2,$3,$4,$5,$6)`,
		incidentID, orgID, swID, "Test incident for war room", "high", "open")
	if err != nil {
		t.Fatalf("failed to create test incident: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM incidents WHERE id=$1`, incidentID)
	})
	return incidentID
}

func TestWarRoomRepository_CreateGetUpdate(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	incidentID := createTestIncidentForWarRoom(t, pool, orgID)

	repo := NewWarRoomRepository(pool)

	now := time.Now().UTC().Truncate(time.Millisecond)
	meeting := &models.WarRoomMeeting{
		ID:                uuid.New(),
		OrgID:             orgID,
		IncidentID:        incidentID,
		Provider:          "teams",
		ExternalMeetingID: "mock-meeting-1",
		JoinURL:           "https://teams.microsoft.com/l/meetup-join/mock-1",
		Status:            "scheduled",
		StartedAt:         &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := repo.Create(ctx, meeting); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM war_room_meetings WHERE id=$1`, meeting.ID)
	})

	got, err := repo.GetByID(ctx, meeting.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got.Status != "scheduled" || got.ExternalMeetingID != "mock-meeting-1" {
		t.Errorf("unexpected meeting: %+v", got)
	}

	latest, err := repo.GetLatestByIncident(ctx, incidentID)
	if err != nil {
		t.Fatalf("GetLatestByIncident error: %v", err)
	}
	if latest.ID != meeting.ID {
		t.Errorf("expected latest meeting %s, got %s", meeting.ID, latest.ID)
	}

	transcript := "Alice: let's investigate the outage."
	attendance, _ := json.Marshal([]models.WarRoomAttendee{{Name: "Alice", Email: "alice@example.com"}})
	summary, _ := json.Marshal(models.WarRoomSummary{ExecutiveSummary: "Outage resolved."})
	endedAt := time.Now().UTC().Truncate(time.Millisecond)

	got.Status = "summarized"
	got.EndedAt = &endedAt
	got.RawTranscript = &transcript
	got.Attendance = attendance
	got.Summary = summary
	got.UpdatedAt = endedAt

	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	updated, err := repo.GetByID(ctx, meeting.ID)
	if err != nil {
		t.Fatalf("GetByID after update error: %v", err)
	}
	if updated.Status != "summarized" {
		t.Errorf("expected status summarized, got %s", updated.Status)
	}
	if updated.RawTranscript == nil || *updated.RawTranscript != transcript {
		t.Errorf("transcript not persisted correctly: %+v", updated.RawTranscript)
	}
	if updated.EndedAt == nil {
		t.Errorf("expected ended_at to be set")
	}
	var gotSummary models.WarRoomSummary
	if err := json.Unmarshal(updated.Summary, &gotSummary); err != nil {
		t.Fatalf("failed to unmarshal persisted summary: %v", err)
	}
	if gotSummary.ExecutiveSummary != "Outage resolved." {
		t.Errorf("unexpected summary: %+v", gotSummary)
	}
}
