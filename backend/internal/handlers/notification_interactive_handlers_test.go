package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockNotificationInteractionSvc struct{ mock.Mock }

func (m *MockNotificationInteractionSvc) SlackSigningSecret(ctx context.Context, channelID uuid.UUID) (string, error) {
	args := m.Called(ctx, channelID)
	return args.String(0), args.Error(1)
}

func (m *MockNotificationInteractionSvc) TeamsVerificationToken(ctx context.Context, channelID uuid.UUID) (string, error) {
	args := m.Called(ctx, channelID)
	return args.String(0), args.Error(1)
}

func (m *MockNotificationInteractionSvc) Dispatch(ctx context.Context, incidentID uuid.UUID, channelID *uuid.UUID, channelType, action, actor, requestToken string) (*services.ActionResult, error) {
	args := m.Called(ctx, incidentID, channelID, channelType, action, actor, requestToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.ActionResult), args.Error(1)
}

func setupNotificationInteractiveRouter(h *NotificationInteractiveHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	api.POST("/webhooks/slack/interactive", h.SlackInteractive)
	api.POST("/webhooks/teams/interactive", h.TeamsInteractive)
	return r
}

// --- Slack signature helpers (mirrors verifySlackSignature's construction,
// used here only to build valid *requests* for the happy-path tests) ---

func signSlackBody(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":" + body))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func slackFormBody(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	v := url.Values{}
	v.Set("payload", string(raw))
	return v.Encode()
}

func slackPayload(incidentID, channelID uuid.UUID, action, actionTS string) map[string]any {
	value, _ := json.Marshal(map[string]string{
		"incident_id": incidentID.String(),
		"channel_id":  channelID.String(),
		"action":      action,
	})
	return map[string]any{
		"type": "block_actions",
		"actions": []map[string]any{
			{"action_id": "rootcauseway_" + action, "value": string(value), "action_ts": actionTS},
		},
		"user": map[string]any{"id": "U123", "username": "alice"},
	}
}

// --- Slack tests ---

func TestSlackInteractive_ValidSignature_Dispatches(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	incidentID := uuid.New()
	channelID := uuid.New()
	secret := "test-signing-secret"

	body := slackFormBody(t, slackPayload(incidentID, channelID, "acknowledge", "111.222"))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := signSlackBody(secret, timestamp, body)

	svc.On("SlackSigningSecret", mock.Anything, channelID).Return(secret, nil)
	svc.On("Dispatch", mock.Anything, incidentID, mock.AnythingOfType("*uuid.UUID"), "slack", "acknowledge", "alice", "111.222").
		Return(&services.ActionResult{Message: "Incident acknowledged"}, nil)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/slack/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Incident acknowledged", resp["text"])
	svc.AssertExpectations(t)
}

func TestSlackInteractive_InvalidSignature_Rejected(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	incidentID := uuid.New()
	channelID := uuid.New()

	body := slackFormBody(t, slackPayload(incidentID, channelID, "resolve", "222.333"))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	svc.On("SlackSigningSecret", mock.Anything, channelID).Return("real-secret", nil)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/slack/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", "v0=deadbeef")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "Dispatch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSlackInteractive_StaleTimestamp_Rejected(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	incidentID := uuid.New()
	channelID := uuid.New()
	secret := "test-signing-secret"

	body := slackFormBody(t, slackPayload(incidentID, channelID, "view_rca", "333.444"))
	staleTimestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	signature := signSlackBody(secret, staleTimestamp, body)

	svc.On("SlackSigningSecret", mock.Anything, channelID).Return(secret, nil)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/slack/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", staleTimestamp)
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "Dispatch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSlackInteractive_MalformedBody_Returns400(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/slack/interactive", strings.NewReader("not=a%valid%payload"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSlackInteractive_MissingPayloadField_Returns400(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	v := url.Values{}
	v.Set("not_payload", "{}")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/slack/interactive", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSlackInteractive_UnknownAction_Returns400(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	incidentID := uuid.New()
	channelID := uuid.New()
	secret := "test-signing-secret"

	body := slackFormBody(t, slackPayload(incidentID, channelID, "delete_incident", "444.555"))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := signSlackBody(secret, timestamp, body)

	svc.On("SlackSigningSecret", mock.Anything, channelID).Return(secret, nil)
	svc.On("Dispatch", mock.Anything, incidentID, mock.AnythingOfType("*uuid.UUID"), "slack", "delete_incident", "alice", "444.555").
		Return(nil, services.ErrUnknownInteractionAction)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/slack/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Slack signature unit tests ---

func TestVerifySlackSignature(t *testing.T) {
	secret := "abc123"
	body := []byte(`payload=%7B%7D`)
	now := time.Now()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	signature := signSlackBody(secret, timestamp, string(body))

	assert.True(t, verifySlackSignature(secret, timestamp, signature, body, now))
	assert.False(t, verifySlackSignature(secret, timestamp, "v0=wrong", body, now))
	assert.False(t, verifySlackSignature("wrong-secret", timestamp, signature, body, now))
	assert.False(t, verifySlackSignature(secret, "", signature, body, now))

	staleTimestamp := strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10)
	staleSig := signSlackBody(secret, staleTimestamp, string(body))
	assert.False(t, verifySlackSignature(secret, staleTimestamp, staleSig, body, now))
}

// --- Teams tests ---

func teamsActivityBody(t *testing.T, incidentID, channelID uuid.UUID, action string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type": "message",
		"id":   "activity-1",
		"from": map[string]string{"id": "29:abc", "name": "Priya"},
		"value": map[string]string{
			"incident_id": incidentID.String(),
			"channel_id":  channelID.String(),
			"action":      action,
		},
	})
	require.NoError(t, err)
	return string(raw)
}

func TestTeamsInteractive_ValidToken_Dispatches(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	incidentID := uuid.New()
	channelID := uuid.New()
	token := "shared-teams-token"

	svc.On("TeamsVerificationToken", mock.Anything, channelID).Return(token, nil)
	svc.On("Dispatch", mock.Anything, incidentID, mock.AnythingOfType("*uuid.UUID"), "teams", "resolve", "Priya", "activity-1").
		Return(&services.ActionResult{Message: "Incident resolved"}, nil)

	body := teamsActivityBody(t, incidentID, channelID, "resolve")
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/teams/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Incident resolved", resp["text"])
}

func TestTeamsInteractive_InvalidToken_Rejected(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	incidentID := uuid.New()
	channelID := uuid.New()

	svc.On("TeamsVerificationToken", mock.Anything, channelID).Return("real-token", nil)

	body := teamsActivityBody(t, incidentID, channelID, "acknowledge")
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/teams/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "Dispatch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsInteractive_MalformedBody_Returns400(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/teams/interactive", strings.NewReader("{not valid json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTeamsInteractive_MissingValue_Returns400(t *testing.T) {
	svc := new(MockNotificationInteractionSvc)
	h := &NotificationInteractiveHandler{Interactions: svc}
	r := setupNotificationInteractiveRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/teams/interactive", strings.NewReader(`{"type":"message","id":"a1"}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
