package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coldforge/vault/internal/config"
	"github.com/gin-gonic/gin"
)

// The address logged as client_ip must come from X-Real-IP, which the public
// edge overwrites with the real peer, never from X-Forwarded-For, which the
// edge only appends to. Gin's defaults returned the client's forged XFF.
func TestClientIPIgnoresForgedForwardedFor(t *testing.T) {
	r := SetupRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", config.ClientConfig{})
	r.GET("/__client_ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"edge request: forged XFF, X-Real-IP set by edge",
			map[string]string{"X-Forwarded-For": "203.0.113.77, 198.51.100.9", "X-Real-IP": "198.51.100.9"}, "198.51.100.9"},
		{"no X-Real-IP: forged XFF ignored, TCP peer used",
			map[string]string{"X-Forwarded-For": "203.0.113.77"}, "10.0.0.5"},
		{"no headers", nil, "10.0.0.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/__client_ip", nil)
			req.RemoteAddr = "10.0.0.5:41234"
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if got := w.Body.String(); got != tc.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
