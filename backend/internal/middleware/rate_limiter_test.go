package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// These two tests pin down RateLimiter's auth-detection contract: it reads
// "user_id" from the gin context, which only exists if something upstream
// in the middleware chain already set it (in production, UnifiedAuthMiddleware).
// Wiring RateLimiter before that auth middleware -- as cmd/api/main.go did
// via a single global r.Use(RateLimiter(...)) ahead of protected.Use(Auth)
// -- silently downgrades every authenticated request to the low public
// bucket, since "user_id" is never set yet at the point RateLimiter runs.
// That real bug (all traffic capped at publicRPM regardless of auth) is why
// RateLimiter is now wired twice in main.go: once ahead of the public
// routes, once on the protected group after UnifiedAuthMiddleware.

func TestRateLimiter_UsesAuthenticatedBucketWhenUserIDAlreadyInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Simulates UnifiedAuthMiddleware having already run and set user_id,
	// exactly as protected routes see it once RateLimiter is registered
	// after the auth middleware.
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "some-user")
		c.Next()
	})
	r.Use(RateLimiter(2, 1)) // authenticatedRPM=2, publicRPM=1
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		require.Equal(t, http.StatusOK, w.Code, "request %d should be allowed under the authenticated bucket (size 2)", i+1)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusTooManyRequests, w.Code, "3rd request should exceed the authenticated bucket")
}

func TestRateLimiter_UsesPublicBucketWhenNoUserIDInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// authenticatedRPM=100 would never trip in this test -- if a request
	// is rejected on the 2nd call, it proves the LOW publicRPM=1 bucket
	// was used, i.e. the request was (correctly, here) treated as
	// unauthenticated.
	r.Use(RateLimiter(100, 1))
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusTooManyRequests, w2.Code, "no user_id in context must fall back to the public bucket, not the authenticated one")
}
