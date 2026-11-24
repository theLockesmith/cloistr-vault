package api_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// A passkey login challenge is single use. Before the claim was made atomic,
// finish read the challenge, validated, and deleted it afterwards, so the
// same assertion submitted concurrently minted a session per request that
// read the row before the first delete landed (cloistr-audit's e2e run: one
// assertion sent 8x concurrently, 12 sessions from 5 assertions). A failed
// finish also left the challenge in place for another attempt.
//
// Skipped unless VAULT_TEST_PG_DSN is set; see internal/testutil/pgtest.

const (
	replayFanout = 8
	// The race window is a few queries wide; one round can miss it.
	replayRounds = 10
)

// passkeyUser registers an email account with one passkey and returns the
// server, its email, the authenticator and the WebAuthn user handle.
func passkeyUser(t *testing.T) (*httptest.Server, string, *authenticator, []byte) {
	t.Helper()
	srv := newE2EServer(t)
	email := fmt.Sprintf("passkey-claim-%s@example.test", uuid.NewString())

	var reg loginResult
	call(t, srv, http.MethodPost, "/api/v1/auth/register", "", mustJSON(t, map[string]interface{}{
		"method":     "email",
		"email":      email,
		"auth_key":   zkAuthKey,
		"vault_data": []byte(`{"v":2}`),
	}), http.StatusCreated, &reg)

	authn := newAuthenticator(t)
	var creation publicKeyOptions
	call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/begin", reg.Token, nil, http.StatusOK, &creation)
	userHandle, err := b64.DecodeString(creation.PublicKey.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/finish", reg.Token,
		authn.attestation(t, creation.PublicKey.Challenge), http.StatusCreated, nil)
	return srv, email, authn, userHandle
}

// finishConcurrently sends body to login/finish replayFanout times at once
// and returns how many came back 200.
func finishConcurrently(t *testing.T, srv *httptest.Server, body []byte) int {
	t.Helper()
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		ok    int
		start = make(chan struct{})
	)
	for i := 0; i < replayFanout; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resp, err := srv.Client().Post(srv.URL+"/api/v1/auth/webauthn/login/finish",
				"application/json", bytes.NewReader(body))
			if err != nil {
				t.Error(err)
				return
			}
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	return ok
}

func TestPasskeyChallenge_ConcurrentReplay_EmailLogin(t *testing.T) {
	srv, email, authn, userHandle := passkeyUser(t)

	for round := 0; round < replayRounds; round++ {
		var opts publicKeyOptions
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin", "",
			mustJSON(t, map[string]string{"email": email}), http.StatusOK, &opts)
		finish := authn.assertion(t, authn.key, opts.PublicKey.Challenge, userHandle, map[string]string{"email": email})

		if got := finishConcurrently(t, srv, finish); got != 1 {
			t.Fatalf("round %d: one assertion sent %dx concurrently minted %d sessions, want 1", round, replayFanout, got)
		}
	}
}

func TestPasskeyChallenge_ConcurrentReplay_DiscoverableLogin(t *testing.T) {
	srv, _, authn, userHandle := passkeyUser(t)

	for round := 0; round < replayRounds; round++ {
		var disc struct {
			Options   publicKeyOptions `json:"options"`
			SessionID string           `json:"session_id"`
		}
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin/discoverable", "", nil, http.StatusOK, &disc)
		finish := authn.assertion(t, authn.key, disc.Options.PublicKey.Challenge, userHandle,
			map[string]string{"session_id": disc.SessionID})

		if got := finishConcurrently(t, srv, finish); got != 1 {
			t.Fatalf("round %d: one assertion sent %dx concurrently minted %d sessions, want 1", round, replayFanout, got)
		}
	}
}

func TestPasskeyChallenge_FailedFinishSpendsChallenge(t *testing.T) {
	srv, email, authn, userHandle := passkeyUser(t)
	impostor, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("email", func(t *testing.T) {
		var opts publicKeyOptions
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin", "",
			mustJSON(t, map[string]string{"email": email}), http.StatusOK, &opts)
		routing := map[string]string{"email": email}
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "",
			authn.assertion(t, impostor, opts.PublicKey.Challenge, userHandle, routing), http.StatusBadRequest, nil)

		got := call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "",
			authn.assertion(t, authn.key, opts.PublicKey.Challenge, userHandle, routing), http.StatusBadRequest, nil)
		if !strings.Contains(got, "session not found") {
			t.Fatalf("challenge reused after a failed finish: %s", got)
		}
	})

	t.Run("discoverable", func(t *testing.T) {
		var disc struct {
			Options   publicKeyOptions `json:"options"`
			SessionID string           `json:"session_id"`
		}
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin/discoverable", "", nil, http.StatusOK, &disc)
		routing := map[string]string{"session_id": disc.SessionID}
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "",
			authn.assertion(t, impostor, disc.Options.PublicKey.Challenge, userHandle, routing), http.StatusBadRequest, nil)

		got := call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "",
			authn.assertion(t, authn.key, disc.Options.PublicKey.Challenge, userHandle, routing), http.StatusBadRequest, nil)
		if !strings.Contains(got, "session not found") {
			t.Fatalf("challenge reused after a failed finish: %s", got)
		}
	})
}
