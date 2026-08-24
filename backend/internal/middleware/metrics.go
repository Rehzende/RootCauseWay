package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestDuration observes request latency in seconds, labeled by the
// matched route pattern (not the raw path, to keep cardinality bounded),
// HTTP method and response status code.
var HTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "rootcauseway_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds, labeled by route, method and status.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"route", "method", "status"},
)

// HTTPRequestsTotal counts HTTP requests, labeled by the matched route
// pattern, HTTP method and response status code.
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rootcauseway_http_requests_total",
		Help: "Total number of HTTP requests, labeled by route, method and status.",
	},
	[]string{"route", "method", "status"},
)

// RateLimitRejectionsTotal counts requests rejected by the RateLimiter
// middleware. Incremented from rate_limiter.go.
var RateLimitRejectionsTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "rootcauseway_rate_limit_rejections_total",
		Help: "Total number of requests rejected by the rate limiter.",
	},
)

// Metrics returns a Gin middleware that records HTTP request duration and
// count metrics per matched route, method and status code.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		duration := time.Since(start).Seconds()

		HTTPRequestDuration.WithLabelValues(route, method, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(route, method, status).Inc()
	}
}
