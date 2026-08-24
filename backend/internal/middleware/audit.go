package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// AuditLogRepository defines DB operations for audit logging.
type AuditLogRepository interface {
	Create(ctx context.Context, entry *models.AuditLogEntry) error
}

// AuditMiddleware logs write operations to the audit log.
func AuditMiddleware(auditRepo AuditLogRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodOptions || method == http.MethodHead {
			return
		}

		// Only log successful write operations
		if c.Writer.Status() >= 400 {
			return
		}

		var userID *uuid.UUID
		if v, exists := c.Get("user_id"); exists {
			if uid, ok := v.(uuid.UUID); ok {
				userID = &uid
			}
		}

		orgID := uuid.Nil
		if v, exists := c.Get("org_id"); exists {
			if oid, ok := v.(uuid.UUID); ok {
				orgID = oid
			}
		}

		// Extract resource type and ID from path
		path := c.Request.URL.Path
		resourceType, resourceID := extractResource(path)

		action := method + " " + path

		details, _ := json.Marshal(map[string]interface{}{
			"status_code": c.Writer.Status(),
			"path":        path,
		})

		entry := &models.AuditLogEntry{
			ID:           uuid.New(),
			OrgID:        orgID,
			UserID:       userID,
			Action:       action,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Details:      details,
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestID:    c.GetString("request_id"),
			CreatedAt:    time.Now(),
		}

		// Fire and forget - don't block the response
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = auditRepo.Create(ctx, entry)
		}()
	}
}

// extractResource parses the API path to determine resource type and ID.
func extractResource(path string) (resourceType, resourceID string) {
	// Strip /api/v1/ prefix
	path = strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(path, "/")

	if len(parts) >= 1 {
		resourceType = parts[0]
	}
	if len(parts) >= 2 {
		resourceID = parts[1]
	}
	return
}
