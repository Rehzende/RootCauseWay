package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
)

// NotificationInteractionServiceInterface is the surface
// NotificationInteractiveHandler depends on. Satisfied by
// *services.NotificationInteractionService.
type NotificationInteractionServiceInterface interface {
	SlackSigningSecret(ctx context.Context, channelID uuid.UUID) (string, error)
	TeamsVerificationToken(ctx context.Context, channelID uuid.UUID) (string, error)
	Dispatch(ctx context.Context, incidentID uuid.UUID, channelID *uuid.UUID, channelType, action, actor, requestToken string) (*services.ActionResult, error)
}

// NotificationInteractiveHandler handles the inbound side of bidirectional
// Slack/Teams notifications: POST /api/v1/webhooks/slack/interactive and
// POST /api/v1/webhooks/teams/interactive. Both are registered outside the
// JWT-protected route group (same pattern as POST /api/v1/ingest/:token) --
// they're called by Slack/Teams themselves, not by an authenticated RootCauseway
// user, so authenticity is instead established by verifying the
// provider-specific request signature/token against the secret configured
// on the notification channel that sent the original message.
type NotificationInteractiveHandler struct {
	Interactions NotificationInteractionServiceInterface
}

// --- Slack ---

type slackAction struct {
	ActionID string `json:"action_id"`
	Value    string `json:"value"`
	ActionTS string `json:"action_ts"`
}

type slackUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type slackInteractivePayload struct {
	Type        string        `json:"type"`
	Actions     []slackAction `json:"actions"`
	User        slackUser     `json:"user"`
	ResponseURL string        `json:"response_url"`
}

// interactionActionValue is the JSON we ourselves embed as the Slack
// button's "value" (see agent-service/app/notifications/dispatcher.py
// _build_slack_actions_block) and as the Teams Action.Submit "data"/"value".
type interactionActionValue struct {
	IncidentID string `json:"incident_id"`
	ChannelID  string `json:"channel_id"`
	Action     string `json:"action"`
}

const slackSignatureMaxAge = 5 * time.Minute

// SlackInteractive handles POST /api/v1/webhooks/slack/interactive.
//
// Slack posts the interactive payload as a form-encoded body with a single
// "payload" field containing the JSON. Signature verification must run over
// the *raw* body bytes (see https://api.slack.com/authentication/verifying-requests),
// so the body is read once, verified, and only then parsed/acted on.
func (h *NotificationInteractiveHandler) SlackInteractive(c *gin.Context) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "unable to read request body"})
		return
	}

	values, err := url.ParseQuery(string(rawBody))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "malformed form body"})
		return
	}
	payloadStr := values.Get("payload")
	if payloadStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing payload field"})
		return
	}

	var sp slackInteractivePayload
	if err := json.Unmarshal([]byte(payloadStr), &sp); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "malformed slack payload"})
		return
	}
	if len(sp.Actions) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "payload has no actions"})
		return
	}

	var av interactionActionValue
	if err := json.Unmarshal([]byte(sp.Actions[0].Value), &av); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "malformed action value"})
		return
	}
	incidentID, err := uuid.Parse(av.IncidentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "malformed incident_id"})
		return
	}
	channelID, err := parseOptionalUUID(av.ChannelID)
	if err != nil || channelID == nil {
		// We need a channel to look up the signing secret; without one we
		// cannot verify authenticity, so reject rather than trust blindly.
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unable to verify request: missing channel"})
		return
	}

	secret, err := h.Interactions.SlackSigningSecret(c.Request.Context(), *channelID)
	if err != nil || secret == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unable to verify request signature"})
		return
	}

	timestamp := c.GetHeader("X-Slack-Request-Timestamp")
	signature := c.GetHeader("X-Slack-Signature")
	if !verifySlackSignature(secret, timestamp, signature, rawBody, time.Now()) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid signature"})
		return
	}

	actor := sp.User.Username
	if actor == "" {
		actor = sp.User.Name
	}
	if actor == "" {
		actor = sp.User.ID
	}

	result, err := h.Interactions.Dispatch(c.Request.Context(), incidentID, channelID, "slack", av.Action, actor, sp.Actions[0].ActionTS)
	if err != nil {
		writeDispatchError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response_type":    "ephemeral",
		"replace_original": false,
		"text":             result.Message,
	})
}

// verifySlackSignature implements Slack's HMAC-SHA256 request signing
// scheme: expected = "v0=" + hex(HMAC_SHA256(signingSecret, "v0:{timestamp}:{body}")).
// Requests with a timestamp more than 5 minutes old or in the future are
// rejected to guard against replay attacks.
func verifySlackSignature(signingSecret, timestamp, signature string, body []byte, now time.Time) bool {
	if signingSecret == "" || timestamp == "" || signature == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	reqTime := time.Unix(ts, 0)
	age := now.Sub(reqTime)
	if age > slackSignatureMaxAge || age < -slackSignatureMaxAge {
		return false
	}

	base := "v0:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// --- Teams ---

type teamsFrom struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// teamsActivity is a minimal parser for the Bot Framework "message" activity
// Teams posts back when a user clicks an Adaptive Card Action.Submit button
// (see agent-service/app/notifications/dispatcher.py _build_teams_adaptive_card).
// Full Bot Framework JWT verification against Microsoft's OpenID
// configuration is out of scope; instead a shared verification token
// configured per-channel (models.TeamsChannelConfig.VerificationToken) is
// compared against an Authorization: Bearer header or the
// X-Teams-Verification-Token header.
type teamsActivity struct {
	Type  string                  `json:"type"`
	ID    string                  `json:"id"`
	From  teamsFrom               `json:"from"`
	Value *interactionActionValue `json:"value"`
}

// TeamsInteractive handles POST /api/v1/webhooks/teams/interactive.
func (h *NotificationInteractiveHandler) TeamsInteractive(c *gin.Context) {
	var activity teamsActivity
	if err := c.ShouldBindJSON(&activity); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "malformed teams activity"})
		return
	}
	if activity.Value == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing action value"})
		return
	}

	incidentID, err := uuid.Parse(activity.Value.IncidentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "malformed incident_id"})
		return
	}
	channelID, err := parseOptionalUUID(activity.Value.ChannelID)
	if err != nil || channelID == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unable to verify request: missing channel"})
		return
	}

	expected, err := h.Interactions.TeamsVerificationToken(c.Request.Context(), *channelID)
	if err != nil || expected == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unable to verify request"})
		return
	}

	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		token = c.GetHeader("X-Teams-Verification-Token")
	}
	if token == "" || !hmac.Equal([]byte(token), []byte(expected)) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid verification token"})
		return
	}

	actor := activity.From.Name
	if actor == "" {
		actor = activity.From.ID
	}

	result, err := h.Interactions.Dispatch(c.Request.Context(), incidentID, channelID, "teams", activity.Value.Action, actor, activity.ID)
	if err != nil {
		writeDispatchError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type": "message",
		"text": result.Message,
	})
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimPrefix(header, prefix)
	}
	return ""
}

// --- shared helpers ---

func parseOptionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func writeDispatchError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrUnknownInteractionAction) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
}
