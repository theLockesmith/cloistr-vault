package auth

import (
	"bytes"
	"database/sql"
	"errors"
	"log"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/observability"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

// Each test here drives one database call whose error used to be dropped
// on the floor, makes that call fail, and asserts the failure is either
// returned or logged. Without the check, the test fails.

var errDBDown = errors.New("db down: connection reset")

// captureLogs routes both loggers this package uses into one buffer.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevSlog := observability.Logger
	observability.Logger = slog.New(slog.NewTextHandler(&buf, nil))
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		observability.Logger = prevSlog
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func mockAuth(t *testing.T) (*AuthService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &AuthService{db: db, challenges: newMemoryChallengeStore()}, mock
}

func TestStoreWebAuthnSession_ReturnsDeleteFailure(t *testing.T) {
	a, mock := mockAuth(t)
	mock.ExpectExec(`DELETE FROM webauthn_sessions`).WillReturnError(errDBDown)
	// No INSERT is expected: a failed clear must stop the store.

	err := a.storeWebAuthnSession(uuid.New(), &webauthn.SessionData{Challenge: "c"}, "registration")
	if err == nil || !errors.Is(err, errDBDown) {
		t.Fatalf("storeWebAuthnSession = %v, want the DELETE failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWebAuthnSession_LogsFailure(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectExec(`DELETE FROM webauthn_sessions`).WillReturnError(errDBDown)

	a.deleteWebAuthnSession(uuid.New(), "authentication")
	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("failure not logged: %q", logs.String())
	}
}

func TestDeleteDiscoverableSession_LogsFailure(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectExec(`DELETE FROM webauthn_sessions`).WillReturnError(errDBDown)

	a.deleteDiscoverableSession("sess")
	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("failure not logged: %q", logs.String())
	}
}

func TestValidateSession_ExpiredCleanupFailureIsLogged(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectQuery(`FROM users u`).WithArgs("tok").WillReturnRows(
		sqlmock.NewRows([]string{"id", "email", "created_at", "updated_at", "expires_at", "type", "nostr_pubkey"}).
			AddRow(uuid.New(), "a@b", time.Now(), time.Now(), time.Now().Add(-time.Hour), "email", nil))
	mock.ExpectExec(`DELETE FROM sessions`).WithArgs("tok").WillReturnError(errDBDown)

	if _, err := a.ValidateSession("tok"); err != ErrInvalidCredentials {
		t.Fatalf("expired session: %v, want ErrInvalidCredentials", err)
	}
	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("cleanup failure not logged: %q", logs.String())
	}
}

func TestGetDisplayNameForUser_LightningLookupFailureIsLogged(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectQuery(`am.type = 'nostr'`).WillReturnRows(sqlmock.NewRows([]string{"nostr_pubkey", "nip05_address"}))
	mock.ExpectQuery(`am.type = 'lightning_address'`).WillReturnError(errDBDown)

	_ = a.GetDisplayNameForUser(uuid.New())
	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("lookup failure not logged: %q", logs.String())
	}
}

// No row is the normal case and must stay quiet.
func TestGetDisplayNameForUser_NoLightningAddressIsQuiet(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectQuery(`am.type = 'nostr'`).WillReturnRows(sqlmock.NewRows([]string{"nostr_pubkey", "nip05_address"}))
	mock.ExpectQuery(`am.type = 'lightning_address'`).WillReturnError(sql.ErrNoRows)

	_ = a.GetDisplayNameForUser(uuid.New())
	if logs.Len() != 0 {
		t.Fatalf("ErrNoRows was logged: %q", logs.String())
	}
}

func TestPopulateUserDisplayInfo_LightningLookupFailureIsLogged(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectQuery(`ORDER BY CASE am.type`).WillReturnRows(sqlmock.NewRows([]string{"nostr_pubkey", "nip05_address", "type"}))
	mock.ExpectQuery(`am.type = 'lightning_address'`).WillReturnError(errDBDown)

	_ = a.PopulateUserDisplayInfo(&models.User{ID: uuid.New()})
	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("lookup failure not logged: %q", logs.String())
	}
}

func TestGetUserForWebAuthn_DisplayNameFailureIsLogged(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectQuery(`SELECT email FROM users`).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("a@b"))
	mock.ExpectQuery(`COALESCE\(nip05_address`).WillReturnError(errDBDown)

	if _, err := a.getUserForWebAuthn(uuid.New()); err != nil {
		t.Fatalf("display name failure should not fail the lookup: %v", err)
	}
	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("display name failure not logged: %q", logs.String())
	}
}

func TestGetWebAuthnCredentials_UnreadableTransportsAreLogged(t *testing.T) {
	logs := captureLogs(t)
	a, mock := mockAuth(t)
	mock.ExpectQuery(`FROM webauthn_credentials`).WillReturnRows(sqlmock.NewRows([]string{
		"credential_id", "public_key", "sign_count", "aaguid", "transports",
		"flags_user_present", "flags_user_verified", "flags_backup_eligible", "flags_backup_state",
	}).AddRow([]byte("id"), []byte("pk"), 1, []byte{}, "{not json", true, true, false, false))

	creds, err := a.getWebAuthnCredentials(uuid.New())
	if err != nil || len(creds) != 1 {
		t.Fatalf("credentials = %v, %v; a bad transports column must not drop the credential", creds, err)
	}
	if !strings.Contains(logs.String(), "transports") {
		t.Fatalf("unreadable transports not logged: %q", logs.String())
	}
}
