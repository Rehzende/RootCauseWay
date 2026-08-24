package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/export"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// ExportHandler exposes document-export endpoints (Markdown/PDF) for
// incident postmortems. It is a standalone handler group (not a method set
// on *Handler) so it can be wired into main.go additively without touching
// the shared Handler struct/constructor that other in-flight work also
// touches — see PipelineGateHandler for the same pattern.
type ExportHandler struct {
	Postmortem PostmortemServiceInterface
	Incidents  IncidentServiceInterface
}

func NewExportHandler(pm PostmortemServiceInterface, inc IncidentServiceInterface) *ExportHandler {
	return &ExportHandler{Postmortem: pm, Incidents: inc}
}

// ExportPostmortem handles GET /api/v1/incidents/:id/postmortem/export?format=markdown|pdf.
//
// Both formats share the same data-gathering step (fetch the incident and
// its postmortem) and branch only at the rendering step, delegating to the
// export package's pure RenderMarkdown/RenderPDF functions.
func (h *ExportHandler) ExportPostmortem(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	format := c.DefaultQuery("format", "markdown")
	if format == "" {
		format = "markdown"
	}
	if format != "markdown" && format != "pdf" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "unsupported format: " + format + " (must be markdown or pdf)"})
		return
	}

	ctx := c.Request.Context()

	pm, err := h.Postmortem.GetByIncidentID(ctx, incidentID)
	if err != nil || pm == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}

	// Incident lookup enriches the export header (title/severity/status)
	// but isn't required for the postmortem content itself, so a lookup
	// failure here doesn't fail the export.
	var incident *models.Incident
	if h.Incidents != nil {
		incident, _ = h.Incidents.GetByID(ctx, incidentID)
	}

	filename := fmt.Sprintf("postmortem-%s", incidentID.String())

	switch format {
	case "pdf":
		out, err := export.RenderPDF(pm, incident)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to render pdf"})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, filename))
		c.Data(http.StatusOK, "application/pdf", out)
	default: // "markdown"
		out, err := export.RenderMarkdown(pm, incident)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to render markdown"})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, filename))
		c.Data(http.StatusOK, "text/markdown", []byte(out))
	}
}
