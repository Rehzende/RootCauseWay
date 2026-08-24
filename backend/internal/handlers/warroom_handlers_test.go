package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockWarRoomSvc struct{ mock.Mock }

func (m *MockWarRoomSvc) CreateWarRoom(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}

func (m *MockWarRoomSvc) GetByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}

func (m *MockWarRoomSvc) GetByID(ctx context.Context, meetingID uuid.UUID) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, meetingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}

func (m *MockWarRoomSvc) EndWarRoom(ctx context.Context, meetingID uuid.UUID) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, meetingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}

func (m *MockWarRoomSvc) AttachSummary(ctx context.Context, meetingID uuid.UUID, summary models.WarRoomSummary, participants []models.WarRoomAttendee) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, meetingID, summary, participants)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}

func setupWarRoomRouter(h *WarRoomHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api/v1")
	api.POST("/incidents/:id/warroom", h.CreateWarRoom)
	api.GET("/incidents/:id/warroom", h.GetWarRoom)
	api.POST("/warroom/:meetingId/end", h.EndWarRoom)

	internal := api.Group("/internal")
	internal.GET("/warroom/:meetingId", h.GetWarRoomByID)
	internal.PATCH("/warroom/:meetingId/summary", h.AttachWarRoomSummary)

	return r
}

func TestWarRoomHandler_GetWarRoomByID(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	meetingID := uuid.New()
	transcript := "Alice: kicking off the war room."
	meeting := &models.WarRoomMeeting{ID: meetingID, Status: "ended", RawTranscript: &transcript}
	svc.On("GetByID", mock.Anything, meetingID).Return(meeting, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/internal/warroom/"+meetingID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.WarRoomMeeting
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.RawTranscript)
	assert.Equal(t, transcript, *resp.RawTranscript)
}

func TestWarRoomHandler_GetWarRoomByID_NotFound(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	meetingID := uuid.New()
	svc.On("GetByID", mock.Anything, meetingID).Return(nil, errors.New("no rows"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/internal/warroom/"+meetingID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWarRoomHandler_CreateWarRoom(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	incidentID := uuid.New()
	meeting := &models.WarRoomMeeting{
		ID:                uuid.New(),
		IncidentID:        incidentID,
		Provider:          "teams",
		ExternalMeetingID: "mock-meeting-1",
		JoinURL:           "https://teams.microsoft.com/l/meetup-join/mock-1",
		Status:            "scheduled",
	}
	svc.On("CreateWarRoom", mock.Anything, incidentID).Return(meeting, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/incidents/"+incidentID.String()+"/warroom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp models.WarRoomMeeting
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, meeting.ID, resp.ID)
	assert.Equal(t, "scheduled", resp.Status)
}

func TestWarRoomHandler_CreateWarRoom_ProviderError(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	incidentID := uuid.New()
	svc.On("CreateWarRoom", mock.Anything, incidentID).
		Return(nil, errors.New("Teams integration not configured"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/incidents/"+incidentID.String()+"/warroom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestWarRoomHandler_GetWarRoom(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	incidentID := uuid.New()
	meeting := &models.WarRoomMeeting{ID: uuid.New(), IncidentID: incidentID, Status: "summarized"}
	svc.On("GetByIncident", mock.Anything, incidentID).Return(meeting, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/incidents/"+incidentID.String()+"/warroom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWarRoomHandler_GetWarRoom_NotFound(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	incidentID := uuid.New()
	svc.On("GetByIncident", mock.Anything, incidentID).Return(nil, errors.New("no rows"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/incidents/"+incidentID.String()+"/warroom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWarRoomHandler_EndWarRoom(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	meetingID := uuid.New()
	meeting := &models.WarRoomMeeting{ID: meetingID, Status: "ended"}
	svc.On("EndWarRoom", mock.Anything, meetingID).Return(meeting, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/warroom/"+meetingID.String()+"/end", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.WarRoomMeeting
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ended", resp.Status)
}

func TestWarRoomHandler_AttachWarRoomSummary(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	meetingID := uuid.New()
	summary := models.WarRoomSummary{
		ExecutiveSummary: "Outage resolved via rollback.",
		KeyPoints:        []string{"502s spiked"},
		ActionItems:      []models.WarRoomActionItem{{Description: "Add canary check"}},
	}
	participants := []models.WarRoomAttendee{{Name: "Alice", Email: "alice@example.com"}}
	reqBody := models.AttachWarRoomSummaryRequest{Summary: summary, Participants: participants}
	body, _ := json.Marshal(reqBody)

	updated := &models.WarRoomMeeting{ID: meetingID, Status: "summarized"}
	svc.On("AttachSummary", mock.Anything, meetingID, summary, participants).Return(updated, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/internal/warroom/"+meetingID.String()+"/summary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.WarRoomMeeting
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "summarized", resp.Status)
}

func TestWarRoomHandler_AttachWarRoomSummary_InvalidBody(t *testing.T) {
	svc := new(MockWarRoomSvc)
	h := &WarRoomHandler{WarRooms: svc}
	r := setupWarRoomRouter(h)

	meetingID := uuid.New()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/internal/warroom/"+meetingID.String()+"/summary", bytes.NewReader([]byte(`not-json`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
