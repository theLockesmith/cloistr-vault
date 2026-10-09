package api_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"

	"github.com/coldforge/vault/internal/api"
	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/config"
	"github.com/coldforge/vault/internal/security"
	"github.com/coldforge/vault/internal/testutil/pgtest"
	"github.com/coldforge/vault/internal/vault"
)

// Passkey register + login, end to end: the production router from
// api.SetupRouter, real HTTP requests, a real Postgres with every migration
// applied, and a software authenticator that produces genuine P-256
// attestations and assertions.
//
// Passkey login never worked in production from February to October 2026:
// the finish-login handler drained the request body before the WebAuthn
// parser read it. Unit tests passed throughout. This test fails if any link
// in the chain breaks, from routing and middleware to signature verification
// and the credential/session rows in the database.
//
// Skipped unless VAULT_TEST_PG_DSN is set; see internal/testutil/pgtest.

const (
	e2eRPID   = "vault.test"
	e2eOrigin = "https://vault.test"
)

var b64 = base64.RawURLEncoding

func newE2EServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := pgtest.FreshDB(t)

	authService := auth.NewAuthService(db.DB, nil)
	if err := authService.InitWebAuthn(e2eRPID, e2eOrigin, "Vault E2E"); err != nil {
		t.Fatal(err)
	}
	entryService := vault.NewEntryService(db)
	folderService := vault.NewFolderService(db)
	tagService := vault.NewTagService(db)
	router := api.SetupRouter(
		authService,
		vault.NewService(db),
		folderService,
		entryService,
		vault.NewSecretService(db),
		vault.NewPasswordService(db),
		tagService,
		vault.NewSearchService(db, entryService, folderService, tagService),
		security.NewSecurityService(db),
		vault.NewAttachmentService(db),
		vault.NewSharingService(db),
		"", "", config.ClientConfig{},
	)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

// authenticator is a software passkey: one P-256 key, one credential ID.
type authenticator struct {
	key       *ecdsa.PrivateKey
	credID    []byte
	signCount uint32
}

func newAuthenticator(t *testing.T) *authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	return &authenticator{key: key, credID: id}
}

