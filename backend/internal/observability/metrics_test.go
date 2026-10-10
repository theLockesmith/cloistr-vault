package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Every rate-limit series must be on /metrics at 0 before anything is limited:
// an absent series reads the same as an emitter that is broken.
func TestRateLimitSeriesExportedBeforeAnyRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsHandler())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()

	for _, want := range []string{
		`coldforge_vault_rate_limit_total{bucket="api",outcome="limited"} 0`,
		`coldforge_vault_rate_limit_total{bucket="api",outcome="store_error"} 0`,
		`coldforge_vault_rate_limit_total{bucket="auth",outcome="limited"} 0`,
		`coldforge_vault_rate_limit_total{bucket="auth",outcome="store_error"} 0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics is missing %s", want)
		}
	}
}
