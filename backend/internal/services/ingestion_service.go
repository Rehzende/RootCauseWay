package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/webhooks"
)

type EventPublisher interface {
	Publish(ctx context.Context, channel string, event models.EventEnvelope) error
}

type QuarantineRepository interface {
	Create(ctx context.Context, q *models.AlertQuarantine) error
}

type IngestionService struct {
	webhookRepo    WebhookRepository
	incidentRepo   IncidentRepository
	snapshotRepo   AlertSnapshotRepository
	publisher      EventPublisher
	softwareRepo   SoftwareRepository
	quarantineRepo QuarantineRepository
}

func NewIngestionService(
	webhookRepo WebhookRepository,
	incidentRepo IncidentRepository,
	snapshotRepo AlertSnapshotRepository,
	publisher EventPublisher,
) *IngestionService {
	return &IngestionService{
		webhookRepo:  webhookRepo,
		incidentRepo: incidentRepo,
		snapshotRepo: snapshotRepo,
		publisher:    publisher,
	}
}

func (s *IngestionService) SetSoftwareRepo(repo SoftwareRepository) {
	s.softwareRepo = repo
}

func (s *IngestionService) SetQuarantineRepo(repo QuarantineRepository) {
	s.quarantineRepo = repo
}

type IngestionResult struct {
	IncidentID      uuid.UUID  `json:"incident_id"`
	AlertSnapshotID uuid.UUID  `json:"alert_snapshot_id"`
	Quarantined     bool       `json:"quarantined,omitempty"`
	QuarantineID    *uuid.UUID `json:"quarantine_id,omitempty"`
}

// validateWebhookToken checks that the token has a reasonable format.
func validateWebhookToken(token string) error {
	if len(token) < 8 || len(token) > 256 {
		return fmt.Errorf("invalid webhook token format")
	}
	return nil
}

const maxIngestionPayloadBytes = 1 << 20 // 1MB

