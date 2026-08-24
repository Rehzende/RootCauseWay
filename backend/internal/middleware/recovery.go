package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Recovery returns a middleware that recovers from panics, logs the stack trace,
// and returns a 500 response without exposing internal details.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				requestID := c.GetString("request_id")
				slog.Error("panic recovered",
					"request_id", requestID,
					"error", r,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, models.ErrorResponse{
					Error: "internal server error",
				})
			}
		}()
		c.Next()
	}
}
