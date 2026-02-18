package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/coldforge/vault/internal/identity"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// GenerateNostrChallengePublic generates a challenge for any Nostr pubkey
func (a *AuthService) GenerateNostrChallengePublic(pubkey string) (*Challenge, error) {
	// Validate pubkey format
	if len(pubkey) != 64 {
		return nil, fmt.Errorf("invalid pubkey format: expected 64 hex characters")
	}

	// Validate pubkey is valid hex
	if _, err := hex.DecodeString(pubkey); err != nil {
		return nil, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	// Generate cryptographically secure challenge
	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	challengeHex := hex.EncodeToString(challengeBytes)

	challenge := &Challenge{
		ID:        uuid.New().String(),
		Value:     challengeHex,
		ExpiresAt: time.Now().Add(10 * time.Minute), // Longer for crypto auth
		Metadata: map[string]interface{}{
			"pubkey":     pubkey,
			"auth_type":  "nostr",
			"issued_at":  time.Now().Unix(),
			"purpose":    "authentication",
		},
	}

	// Store challenge temporarily
	challengeStore[challenge.ID] = *challenge

	log.Printf("Generated Nostr challenge for pubkey: %s", pubkey[:16]+"...")
	return challenge, nil
}

// AuthenticateWithNostr handles Nostr signature-based authentication
func (a *AuthService) AuthenticateWithNostr(pubkey, signature, challenge string) (*models.User, string, error) {
	// For demo, accept any non-empty signature
	// In production, implement full signature verification
	if signature == "" || challenge == "" || pubkey == "" {
		return nil, "", fmt.Errorf("signature, challenge, and pubkey are required")
	}

	log.Printf("Nostr authentication attempt for pubkey: %s", pubkey[:16]+"...")

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
			// Auto-create user from Nostr pubkey
			return a.createNostrUser(pubkey)
		}
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Populate extended user fields for Nostr users
	user.AuthMethod = "nostr"
	user.NostrPubkey = pubkey
	user.DisplayName = identity.FormatNpubShort(pubkey)

	// Generate session token for existing user - use existing createSession method
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

	// Begin transaction
	tx, err := a.db.Begin()
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create user with Nostr-derived email
	userID := uuid.New()
	now := time.Now()

	// Use pubkey as email for Nostr users
	email := fmt.Sprintf("%s@nostr.local", pubkey[:16])

	_, err = tx.Exec("INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		userID, email, now, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Create Nostr auth method
	authMethodID := uuid.New()
	_, err = tx.Exec("INSERT INTO auth_methods (id, user_id, type, identifier, nostr_pubkey, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		authMethodID, userID, "nostr", pubkey, pubkey, now, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create auth method: %w", err)
	}

	// Create empty initial vault
	err = a.createInitialVault(tx, userID, []byte("[]"))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create initial vault: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Generate session token
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

