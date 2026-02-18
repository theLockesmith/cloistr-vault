package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/coldforge/vault/internal/models"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/google/uuid"
)

// GenerateLightningChallenge generates a k1 challenge for LNURL-auth
func (a *AuthService) GenerateLightningChallenge(lightningAddress string) (*Challenge, error) {
	// Validate Lightning Address format (basic check)
	if lightningAddress == "" {
		return nil, fmt.Errorf("lightning address is required")
	}

	// Generate cryptographically secure k1 challenge (32 bytes per LNURL-auth spec)
	k1Bytes := make([]byte, 32)
	if _, err := rand.Read(k1Bytes); err != nil {
		return nil, fmt.Errorf("failed to generate k1 challenge: %w", err)
	}
	k1Hex := hex.EncodeToString(k1Bytes)

	challenge := &Challenge{
		ID:        uuid.New().String(),
		Value:     k1Hex,
		ExpiresAt: time.Now().Add(10 * time.Minute), // LNURL-auth typically uses longer expiry
		Metadata: map[string]interface{}{
			"lightning_address": lightningAddress,
			"auth_type":         "lnurl_auth",
			"issued_at":         time.Now().Unix(),
			"purpose":           "authentication",
		},
	}

	// Store challenge temporarily
	challengeStore[challenge.ID] = *challenge

	log.Printf("Generated LNURL-auth challenge for Lightning Address: %s", lightningAddress)
	return challenge, nil
}

// AuthenticateWithLightning handles LNURL-auth signature-based authentication
func (a *AuthService) AuthenticateWithLightning(lightningAddress, signature, k1, linkingKey string) (*models.User, string, error) {
	// Validate required fields
	if lightningAddress == "" || signature == "" || k1 == "" || linkingKey == "" {
		return nil, "", fmt.Errorf("lightning_address, signature, k1, and linking_key are required")
	}

	log.Printf("LNURL-auth authentication attempt for: %s", lightningAddress)

	// Verify the k1 challenge exists and is valid
	challenge, exists := challengeStore[k1]
	if !exists {
		// Also check if k1 is the challenge value itself (alternative flow)
		found := false
		for id, ch := range challengeStore {
			if ch.Value == k1 {
				challenge = ch
				delete(challengeStore, id)
				found = true
				break
			}
		}
		if !found {
			return nil, "", fmt.Errorf("invalid or expired k1 challenge")
		}
	} else {
		delete(challengeStore, k1)
	}

	if time.Now().After(challenge.ExpiresAt) {
		return nil, "", fmt.Errorf("k1 challenge has expired")
	}

	// Verify the LNURL-auth signature using secp256k1
	valid, err := verifyLNURLAuthSignature(k1, signature, linkingKey)
	if err != nil {
		return nil, "", fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return nil, "", fmt.Errorf("invalid signature")
	}

	// Check if user exists with this Lightning Address
	var user models.User
	err = a.db.QueryRow(`
		SELECT u.id, u.email, u.created_at, u.updated_at
		FROM users u
		JOIN auth_methods am ON u.id = am.user_id
		WHERE am.identifier = $1 AND am.type = 'lightning_address'`,
		lightningAddress).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			// Auto-create user from Lightning Address
			return a.createLightningUser(lightningAddress, linkingKey)
		}
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Populate extended user fields
	user.AuthMethod = "lightning_address"
	user.LightningAddress = lightningAddress
	user.DisplayName = extractLightningUsername(lightningAddress)

	// Generate session token
	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = a.db.Exec("INSERT INTO sessions (id, user_id, token, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)",
		uuid.New(), user.ID, token, expiresAt, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("LNURL-auth authentication successful for user: %s", user.ID.String())
	return &user, token, nil
}

// createLightningUser auto-creates a user account from Lightning Address
func (a *AuthService) createLightningUser(lightningAddress, linkingKey string) (*models.User, string, error) {
	log.Printf("Auto-creating user from Lightning Address: %s", lightningAddress)

	// Begin transaction
	tx, err := a.db.Begin()
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create user with Lightning-derived email
	userID := uuid.New()
	now := time.Now()
	username := extractLightningUsername(lightningAddress)
	email := fmt.Sprintf("%s@lightning.local", username)

	_, err = tx.Exec("INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		userID, email, now, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Create Lightning auth method
	authMethodID := uuid.New()
	_, err = tx.Exec(`
		INSERT INTO auth_methods (id, user_id, type, identifier, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		authMethodID, userID, "lightning_address", lightningAddress, now, now)
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
		ID:               userID,
		Email:            email,
		CreatedAt:        now,
		UpdatedAt:        now,
		AuthMethod:       "lightning_address",
		LightningAddress: lightningAddress,
		DisplayName:      username,
	}

	log.Printf("Auto-created Lightning user: %s with display name: %s", userID.String(), user.DisplayName)
	return user, token, nil
}

// extractLightningUsername extracts the username part from a Lightning Address
func extractLightningUsername(lightningAddress string) string {
	for i, c := range lightningAddress {
		if c == '@' {
			return lightningAddress[:i]
		}
	}
	// Return first 12 chars if no @ found
	if len(lightningAddress) > 12 {
		return lightningAddress[:12]
	}
	return lightningAddress
}

// verifyLNURLAuthSignature verifies an LNURL-auth signature using secp256k1
// LNURL-auth (LUD-04) specifies:
// - k1: 32-byte random challenge (hex-encoded)
// - sig: 64-byte compact signature (hex-encoded, R || S format)
// - key: compressed public key (33 bytes, hex-encoded)
func verifyLNURLAuthSignature(k1Hex, signatureHex, linkingKeyHex string) (bool, error) {
	// Decode the k1 challenge (32 bytes)
	k1Bytes, err := hex.DecodeString(k1Hex)
	if err != nil {
		return false, fmt.Errorf("invalid k1 challenge hex: %w", err)
	}
	if len(k1Bytes) != 32 {
		return false, fmt.Errorf("k1 must be 32 bytes, got %d", len(k1Bytes))
	}

	// Decode the signature (64 bytes: 32 for R, 32 for S - compact format)
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	// LNURL-auth uses compact 64-byte signatures (R || S)
	if len(sigBytes) != 64 {
		return false, fmt.Errorf("signature must be 64 bytes (compact), got %d", len(sigBytes))
	}

	// Decode the linking key (33 bytes compressed)
	keyBytes, err := hex.DecodeString(linkingKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid linking key hex: %w", err)
	}

	// Parse the public key
	pubKey, err := secp256k1.ParsePubKey(keyBytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse linking key: %w", err)
	}

	// Parse the compact signature
	// Compact format: first 32 bytes = R, last 32 bytes = S
	var r, s secp256k1.ModNScalar
	r.SetByteSlice(sigBytes[:32])
	s.SetByteSlice(sigBytes[32:])
	signature := ecdsa.NewSignature(&r, &s)

	// Hash the k1 challenge for verification
	// According to LUD-04, the message signed is sha256(k1)
	msgHash := sha256.Sum256(k1Bytes)

	// Verify the signature
	return signature.Verify(msgHash[:], pubKey), nil
}
