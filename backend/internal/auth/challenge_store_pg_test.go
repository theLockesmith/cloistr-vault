package auth

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// Runs against a real Postgres with migrations applied. Skipped unless
// VAULT_TEST_PG_DSN is set, e.g.
// VAULT_TEST_PG_DSN="host=localhost port=55432 user=postgres password=x dbname=postgres sslmode=disable"
func pgStoreForTest(t *testing.T) *pgChallengeStore {
	t.Helper()
	dsn := os.Getenv("VAULT_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("VAULT_TEST_PG_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return newPGChallengeStore(db)
}

func TestPGChallengeStore_Integration_SingleWinnerAcrossConnections(t *testing.T) {
	s := pgStoreForTest(t)
	ctx := context.Background()
	ch := Challenge{
		ID:        uuid.New().String(),
		Value:     uuid.New().String(),
		ExpiresAt: time.Now().Add(time.Minute),
		Metadata:  map[string]interface{}{"pubkey": "abc"},
	}
	if err := s.Put(ctx, ch); err != nil {
		t.Fatal(err)
	}
	got, err := s.Lookup(ctx, ch.Value)
	if err != nil || got == nil || got.ID != ch.ID || got.Metadata["pubkey"] != "abc" {
		t.Fatalf("Lookup = %+v, %v", got, err)
	}

	var wins int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.Consume(ctx, ch.ID)
			if err != nil {
				t.Error(err)
			}
			if ok {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("Consume winners = %d, want exactly 1", wins)
	}
	if got, _ := s.Lookup(ctx, ch.Value); got != nil {
		t.Fatalf("challenge still present after consume: %+v", got)
	}
}

func TestPGChallengeStore_Integration_PrunesOldExpired(t *testing.T) {
	s := pgStoreForTest(t)
	ctx := context.Background()
	old := Challenge{ID: uuid.New().String(), Value: uuid.New().String(), ExpiresAt: time.Now().Add(-2 * time.Hour)}
	recent := Challenge{ID: uuid.New().String(), Value: uuid.New().String(), ExpiresAt: time.Now().Add(-time.Minute)}
	for _, c := range []Challenge{old, recent} {
		if err := s.Put(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// The next Put prunes anything expired more than an hour ago.
	if err := s.Put(ctx, Challenge{ID: uuid.New().String(), Value: uuid.New().String(), ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Lookup(ctx, old.Value); got != nil {
		t.Fatal("challenge expired 2h ago was not pruned")
	}
	if got, _ := s.Lookup(ctx, recent.Value); got == nil {
		t.Fatal("recently expired challenge was pruned too early (needed for the 'expired' error)")
	}
}
