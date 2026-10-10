package api

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
	"github.com/coldforge/vault/internal/observability"
	"github.com/gin-gonic/gin"
)

// RateLimitStore counts requests per key in fixed windows. ratelimit.Store is
// the shared Postgres implementation.
type RateLimitStore interface {
	Hit(ctx context.Context, key string, window time.Duration) (count int, resetIn time.Duration, err error)
}

// RateLimitConfig sets how many requests one client address may make per
// Window. A limit of 0 turns that bucket off.
type RateLimitConfig struct {
	Window time.Duration
	// APILimit covers every /api/ request outside the auth bucket.
	APILimit int
	// AuthLimit covers /api/v1/auth/*: login, registration, recovery and the
	// login challenges, where a low limit slows guessing.
	AuthLimit int
}

// rateLimitStoreTimeout bounds the counter lookup. On timeout the request
// goes through: a slow counter store must not stall the API.
const rateLimitStoreTimeout = 2 * time.Second

// RateLimitingMiddleware limits requests per client address, keyed on
// c.ClientIP(), which newEngine takes from the edge-set X-Real-IP and never
// from client-supplied X-Forwarded-For.
//
// Only /api/ is limited. The health check is exempt (kubelet probes), and so
// are the SPA's static files, which a single page load fetches by the dozen.
//
// If the store fails the request is let through and the failure is logged:
// the counters share the database with everything else, so an outage there
// already takes the API down, and refusing on top of it adds nothing.
func RateLimitingMiddleware(store RateLimitStore, cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucket, limit := rateLimitBucket(c.Request.URL.Path, cfg)
		if limit <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), rateLimitStoreTimeout)
		count, resetIn, err := store.Hit(ctx, bucket+":"+c.ClientIP(), cfg.Window)
		cancel()
		if err != nil {
			observability.RecordRateLimit(bucket, "store_error")
			observability.Warn("rate limit store failed; allowing request", "bucket", bucket, "error", err)
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(limit-count, 0)))
		if count > limit {
			observability.RecordRateLimit(bucket, "limited")
			// Round up so a client that waits exactly this long arrives after
			// the window has closed, not just before.
			retryAfter := max(int(math.Ceil(resetIn.Seconds())), 1)
			errors.TooManyRequests(errors.CodeRateLimitExceeded, "Too many requests, slow down", retryAfter).Abort(c)
			return
		}
		c.Next()
	}
}

// rateLimitBucket names the counter a path draws on and its limit. A limit of
// 0 means the path is not limited.
func rateLimitBucket(path string, cfg RateLimitConfig) (string, int) {
	switch {
	case !strings.HasPrefix(path, "/api/"), path == "/api/v1/health":
		return "", 0
	case strings.HasPrefix(path, "/api/v1/auth/"):
		return "auth", cfg.AuthLimit
	default:
		return "api", cfg.APILimit
	}
}
