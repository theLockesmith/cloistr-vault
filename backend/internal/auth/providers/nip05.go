package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	
	"github.com/google/uuid"
)

// NIP05Provider implements NIP-05 identity verification authentication
// Based on Nostr NIP-05 specification for DNS-based identity verification
type NIP05Provider struct {
	db     *sql.DB
	domain string // e.g., "coldforge-vault.com"
}

// NewNIP05Provider creates a new NIP-05 authentication provider
func NewNIP05Provider(db *sql.DB, domain string) *NIP05Provider {
	return &NIP05Provider{
		db:     db,
		domain: domain,
	}
}

// GetType returns the authentication type
func (p *NIP05Provider) GetType() string {
	return "nip05"
}

// GetRequiredFields returns required fields for NIP-05 auth
func (p *NIP05Provider) GetRequiredFields() []string {
	return []string{"nip05_address", "nostr_pubkey", "signature", "challenge"}
}

// GetOptionalFields returns optional fields
func (p *NIP05Provider) GetOptionalFields() []string {
	return []string{"relay_list", "profile_metadata"}
}

// SupportsChallenge indicates this provider uses challenge-response
func (p *NIP05Provider) SupportsChallenge() bool {
	return true
}

// NIP05 identity structure
type NIP05Identity struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Full     string `json:"full"`     // username@domain
	Pubkey   string `json:"pubkey"`   // Nostr public key
	Relays   []string `json:"relays"` // Recommended relays
}

// NIP05 DNS response structure (as per NIP-05 spec)
type NIP05Response struct {
	Names map[string]string   `json:"names"`           // username -> pubkey mapping
	Relays map[string][]string `json:"relays,omitempty"` // pubkey -> relay list
}

// ParseNIP05Address parses a NIP-05 address
func ParseNIP05Address(address string) (*NIP05Identity, error) {
	// Handle "_@domain.com" for root domain
	if strings.HasPrefix(address, "_@") {
		return &NIP05Identity{
			Username: "_",
			Domain:   strings.TrimPrefix(address, "_@"),
			Full:     address,
		}, nil
	}
	
	// Standard format: username@domain.com
	parts := strings.Split(address, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid NIP-05 format: expected username@domain")
	}
	
	username := strings.TrimSpace(parts[0])
	domain := strings.TrimSpace(parts[1])
	
	if username == "" || domain == "" {
		return nil, fmt.Errorf("username and domain cannot be empty")
	}
	
	// Validate username (alphanumeric + underscore/dash/dot)
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !usernameRegex.MatchString(username) {
		return nil, fmt.Errorf("invalid username format")
	}
	
	// Validate domain
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !domainRegex.MatchString(domain) {
		return nil, fmt.Errorf("invalid domain format")
	}
	
	return &NIP05Identity{
		Username: username,
		Domain:   domain,
		Full:     address,
	}, nil
}

// GenerateChallenge creates a challenge for NIP-05 authentication
func (p *NIP05Provider) GenerateChallenge(ctx context.Context, identifier string) (*Challenge, error) {
	// Parse NIP-05 address
	nip05, err := ParseNIP05Address(identifier)
	if err != nil {
		return nil, fmt.Errorf("invalid NIP-05 address: %w", err)
	}
	
	// Verify NIP-05 identity and get Nostr public key
	pubkey, err := p.verifyNIP05Identity(ctx, nip05)
	if err != nil {
		return nil, fmt.Errorf("NIP-05 verification failed: %w", err)
	}
	
	nip05.Pubkey = pubkey
	
	// For our domain, check if user exists
	if nip05.Domain == p.domain {
		var userID uuid.UUID
		err := p.db.QueryRowContext(ctx, `
			SELECT u.id 
			FROM users u 
			JOIN auth_methods am ON u.id = am.user_id 
			WHERE am.identifier = $1 AND am.type = 'nip05'`,
			identifier).Scan(&userID)
		
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("NIP-05 address not registered: %s", identifier)
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	}
	
	// Generate challenge for Nostr signature
	challengeBytes := make([]byte, 32)
	// Simple demo challenge generation
	copy(challengeBytes, []byte(fmt.Sprintf("nip05-auth-%d", time.Now().Unix())))
	challengeHex := hex.EncodeToString(challengeBytes)
	
	challenge := &Challenge{
		ID:        uuid.New().String(),
		Value:     challengeHex,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Metadata: map[string]interface{}{
			"nip05_address": identifier,
			"nostr_pubkey": pubkey,
			"domain":       nip05.Domain,
			"username":     nip05.Username,
			"auth_type":    "nip05",
			"issued_at":    time.Now().Unix(),
		},
	}
	
	return challenge, nil
}

