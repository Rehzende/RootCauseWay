package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetrics_RecordsRequestsAndExposesEndpoint verifies that the Metrics
// middleware records HTTP request duration/count metrics per route, and
// that a /metrics endpoint wired with promhttp.Handler() exposes them.
func TestMetrics_RecordsRequestsAndExposesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	r.GET("/things/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})
	r.GET("/boom", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "boom"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Hit a few routes to generate metrics.
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/ping", nil),
		httptest.NewRequest(http.MethodGet, "/things/123", nil),
		httptest.NewRequest(http.MethodGet, "/boom", nil),
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "rootcauseway_http_request_duration_seconds")
	assert.Contains(t, body, "rootcauseway_http_requests_total")
	// The matched route pattern should be used, not the raw path with the
	// concrete id, to avoid high-cardinality labels.
	assert.True(t, strings.Contains(body, `route="/things/:id"`), "expected route label with matched pattern, got body:\n%s", body)
	assert.True(t, strings.Contains(body, `status="200"`))
	assert.True(t, strings.Contains(body, `status="500"`))
}

// TestRateLimiter_IncrementsRejectionCounter verifies the rate limiter
// middleware increments the rootcauseway_rate_limit_rejections_total counter when
// it rejects a request.
func TestRateLimiter_IncrementsRejectionCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimiter(1, 1))
	r.GET("/x", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Metrics are scraped via a separate, unrelated engine (not behind the
	// rate limiter under test) so the scrape itself doesn't consume tokens.
	metricsEngine := gin.New()
	metricsEngine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	before := testutilCounterValue(t, metricsEngine)

	// First request consumes the single token; second should be rejected.
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusTooManyRequests, w2.Code)

	after := testutilCounterValue(t, metricsEngine)
	assert.Greater(t, after, before)
}

// testutilCounterValue scrapes /metrics and extracts the current value of
// rootcauseway_rate_limit_rejections_total (0 if not yet present).
func testutilCounterValue(t *testing.T, r *gin.Engine) float64 {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)
	for _, line := range strings.Split(w.Body.String(), "\n") {
		if strings.HasPrefix(line, "rootcauseway_rate_limit_rejections_total ") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				var v float64
				_, err := fmt.Sscan(parts[1], &v)
				require.NoError(t, err)
				return v
			}
		}
	}
	return 0
}
