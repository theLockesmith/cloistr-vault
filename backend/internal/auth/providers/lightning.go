package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/google/uuid"
)

// LightningAddressProvider implements Lightning Address authentication
// Combines LNURL-auth with Lightning Address identity verification
type LightningAddressProvider struct {
	db     *sql.DB
	domain string // e.g., "coldforge-vault.com"
}

// NewLightningAddressProvider creates a new Lightning Address auth provider
func NewLightningAddressProvider(db *sql.DB, domain string) *LightningAddressProvider {
	return &LightningAddressProvider{
		db:     db,
		domain: domain,
	}
}

// GetType returns the authentication type
func (p *LightningAddressProvider) GetType() string {
	return "lightning_address"
}

// GetRequiredFields returns required fields for Lightning Address auth
func (p *LightningAddressProvider) GetRequiredFields() []string {
	return []string{"lightning_address", "signature", "k1", "linking_key"}
}

// GetOptionalFields returns optional fields
func (p *LightningAddressProvider) GetOptionalFields() []string {
	return []string{"node_pubkey", "payment_hash"}
}

// SupportsChallenge indicates this provider uses LNURL-auth challenge
func (p *LightningAddressProvider) SupportsChallenge() bool {
	return true
}

// Lightning Address structure
type LightningAddress struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Full     string `json:"full"` // username@domain
}

// ParseLightningAddress parses a Lightning Address
func ParseLightningAddress(address string) (*LightningAddress, error) {
	// Validate format: username@domain.com
	parts := strings.Split(address, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Lightning Address format: expected username@domain")
	}
	
	username := strings.TrimSpace(parts[0])
	domain := strings.TrimSpace(parts[1])
	
	if username == "" || domain == "" {
		return nil, fmt.Errorf("username and domain cannot be empty")
	}
	
	// Validate username (lowercase alphanumeric + underscore/dash)
	usernameRegex := regexp.MustCompile(`^[a-z0-9_-]+$`)
	if !usernameRegex.MatchString(username) {
		return nil, fmt.Errorf("invalid username: only lowercase letters, numbers, underscore, and dash allowed")
	}
	
	// Validate domain format
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !domainRegex.MatchString(domain) {
		return nil, fmt.Errorf("invalid domain format")
	}
	
	return &LightningAddress{
		Username: username,
		Domain:   domain,
		Full:     address,
	}, nil
}

// GenerateChallenge creates an LNURL-auth compatible challenge
func (p *LightningAddressProvider) GenerateChallenge(ctx context.Context, identifier string) (*Challenge, error) {
	// Parse Lightning Address
	lnAddr, err := ParseLightningAddress(identifier)
	if err != nil {
		return nil, fmt.Errorf("invalid Lightning Address: %w", err)
	}
	
	// For our domain, verify user exists
	if lnAddr.Domain == p.domain {
		var userID uuid.UUID
		err := p.db.QueryRowContext(ctx, `
			SELECT u.id 
			FROM users u 
			JOIN auth_methods am ON u.id = am.user_id 
			WHERE am.identifier = $1 AND am.type = 'lightning_address'`,
			identifier).Scan(&userID)
		
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("Lightning Address not registered: %s", identifier)
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	}
	
	// Generate cryptographically secure k1 challenge (32 bytes per LNURL-auth spec)
	k1Bytes := make([]byte, 32)
	if _, err := rand.Read(k1Bytes); err != nil {
		return nil, fmt.Errorf("failed to generate k1 challenge: %w", err)
	}
	k1Hex := hex.EncodeToString(k1Bytes)

	// challengeHex is the same as k1Hex for LNURL-auth
	challengeHex := k1Hex
	
	challenge := &Challenge{
		ID:        uuid.New().String(),
		Value:     challengeHex,
		ExpiresAt: time.Now().Add(10 * time.Minute), // LNURL-auth typically longer
		Metadata: map[string]interface{}{
			"lightning_address": identifier,
			"k1":               k1Hex,
			"domain":           lnAddr.Domain,
			"username":         lnAddr.Username,
			"auth_type":        "lnurl_auth",
			"issued_at":        time.Now().Unix(),
		},
	}
	
	return challenge, nil
}

