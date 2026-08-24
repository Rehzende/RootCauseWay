package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Actions supported by the inbound Slack/Teams interactive endpoints.
const (
	InteractionAcknowledge = "acknowledge"
	InteractionResolve     = "resolve"
	InteractionViewRCA     = "view_rca"
)

// ErrUnknownInteractionAction is returned by Dispatch when the button
// "value" payload names an action RootCauseway doesn't recognize.
var ErrUnknownInteractionAction = errors.New("unknown notification interaction action")

// NotificationInteractionIncidentReader is the minimal incident-read
// surface this service needs. Kept narrow (rather than depending on the
// full IncidentServiceInterface from the handlers package) so this package
// has no import-cycle risk and stays read-only where the task requires it.
type NotificationInteractionIncidentReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error)
}

// NotificationInteractionIncidentUpdater is the minimal incident-write
// surface this service needs: only Update (status transitions), never
// Create/Delete. *services.IncidentService already satisfies this.
type NotificationInteractionIncidentUpdater interface {
	Update(ctx context.Context, id uuid.UUID, req models.UpdateIncidentRequest) (*models.Incident, bool, error)
}

// NotificationInteractionRCAReader is the minimal RCA-read surface needed
// for the "View RCA" action. *services.RCAService already satisfies this.
type NotificationInteractionRCAReader interface {
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error)
}

// NotificationChannelReader looks up a notification channel by ID so the
// handler layer can pull the per-channel Slack signing secret / Teams
// verification token out of its Config JSON.
type NotificationChannelReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationChannel, error)
}

// NotificationInteractionRecorder persists the audit trail row for an
// inbound interaction. Errors from Create (including the dedupe unique
// violation) are logged, not propagated -- recording the interaction is
// best-effort and must never block the underlying incident action.
type NotificationInteractionRecorder interface {
	Create(ctx context.Context, interaction *models.NotificationInteraction) error
}

// ActionResult is what an interactive callback handler needs to render a
// reply message (Slack ephemeral response / Teams Adaptive Card response).
type ActionResult struct {
	Incident *models.Incident
	RCA      *models.IncidentRCA // populated only for InteractionViewRCA
	Message  string
}

// NotificationInteractionService dispatches the three inbound actions
// (acknowledge / resolve / view_rca) that a Slack button or Teams
// Action.Submit can trigger, and records each attempt for audit purposes.
type NotificationInteractionService struct {
	incidentReader  NotificationInteractionIncidentReader
	incidentUpdater NotificationInteractionIncidentUpdater
	rca             NotificationInteractionRCAReader
	channels        NotificationChannelReader
	interactions    NotificationInteractionRecorder
}

func NewNotificationInteractionService(
	incidentReader NotificationInteractionIncidentReader,
	incidentUpdater NotificationInteractionIncidentUpdater,
	rca NotificationInteractionRCAReader,
	channels NotificationChannelReader,
	interactions NotificationInteractionRecorder,
) *NotificationInteractionService {
	return &NotificationInteractionService{
		incidentReader:  incidentReader,
		incidentUpdater: incidentUpdater,
		rca:             rca,
		channels:        channels,
		interactions:    interactions,
	}
}

// SlackSigningSecret resolves the signing secret configured on a slack
// notification channel, used to verify X-Slack-Signature. Returns an error
// if the channel can't be found or isn't a slack channel.
func (s *NotificationInteractionService) SlackSigningSecret(ctx context.Context, channelID uuid.UUID) (string, error) {
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return "", fmt.Errorf("get notification channel: %w", err)
	}
	if ch.ChannelType != "slack" {
		return "", fmt.Errorf("notification channel %s is not a slack channel", channelID)
	}
	var cfg models.SlackChannelConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return "", fmt.Errorf("parse slack channel config: %w", err)
	}
	return cfg.SigningSecret, nil
}

// TeamsVerificationToken resolves the shared verification token configured
// on a teams notification channel, used to authenticate inbound
// Action.Submit callbacks.
func (s *NotificationInteractionService) TeamsVerificationToken(ctx context.Context, channelID uuid.UUID) (string, error) {
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return "", fmt.Errorf("get notification channel: %w", err)
	}
	if ch.ChannelType != "teams" {
		return "", fmt.Errorf("notification channel %s is not a teams channel", channelID)
	}
	var cfg models.TeamsChannelConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return "", fmt.Errorf("parse teams channel config: %w", err)
	}
	return cfg.VerificationToken, nil
}

// GetChannelOrgID returns the org a channel belongs to, used by handlers to
// scope the audit record when the incident itself can't be resolved.
func (s *NotificationInteractionService) GetChannelOrgID(ctx context.Context, channelID uuid.UUID) (uuid.UUID, error) {
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return uuid.Nil, err
	}
	return ch.OrgID, nil
}

