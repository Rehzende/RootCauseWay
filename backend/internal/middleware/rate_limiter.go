package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimiter returns a middleware that limits requests per IP using a token bucket.
// authenticatedRPM is the limit for authenticated requests, publicRPM for unauthenticated.
func RateLimiter(authenticatedRPM, publicRPM int) gin.HandlerFunc {
	var buckets sync.Map

	// Cleanup stale buckets every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			buckets.Range(func(key, value any) bool {
				b := value.(*tokenBucket)
				if time.Since(b.lastRefill) > 10*time.Minute {
					buckets.Delete(key)
				}
				return true
			})
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		_, authenticated := c.Get("user_id")

		rpm := publicRPM
		if authenticated {
			rpm = authenticatedRPM
		}

		key := ip
		if authenticated {
			key = "auth:" + ip
		}

		val, _ := buckets.LoadOrStore(key, &tokenBucket{
			tokens:     float64(rpm),
			maxTokens:  float64(rpm),
			refillRate: float64(rpm) / 60.0,
			lastRefill: time.Now(),
		})
		bucket := val.(*tokenBucket)

		if !bucket.allow() {
			RateLimitRejectionsTotal.Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.ErrorResponse{
				Error: "rate limit exceeded, try again later",
			})
			return
		}
		c.Next()
	}
}
