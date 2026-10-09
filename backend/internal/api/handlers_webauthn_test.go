package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coldforge/vault/internal/auth"
	"github.com/gin-gonic/gin"
)

// assertionBody is shaped exactly like the web client's
// POST /auth/webauthn/login/finish: the routing field (session_id or email)
// and the credential assertion side by side in ONE JSON object.
func assertionBody(t *testing.T, extra map[string]string) []byte {
	t.Helper()
	b64 := base64.RawURLEncoding.EncodeToString
	clientData, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": b64([]byte("challenge-bytes-0123456789abcdef")),
		"origin":    "https://vault.cloistr.xyz",
	})
	// rpIdHash (32) + flags (1: user present) + signCount (4)
	authData := append(make([]byte, 32), 0x01, 0, 0, 0, 1)
	body := map[string]interface{}{
		"id":    b64([]byte("credential-id")),
		"rawId": b64([]byte("credential-id")),
		"type":  "public-key",
		"response": map[string]interface{}{
			"authenticatorData": b64(authData),
			"clientDataJSON":    b64(clientData),
			"signature":         b64([]byte("not-a-real-signature")),
			"userHandle":        nil,
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The handler used to bind the routing fields with ShouldBindJSON, which
// drains the request body, and then hand the drained body to the WebAuthn
// parser. Every real login failed with "Invalid credential response".
func TestWebAuthnFinishLogin_ParsesCredentialAfterReadingRoutingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandlers(auth.NewAuthService(nil, nil), nil)

	for _, extra := range []map[string]string{
		{"session_id": "some-session"},
		{"email": "someone@example.com"},
	} {
		r := gin.New()
		r.POST("/finish", h.WebAuthnFinishLogin)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewReader(assertionBody(t, extra)))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// WebAuthn is not configured on this service, so the login itself
		// fails, but only AFTER the credential was parsed and routed.
		if strings.Contains(w.Body.String(), "Invalid credential response") {
			t.Fatalf("%v: credential never reached the parser: %d %s", extra, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Either email or session_id is required") {
			t.Fatalf("%v: routing field was lost: %s", extra, w.Body.String())
		}
	}
}

// The endpoint is unauthenticated; a real assertion is ~2 KB. An oversized
// body must be refused before it is read into memory.
func TestWebAuthnFinishLogin_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandlers(auth.NewAuthService(nil, nil), nil)
	r := gin.New()
	r.POST("/finish", h.WebAuthnFinishLogin)

	big := append([]byte(`{"session_id":"x","pad":"`), bytes.Repeat([]byte("a"), 1<<20)...)
	big = append(big, []byte(`"}`)...)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("1 MiB body: status = %d, want 413 (%s)", w.Code, w.Body.String())
	}
}

// Parser and ceremony error details stay in server logs, not in responses.
func TestWebAuthnFinishLogin_DoesNotEchoParserErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandlers(auth.NewAuthService(nil, nil), nil)
	r := gin.New()
	r.POST("/finish", h.WebAuthnFinishLogin)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/finish", strings.NewReader(`{"session_id":"x","id":"%%%"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "Parse error") || strings.Contains(w.Body.String(), ": ") && strings.Contains(w.Body.String(), "Invalid credential response:") {
		t.Fatalf("parser detail leaked to client: %s", w.Body.String())
	}
}
