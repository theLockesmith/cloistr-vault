package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// A passkey registration challenge is single use, like a login challenge.
// Finish-registration used to read the challenge, validate, and delete it
// only after storing the credential, so several finishes racing on one
// challenge each registered a credential, and a failed finish left the
// challenge for another try.
//
// Skipped unless VAULT_TEST_PG_DSN is set; see internal/testutil/pgtest.

// emailAccount registers an email account and returns its session token.
func emailAccount(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	var reg loginResult
	call(t, srv, http.MethodPost, "/api/v1/auth/register", "", mustJSON(t, map[string]interface{}{
		"method":     "email",
		"email":      fmt.Sprintf("passkey-regclaim-%s@example.test", uuid.NewString()),
		"auth_key":   zkAuthKey,
		"vault_data": []byte(`{"v":2}`),
	}), http.StatusCreated, &reg)
	return reg.Token
}

func credentialCount(t *testing.T, srv *httptest.Server, token string) int {
	t.Helper()
	var list struct {
		Credentials []json.RawMessage `json:"credentials"`
	}
	call(t, srv, http.MethodGet, "/api/v1/user/webauthn/credentials", token, nil, http.StatusOK, &list)
	return len(list.Credentials)
}

func TestPasskeyRegistration_ConcurrentFinish_OneChallengeOneCredential(t *testing.T) {
	srv := newE2EServer(t)
	token := emailAccount(t, srv)

	for round := 0; round < replayRounds; round++ {
		var creation publicKeyOptions
		call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/begin", token, nil, http.StatusOK, &creation)

		// replayFanout distinct passkeys, each answering the same challenge.
		bodies := make([][]byte, replayFanout)
		for i := range bodies {
			bodies[i] = newAuthenticator(t).attestation(t, creation.PublicKey.Challenge)
		}

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			created int
			start   = make(chan struct{})
		)
		for _, body := range bodies {
			wg.Add(1)
			go func(body []byte) {
				defer wg.Done()
				<-start
				req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/user/webauthn/register/finish", bytes.NewReader(body))
				if err != nil {
					t.Error(err)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := srv.Client().Do(req)
				if err != nil {
					t.Error(err)
					return
				}
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusCreated {
					mu.Lock()
					created++
					mu.Unlock()
				}
			}(body)
		}
		close(start)
		wg.Wait()

		if created != 1 {
			t.Fatalf("round %d: %d concurrent finishes on one challenge registered %d credentials, want 1",
				round, replayFanout, created)
		}
	}
	if got := credentialCount(t, srv, token); got != replayRounds {
		t.Fatalf("%d challenges left %d credentials stored, want %d", replayRounds, got, replayRounds)
	}
}

func TestPasskeyRegistration_FailedFinishSpendsChallenge(t *testing.T) {
	srv := newE2EServer(t)
	token := emailAccount(t, srv)

	var creation publicKeyOptions
	call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/begin", token, nil, http.StatusOK, &creation)

	// An attestation over the wrong challenge fails validation.
	call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/finish", token,
		newAuthenticator(t).attestation(t, b64.EncodeToString([]byte("not-the-challenge"))), http.StatusBadRequest, nil)

	got := call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/finish", token,
		newAuthenticator(t).attestation(t, creation.PublicKey.Challenge), http.StatusBadRequest, nil)
	if !strings.Contains(got, "session not found") {
		t.Fatalf("challenge reused after a failed finish: %s", got)
	}
	if n := credentialCount(t, srv, token); n != 0 {
		t.Fatalf("%d credentials stored, want 0", n)
	}
}
