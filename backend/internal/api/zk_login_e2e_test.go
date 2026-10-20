package api_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/coldforge/vault/internal/crypto"
	"github.com/coldforge/vault/internal/database"
)

// Email accounts sign in with an auth key the client derives from the master
// password; the password itself never reaches the server. These tests drive
// the real router against a real Postgres (skipped without VAULT_TEST_PG_DSN).

const zkPassword = "correct horse battery staple"

// What the client sends instead of the password; any 64-hex value works for
// the server, which only ever sees this.
var zkAuthKey = strings.Repeat("ab", 32)

func zkPost(t *testing.T, url string, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, out
}

func zkRegister(t *testing.T, base, email, authKey string) {
	t.Helper()
	status, body := zkPost(t, base+"/api/v1/auth/register", map[string]any{
		"method": "email", "email": email, "auth_key": authKey, "vault_data": []byte("envelope"),
	})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("register = %d %v", status, body)
	}
}

func zkLogin(t *testing.T, base string, body map[string]any) (int, map[string]any) {
	t.Helper()
	body["method"] = "email"
	return zkPost(t, base+"/api/v1/auth/login", body)
}

// insertLegacyAccount writes an email account the way every build before this
// change did: the verifier is over the master password itself.
func insertLegacyAccount(t *testing.T, db *database.DB, email, password string) {
	t.Helper()
	salt, err := crypto.GenerateSalt()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := crypto.HashPassword(password, salt)
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, email); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO auth_methods (id, user_id, type, identifier, salt, password_hash)
		VALUES ($1, $2, 'email', $3, $4, $5)`, uuid.New(), userID, email, salt, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO vaults (id, user_id, encrypted_data) VALUES ($1, $2, $3)`,
		uuid.New(), userID, []byte("legacy-envelope")); err != nil {
		t.Fatal(err)
	}
}

func authVersion(t *testing.T, db *database.DB, email string) int {
	t.Helper()
	var v int
	if err := db.QueryRow(`SELECT auth_version FROM auth_methods WHERE type = 'email' AND identifier = $1`, email).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestZKLogin_RegisterAndLoginWithAuthKeyOnly(t *testing.T) {
	srv, db := newE2EServerWithDB(t)
	zkRegister(t, srv.URL, "new@example.com", zkAuthKey)
	if v := authVersion(t, db, "new@example.com"); v != 2 {
		t.Fatalf("new account auth_version = %d, want 2", v)
	}

	if status, body := zkLogin(t, srv.URL, map[string]any{"email": "new@example.com", "auth_key": zkAuthKey}); status != http.StatusOK || body["token"] == nil {
		t.Fatalf("login with auth key = %d %v", status, body)
	}
	wrong := strings.Repeat("cd", 32)
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "new@example.com", "auth_key": wrong}); status != http.StatusUnauthorized {
		t.Fatalf("wrong auth key = %d, want 401", status)
	}
	// A migrated account never accepts the password again, even if correct
	// for some other reason: the server must not be able to ask for it.
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "new@example.com", "password": zkPassword}); status != http.StatusUnauthorized {
		t.Fatalf("password on a v2 account = %d, want 401", status)
	}
}

func TestZKLogin_RegisterRefusesAPassword(t *testing.T) {
	srv, _ := newE2EServerWithDB(t)
	status, body := zkPost(t, srv.URL+"/api/v1/auth/register", map[string]any{
		"method": "email", "email": "pw@example.com", "password": zkPassword, "vault_data": []byte("envelope"),
	})
	if status != http.StatusBadRequest {
		t.Fatalf("register with a password = %d %v, want 400", status, body)
	}
	status, _ = zkPost(t, srv.URL+"/api/v1/auth/register", map[string]any{
		"method": "email", "email": "pw2@example.com", "auth_key": "not-hex", "vault_data": []byte("envelope"),
	})
	if status != http.StatusBadRequest {
		t.Fatalf("register with a malformed auth key = %d, want 400", status)
	}
}

