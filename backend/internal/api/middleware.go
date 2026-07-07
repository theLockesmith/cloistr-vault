package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/observability"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Allow the cross-origin SSO session check to the signer (unified auth)
		// and inline styles used by the UI's Tailwind/Radix components.
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"connect-src 'self' https://signer.cloistr.xyz; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		c.Next()
	})
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	return gin.Logger()
}

// RateLimitingMiddleware basic rate limiting (in production, use Redis-based solution)
func RateLimitingMiddleware() gin.HandlerFunc {
	// Simple in-memory rate limiting - replace with proper solution in production
	return gin.HandlerFunc(func(c *gin.Context) {
		// TODO: Implement proper rate limiting with Redis
		c.Next()
	})
}

// signerSessionCache caches signer token → pubkey resolutions for ~2 min to
// avoid round-tripping the signer on every request in the hot path.
// It is process-scoped and lives as long as the middleware closure.
type signerSessionCache struct {
	mu      sync.RWMutex
	entries map[string]signerCacheEntry
}

type signerCacheEntry struct {
	pubkey  string
	expires time.Time
}

var globalSignerCache = &signerSessionCache{entries: make(map[string]signerCacheEntry)}

// resolveSignerPubkey validates a Cloistr signer session and returns the
// associated pubkey, or "" if absent / unreachable. It forwards the caller's
// auth_token cookie (or Authorization Bearer) to the signer's /api/v1/users/me.
func resolveSignerPubkey(c *gin.Context, signerURL string) string {
	if signerURL == "" {
		return ""
	}

	var cacheKey, cookieVal, bearer string
	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		cookieVal = cookie
		cacheKey = "c:" + cookie
	} else if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		bearer = strings.TrimPrefix(h, "Bearer ")
		cacheKey = "b:" + bearer
	} else {
		return ""
	}

	// Cache hit?
	globalSignerCache.mu.RLock()
	if e, ok := globalSignerCache.entries[cacheKey]; ok && time.Now().Before(e.expires) {
		globalSignerCache.mu.RUnlock()
		return e.pubkey
	}
	globalSignerCache.mu.RUnlock()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, signerURL+"/api/v1/users/me", nil)
	if err != nil {
		return ""
	}
	if cookieVal != "" {
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: cookieVal})
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := client.Do(req)
	if err != nil {
		observability.Warn("signer session validation failed", "error", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var body struct {
		Pubkey string `json:"pubkey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Pubkey == "" {
		return ""
	}

	globalSignerCache.mu.Lock()
	globalSignerCache.entries[cacheKey] = signerCacheEntry{pubkey: body.Pubkey, expires: time.Now().Add(2 * time.Minute)}
	globalSignerCache.mu.Unlock()

	return body.Pubkey
}

// AuthMiddleware validates vault session tokens. When no valid vault token is
// present it falls back to a Cloistr signer session (unified-auth slice 3):
// the browser-sent auth_token cookie (or Authorization Bearer) is forwarded to
// the signer's /api/v1/users/me; on success the pubkey is resolved to a vault
// user, lazily provisioning one if it does not yet exist.
//
// The existing token-based path is completely unchanged — this is additive only.
func AuthMiddleware(authService *auth.AuthService, signerURL string) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// --- Primary path: vault session token (unchanged) ---
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
				token := tokenParts[1]
				user, err := authService.ValidateSession(token)
				if err == nil {
					c.Set("user", user)
					c.Set("userID", user.ID.String())
					c.Next()
					return
				}
				// Fall through to signer path — token might be a signer JWT.
			}
		}

		// --- Fallback path: Cloistr signer session ---
		if pubkey := resolveSignerPubkey(c, signerURL); pubkey != "" {
			user, err := authService.FindOrProvisionByNostrPubkey(pubkey)
			if err != nil {
				observability.Error("signer user provision failed", "error", err, "pubkey_prefix", pubkey[:min(16, len(pubkey))])
				errors.Unauthorized(errors.CodeAuthInvalid, "Authentication failed").Abort(c)
				return
			}
			c.Set("user", user)
			c.Set("userID", user.ID.String())
			c.Next()
			return
		}

		// No credential at all.
		if authHeader == "" {
			errors.Unauthorized(errors.CodeAuthRequired, "Authorization header required").Abort(c)
			return
		}
		errors.Unauthorized(errors.CodeAuthInvalid, "Invalid or expired token").Abort(c)
	})
}


// ErrorHandlingMiddleware handles panics and errors
func ErrorHandlingMiddleware() gin.HandlerFunc {
	return gin.Recovery()
}

// RequestTimeoutMiddleware adds timeout to requests
func RequestTimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement request timeout
		c.Next()
	}
}

// ContentTypeMiddleware ensures JSON content type for API endpoints
func ContentTypeMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				errors.BadRequest(errors.CodeInvalidInput, "Content-Type must be application/json").Abort(c)
				return
			}
		}
		c.Next()
	})
}