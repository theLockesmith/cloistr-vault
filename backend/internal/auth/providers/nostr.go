package providers

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
	
	"github.com/coldforge/vault/internal/crypto"
	"github.com/google/uuid"
)

// NostrProvider implements Nostr-based authentication
type NostrProvider struct {
	db             *sql.DB
	challengeStore map[string]*Challenge // In production, use Redis
}

// NewNostrProvider creates a new Nostr authentication provider
func NewNostrProvider(db *sql.DB) *NostrProvider {
	return &NostrProvider{
		db:             db,
		challengeStore: make(map[string]*Challenge),
	}
}

// GetType returns the authentication type
func (p *NostrProvider) GetType() string {
	return "nostr"
}

// GetRequiredFields returns required fields for Nostr auth
func (p *NostrProvider) GetRequiredFields() []string {
	return []string{"public_key", "signature", "challenge"}
}

// GetOptionalFields returns optional fields
func (p *NostrProvider) GetOptionalFields() []string {
	return []string{"profile_metadata"}
}

// SupportsChallenge indicates this provider uses challenge-response
func (p *NostrProvider) SupportsChallenge() bool {
	return true
}

// GenerateChallenge creates a challenge for Nostr authentication
func (p *NostrProvider) GenerateChallenge(ctx context.Context, identifier string) (*Challenge, error) {
	// Validate public key format
	if len(identifier) != 64 {
		return nil, fmt.Errorf("invalid nostr public key length: expected 64 chars, got %d", len(identifier))
	}
	
	if _, err := hex.DecodeString(identifier); err != nil {
		return nil, fmt.Errorf("invalid nostr public key format: %w", err)
	}
	
	// Verify user exists
	var userID uuid.UUID
	err := p.db.QueryRowContext(ctx, `
		SELECT u.id 
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.nostr_pubkey = $1 AND am.type = 'nostr'`,
		identifier).Scan(&userID)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found for public key")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Generate challenge
	challengeValue, err := crypto.GenerateChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}
	
	challenge := &Challenge{
		ID:        uuid.New().String(),
		Value:     challengeValue,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Metadata: map[string]interface{}{
			"user_id":    userID.String(),
			"public_key": identifier,
			"issued_at":  time.Now().Unix(),
		},
	}
	
	// Store challenge
	p.challengeStore[challenge.ID] = challenge
	
	// Cleanup expired challenges
	go p.cleanupExpiredChallenges()
	
	return challenge, nil
}

