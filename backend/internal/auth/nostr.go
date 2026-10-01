package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/coldforge/vault/internal/crypto"
	"github.com/coldforge/vault/internal/identity"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// GenerateNostrChallengePublic generates a challenge for any Nostr pubkey
func (a *AuthService) GenerateNostrChallengePublic(pubkey string) (*Challenge, error) {
	if len(pubkey) != 64 {
		return nil, fmt.Errorf("invalid pubkey format: expected 64 hex characters")
	}

	if _, err := hex.DecodeString(pubkey); err != nil {
		return nil, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	challengeHex := hex.EncodeToString(challengeBytes)

	challenge := &Challenge{
		ID:        uuid.New().String(),
		Value:     challengeHex,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Metadata: map[string]interface{}{
			"pubkey":    pubkey,
			"auth_type": "nostr",
			"issued_at": time.Now().Unix(),
			"purpose":   "authentication",
		},
	}

	challengeMu.Lock()
	challengeStore[challenge.ID] = *challenge
	challengeMu.Unlock()

	log.Printf("Generated Nostr challenge for pubkey: %s", pubkey[:16]+"...")
	return challenge, nil
}

// AuthenticateWithNostr verifies a NIP-42 style signed event and authenticates the user.
func (a *AuthService) AuthenticateWithNostr(pubkey, signedEventJSON string) (*models.User, string, error) {
	if pubkey == "" || signedEventJSON == "" {
		return nil, "", fmt.Errorf("pubkey and signed_event are required")
	}

	var event crypto.NostrEvent
	if err := json.Unmarshal([]byte(signedEventJSON), &event); err != nil {
		return nil, "", fmt.Errorf("invalid signed event: %w", err)
	}

	if event.Kind != 22242 {
		return nil, "", fmt.Errorf("invalid event kind: expected 22242, got %d", event.Kind)
	}

	if event.PubKey != pubkey {
		return nil, "", ErrInvalidCredentials
	}

	challengeValue := event.TagValue("challenge")
	if challengeValue == "" {
		return nil, "", ErrInvalidChallenge
	}

	// Look up challenge by value (client sends the value, store is keyed by ID)
	challengeMu.RLock()
	var storedChallenge Challenge
	var challengeKey string
	found := false
	for id, ch := range challengeStore {
		if ch.Value == challengeValue {
			storedChallenge = ch
			challengeKey = id
			found = true
			break
		}
	}
	challengeMu.RUnlock()

	if !found {
		return nil, "", ErrInvalidChallenge
	}

	if time.Now().After(storedChallenge.ExpiresAt) {
		challengeMu.Lock()
		delete(challengeStore, challengeKey)
		challengeMu.Unlock()
		return nil, "", ErrChallengeExpired
	}

	storedPubkey, ok := storedChallenge.Metadata["pubkey"].(string)
	if !ok || storedPubkey != pubkey {
		return nil, "", ErrInvalidChallenge
	}

	// NIP-42: created_at within ±5 minutes
	eventTime := time.Unix(event.CreatedAt, 0)
	diff := time.Since(eventTime)
	if diff < -5*time.Minute || diff > 5*time.Minute {
		return nil, "", ErrInvalidCredentials
	}

	if err := crypto.VerifyNostrEventSignature(&event); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// All checks passed — consume the challenge (single-use)
	challengeMu.Lock()
	delete(challengeStore, challengeKey)
	challengeMu.Unlock()

	log.Printf("Nostr authentication verified for pubkey: %s", pubkey[:16]+"...")

	// Check if user exists with this pubkey
	var user models.User
	err := a.db.QueryRow(`
		SELECT u.id, u.email, u.created_at, u.updated_at
		FROM users u
		JOIN auth_methods am ON u.id = am.user_id
		WHERE am.nostr_pubkey = $1 AND am.type = 'nostr'`,
		pubkey).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return a.createNostrUser(pubkey)
		}
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	user.AuthMethod = "nostr"
	user.NostrPubkey = pubkey
	user.DisplayName = identity.FormatNpubShort(pubkey)

	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = a.db.Exec("INSERT INTO sessions (id, user_id, token, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)",
		uuid.New(), user.ID, token, expiresAt, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("Nostr authentication successful for user: %s", user.ID.String())
	return &user, token, nil
}

// createNostrUser auto-creates a user account from Nostr public key
func (a *AuthService) createNostrUser(pubkey string) (*models.User, string, error) {
	log.Printf("Auto-creating user from Nostr pubkey: %s", pubkey[:16]+"...")

	tx, err := a.db.Begin()
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	userID := uuid.New()
	now := time.Now()
	email := fmt.Sprintf("%s@nostr.local", pubkey[:16])

	_, err = tx.Exec("INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		userID, email, now, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	authMethodID := uuid.New()
	_, err = tx.Exec("INSERT INTO auth_methods (id, user_id, type, identifier, nostr_pubkey, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		authMethodID, userID, "nostr", pubkey, pubkey, now, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create auth method: %w", err)
	}

	err = a.createInitialVault(tx, userID, []byte("[]"))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create initial vault: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = a.db.Exec("INSERT INTO sessions (id, user_id, token, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)",
		uuid.New(), userID, token, expiresAt, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	user := &models.User{
		ID:          userID,
		Email:       email,
		CreatedAt:   now,
		UpdatedAt:   now,
		AuthMethod:  "nostr",
		NostrPubkey: pubkey,
		DisplayName: identity.FormatNpubShort(pubkey),
	}

	log.Printf("Auto-created Nostr user: %s with display name: %s", userID.String(), user.DisplayName)
	return user, token, nil
}
