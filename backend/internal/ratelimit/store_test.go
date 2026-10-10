package ratelimit_test

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/coldforge/vault/internal/ratelimit"
	"github.com/coldforge/vault/internal/testutil/pgtest"
)

func TestHitCountsWithinWindow(t *testing.T) {
	db := pgtest.FreshDB(t)
	s := ratelimit.NewStore(db.DB)
	ctx := context.Background()

	for want := 1; want <= 3; want++ {
		got, resetIn, err := s.Hit(ctx, "api:198.51.100.9", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("hit %d: count = %d", want, got)
		}
		if resetIn < 55*time.Second || resetIn > time.Minute {
			t.Fatalf("hit %d: window ends in %v, want about a minute", want, resetIn)
		}
	}

	// Another key has its own count.
	if got, _, err := s.Hit(ctx, "api:203.0.113.77", time.Minute); err != nil || got != 1 {
		t.Fatalf("second key: count = %d, err = %v", got, err)
	}
}

// The window's end is fixed when the window opens. Later hits must not push
// it forward, or a client that keeps retrying is never let back in.
func TestHitDoesNotSlideTheWindow(t *testing.T) {
	db := pgtest.FreshDB(t)
	s := ratelimit.NewStore(db.DB)
	ctx := context.Background()

	_, first, err := s.Hit(ctx, "k", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	_, second, err := s.Hit(ctx, "k", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// A fixed end is now about 1.1s closer; a sliding one would be back at 60s.
	if second > first-time.Second {
		t.Fatalf("window end moved on a later hit: %v left, then %v left 1.1s later", first, second)
	}
}

func TestHitStartsANewWindowAfterExpiry(t *testing.T) {
	db := pgtest.FreshDB(t)
	s := ratelimit.NewStore(db.DB)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, _, err := s.Hit(ctx, "k", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE rate_limits SET expires_at = NOW() - INTERVAL '1 second' WHERE key = 'k'`); err != nil {
		t.Fatal(err)
	}
	got, resetIn, err := s.Hit(ctx, "k", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("count after expiry = %d, want 1", got)
	}
	if resetIn < 55*time.Second {
		t.Fatalf("new window ends in %v, want about a minute", resetIn)
	}
}

// Replicas hit the same row at once. Every hit must be counted exactly once.
func TestHitIsAtomicUnderConcurrency(t *testing.T) {
	db := pgtest.FreshDB(t)
	s := ratelimit.NewStore(db.DB)
	ctx := context.Background()

	const n = 40
	counts := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _, err := s.Hit(ctx, "k", time.Minute)
			if err != nil {
				t.Error(err)
			}
			counts[i] = c
		}(i)
	}
	wg.Wait()
	sort.Ints(counts)
	for i, c := range counts {
		if c != i+1 {
			t.Fatalf("counts = %v, want 1..%d each once", counts, n)
		}
	}
}

func TestPruneRemovesOnlyEndedWindows(t *testing.T) {
	db := pgtest.FreshDB(t)
	s := ratelimit.NewStore(db.DB)
	ctx := context.Background()

	for _, k := range []string{"old", "live"} {
		if _, _, err := s.Hit(ctx, k, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE rate_limits SET expires_at = NOW() - INTERVAL '1 second' WHERE key = 'old'`); err != nil {
		t.Fatal(err)
	}
	n, err := s.Prune(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned %d rows, want 1", n)
	}
	var left []string
	rows, err := db.Query(`SELECT key FROM rate_limits`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		left = append(left, k)
	}
	if len(left) != 1 || left[0] != "live" {
		t.Fatalf("rows left = %v, want [live]", left)
	}
}
