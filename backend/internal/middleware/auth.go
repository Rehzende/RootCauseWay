package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Claims represents the JWT claims for authenticated users.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	OrgID  uuid.UUID `json:"org_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// APIKeyAuthenticatorInterface defines the interface for API key authentication.
type APIKeyAuthenticatorInterface interface {
	Authenticate(ctx context.Context, keyString string) (*models.AuthContext, error)
}

// RBACEnforcerInterface defines the interface for loading user permissions.
type RBACEnforcerInterface interface {
	GetUserPermissions(ctx context.Context, userID uuid.UUID) (map[string][]string, error)
}

// AuthMiddleware returns a Gin middleware that validates JWT Bearer tokens.
// Kept for backward compatibility.
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid authorization format"})
			return
		}

		token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid token"})
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid claims"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("org_id", claims.OrgID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// UnifiedAuthMiddleware supports both JWT tokens and API keys (rootcauseway_ prefix).
// It also loads user permissions into the auth context.
func UnifiedAuthMiddleware(jwtSecret string, apiKeyAuth APIKeyAuthenticatorInterface, rbac RBACEnforcerInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid authorization format"})
			return
		}

		tokenStr := parts[1]

		// API key authentication
		if strings.HasPrefix(tokenStr, "rootcauseway_") {
			if apiKeyAuth == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "API key authentication not configured"})
				return
			}
			authCtx, err := apiKeyAuth.Authenticate(c.Request.Context(), tokenStr)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid API key"})
				return
			}
			c.Set("user_id", authCtx.UserID)
			c.Set("org_id", authCtx.OrgID)
			c.Set("role", "api_key")
			c.Set("auth_context", authCtx)
			c.Next()
			return
		}

		// JWT authentication
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid token"})
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid claims"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("org_id", claims.OrgID)
		c.Set("role", claims.Role)

		// Load permissions into auth context
		authCtx := &models.AuthContext{
			UserID: claims.UserID,
			OrgID:  claims.OrgID,
			Roles:  []string{claims.Role},
		}
		if rbac != nil {
			perms, err := rbac.GetUserPermissions(c.Request.Context(), claims.UserID)
			if err == nil {
				authCtx.Permissions = perms
			}
		}
		c.Set("auth_context", authCtx)

		c.Next()
	}
}