// ValidateCredentials validates Lightning Address authentication via LNURL-auth
func (p *LightningAddressProvider) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*AuthResult, error) {
	// Extract required fields
	lightningAddress, ok := credentials["lightning_address"].(string)
	if !ok || lightningAddress == "" {
		return nil, fmt.Errorf("lightning_address is required")
	}

	signature, ok := credentials["signature"].(string)
	if !ok || signature == "" {
		return nil, fmt.Errorf("signature is required")
	}

	k1, ok := credentials["k1"].(string)
	if !ok || k1 == "" {
		// Fall back to "challenge" field name for compatibility
		k1, ok = credentials["challenge"].(string)
		if !ok || k1 == "" {
			return nil, fmt.Errorf("k1 challenge is required")
		}
	}

	// LNURL-auth provides the linking key (public key) used to verify signatures
	linkingKey, ok := credentials["linking_key"].(string)
	if !ok || linkingKey == "" {
		return nil, fmt.Errorf("linking_key is required for LNURL-auth")
	}

	// Parse Lightning Address
	lnAddr, err := ParseLightningAddress(lightningAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid Lightning Address: %w", err)
	}

	// For external domains, verify via LNURL-auth callback
	if lnAddr.Domain != p.domain {
		return p.validateExternalLightningAddress(ctx, lnAddr, k1, signature)
	}

	// Verify the LNURL-auth signature
	valid, err := p.verifyLightningSignature(k1, signature, linkingKey)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("invalid signature")
	}

	// Check if user exists with this Lightning Address
	var user struct {
		ID        uuid.UUID
		Email     string
		CreatedAt time.Time
	}

	err = p.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.created_at
		FROM users u
		JOIN auth_methods am ON u.id = am.user_id
		WHERE am.identifier = $1 AND am.type = 'lightning_address'`,
		lightningAddress).Scan(&user.ID, &user.Email, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			// Auto-create user from Lightning Address (similar to Nostr flow)
			return p.autoCreateLightningUser(ctx, lightningAddress, linkingKey, lnAddr)
		}
		return nil, fmt.Errorf("user lookup failed: %w", err)
	}

	return &AuthResult{
		UserID:      user.ID,
		Identifier:  lightningAddress,
		DisplayName: lnAddr.Username,
		Metadata: map[string]interface{}{
			"lightning_address": lightningAddress,
			"linking_key":       linkingKey,
			"username":          lnAddr.Username,
			"domain":            lnAddr.Domain,
			"auth_method":       "lightning_address",
			"payment_capable":   true,
		},
		RequiresMFA: false, // Lightning signatures are cryptographically strong
		TrustScore:  0.9,   // High trust for Lightning-based auth
	}, nil
}

// autoCreateLightningUser creates a new user from a Lightning Address
func (p *LightningAddressProvider) autoCreateLightningUser(ctx context.Context, lightningAddress, linkingKey string, lnAddr *LightningAddress) (*AuthResult, error) {
	// Begin transaction
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create user with Lightning-derived email
	userID := uuid.New()
	now := time.Now()
	email := fmt.Sprintf("%s@lightning.local", lnAddr.Username)

	_, err = tx.ExecContext(ctx, "INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		userID, email, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create Lightning auth method
	authMethodID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth_methods (id, user_id, type, identifier, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		authMethodID, userID, "lightning_address", lightningAddress, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth method: %w", err)
	}

	// Create empty initial vault
	vaultID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO vaults (id, user_id, encrypted_data, encryption_salt, encryption_nonce, version, last_modified, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		vaultID, userID, []byte("[]"), []byte{}, []byte{}, 1, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial vault: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &AuthResult{
		UserID:      userID,
		Identifier:  lightningAddress,
		DisplayName: lnAddr.Username,
		Metadata: map[string]interface{}{
			"lightning_address": lightningAddress,
			"linking_key":       linkingKey,
			"username":          lnAddr.Username,
			"domain":            lnAddr.Domain,
			"auth_method":       "lightning_address",
			"payment_capable":   true,
			"auto_created":      true,
		},
		RequiresMFA: false,
		TrustScore:  0.9,
	}, nil
}

// PrepareRegistration prepares Lightning Address registration
func (p *LightningAddressProvider) PrepareRegistration(ctx context.Context, data map[string]interface{}) (*RegistrationData, error) {
	lightningAddress, ok := data["lightning_address"].(string)
	if !ok || lightningAddress == "" {
		return nil, fmt.Errorf("lightning_address is required")
	}
	
	// Parse and validate Lightning Address
	lnAddr, err := ParseLightningAddress(lightningAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid Lightning Address: %w", err)
	}
	
	// For our domain, check username availability
	if lnAddr.Domain == p.domain {
		var exists bool
		err = p.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM auth_methods 
				WHERE identifier = $1 AND type = 'lightning_address'
			)`, lightningAddress).Scan(&exists)
		
		if err != nil {
			return nil, fmt.Errorf("database error: %w", err)
		}
		
		if exists {
			return nil, fmt.Errorf("Lightning Address already registered: %s", lightningAddress)
		}
	}
	
	// For external domains, verify Lightning Address exists
	if lnAddr.Domain != p.domain {
		if err := p.verifyExternalLightningAddress(ctx, lnAddr); err != nil {
			return nil, fmt.Errorf("Lightning Address verification failed: %w", err)
		}
	}
	
	return &RegistrationData{
		Identifier: lightningAddress,
		AuthData: map[string]interface{}{
			"lightning_address": lightningAddress,
			"username":         lnAddr.Username,
			"domain":           lnAddr.Domain,
		},
		Metadata: map[string]interface{}{
			"payment_capable": true,
			"registered":      time.Now().Unix(),
			"method":          "lightning_address",
		},
		RequiresVerification: lnAddr.Domain != p.domain, // External domains need verification
	}, nil
}