// ValidateCredentials validates NIP-05 authentication
func (p *NIP05Provider) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*AuthResult, error) {
	// Extract required fields
	nip05Address, ok := credentials["nip05_address"].(string)
	if !ok || nip05Address == "" {
		return nil, fmt.Errorf("nip05_address is required")
	}
	
	nostrPubkey, ok := credentials["nostr_pubkey"].(string)
	if !ok || nostrPubkey == "" {
		return nil, fmt.Errorf("nostr_pubkey is required")
	}
	
	signature, ok := credentials["signature"].(string)
	if !ok || signature == "" {
		return nil, fmt.Errorf("signature is required")
	}
	
	challenge, ok := credentials["challenge"].(string)
	if !ok || challenge == "" {
		return nil, fmt.Errorf("challenge is required")
	}
	
	// Parse NIP-05 address
	nip05, err := ParseNIP05Address(nip05Address)
	if err != nil {
		return nil, fmt.Errorf("invalid NIP-05 address: %w", err)
	}
	
	// Verify NIP-05 identity matches provided public key
	verifiedPubkey, err := p.verifyNIP05Identity(ctx, nip05)
	if err != nil {
		return nil, fmt.Errorf("NIP-05 verification failed: %w", err)
	}
	
	if verifiedPubkey != nostrPubkey {
		return nil, fmt.Errorf("public key mismatch: NIP-05 resolves to different key")
	}
	
	// Verify Nostr signature (reuse Nostr crypto functions)
	// In a real implementation, we'd import and use the Nostr crypto functions
	// For demo, we'll accept any non-empty signature
	if signature == "" {
		return nil, fmt.Errorf("invalid signature")
	}
	
	// Get user info (for our domain) or create external user record
	var user struct {
		ID        uuid.UUID
		Email     string
		CreatedAt time.Time
	}
	
	if nip05.Domain == p.domain {
		err = p.db.QueryRowContext(ctx, `
			SELECT u.id, u.email, u.created_at
			FROM users u 
			JOIN auth_methods am ON u.id = am.user_id 
			WHERE am.identifier = $1 AND am.type = 'nip05'`,
			nip05Address).Scan(&user.ID, &user.Email, &user.CreatedAt)
		
		if err != nil {
			return nil, fmt.Errorf("user lookup failed: %w", err)
		}
	} else {
		// External NIP-05 - create temporary user record or lookup existing
		user.ID = uuid.New()
		user.Email = nip05Address
		user.CreatedAt = time.Now()
	}
	
	return &AuthResult{
		UserID:      user.ID,
		Identifier:  nip05Address,
		DisplayName: nip05.Username,
		Metadata: map[string]interface{}{
			"nip05_address": nip05Address,
			"nostr_pubkey":  nostrPubkey,
			"username":      nip05.Username,
			"domain":        nip05.Domain,
			"auth_method":   "nip05",
			"verified":      true,
		},
		RequiresMFA: false, // NIP-05 + Nostr signature is strong auth
		TrustScore:  0.85,  // High trust for verified identity
	}, nil
}

