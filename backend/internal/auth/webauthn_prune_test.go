package auth

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

// Discoverable passkey challenges carry no user, so the per-user clear in
// storeWebAuthnSession never reaches them, and the login page's passkey
// autofill issues one on every visit and renews it every few minutes. Most
// are never redeemed. Issuing a new one must prune expired ones, or the
// table grows without bound.

func TestStoreDiscoverableSession_PrunesExpiredFirst(t *testing.T) {
	a, mock := mockAuth(t)
	mock.ExpectExec(`DELETE FROM webauthn_sessions WHERE expires_at < NOW\(\) - INTERVAL '1 hour'`).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`INSERT INTO webauthn_sessions`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := a.storeDiscoverableSession(uuid.NewString(), &webauthn.SessionData{Challenge: "c"}); err != nil {
		t.Fatalf("storeDiscoverableSession: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreDiscoverableSession_ReturnsPruneFailure(t *testing.T) {
	a, mock := mockAuth(t)
	mock.ExpectExec(`DELETE FROM webauthn_sessions`).WillReturnError(errDBDown)

	err := a.storeDiscoverableSession(uuid.NewString(), &webauthn.SessionData{Challenge: "c"})
	if err == nil || !errors.Is(err, errDBDown) {
		t.Fatalf("storeDiscoverableSession = %v, want the prune failure", err)
	}
}
