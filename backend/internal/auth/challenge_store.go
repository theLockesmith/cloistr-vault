package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// ChallengeStore holds issued login challenges. It must be shared by every
// replica: a challenge issued by one pod has to be redeemable on another.
type ChallengeStore interface {
	Put(ctx context.Context, ch Challenge) error
	// Lookup finds a challenge by the value the client signed. Returns nil, nil
	// when there is none. Expired challenges are still returned so callers can
	// report them as expired.
	Lookup(ctx context.Context, value string) (*Challenge, error)
	// Consume removes the challenge and reports whether THIS call removed it.
	// Exactly one concurrent caller gets true; that is the single-use guarantee.
	Consume(ctx context.Context, id string) (bool, error)
}

// pgChallengeStore keeps challenges in the auth_challenges table.
type pgChallengeStore struct {
	db *sql.DB
}

func newPGChallengeStore(db *sql.DB) *pgChallengeStore {
	return &pgChallengeStore{db: db}
}

func (s *pgChallengeStore) Put(ctx context.Context, ch Challenge) error {
	meta, err := json.Marshal(ch.Metadata)
	if err != nil {
		return fmt.Errorf("encode challenge metadata: %w", err)
	}
	var userID interface{}
	if ch.UserID != uuid.Nil {
		userID = ch.UserID
	}
	// Expired rows are only kept long enough to answer "expired" rather than
	// "unknown"; prune as we go so the table stays bounded by the issue rate.
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM auth_challenges WHERE expires_at < NOW() - INTERVAL '1 hour'`); err != nil {
		return fmt.Errorf("prune challenges: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO auth_challenges (id, value, user_id, metadata, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		ch.ID, ch.Value, userID, meta, ch.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store challenge: %w", err)
	}
	return nil
}

func (s *pgChallengeStore) Lookup(ctx context.Context, value string) (*Challenge, error) {
	var (
		ch     Challenge
		userID uuid.NullUUID
		meta   []byte
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, value, user_id, metadata, expires_at FROM auth_challenges WHERE value = $1`,
		value).Scan(&ch.ID, &ch.Value, &userID, &meta, &ch.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup challenge: %w", err)
	}
	if userID.Valid {
		ch.UserID = userID.UUID
	}
	if len(meta) > 0 {
		if err := json.Unmarshal(meta, &ch.Metadata); err != nil {
			return nil, fmt.Errorf("decode challenge metadata: %w", err)
		}
	}
	return &ch, nil
}

func (s *pgChallengeStore) Consume(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_challenges WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("consume challenge: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("consume challenge: %w", err)
	}
	return n == 1, nil
}

// memoryChallengeStore is a single-process store for tests. Never put it
// behind more than one replica.
type memoryChallengeStore struct {
	mu sync.Mutex
	m  map[string]Challenge
}

func newMemoryChallengeStore() *memoryChallengeStore {
	return &memoryChallengeStore{m: make(map[string]Challenge)}
}

func (s *memoryChallengeStore) Put(_ context.Context, ch Challenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[ch.ID] = ch
	return nil
}

func (s *memoryChallengeStore) Lookup(_ context.Context, value string) (*Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.m {
		if ch.Value == value {
			c := ch
			return &c, nil
		}
	}
	return nil, nil
}

func (s *memoryChallengeStore) Consume(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[id]; !ok {
		return false, nil
	}
	delete(s.m, id)
	return true, nil
}