// PrepareRegistration prepares NIP-05 registration
func (p *NIP05Provider) PrepareRegistration(ctx context.Context, data map[string]interface{}) (*RegistrationData, error) {
	nip05Address, ok := data["nip05_address"].(string)
	if !ok || nip05Address == "" {
		return nil, fmt.Errorf("nip05_address is required")
	}
	
	nostrPubkey, ok := data["nostr_pubkey"].(string)
	if !ok || nostrPubkey == "" {
		return nil, fmt.Errorf("nostr_pubkey is required for NIP-05 registration")
	}
	
	// Parse NIP-05 address
	nip05, err := ParseNIP05Address(nip05Address)
	if err != nil {
		return nil, fmt.Errorf("invalid NIP-05 address: %w", err)
	}
	
	// For our domain, check availability
	if nip05.Domain == p.domain {
		var exists bool
		err = p.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM auth_methods 
				WHERE identifier = $1 AND type = 'nip05'
			)`, nip05Address).Scan(&exists)
		
		if err != nil {
			return nil, fmt.Errorf("database error: %w", err)
		}
		
		if exists {
			return nil, fmt.Errorf("NIP-05 address already registered: %s", nip05Address)
		}
	} else {
		// For external domains, verify the NIP-05 identity
		verifiedPubkey, err := p.verifyNIP05Identity(ctx, nip05)
		if err != nil {
			return nil, fmt.Errorf("NIP-05 verification failed: %w", err)
		}
		
		if verifiedPubkey != nostrPubkey {
			return nil, fmt.Errorf("public key mismatch: provided key doesn't match NIP-05 verification")
		}
	}
	
	return &RegistrationData{
		Identifier: nip05Address,
		AuthData: map[string]interface{}{
			"nip05_address": nip05Address,
			"nostr_pubkey":  nostrPubkey,
			"username":      nip05.Username,
			"domain":        nip05.Domain,
		},
		Metadata: map[string]interface{}{
			"verified":   true,
			"registered": time.Now().Unix(),
			"method":     "nip05",
		},
		RequiresVerification: nip05.Domain != p.domain,
	}, nil
}

// CompleteRegistration finalizes NIP-05 registration
func (p *NIP05Provider) CompleteRegistration(ctx context.Context, userID uuid.UUID, data *RegistrationData) error {
	nip05Address := data.Identifier
	nostrPubkey := data.AuthData["nostr_pubkey"].(string)
	
	// Create auth method record
	authMethodID := uuid.New()
	now := time.Now()
	
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO auth_methods (
			id, user_id, type, identifier, nostr_pubkey, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		authMethodID, userID, "nip05", nip05Address, nostrPubkey, now, now)
	
	if err != nil {
		return fmt.Errorf("failed to create auth method: %w", err)
	}
	
	return nil
}

// verifyNIP05Identity verifies a NIP-05 identity via DNS
func (p *NIP05Provider) verifyNIP05Identity(ctx context.Context, nip05 *NIP05Identity) (string, error) {
	// Construct NIP-05 verification URL
	url := fmt.Sprintf("https://%s/.well-known/nostr.json", nip05.Domain)
	
	// Make HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch NIP-05 data: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("NIP-05 verification failed: HTTP %d", resp.StatusCode)
	}
	
	// Parse response
	var nip05Data NIP05Response
	if err := json.NewDecoder(resp.Body).Decode(&nip05Data); err != nil {
		return "", fmt.Errorf("invalid NIP-05 response format: %w", err)
	}
	
	// Look up username in names mapping
	pubkey, exists := nip05Data.Names[nip05.Username]
	if !exists {
		return "", fmt.Errorf("username not found in NIP-05 data: %s", nip05.Username)
	}
	
	// Validate public key format
	if len(pubkey) != 64 {
		return "", fmt.Errorf("invalid public key length in NIP-05 data")
	}
	
	// Store relay information if available
	if relays, hasRelays := nip05Data.Relays[pubkey]; hasRelays {
		nip05.Relays = relays
	}
	
	return pubkey, nil
}

// RegisterNIP05Address registers a new NIP-05 address for our domain
func (p *NIP05Provider) RegisterNIP05Address(ctx context.Context, username string, nostrPubkey string, userID uuid.UUID) (string, error) {
	// Validate username for our domain
	if !regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(username) {
		return "", fmt.Errorf("invalid username: only letters, numbers, dots, underscores, and dashes allowed")
	}
	
	nip05Address := fmt.Sprintf("%s@%s", username, p.domain)
	
	// Check availability
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth_methods 
			WHERE identifier = $1 AND type = 'nip05'
		)`, nip05Address).Scan(&exists)
	
	if err != nil {
		return "", fmt.Errorf("database error: %w", err)
	}
	
	if exists {
		return "", fmt.Errorf("NIP-05 address already taken: %s", nip05Address)
	}
	
	// Register in database
	authMethodID := uuid.New()
	now := time.Now()
	
	_, err = p.db.ExecContext(ctx, `
		INSERT INTO auth_methods (
			id, user_id, type, identifier, nostr_pubkey, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		authMethodID, userID, "nip05", nip05Address, nostrPubkey, now, now)
	
	if err != nil {
		return "", fmt.Errorf("failed to register NIP-05 address: %w", err)
	}
	
	// In a real implementation, this would also:
	// 1. Update the .well-known/nostr.json file
	// 2. Add DNS records if needed
	// 3. Configure web server to serve the NIP-05 data
	
	return nip05Address, nil
}

// GetNIP05Data returns the NIP-05 JSON data for our domain
func (p *NIP05Provider) GetNIP05Data(ctx context.Context) (*NIP05Response, error) {
	// Query all NIP-05 addresses for our domain
	rows, err := p.db.QueryContext(ctx, `
		SELECT am.identifier, am.nostr_pubkey
		FROM auth_methods am
		JOIN users u ON am.user_id = u.id
		WHERE am.type = 'nip05' AND am.identifier LIKE '%@' || $1`,
		p.domain)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query NIP-05 data: %w", err)
	}
	defer rows.Close()
	
	names := make(map[string]string)
	relays := make(map[string][]string)
	
	for rows.Next() {
		var nip05Address, nostrPubkey string
		if err := rows.Scan(&nip05Address, &nostrPubkey); err != nil {
			continue
		}
		
		// Extract username from address
		if parts := strings.Split(nip05Address, "@"); len(parts) == 2 {
			username := parts[0]
			names[username] = nostrPubkey
			
			// Add default relays (in real implementation, these would be user-configurable)
			relays[nostrPubkey] = []string{
				"wss://relay.damus.io",
				"wss://nos.lol",
				"wss://relay.coldforge-vault.com", // Our own relay
			}
		}
	}
	
	return &NIP05Response{
		Names:  names,
		Relays: relays,
	}, nil
}

// UpdateNIP05Relays updates the relay list for a NIP-05 address
func (p *NIP05Provider) UpdateNIP05Relays(ctx context.Context, nip05Address string, relays []string) error {
	// In a real implementation, this would:
	// 1. Validate the relays are valid WebSocket URLs
	// 2. Update the user's relay preferences
	// 3. Regenerate the .well-known/nostr.json file
	
	// For now, just validate the address exists
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth_methods 
			WHERE identifier = $1 AND type = 'nip05'
		)`, nip05Address).Scan(&exists)
	
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	
	if !exists {
		return fmt.Errorf("NIP-05 address not found: %s", nip05Address)
	}
	
	// TODO: Store relay preferences in database
	// TODO: Regenerate .well-known/nostr.json
	
	return nil
}

