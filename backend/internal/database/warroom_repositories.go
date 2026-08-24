package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- War Room Meeting Repository ---

type PgWarRoomRepository struct{ pool *pgxpool.Pool }

func NewWarRoomRepository(pool *pgxpool.Pool) *PgWarRoomRepository {
	return &PgWarRoomRepository{pool: pool}
}

func (r *PgWarRoomRepository) Create(ctx context.Context, m *models.WarRoomMeeting) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO war_room_meetings (id, org_id, incident_id, provider, external_meeting_id, join_url, status, started_at, ended_at, raw_transcript, attendance, summary, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		m.ID, m.OrgID, m.IncidentID, m.Provider, m.ExternalMeetingID, m.JoinURL, m.Status, m.StartedAt, m.EndedAt, m.RawTranscript, m.Attendance, m.Summary, m.CreatedAt, m.UpdatedAt)
	return err
}

func (r *PgWarRoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.WarRoomMeeting, error) {
	var m models.WarRoomMeeting
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, incident_id, provider, external_meeting_id, join_url, status, started_at, ended_at, raw_transcript, attendance, summary, created_at, updated_at
		 FROM war_room_meetings WHERE id=$1`, id).
		Scan(&m.ID, &m.OrgID, &m.IncidentID, &m.Provider, &m.ExternalMeetingID, &m.JoinURL, &m.Status, &m.StartedAt, &m.EndedAt, &m.RawTranscript, &m.Attendance, &m.Summary, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetLatestByIncident returns the most recently created war room meeting
// for an incident (there may be several across the incident's lifetime).
func (r *PgWarRoomRepository) GetLatestByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	var m models.WarRoomMeeting
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, incident_id, provider, external_meeting_id, join_url, status, started_at, ended_at, raw_transcript, attendance, summary, created_at, updated_at
		 FROM war_room_meetings WHERE incident_id=$1 ORDER BY created_at DESC LIMIT 1`, incidentID).
		Scan(&m.ID, &m.OrgID, &m.IncidentID, &m.Provider, &m.ExternalMeetingID, &m.JoinURL, &m.Status, &m.StartedAt, &m.EndedAt, &m.RawTranscript, &m.Attendance, &m.Summary, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgWarRoomRepository) Update(ctx context.Context, m *models.WarRoomMeeting) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE war_room_meetings SET status=$1, started_at=$2, ended_at=$3, raw_transcript=$4, attendance=$5, summary=$6, updated_at=$7 WHERE id=$8`,
		m.Status, m.StartedAt, m.EndedAt, m.RawTranscript, m.Attendance, m.Summary, m.UpdatedAt, m.ID)
	return err
}