// ValidateCredentials validates Nostr signature authentication
func (p *NostrProvider) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*AuthResult, error) {
	// Extract required fields
	publicKey, ok := credentials["public_key"].(string)
	if !ok || publicKey == "" {
		return nil, fmt.Errorf("public_key is required")
	}
	
	signature, ok := credentials["signature"].(string)
	if !ok || signature == "" {
		return nil, fmt.Errorf("signature is required")
	}
	
	challengeID, ok := credentials["challenge"].(string)
	if !ok || challengeID == "" {
		return nil, fmt.Errorf("challenge is required")
	}
	
	// Verify challenge exists and is valid
	challenge, exists := p.challengeStore[challengeID]
	if !exists {
		return nil, fmt.Errorf("invalid or expired challenge")
	}
	
	if time.Now().After(challenge.ExpiresAt) {
		delete(p.challengeStore, challengeID)
		return nil, fmt.Errorf("challenge expired")
	}
	
	// Verify challenge belongs to this public key
	challengePublicKey, ok := challenge.Metadata["public_key"].(string)
	if !ok || challengePublicKey != publicKey {
		return nil, fmt.Errorf("challenge does not match public key")
	}
	
	// Verify signature
	if !crypto.VerifyNostrSignature(challenge.Value, signature, publicKey) {
		return nil, fmt.Errorf("invalid signature")
	}
	
	// Clean up used challenge
	delete(p.challengeStore, challengeID)
	
	// Get user info
	var user struct {
		ID        uuid.UUID
		Email     string
		CreatedAt time.Time
	}
	
	err := p.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.created_at
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.nostr_pubkey = $1 AND am.type = 'nostr'`,
		publicKey).Scan(&user.ID, &user.Email, &user.CreatedAt)
	
	if err != nil {
		return nil, fmt.Errorf("user lookup failed: %w", err)
	}
	
	// Calculate trust score based on account age, etc.
	trustScore := p.calculateTrustScore(user.CreatedAt)
	
	return &AuthResult{
		UserID:      user.ID,
		Identifier:  publicKey,
		DisplayName: fmt.Sprintf("%.8s...%.8s", publicKey[:8], publicKey[56:]),
		Metadata: map[string]interface{}{
			"public_key":   publicKey,
			"auth_method":  "nostr",
			"account_age":  time.Since(user.CreatedAt).Hours() / 24, // days
		},
		RequiresMFA: false, // Nostr signatures are inherently strong
		TrustScore:  trustScore,
	}, nil
}

// PrepareRegistration prepares Nostr registration data
func (p *NostrProvider) PrepareRegistration(ctx context.Context, data map[string]interface{}) (*RegistrationData, error) {
	publicKey, ok := data["public_key"].(string)
	if !ok || publicKey == "" {
		return nil, fmt.Errorf("public_key is required for Nostr registration")
	}
	
	// Validate public key format
	if len(publicKey) != 64 {
		return nil, fmt.Errorf("invalid public key length: expected 64 chars, got %d", len(publicKey))
	}
	
	if _, err := hex.DecodeString(publicKey); err != nil {
		return nil, fmt.Errorf("invalid public key format: %w", err)
	}
	
	// Validate public key cryptographically
	_, err := crypto.NostrPublicKeyFromHex(publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid nostr public key: %w", err)
	}
	
	// Check if public key is already registered
	var exists bool
	err = p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth_methods 
			WHERE nostr_pubkey = $1 AND type = 'nostr'
		)`, publicKey).Scan(&exists)
	
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	if exists {
		return nil, fmt.Errorf("public key already registered")
	}
	
	// Extract optional profile metadata
	profileMetadata, _ := data["profile_metadata"].(map[string]interface{})
	
	return &RegistrationData{
		Identifier: publicKey,
		AuthData: map[string]interface{}{
			"public_key": publicKey,
			"key_hash":   hex.EncodeToString(crypto.HashNostrPublicKey(publicKey)),
		},
		Metadata: map[string]interface{}{
			"profile":    profileMetadata,
			"registered": time.Now().Unix(),
			"method":     "nostr",
		},
		RequiresVerification: false, // Cryptographic proof is sufficient
	}, nil
}

// CompleteRegistration finalizes Nostr registration
func (p *NostrProvider) CompleteRegistration(ctx context.Context, userID uuid.UUID, data *RegistrationData) error {
	publicKey := data.Identifier
	
	// Create auth method record
	authMethodID := uuid.New()
	now := time.Now()
	
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO auth_methods (
			id, user_id, type, identifier, nostr_pubkey, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		authMethodID, userID, "nostr", publicKey, publicKey, now, now)
	
	if err != nil {
		return fmt.Errorf("failed to create auth method: %w", err)
	}
	
	return nil
}

// calculateTrustScore calculates a trust score based on various factors
func (p *NostrProvider) calculateTrustScore(createdAt time.Time) float64 {
	// Base score
	score := 0.7
	
	// Account age bonus (up to +0.2)
	accountAgeDays := time.Since(createdAt).Hours() / 24
	ageBonus := min(accountAgeDays/365*0.2, 0.2) // Max bonus for 1+ year old account
	
	return min(score+ageBonus, 1.0)
}

// cleanupExpiredChallenges removes expired challenges
func (p *NostrProvider) cleanupExpiredChallenges() {
	now := time.Now()
	for id, challenge := range p.challengeStore {
		if now.After(challenge.ExpiresAt) {
			delete(p.challengeStore, id)
		}
	}
}

// Helper function for min
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}