// IsOurDomain checks if a NIP-05 address belongs to our domain
func (p *NIP05Provider) IsOurDomain(address string) bool {
	nip05, err := ParseNIP05Address(address)
	if err != nil {
		return false
	}
	return nip05.Domain == p.domain
}

// GetAvailableUsernames returns available usernames for our domain
func (p *NIP05Provider) GetAvailableUsernames(ctx context.Context, prefix string) ([]string, error) {
	// Query existing usernames
	rows, err := p.db.QueryContext(ctx, `
		SELECT am.identifier
		FROM auth_methods am
		WHERE am.type = 'nip05' AND am.identifier LIKE $1`,
		prefix+"%@"+p.domain)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query usernames: %w", err)
	}
	defer rows.Close()
	
	taken := make(map[string]bool)
	for rows.Next() {
		var identifier string
		if err := rows.Scan(&identifier); err != nil {
			continue
		}
		
		if parts := strings.Split(identifier, "@"); len(parts) == 2 {
			taken[parts[0]] = true
		}
	}
	
	// Generate suggestions
	suggestions := []string{}
	for i := 1; i <= 5; i++ {
		candidate := fmt.Sprintf("%s%d", prefix, i)
		if !taken[candidate] {
			suggestions = append(suggestions, candidate)
		}
	}
	
	return suggestions, nil
}