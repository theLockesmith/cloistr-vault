package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coldforge/vault/internal/config"
	"github.com/gin-gonic/gin"
)

// memLimitStore is a fixed-window counter in memory, for exercising the
// middleware. The Postgres store has its own tests in internal/ratelimit.
type memLimitStore struct {
	mu    sync.Mutex
	now   time.Time
	rows  map[string]*memLimitRow
	err   error
	calls int
}

type memLimitRow struct {
	count   int
	resetAt time.Time
}

func newMemLimitStore() *memLimitStore {
	return &memLimitStore{now: time.Unix(1_800_000_000, 0), rows: map[string]*memLimitRow{}}
}

func (m *memLimitStore) Hit(_ context.Context, key string, window time.Duration) (int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return 0, 0, m.err
	}
	r, ok := m.rows[key]
	if !ok || !m.now.Before(r.resetAt) {
		r = &memLimitRow{resetAt: m.now.Add(window)}
		m.rows[key] = r
	}
	r.count++
	return r.count, r.resetAt.Sub(m.now), nil
}

func rateLimitedRouter(store RateLimitStore, cfg RateLimitConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := newEngine()
	r.Use(RateLimitingMiddleware(store, cfg))
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	r.GET("/api/v1/health", ok)
	r.GET("/api/v1/vault", ok)
	r.POST("/api/v1/auth/login", ok)
	r.POST("/api/v1/auth/logout", ok)
	r.NoRoute(ok) // stands in for the SPA's static files
	return r
}

func doFrom(r http.Handler, method, path, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "10.0.0.5:41234"
	if ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var testLimits = RateLimitConfig{Window: time.Minute, APILimit: 5, AuthLimit: 2}

func TestRateLimitBurstFromOneAddressGets429(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, testLimits)

	for i := 1; i <= 5; i++ {
		if w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9"); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i, w.Code)
		}
	}
	w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 6: status %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want \"60\"", got)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != "RATE_LIMIT_EXCEEDED" {
		t.Fatalf("body = %s, want code RATE_LIMIT_EXCEEDED", w.Body.String())
	}

	// A second address is unaffected by the first one's burst.
	if w := doFrom(r, http.MethodGet, "/api/v1/vault", "203.0.113.77"); w.Code != http.StatusOK {
		t.Fatalf("second address: status %d, want 200", w.Code)
	}
}

func TestRateLimitWindowEndLetsTheClientBackIn(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, testLimits)
	for i := 0; i < 6; i++ {
		doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9")
	}
	store.now = store.now.Add(time.Minute)
	if w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9"); w.Code != http.StatusOK {
		t.Fatalf("after the window: status %d, want 200", w.Code)
	}
}

func TestRateLimitRetryAfterCountsDownToWindowEnd(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, testLimits)
	for i := 0; i < 5; i++ {
		doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9")
	}
	store.now = store.now.Add(45*time.Second + 300*time.Millisecond)
	w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", w.Code)
	}
	// 14.7s left rounds up: a client that waits exactly Retry-After must not
	// arrive before the window ends.
	if got := w.Header().Get("Retry-After"); got != "15" {
		t.Fatalf("Retry-After = %q, want \"15\"", got)
	}
}

func TestRateLimitAuthBucketIsStricterAndSeparate(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, testLimits)

	for i := 1; i <= 2; i++ {
		if w := doFrom(r, http.MethodPost, "/api/v1/auth/login", "198.51.100.9"); w.Code != http.StatusOK {
			t.Fatalf("login %d: status %d, want 200", i, w.Code)
		}
	}
	if w := doFrom(r, http.MethodPost, "/api/v1/auth/login", "198.51.100.9"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("login 3: status %d, want 429", w.Code)
	}
	// Exhausting the auth bucket leaves the general API alone.
	if w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9"); w.Code != http.StatusOK {
		t.Fatalf("vault after auth limit: status %d, want 200", w.Code)
	}
}

func TestRateLimitSkipsHealthAndStaticFiles(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, testLimits)
	for i := 0; i < 20; i++ {
		for _, p := range []string{"/api/v1/health", "/", "/static/js/main.js"} {
			if w := doFrom(r, http.MethodGet, p, "198.51.100.9"); w.Code != http.StatusOK {
				t.Fatalf("%s: status %d, want 200", p, w.Code)
			}
		}
	}
	if store.calls != 0 {
		t.Fatalf("store was hit %d times for exempt paths", store.calls)
	}
}

func TestRateLimitKeysOnTheTrustedAddressNotForwardedFor(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, testLimits)
	// Rotating a forged X-Forwarded-For must not buy a fresh allowance.
	for i := 1; i <= 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/vault", nil)
		req.RemoteAddr = "10.0.0.5:41234"
		req.Header.Set("X-Real-IP", "198.51.100.9")
		req.Header.Set("X-Forwarded-For", "203.0.113."+strconv.Itoa(i))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		want := http.StatusOK
		if i == 6 {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("request %d: status %d, want %d", i, w.Code, want)
		}
	}
}

// A broken counter store must not take the API down with it.
func TestRateLimitFailsOpenWhenTheStoreErrors(t *testing.T) {
	store := newMemLimitStore()
	store.err = errors.New("connection refused")
	r := rateLimitedRouter(store, testLimits)
	for i := 0; i < 10; i++ {
		if w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9"); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i, w.Code)
		}
	}
	if store.calls == 0 {
		t.Fatal("store was never consulted")
	}
}

func TestRateLimitZeroTurnsABucketOff(t *testing.T) {
	store := newMemLimitStore()
	r := rateLimitedRouter(store, RateLimitConfig{Window: time.Minute, APILimit: 0, AuthLimit: 2})
	for i := 0; i < 20; i++ {
		if w := doFrom(r, http.MethodGet, "/api/v1/vault", "198.51.100.9"); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i, w.Code)
		}
	}
	for i := 0; i < 2; i++ {
		doFrom(r, http.MethodPost, "/api/v1/auth/login", "198.51.100.9")
	}
	if w := doFrom(r, http.MethodPost, "/api/v1/auth/login", "198.51.100.9"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("auth bucket still on: status %d, want 429", w.Code)
	}
}

// SetupRouter must actually install the limiter it is given.
func TestSetupRouterInstallsTheRateLimiter(t *testing.T) {
	store := newMemLimitStore()
	r := SetupRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", config.ClientConfig{},
		RateLimitingMiddleware(store, testLimits))
	for i := 1; i <= 5; i++ {
		if w := doFrom(r, http.MethodGet, "/api/v1/info", "198.51.100.9"); w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d limited early", i)
		}
	}
	if w := doFrom(r, http.MethodGet, "/api/v1/info", "198.51.100.9"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 6: status %d, want 429", w.Code)
	}
}
