package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coldforge/vault/internal/config"
	"github.com/gin-gonic/gin"
)

func runtimeConfigRouter(cc config.ClientConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeadersMiddleware(cc.SignerURL))
	r.GET("/config.js", RuntimeConfigHandler(cc))
	r.HEAD("/config.js", RuntimeConfigHandler(cc))
	return r
}

// parseConfigJS pulls the JSON object back out of
// `window.__CLOISTR_CONFIG__=<json>;`.
func parseConfigJS(t *testing.T, body string) map[string]string {
	t.Helper()
	const prefix = "window.__CLOISTR_CONFIG__="
	if !strings.HasPrefix(body, prefix) || !strings.HasSuffix(body, ";") {
		t.Fatalf("unexpected config.js body: %q", body)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(body, prefix), ";")), &out); err != nil {
		t.Fatalf("config.js payload is not JSON: %v (%q)", err, body)
	}
	return out
}

// prodClient is what production's deployment config sets (cloistr-config
// base/vault); the binary no longer defaults any of it.
var prodClient = config.ClientConfig{
	RelayURL:     "wss://relay.cloistr.xyz",
	SignerURL:    "https://signer.cloistr.xyz",
	BlossomURL:   "https://files.cloistr.xyz",
	DiscoveryURL: "https://discover.cloistr.xyz",
	AppURL:       "https://vault.cloistr.xyz",
	Environment:  "production",
}

func TestLoadClientConfig_ReadsEnvironmentWithoutDefaults(t *testing.T) {
	for _, k := range []string{"CLOISTR_RELAY_URL", "CLOISTR_SIGNER_URL", "CLOISTR_BLOSSOM_URL",
		"CLOISTR_DISCOVERY_URL", "CLOISTR_APP_URL", "CLOISTR_ENVIRONMENT"} {
		t.Setenv(k, "")
	}
	if cc := config.LoadClientConfig(); cc != (config.ClientConfig{}) {
		t.Fatalf("LoadClientConfig() with nothing set = %+v; production values must come from deployment config, not code", cc)
	}
	t.Setenv("CLOISTR_SIGNER_URL", "https://signer.staging.cloistr.xyz")
	if cc := config.LoadClientConfig(); cc.SignerURL != "https://signer.staging.cloistr.xyz" {
		t.Fatalf("SignerURL = %q", cc.SignerURL)
	}
}

func TestRuntimeConfig_ServesProductionConfig(t *testing.T) {
	r := runtimeConfigRouter(prodClient)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config.js", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store (a cached config pins a browser to one environment)", got)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("Content-Type = %q", ct)
	}
	cfg := parseConfigJS(t, w.Body.String())
	if cfg["environment"] != "production" || cfg["signerUrl"] != "https://signer.cloistr.xyz" ||
		cfg["relayUrl"] != "wss://relay.cloistr.xyz" || cfg["appUrl"] != "https://vault.cloistr.xyz" {
		t.Fatalf("config = %v", cfg)
	}
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self' https://signer.cloistr.xyz;") {
		t.Fatalf("CSP = %q", csp)
	}
}

func TestRuntimeConfig_StagingEnvironmentReplacesProductionHosts(t *testing.T) {
	r := runtimeConfigRouter(config.ClientConfig{
		SignerURL:   "https://signer.staging.cloistr.xyz",
		RelayURL:    "wss://relay.staging.cloistr.xyz",
		AppURL:      "https://vault.staging.cloistr.xyz",
		Environment: "staging",
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config.js", nil))

	cfg := parseConfigJS(t, w.Body.String())
	if cfg["environment"] != "staging" || cfg["signerUrl"] != "https://signer.staging.cloistr.xyz" ||
		cfg["relayUrl"] != "wss://relay.staging.cloistr.xyz" {
		t.Fatalf("config = %v", cfg)
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://signer.staging.cloistr.xyz") || strings.Contains(csp, "https://signer.cloistr.xyz") {
		t.Fatalf("CSP must allow the staging signer and only it: %q", csp)
	}
}

func TestRuntimeConfig_ValuesAreJSONEncoded(t *testing.T) {
	// A hostile or mistyped value must not be able to break out of the string.
	r := runtimeConfigRouter(config.ClientConfig{SignerURL: `x";alert(1);//`, Environment: "staging"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config.js", nil))
	cfg := parseConfigJS(t, w.Body.String())
	if cfg["signerUrl"] != `x";alert(1);//` {
		t.Fatalf("signerUrl = %q", cfg["signerUrl"])
	}
}

func TestRuntimeConfig_ExactPathOnly(t *testing.T) {
	r := runtimeConfigRouter(prodClient)
	for _, p := range []string{"/config.json", "/config.js/x", "/assets/config.js"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code == http.StatusOK {
			t.Fatalf("%s served the runtime config", p)
		}
	}
}

// With a web build present, /config.js must come from the handler (no-store),
// never from a static file of the same name, while hashed build assets keep a
// long immutable cache. The second half is the control: it shows the no-store
// above is the handler's doing, not an absence of caching everywhere.
func TestRuntimeConfig_BeatsStaticFilesAndAssetsStayCached(t *testing.T) {
	webDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(webDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"index.html":            "<html></html>",
		"config.js":             "window.__CLOISTR_CONFIG__={\"environment\":\"stale-file\"};",
		"assets/index-abc12.js": "console.log(1)",
	} {
		if err := os.WriteFile(filepath.Join(webDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := runtimeConfigRouter(prodClient)
	r.NoRoute(spaHandler(webDir))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config.js", nil))
	if cfg := parseConfigJS(t, w.Body.String()); cfg["environment"] != "production" {
		t.Fatalf("/config.js served the static file, not the handler: %v", cfg)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("/config.js Cache-Control = %q", got)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/index-abc12.js", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("asset status = %d", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control = %q, want the long immutable cache", got)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := w.Header().Get("Cache-Control"); got == "public, max-age=31536000, immutable" {
		t.Fatal("index.html must not get the immutable cache (it names the current bundle)")
	}
}

// HEAD must get the same headers as GET; it used to fall through to the
// static handler and come back "private".
func TestRuntimeConfig_HeadMatchesGet(t *testing.T) {
	r := runtimeConfigRouter(prodClient)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/config.js", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("HEAD Cache-Control = %q, want no-store", got)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("HEAD Content-Type = %q", ct)
	}
}