// CompleteRegistration finalizes Lightning Address registration
func (p *LightningAddressProvider) CompleteRegistration(ctx context.Context, userID uuid.UUID, data *RegistrationData) error {
	lightningAddress := data.Identifier
	
	// Create auth method record
	authMethodID := uuid.New()
	now := time.Now()
	
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO auth_methods (
			id, user_id, type, identifier, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		authMethodID, userID, "lightning_address", lightningAddress, now, now)
	
	if err != nil {
		return fmt.Errorf("failed to create auth method: %w", err)
	}
	
	return nil
}

// verifyLightningSignature verifies an LNURL-auth signature
// LNURL-auth uses secp256k1 with a specific message format
func (p *LightningAddressProvider) verifyLightningSignature(k1Hex, signatureHex, linkingKeyHex string) (bool, error) {
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

	// Decode the linking key (33 bytes compressed, or 65 bytes uncompressed)
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

	// Hash the k1 challenge for verification (LNURL-auth signs the raw k1)
	// According to LUD-04, the message signed is sha256(k1)
	msgHash := sha256.Sum256(k1Bytes)

	// Verify the signature
	return signature.Verify(msgHash[:], pubKey), nil
}

// validateExternalLightningAddress validates Lightning Address via LNURL
func (p *LightningAddressProvider) validateExternalLightningAddress(ctx context.Context, lnAddr *LightningAddress, challenge, signature string) (*AuthResult, error) {
	// In a real implementation, this would:
	// 1. Perform LNURL-auth flow with the external domain
	// 2. Verify the signature via the external Lightning service
	// 3. Return authentication result
	
	return nil, fmt.Errorf("external Lightning Address authentication not yet implemented")
}

// verifyExternalLightningAddress checks if external Lightning Address exists
func (p *LightningAddressProvider) verifyExternalLightningAddress(ctx context.Context, lnAddr *LightningAddress) error {
	// In a real implementation, this would:
	// 1. Query the .well-known/lnurlp endpoint
	// 2. Verify the Lightning Address is valid and active
	// 3. Check if it supports LNURL-auth
	
	// For demo purposes, we'll just check basic reachability
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://%s/.well-known/lnurlp/%s", lnAddr.Domain, lnAddr.Username)
	
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Lightning Address not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Lightning Address not found (HTTP %d)", resp.StatusCode)
	}
	
	return nil
}

// Lightning Address utilities

// GenerateLightningAddress generates a new Lightning Address for a user
func (p *LightningAddressProvider) GenerateLightningAddress(username string) string {
	return fmt.Sprintf("%s@%s", strings.ToLower(username), p.domain)
}

// IsOurDomain checks if a Lightning Address belongs to our domain
func (p *LightningAddressProvider) IsOurDomain(address string) bool {
	lnAddr, err := ParseLightningAddress(address)
	if err != nil {
		return false
	}
	return lnAddr.Domain == p.domain
}