func (s *IngestionService) IngestAlert(ctx context.Context, token string, rawPayload json.RawMessage) (*IngestionResult, error) {
	// 0. Validate token format
	if err := validateWebhookToken(token); err != nil {
		slog.Warn("ingestion rejected: invalid token format", "token_prefix", token[:min(8, len(token))])
		return nil, err
	}

	// 0b. Validate payload size
	if len(rawPayload) > maxIngestionPayloadBytes {
		slog.Warn("ingestion rejected: payload too large", "size", len(rawPayload))
		return nil, fmt.Errorf("payload too large: %d bytes exceeds maximum of %d", len(rawPayload), maxIngestionPayloadBytes)
	}

	// 1. Find webhook by token
	webhook, err := s.webhookRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("webhook not found: %w", err)
	}

	if !webhook.Enabled {
		return nil, fmt.Errorf("webhook is disabled")
	}

	// 2. Normalize the alert
	normalizer, err := webhooks.GetNormalizer(webhook.Source)
	if err != nil {
		return nil, fmt.Errorf("get normalizer: %w", err)
	}

	normalized, err := normalizer.Normalize(rawPayload)
	if err != nil {
		return nil, fmt.Errorf("normalize alert: %w", err)
	}

	// 2b. Auto-link software by alert labels (service, namespace, rootcauseway.io/software)
	var softwareID uuid.UUID
	softwareMatched := false

	// Try webhook's software_id first (if set)
	if webhook.SoftwareID != uuid.Nil {
		softwareID = webhook.SoftwareID
		softwareMatched = true
	}

	// Override with label-based lookup
	if s.softwareRepo != nil {
		label := normalized.Service
		if v, ok := normalized.Labels["rootcauseway.io/software"]; ok && v != "" {
			label = v
		}
		if label != "" {
			if sw, err := s.softwareRepo.FindBySlugOrTag(ctx, webhook.OrgID, label); err == nil {
				slog.Info("auto-linked alert to software by label", "label", label, "software_id", sw.ID, "software_name", sw.Name)
				softwareID = sw.ID
				softwareMatched = true
			}
		}
	}

	// 2c. If no software match, quarantine the alert
	if !softwareMatched {
		if s.quarantineRepo != nil {
			labelsJSON, _ := json.Marshal(normalized.Labels)
			qID := uuid.New()
			q := &models.AlertQuarantine{
				ID:                 qID,
				OrgID:              webhook.OrgID,
				WebhookID:          webhook.ID,
				Source:             webhook.Source,
				RawPayload:         rawPayload,
				NormalizedTitle:    normalized.Title,
				NormalizedSeverity: normalized.Severity,
				Labels:             labelsJSON,
				Reason:             "no_software_match",
				CreatedAt:          time.Now(),
			}
			if err := s.quarantineRepo.Create(ctx, q); err != nil {
				slog.Error("failed to quarantine alert", "error", err)
			} else {
				slog.Warn("alert quarantined: no software match", "quarantine_id", qID, "title", normalized.Title, "labels", normalized.Labels)
			}
			return &IngestionResult{Quarantined: true, QuarantineID: &qID}, nil
		}
		slog.Warn("alert has no software match and no quarantine repo, dropping", "title", normalized.Title)
		return nil, fmt.Errorf("no software match for alert and quarantine not configured")
	}

	// 3. Create incident
	incident := &models.Incident{
		ID:          uuid.New(),
		OrgID:       webhook.OrgID,
		SoftwareID:  softwareID,
		Title:       normalized.Title,
		Description: normalized.Description,
		Severity:    normalized.Severity,
		Status:      "open",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.incidentRepo.Create(ctx, incident); err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}

	// 4. Create alert snapshot
	snapshot := &models.AlertSnapshot{
		ID:         uuid.New(),
		IncidentID: incident.ID,
		SoftwareID: softwareID,
		RawPayload: rawPayload,
		Normalized: *normalized,
		Snapshots:  json.RawMessage("{}"),
		CreatedAt:  time.Now(),
	}

	if err := s.snapshotRepo.Create(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("create alert snapshot: %w", err)
	}

	// 5. Publish Redis event
	channel := fmt.Sprintf("rootcauseway:%s:alert.received", webhook.OrgID.String())
	envelope := models.EventEnvelope{
		EventID:   uuid.New(),
		EventType: "alert.received",
		OrgID:     webhook.OrgID,
		Timestamp: time.Now(),
		Payload: models.AlertReceivedPayload{
			AlertSnapshotID: snapshot.ID,
			IncidentID:      incident.ID,
			SoftwareID:      softwareID,
			WebhookSource:   webhook.Source,
			NormalizedAlert: *normalized,
		},
	}

	// Non-blocking publish - log error but don't fail ingestion
	if err := s.publisher.Publish(ctx, channel, envelope); err != nil {
		slog.Error("failed to publish ingestion event", "error", err, "incident_id", incident.ID)
	}

	// incident.created -- separate from alert.received above (that one is
	// agent-service's trigger to run the pipeline; this one is purely for
	// the WebSocket bridge -> frontend's live "new incident" toast, which
	// found nothing publishing this event type at all until now).
	createdChannel := fmt.Sprintf("rootcauseway:%s:incident.created", webhook.OrgID.String())
	createdEnvelope := models.EventEnvelope{
		EventID:   uuid.New(),
		EventType: "incident.created",
		OrgID:     webhook.OrgID,
		Timestamp: time.Now(),
		Payload: models.IncidentCreatedPayload{
			IncidentID: incident.ID,
			Title:      incident.Title,
			Severity:   incident.Severity,
			Status:     incident.Status,
			SoftwareID: softwareID,
		},
	}
	if err := s.publisher.Publish(ctx, createdChannel, createdEnvelope); err != nil {
		slog.Error("failed to publish incident.created event", "error", err, "incident_id", incident.ID)
	}

	slog.Info("alert ingested successfully",
		"incident_id", incident.ID,
		"snapshot_id", snapshot.ID,
		"org_id", webhook.OrgID,
		"source", webhook.Source,
	)

	return &IngestionResult{
		IncidentID:      incident.ID,
		AlertSnapshotID: snapshot.ID,
	}, nil
}
