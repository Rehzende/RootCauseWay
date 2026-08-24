package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/Rehzende/RootCauseway/backend/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, restrict origins
	},
}

// HandleWebSocket upgrades the connection to WebSocket and registers the client.
func HandleWebSocket(hub *ws.Hub, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate JWT from query param since WebSocket cannot use Authorization header
		tokenStr := c.Query("token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token query parameter required"})
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		orgID, _ := claims["org_id"].(string)
		userID, _ := claims["sub"].(string)

		if orgID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing org_id in token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Error("ws upgrade failed", "error", err)
			return
		}

		client := ws.NewClient(hub, conn, orgID, userID)
		go client.WritePump()
		go client.ReadPump()
	}
}