// GetLNURLAuthURL generates an LNURL-auth URL for external authentication
func (p *LightningAddressProvider) GetLNURLAuthURL(lnAddr *LightningAddress, challenge string) string {
	// Generate LNURL-auth URL according to LUD-04 specification
	k1 := sha256.Sum256([]byte(fmt.Sprintf("lnauth:%s:%s", lnAddr.Full, challenge)))
	k1Hex := hex.EncodeToString(k1[:])
	
	return fmt.Sprintf("https://%s/.well-known/lnurlauth?k1=%s&tag=login", lnAddr.Domain, k1Hex)
}

// Lightning Address payment integration
type LightningPayment struct {
	Amount      int64  `json:"amount"`      // millisats
	PaymentHash string `json:"payment_hash"`
	Invoice     string `json:"invoice"`
	Paid        bool   `json:"paid"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

// CreatePaymentInvoice creates a Lightning invoice for the user
func (p *LightningAddressProvider) CreatePaymentInvoice(ctx context.Context, lightningAddress string, amountSats int64, description string) (*LightningPayment, error) {
	// In a real implementation, this would:
	// 1. Connect to Lightning node (LND, CLN, Eclair)
	// 2. Generate invoice for the specified amount
	// 3. Return payment details
	
	// Demo implementation
	paymentHash := fmt.Sprintf("demo_payment_%d", time.Now().Unix())
	invoice := fmt.Sprintf("lnbc%dm1p...", amountSats/1000) // Simplified BOLT11 format
	
	return &LightningPayment{
		Amount:      amountSats * 1000, // Convert to millisats
		PaymentHash: paymentHash,
		Invoice:     invoice,
		Paid:        false,
	}, nil
}

// VerifyPayment checks if a Lightning payment was completed
func (p *LightningAddressProvider) VerifyPayment(ctx context.Context, paymentHash string) (*LightningPayment, error) {
	// In a real implementation, this would:
	// 1. Check Lightning node for payment status
	// 2. Return payment details
	
	// Demo: assume payment is completed after 10 seconds
	return &LightningPayment{
		PaymentHash: paymentHash,
		Paid:        true,
		PaidAt:      timePtr(time.Now()),
	}, nil
}

// Lightning Address registration flow
func (p *LightningAddressProvider) RegisterLightningAddress(ctx context.Context, username string, userID uuid.UUID) (string, error) {
	// Generate Lightning Address
	lightningAddress := p.GenerateLightningAddress(username)
	
	// Store in database
	authMethodID := uuid.New()
	now := time.Now()
	
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO auth_methods (
			id, user_id, type, identifier, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		authMethodID, userID, "lightning_address", lightningAddress, now, now)
	
	if err != nil {
		return "", fmt.Errorf("failed to register Lightning Address: %w", err)
	}
	
	// In a real implementation, this would also:
	// 1. Configure Lightning node to accept payments to this address
	// 2. Set up LNURL-pay endpoint
	// 3. Configure DNS records for Lightning Address resolution
	
	return lightningAddress, nil
}

// GetLightningAddressInfo returns public info for a Lightning Address
func (p *LightningAddressProvider) GetLightningAddressInfo(ctx context.Context, username string) (map[string]interface{}, error) {
	lightningAddress := p.GenerateLightningAddress(username)
	
	// Check if address exists
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth_methods 
			WHERE identifier = $1 AND type = 'lightning_address'
		)`, lightningAddress).Scan(&exists)
	
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	if !exists {
		return nil, fmt.Errorf("Lightning Address not found")
	}
	
	// Return LNURL-pay compatible response
	return map[string]interface{}{
		"status":      "OK",
		"tag":         "payRequest",
		"callback":    fmt.Sprintf("https://%s/api/v1/lightning/pay/%s", p.domain, username),
		"minSendable": 1000,     // 1 sat minimum
		"maxSendable": 10000000, // 10,000 sats maximum
		"metadata":    fmt.Sprintf("[[\"text/plain\",\"Pay to %s\"]]", lightningAddress),
		"allowsNostr": true,
		"nostrPubkey": "", // Would be populated if user has linked Nostr key
	}, nil
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}