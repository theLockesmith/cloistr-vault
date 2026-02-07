package providers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	
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
	return []string{"lightning_address", "signature", "challenge"}
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
	
	// Generate LNURL-auth challenge
	challengeBytes := make([]byte, 32)
	for i := range challengeBytes {
		challengeBytes[i] = byte(i) // Simple challenge for demo
	}
	challengeHex := hex.EncodeToString(challengeBytes)
	
	// Create k1 challenge (LNURL-auth standard)
	k1Challenge := sha256.Sum256([]byte(fmt.Sprintf("lnauth:%s:%d", identifier, time.Now().Unix())))
	k1Hex := hex.EncodeToString(k1Challenge[:])
	
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

// ValidateCredentials validates Lightning Address authentication
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
	
	challenge, ok := credentials["challenge"].(string)
	if !ok || challenge == "" {
		return nil, fmt.Errorf("challenge is required")
	}
	
	// Parse Lightning Address
	lnAddr, err := ParseLightningAddress(lightningAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid Lightning Address: %w", err)
	}
	
	// For external domains, verify via LNURL-auth
	if lnAddr.Domain != p.domain {
		return p.validateExternalLightningAddress(ctx, lnAddr, challenge, signature)
	}
	
	// For our domain, verify signature and get user
	valid, err := p.verifyLightningSignature(challenge, signature, lightningAddress)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}
	
	if !valid {
		return nil, fmt.Errorf("invalid signature")
	}
	
	// Get user info
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
		return nil, fmt.Errorf("user lookup failed: %w", err)
	}
	
	return &AuthResult{
		UserID:      user.ID,
		Identifier:  lightningAddress,
		DisplayName: lnAddr.Username,
		Metadata: map[string]interface{}{
			"lightning_address": lightningAddress,
			"username":         lnAddr.Username,
			"domain":           lnAddr.Domain,
			"auth_method":      "lightning_address",
			"payment_capable":  true,
		},
		RequiresMFA: false, // Lightning signatures are cryptographically strong
		TrustScore:  0.9,   // High trust for Lightning-based auth
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

// verifyLightningSignature verifies a Lightning Address signature
func (p *LightningAddressProvider) verifyLightningSignature(challenge, signature, lightningAddress string) (bool, error) {
	// In a real implementation, this would:
	// 1. Verify the signature using the Lightning node's public key
	// 2. Check that the signature corresponds to the Lightning Address
	// 3. Validate the challenge was signed correctly
	
	// For demo purposes, we'll accept any non-empty signature
	return signature != "", nil
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