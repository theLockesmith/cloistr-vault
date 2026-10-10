// Package ratelimit counts requests per client in fixed windows.
package ratelimit

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Store keeps the counters in Postgres (the rate_limits table), so every
// replica sees the same count for a client.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Hit records one request against key and returns the number of requests in
// the current window, including this one, and how long until that window
// ends. The remainder is computed by the database, so the caller's clock
// never has to agree with it.
//
// The window's end is written only when the window opens: on first insert, or
// when a hit finds the previous window over. Later hits increment the count
// and leave expires_at alone. Moving it on every hit would keep a client that
// retries steadily locked out forever (cloistr-signer !217).
//
// One statement, so concurrent hits from any replica serialise on the row
// lock and each is counted exactly once.
func (s *Store) Hit(ctx context.Context, key string, window time.Duration) (int, time.Duration, error) {
	var (
		count   int
		resetIn float64
	)
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO rate_limits AS r (key, count, expires_at)
		VALUES ($1, 1, NOW() + make_interval(secs => $2))
		ON CONFLICT (key) DO UPDATE SET
			count      = CASE WHEN r.expires_at <= NOW() THEN 1 ELSE r.count + 1 END,
			expires_at = CASE WHEN r.expires_at <= NOW() THEN EXCLUDED.expires_at ELSE r.expires_at END
		RETURNING count, EXTRACT(EPOCH FROM expires_at - NOW())::float8`,
		key, window.Seconds()).Scan(&count, &resetIn)
	if err != nil {
		return 0, 0, fmt.Errorf("rate limit hit: %w", err)
	}
	return count, time.Duration(resetIn * float64(time.Second)), nil
}

// Prune deletes counters whose window has ended and returns how many it removed.
func (s *Store) Prune(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM rate_limits WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, fmt.Errorf("prune rate limits: %w", err)
	}
	return res.RowsAffected()
}
