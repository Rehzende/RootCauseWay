package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// PgNotificationInteractionRepository persists the audit trail for actions
// taken from inside a Slack message or Teams Adaptive Card (see
// backend/migrations/019_notification_interactions.up.sql).
type PgNotificationInteractionRepository struct{ pool *pgxpool.Pool }

func NewNotificationInteractionRepository(pool *pgxpool.Pool) *PgNotificationInteractionRepository {
	return &PgNotificationInteractionRepository{pool: pool}
}

// Create inserts a new interaction record. A duplicate (channel_id,
// request_token) pair -- e.g. Slack retrying delivery of the same button
// click -- violates the partial unique index and returns an error; callers
// should treat that as "already recorded" rather than a hard failure.
func (r *PgNotificationInteractionRepository) Create(ctx context.Context, ni *models.NotificationInteraction) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notification_interactions (id, org_id, incident_id, channel_id, channel_type, action, actor, request_token, status, error_message, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		ni.ID, ni.OrgID, ni.IncidentID, ni.ChannelID, ni.ChannelType, ni.Action, ni.Actor, nullableString(ni.RequestToken), ni.Status, ni.ErrorMessage, ni.CreatedAt)
	return err
}

// ListByIncident returns interactions for an incident, most recent first.
func (r *PgNotificationInteractionRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.NotificationInteraction, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, incident_id, channel_id, channel_type, action, COALESCE(actor,''), COALESCE(request_token,''), status, COALESCE(error_message,''), created_at
		 FROM notification_interactions WHERE incident_id=$1 ORDER BY created_at DESC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.NotificationInteraction
	for rows.Next() {
		var ni models.NotificationInteraction
		if err := rows.Scan(&ni.ID, &ni.OrgID, &ni.IncidentID, &ni.ChannelID, &ni.ChannelType, &ni.Action, &ni.Actor, &ni.RequestToken, &ni.Status, &ni.ErrorMessage, &ni.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, ni)
	}
	if items == nil {
		items = []models.NotificationInteraction{}
	}
	return items, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