func TestZKLogin_LegacyAccountUpgradesOnFirstNewLogin(t *testing.T) {
	srv, db := newE2EServerWithDB(t)
	insertLegacyAccount(t, db, "old@example.com", zkPassword)

	// 1. The new client sends only the auth key; the server cannot check it
	// against a password verifier and asks for the one-time upgrade.
	status, body := zkLogin(t, srv.URL, map[string]any{"email": "old@example.com", "auth_key": zkAuthKey})
	if status != http.StatusConflict || body["code"] != "AUTH_UPGRADE_REQUIRED" {
		t.Fatalf("auth key on a legacy account = %d %v, want 409 AUTH_UPGRADE_REQUIRED", status, body)
	}

	// 2. A wrong password does not upgrade anything.
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "old@example.com", "password": "wrong", "auth_key": zkAuthKey}); status != http.StatusUnauthorized {
		t.Fatalf("wrong password during upgrade = %d, want 401", status)
	}
	if v := authVersion(t, db, "old@example.com"); v != 1 {
		t.Fatalf("auth_version after failed upgrade = %d, want 1", v)
	}

	// 3. The upgrade login: password once, plus the auth key to store.
	if status, body := zkLogin(t, srv.URL, map[string]any{"email": "old@example.com", "password": zkPassword, "auth_key": zkAuthKey}); status != http.StatusOK || body["token"] == nil {
		t.Fatalf("upgrade login = %d %v", status, body)
	}
	if v := authVersion(t, db, "old@example.com"); v != 2 {
		t.Fatalf("auth_version after upgrade = %d, want 2", v)
	}

	// 4. From now on the auth key alone signs in, and the password is refused.
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "old@example.com", "auth_key": zkAuthKey}); status != http.StatusOK {
		t.Fatalf("auth key after upgrade = %d, want 200", status)
	}
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "old@example.com", "password": zkPassword}); status != http.StatusUnauthorized {
		t.Fatalf("password after upgrade = %d, want 401", status)
	}
}

func TestZKLogin_LegacyAccountStillOpensForAnOldClient(t *testing.T) {
	// A tab loaded before the deploy sends only the password. It keeps
	// working (that password was already being sent) and leaves the account
	// on v1 for the new client to upgrade.
	srv, db := newE2EServerWithDB(t)
	insertLegacyAccount(t, db, "tab@example.com", zkPassword)
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "tab@example.com", "password": zkPassword}); status != http.StatusOK {
		t.Fatalf("old client login = %d, want 200", status)
	}
	if v := authVersion(t, db, "tab@example.com"); v != 1 {
		t.Fatalf("auth_version = %d, want 1", v)
	}
}

func TestZKLogin_UnknownEmailLooksLikeAWrongKey(t *testing.T) {
	srv, _ := newE2EServerWithDB(t)
	zkRegister(t, srv.URL, "exists@example.com", zkAuthKey)
	wrong := strings.Repeat("cd", 32)

	s1, b1 := zkLogin(t, srv.URL, map[string]any{"email": "exists@example.com", "auth_key": wrong})
	s2, b2 := zkLogin(t, srv.URL, map[string]any{"email": "nobody@example.com", "auth_key": wrong})
	if s1 != http.StatusUnauthorized || s2 != http.StatusUnauthorized {
		t.Fatalf("statuses = %d, %d; want 401, 401", s1, s2)
	}
	j1, _ := json.Marshal(b1)
	j2, _ := json.Marshal(b2)
	if string(j1) != string(j2) {
		t.Fatalf("bodies differ, so they reveal which email exists:\n%s\n%s", j1, j2)
	}

	// Both paths must run the slow verifier. Without the dummy hash an
	// unknown email answers in microseconds instead of tens of milliseconds.
	median := func(body map[string]any) time.Duration {
		var d []time.Duration
		for i := 0; i < 5; i++ {
			start := time.Now()
			zkLogin(t, srv.URL, body)
			d = append(d, time.Since(start))
		}
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		return d[len(d)/2]
	}
	known := median(map[string]any{"email": "exists@example.com", "auth_key": wrong})
	unknown := median(map[string]any{"email": "nobody@example.com", "auth_key": wrong})
	if ratio := float64(unknown) / float64(known); ratio < 0.5 || ratio > 2 {
		t.Fatalf("unknown email took %v, known email %v (ratio %.2f): timing reveals which exists", unknown, known, ratio)
	}
}

func TestZKLogin_AuthKeyIsNormalisedHex(t *testing.T) {
	// Upper-case hex from some other client must sign in to the same account.
	srv, _ := newE2EServerWithDB(t)
	zkRegister(t, srv.URL, "hex@example.com", zkAuthKey)
	upper := strings.ToUpper(zkAuthKey)
	if _, err := hex.DecodeString(upper); err != nil {
		t.Fatal(err)
	}
	if status, _ := zkLogin(t, srv.URL, map[string]any{"email": "hex@example.com", "auth_key": upper}); status != http.StatusOK {
		t.Fatalf("upper-case auth key = %d, want 200", status)
	}
}