func clientData(t *testing.T, typ, challenge string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]string{
		"type":      typ,
		"challenge": challenge,
		"origin":    e2eOrigin,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// authData: rpIdHash(32) | flags(1) | signCount(4) | [attestedCredentialData]
func (a *authenticator) authData(flags byte, attested []byte) []byte {
	rpHash := sha256.Sum256([]byte(e2eRPID))
	out := append([]byte{}, rpHash[:]...)
	out = append(out, flags)
	out = binary.BigEndian.AppendUint32(out, a.signCount)
	return append(out, attested...)
}

const (
	flagUP = 0x01
	flagUV = 0x04
	flagAT = 0x40
)

// attestation answers navigator.credentials.create() with "none" attestation.
func (a *authenticator) attestation(t *testing.T, challenge string) []byte {
	t.Helper()
	enc, err := cbor.CTAP2EncOptions().EncMode()
	if err != nil {
		t.Fatal(err)
	}
	coseKey, err := enc.Marshal(map[int]interface{}{
		1:  2,  // kty: EC2
		3:  -7, // alg: ES256
		-1: 1,  // crv: P-256
		-2: a.key.X.FillBytes(make([]byte, 32)),
		-3: a.key.Y.FillBytes(make([]byte, 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	attested := make([]byte, 16) // AAGUID, all zero
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(a.credID)))
	attested = append(attested, a.credID...)
	attested = append(attested, coseKey...)

	attObj, err := enc.Marshal(map[string]interface{}{
		"fmt":      "none",
		"attStmt":  map[string]interface{}{},
		"authData": a.authData(flagUP|flagUV|flagAT, attested),
	})
	if err != nil {
		t.Fatal(err)
	}
	return mustJSON(t, map[string]interface{}{
		"id":    b64.EncodeToString(a.credID),
		"rawId": b64.EncodeToString(a.credID),
		"type":  "public-key",
		"response": map[string]string{
			"attestationObject": b64.EncodeToString(attObj),
			"clientDataJSON":    b64.EncodeToString(clientData(t, "webauthn.create", challenge)),
		},
	})
}

// assertion answers navigator.credentials.get(), signed with signer (normally
// a.key). routing carries email or session_id, sent alongside in one body as
// the web client does.
func (a *authenticator) assertion(t *testing.T, signer *ecdsa.PrivateKey, challenge string, userHandle []byte, routing map[string]string) []byte {
	t.Helper()
	a.signCount++
	authData := a.authData(flagUP|flagUV, nil)
	cd := clientData(t, "webauthn.get", challenge)
	cdHash := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, authData...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, signer, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	resp := map[string]interface{}{
		"authenticatorData": b64.EncodeToString(authData),
		"clientDataJSON":    b64.EncodeToString(cd),
		"signature":         b64.EncodeToString(sig),
	}
	if userHandle != nil {
		resp["userHandle"] = b64.EncodeToString(userHandle)
	}
	body := map[string]interface{}{
		"id":       b64.EncodeToString(a.credID),
		"rawId":    b64.EncodeToString(a.credID),
		"type":     "public-key",
		"response": resp,
	}
	for k, v := range routing {
		body[k] = v
	}
	return mustJSON(t, body)
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// call sends one request, checks its status, decodes the JSON reply into out
// (if non-nil) and returns the raw reply.
func call(t *testing.T, srv *httptest.Server, method, path, token string, body []byte, wantStatus int, out interface{}) string {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var raw bytes.Buffer
	_, _ = raw.ReadFrom(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, wantStatus, raw.String())
	}
	if out != nil {
		if err := json.Unmarshal(raw.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, raw.String(), err)
		}
	}
	return raw.String()
}

type publicKeyOptions struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"publicKey"`
}

type loginResult struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

func TestPasskeyE2E_RegisterThenLogin(t *testing.T) {
	srv := newE2EServer(t)
	email := fmt.Sprintf("passkey-e2e-%s@example.test", uuid.NewString())

	// 1. Email/password account and its session.
	var reg loginResult
	call(t, srv, http.MethodPost, "/api/v1/auth/register", "", mustJSON(t, map[string]interface{}{
		"method":     "email",
		"email":      email,
		"password":   "correct horse battery staple",
		"vault_data": []byte(`{"v":2}`),
	}), http.StatusCreated, &reg)
	if reg.Token == "" || reg.User.ID == "" {
		t.Fatalf("register returned no token/user: %+v", reg)
	}

	// 2. Register a passkey on it.
	authn := newAuthenticator(t)
	var creation publicKeyOptions
	call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/begin", reg.Token, nil, http.StatusOK, &creation)
	userHandle, err := b64.DecodeString(creation.PublicKey.User.ID)
	if err != nil || len(userHandle) == 0 {
		t.Fatalf("user handle %q: %v", creation.PublicKey.User.ID, err)
	}
	call(t, srv, http.MethodPost, "/api/v1/user/webauthn/register/finish", reg.Token,
		authn.attestation(t, creation.PublicKey.Challenge), http.StatusCreated, nil)

	var list struct {
		Credentials []json.RawMessage `json:"credentials"`
	}
	call(t, srv, http.MethodGet, "/api/v1/user/webauthn/credentials", reg.Token, nil, http.StatusOK, &list)
	if len(list.Credentials) != 1 {
		t.Fatalf("want 1 registered credential, got %d", len(list.Credentials))
	}

	// assertLogin finishes a login and checks the session it returns works.
	assertLogin := func(t *testing.T, name string, finish []byte) {
		t.Helper()
		var res loginResult
		call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "", finish, http.StatusOK, &res)
		if res.Token == "" || res.User.ID != reg.User.ID {
			t.Fatalf("%s: login returned token=%q user=%q, want user %q", name, res.Token, res.User.ID, reg.User.ID)
		}
		// The new session is a working one.
		var profile map[string]interface{}
		call(t, srv, http.MethodGet, "/api/v1/user/profile", res.Token, nil, http.StatusOK, &profile)
	}

	// 3. Login by email.
	var opts publicKeyOptions
	call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin", "",
		mustJSON(t, map[string]string{"email": email}), http.StatusOK, &opts)
	byEmail := authn.assertion(t, authn.key, opts.PublicKey.Challenge, userHandle, map[string]string{"email": email})
	assertLogin(t, "email login", byEmail)

	// Replaying that exact assertion: its login session is spent.
	if got := call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "", byEmail, http.StatusBadRequest, nil); !strings.Contains(got, "session not found") {
		t.Fatalf("replay refused for the wrong reason: %s", got)
	}

	// 4. Discoverable (usernameless) login.
	var disc struct {
		Options   publicKeyOptions `json:"options"`
		SessionID string           `json:"session_id"`
	}
	call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin/discoverable", "", nil, http.StatusOK, &disc)
	if disc.SessionID == "" {
		t.Fatal("discoverable begin returned no session_id")
	}
	assertLogin(t, "discoverable login",
		authn.assertion(t, authn.key, disc.Options.PublicKey.Challenge, userHandle, map[string]string{"session_id": disc.SessionID}))

	// 5. Right credential ID, wrong private key.
	impostor, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/begin", "",
		mustJSON(t, map[string]string{"email": email}), http.StatusOK, &opts)
	if got := call(t, srv, http.MethodPost, "/api/v1/auth/webauthn/login/finish", "",
		authn.assertion(t, impostor, opts.PublicKey.Challenge, userHandle, map[string]string{"email": email}),
		http.StatusBadRequest, nil); !strings.Contains(got, "Authentication failed") {
		t.Fatalf("forged assertion refused for the wrong reason: %s", got)
	}
}
