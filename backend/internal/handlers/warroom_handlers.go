package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// WarRoomServiceInterface is the surface WarRoomHandler depends on.
type WarRoomServiceInterface interface {
	CreateWarRoom(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error)
	GetByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error)
	GetByID(ctx context.Context, meetingID uuid.UUID) (*models.WarRoomMeeting, error)
	EndWarRoom(ctx context.Context, meetingID uuid.UUID) (*models.WarRoomMeeting, error)
	AttachSummary(ctx context.Context, meetingID uuid.UUID, summary models.WarRoomSummary, participants []models.WarRoomAttendee) (*models.WarRoomMeeting, error)
}

// WarRoomHandler exposes war room meeting lifecycle endpoints. Registered
// additively in cmd/api/main.go alongside the other feature handlers
// (ObservabilityHandler, MarketplaceHandler, ...).
type WarRoomHandler struct {
	WarRooms WarRoomServiceInterface
}

// CreateWarRoom handles POST /api/v1/incidents/:id/warroom -- spins up a
// Teams meeting for the incident and persists it.
func (h *WarRoomHandler) CreateWarRoom(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	meeting, err := h.WarRooms.CreateWarRoom(c.Request.Context(), incidentID)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, meeting)
}

// GetWarRoom handles GET /api/v1/incidents/:id/warroom -- returns the
// current (most recent) war room meeting for the incident, including
// status and summary once available.
func (h *WarRoomHandler) GetWarRoom(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	meeting, err := h.WarRooms.GetByIncident(c.Request.Context(), incidentID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "war room not found"})
		return
	}
	c.JSON(http.StatusOK, meeting)
}

// GetWarRoomByID handles GET /api/v1/internal/warroom/:meetingId -- internal
// endpoint used by agent-service to fetch a meeting (including its raw
// transcript) by meeting ID once it's ended, ahead of summarization.
func (h *WarRoomHandler) GetWarRoomByID(c *gin.Context) {
	meetingID, err := uuid.Parse(c.Param("meetingId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid meetingId"})
		return
	}

	meeting, err := h.WarRooms.GetByID(c.Request.Context(), meetingID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "war room not found"})
		return
	}
	c.JSON(http.StatusOK, meeting)
}

// EndWarRoom handles POST /api/v1/warroom/:meetingId/end -- marks the
// meeting ended, fetches transcript + attendance from the Teams
// provider, and publishes warroom.meeting.ended.
//
// v1 limitation: this is a manual trigger since there is no real Graph
// subscription webhook receiver available without an Azure AD tenant.
// The handler simply delegates to WarRoomService.EndWarRoom, so a future
// Graph webhook handler can call that same service method directly.
func (h *WarRoomHandler) EndWarRoom(c *gin.Context) {
	meetingID, err := uuid.Parse(c.Param("meetingId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid meetingId"})
		return
	}

	meeting, err := h.WarRooms.EndWarRoom(c.Request.Context(), meetingID)
	if err != nil {
		handleDBError(c, err, "war room meeting")
		return
	}
	c.JSON(http.StatusOK, meeting)
}

// AttachWarRoomSummary handles
// PATCH /api/v1/internal/warroom/:meetingId/summary -- internal endpoint
// used by agent-service to write back the LLM-generated summary and
// participant list once it has processed the transcript.
func (h *WarRoomHandler) AttachWarRoomSummary(c *gin.Context) {
	meetingID, err := uuid.Parse(c.Param("meetingId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid meetingId"})
		return
	}

	var req models.AttachWarRoomSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	meeting, err := h.WarRooms.AttachSummary(c.Request.Context(), meetingID, req.Summary, req.Participants)
	if err != nil {
		handleDBError(c, err, "war room meeting")
		return
	}
	c.JSON(http.StatusOK, meeting)
}
