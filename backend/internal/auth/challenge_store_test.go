package auth

import (
	"context"
	"encoding/hex"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/btcsuite/btcd/btcec/v2"
	btcschnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/google/uuid"
)

func newTestAuth() *AuthService {
	return &AuthService{challenges: newMemoryChallengeStore()}
}

func TestMemoryChallengeStore_LookupByValue(t *testing.T) {
	s := newMemoryChallengeStore()
	ctx := context.Background()
	ch := Challenge{ID: "id-1", Value: "val-1", ExpiresAt: time.Now().Add(time.Minute)}
	if err := s.Put(ctx, ch); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Lookup(ctx, "val-1"); err != nil || got == nil || got.ID != "id-1" {
		t.Fatalf("Lookup(val-1) = %v, %v", got, err)
	}
	// The ID is not a valid thing to sign; it must not match.
	if got, _ := s.Lookup(ctx, "id-1"); got != nil {
		t.Fatalf("Lookup(id) = %v, want nil", got)
	}
	if got, _ := s.Lookup(ctx, "missing"); got != nil {
		t.Fatalf("Lookup(missing) = %v, want nil", got)
	}
}

func TestMemoryChallengeStore_ConsumeHasOneWinner(t *testing.T) {
	s := newMemoryChallengeStore()
	ctx := context.Background()
	_ = s.Put(ctx, Challenge{ID: "id-1", Value: "v", ExpiresAt: time.Now().Add(time.Minute)})

	var wins int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := s.Consume(ctx, "id-1"); ok {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("Consume winners = %d, want exactly 1", wins)
	}
}

func TestPGChallengeStore_ConsumeReportsLoser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := newPGChallengeStore(db)

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM auth_challenges WHERE id = $1`)).
		WithArgs("id-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM auth_challenges WHERE id = $1`)).
		WithArgs("id-1").WillReturnResult(sqlmock.NewResult(0, 0))

	if ok, err := s.Consume(context.Background(), "id-1"); err != nil || !ok {
		t.Fatalf("first Consume = %v, %v; want true", ok, err)
	}
	if ok, err := s.Consume(context.Background(), "id-1"); err != nil || ok {
		t.Fatalf("second Consume = %v, %v; want false", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPGChallengeStore_LookupRoundTrip(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := newPGChallengeStore(db)
	exp := time.Now().Add(time.Minute).UTC()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM auth_challenges WHERE value = $1`)).
		WithArgs("val-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "value", "user_id", "metadata", "expires_at"}).
			AddRow("id-1", "val-1", nil, []byte(`{"pubkey":"abc"}`), exp))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM auth_challenges WHERE value = $1`)).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id", "value", "user_id", "metadata", "expires_at"}))

	got, err := s.Lookup(context.Background(), "val-1")
	if err != nil || got == nil {
		t.Fatalf("Lookup = %v, %v", got, err)
	}
	if got.ID != "id-1" || got.Metadata["pubkey"] != "abc" || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("Lookup returned %+v", got)
	}
	if got, err := s.Lookup(context.Background(), "missing"); err != nil || got != nil {
		t.Fatalf("Lookup(missing) = %v, %v; want nil, nil", got, err)
	}
}

// A challenge issued by one replica must be redeemable on the other, exactly
// once. Both services share one store, as both pods share one database.
func TestAuthenticateWithNostr_CrossReplicaAdmitsOnce(t *testing.T) {
	shared := newMemoryChallengeStore()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	podA := &AuthService{challenges: shared}
	podB := &AuthService{challenges: shared, db: db}

	privKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))

	ch, err := podA.GenerateNostrChallengePublic(pubKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	eventJSON := makeTestEvent(t, privKey, ch.Value)

	userID := uuid.New()
	mock.ExpectQuery(`FROM users u`).WithArgs(pubKeyHex).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "created_at", "updated_at"}).
			AddRow(userID, "x@nostr.local", time.Now(), time.Now()))
	mock.ExpectExec(`INSERT INTO sessions`).WillReturnResult(sqlmock.NewResult(0, 1))

	user, token, err := podB.AuthenticateWithNostr(pubKeyHex, eventJSON)
	if err != nil {
		t.Fatalf("valid signature on the other replica refused: %v", err)
	}
	if user.ID != userID || token == "" {
		t.Fatalf("got user %v token %q", user.ID, token)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := podA.AuthenticateWithNostr(pubKeyHex, eventJSON); err != ErrInvalidChallenge {
		t.Fatalf("replay on first replica = %v, want ErrInvalidChallenge", err)
	}
}