// Dispatch runs the requested action against the incident and records the
// attempt in the notification_interactions audit table. channelID may be
// nil (action button was clicked but we couldn't resolve which channel sent
// it) -- the action still runs, it's just not attributable to a channel.
func (s *NotificationInteractionService) Dispatch(ctx context.Context, incidentID uuid.UUID, channelID *uuid.UUID, channelType, action, actor, requestToken string) (*ActionResult, error) {
	var (
		result  *ActionResult
		dispErr error
	)

	switch action {
	case InteractionAcknowledge:
		result, dispErr = s.acknowledge(ctx, incidentID, actor)
	case InteractionResolve:
		result, dispErr = s.resolve(ctx, incidentID, actor)
	case InteractionViewRCA:
		result, dispErr = s.viewRCA(ctx, incidentID)
	default:
		dispErr = ErrUnknownInteractionAction
	}

	s.recordInteraction(ctx, incidentID, channelID, channelType, action, actor, requestToken, dispErr)

	if dispErr != nil {
		return nil, dispErr
	}
	return result, nil
}

func (s *NotificationInteractionService) acknowledge(ctx context.Context, incidentID uuid.UUID, actor string) (*ActionResult, error) {
	status := "investigating"
	inc, _, err := s.incidentUpdater.Update(ctx, incidentID, models.UpdateIncidentRequest{Status: &status})
	if err != nil {
		return nil, fmt.Errorf("acknowledge incident: %w", err)
	}
	return &ActionResult{
		Incident: inc,
		Message:  fmt.Sprintf("Incident %s acknowledged by %s (status: investigating).", incidentID, actorLabel(actor)),
	}, nil
}

// resolve currently discards the "just terminalized" flag Update returns --
// unlike the HTTP PATCH /incidents/{id} path (incident_handlers.go), this
// Slack/Teams button path does not publish incident.resolved, so resolving
// an incident from a chat action does not (yet) trigger postmortem
// generation. Known gap, not fixed here: wiring it needs an EventPublisher
// dependency threaded through this service the same way the HTTP handler
// has one.
func (s *NotificationInteractionService) resolve(ctx context.Context, incidentID uuid.UUID, actor string) (*ActionResult, error) {
	status := "resolved"
	inc, _, err := s.incidentUpdater.Update(ctx, incidentID, models.UpdateIncidentRequest{Status: &status})
	if err != nil {
		return nil, fmt.Errorf("resolve incident: %w", err)
	}
	return &ActionResult{
		Incident: inc,
		Message:  fmt.Sprintf("Incident %s resolved by %s.", incidentID, actorLabel(actor)),
	}, nil
}

func (s *NotificationInteractionService) viewRCA(ctx context.Context, incidentID uuid.UUID) (*ActionResult, error) {
	inc, err := s.incidentReader.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}

	rca, err := s.rca.GetByIncidentID(ctx, incidentID)
	if err != nil {
		// No RCA yet is an expected, non-fatal state for this action.
		return &ActionResult{
			Incident: inc,
			Message:  fmt.Sprintf("No RCA is available yet for incident %s (%s).", incidentID, inc.Title),
		}, nil
	}

	summary := rca.RootCauseSummary
	if summary == "" {
		summary = "(root cause summary not yet filled in)"
	}
	return &ActionResult{
		Incident: inc,
		RCA:      rca,
		Message: fmt.Sprintf("RCA for incident %s (%s) -- status: %s, confidence: %.0f%%\nRoot cause: %s",
			incidentID, inc.Title, rca.Status, rca.Confidence*100, summary),
	}, nil
}

func actorLabel(actor string) string {
	if actor == "" {
		return "someone"
	}
	return actor
}

func (s *NotificationInteractionService) recordInteraction(ctx context.Context, incidentID uuid.UUID, channelID *uuid.UUID, channelType, action, actor, requestToken string, dispErr error) {
	if s.interactions == nil {
		return
	}

	status := "ok"
	errMsg := ""
	if dispErr != nil {
		status = "error"
		errMsg = dispErr.Error()
	}

	orgID := uuid.Nil
	if channelID != nil {
		if oid, err := s.GetChannelOrgID(ctx, *channelID); err == nil {
			orgID = oid
		}
	}
	if orgID == uuid.Nil {
		if inc, err := s.incidentReader.GetByID(ctx, incidentID); err == nil {
			orgID = inc.OrgID
		}
	}

	interaction := &models.NotificationInteraction{
		ID:           uuid.New(),
		OrgID:        orgID,
		IncidentID:   incidentID,
		ChannelID:    channelID,
		ChannelType:  channelType,
		Action:       action,
		Actor:        actor,
		RequestToken: requestToken,
		Status:       status,
		ErrorMessage: errMsg,
		CreatedAt:    time.Now(),
	}

	if err := s.interactions.Create(ctx, interaction); err != nil {
		// Best-effort: a duplicate (channel_id, request_token) from a
		// provider retry, or a transient DB error, must never fail the
		// underlying incident action which has already been applied.
		slog.Warn("failed to record notification interaction", "incident_id", incidentID, "action", action, "error", err)
	}
}
