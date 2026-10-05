package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func clearClientEnv(t *testing.T) {
	for _, k := range []string{"CLOISTR_RELAY_URL", "CLOISTR_SIGNER_URL", "CLOISTR_BLOSSOM_URL",
		"CLOISTR_DISCOVERY_URL", "CLOISTR_APP_URL", "CLOISTR_ENVIRONMENT"} {
		t.Setenv(k, "")
	}
}

func TestLoadClientConfig_DefaultsAreProduction(t *testing.T) {
	clearClientEnv(t)
	cc := config.LoadClientConfig()
	want := config.ClientConfig{
		RelayURL:     "wss://relay.cloistr.xyz",
		SignerURL:    "https://signer.cloistr.xyz",
		BlossomURL:   "https://files.cloistr.xyz",
		DiscoveryURL: "https://discover.cloistr.xyz",
		AppURL:       "https://vault.cloistr.xyz",
		Environment:  "production",
	}
	if cc != want {
		t.Fatalf("LoadClientConfig() = %+v, want %+v", cc, want)
	}
}

func TestRuntimeConfig_ServesProductionWithNoEnvironment(t *testing.T) {
	clearClientEnv(t)
	r := runtimeConfigRouter(config.LoadClientConfig())

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
	clearClientEnv(t)
	t.Setenv("CLOISTR_SIGNER_URL", "https://signer.staging.cloistr.xyz")
	t.Setenv("CLOISTR_RELAY_URL", "wss://relay.staging.cloistr.xyz")
	t.Setenv("CLOISTR_APP_URL", "https://vault.staging.cloistr.xyz")
	t.Setenv("CLOISTR_ENVIRONMENT", "staging")
	r := runtimeConfigRouter(config.LoadClientConfig())

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
	r := runtimeConfigRouter(config.LoadClientConfig())
	for _, p := range []string{"/config.json", "/config.js/x", "/assets/config.js"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code == http.StatusOK {
			t.Fatalf("%s served the runtime config", p)
		}
	}
}
