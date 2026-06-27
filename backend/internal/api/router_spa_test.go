package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// newSPATestRouter builds a minimal router that mirrors how SetupRouter wires
// the SPA fallback: a real /api route plus NoRoute(spaHandler(webDir)).
func newSPATestRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><div id=root></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	staticDir := filepath.Join(webDir, "static", "js")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "main.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.GET("/api/v1/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.NoRoute(spaHandler(webDir))
	return r, webDir
}

func TestSPAHandler(t *testing.T) {
	r, _ := newSPATestRouter(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string // substring
	}{
		{"root serves index", "/", http.StatusOK, "id=root"},
		{"spa route falls back to index", "/login", http.StatusOK, "id=root"},
		{"nested spa route falls back to index", "/settings/profile", http.StatusOK, "id=root"},
		{"real static asset is served", "/static/js/main.js", http.StatusOK, "console.log(1)"},
		{"matched api route still works", "/api/v1/health", http.StatusOK, "ok"},
		{"unknown api path stays JSON 404", "/api/v1/nope", http.StatusNotFound, "does not exist"},
		{"metrics 404 stays JSON", "/metrics/extra", http.StatusNotFound, "does not exist"},
		{"well-known 404 stays JSON", "/.well-known/foo", http.StatusNotFound, "does not exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("path %s: status = %d, want %d", tt.path, w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !contains(w.Body.String(), tt.wantBody) {
				t.Fatalf("path %s: body %q does not contain %q", tt.path, w.Body.String(), tt.wantBody)
			}
		})
	}
}

// TestSPAHandlerNoTraversal ensures path traversal cannot escape the web dir.
func TestSPAHandlerNoTraversal(t *testing.T) {
	r, _ := newSPATestRouter(t)

	w := httptest.NewRecorder()
	// Attempt to read /etc/passwd via traversal; must fall back to index, not leak.
	req := httptest.NewRequest(http.MethodGet, "/../../../../etc/passwd", nil)
	r.ServeHTTP(w, req)

	if contains(w.Body.String(), "root:") {
		t.Fatalf("path traversal leaked file contents: %q", w.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
