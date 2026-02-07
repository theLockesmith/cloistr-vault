package identity

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LightningIdentityService manages @coldforge.xyz Lightning Address identities
type LightningIdentityService struct {
	db     *sql.DB
	domain string // coldforge.xyz
}

// LightningIdentity represents a Lightning Address identity
type LightningIdentity struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	Username      string    `json:"username"`
	LightningAddr string    `json:"lightning_address"`
	NostrPubkey   *string   `json:"nostr_pubkey,omitempty"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LNURL-pay response for Lightning Address resolution
type LNURLPayResponse struct {
	Status      string   `json:"status"`
	Tag         string   `json:"tag"`
	Callback    string   `json:"callback"`
	MinSendable int64    `json:"minSendable"`
	MaxSendable int64    `json:"maxSendable"`
	Metadata    string   `json:"metadata"`
	AllowsNostr bool     `json:"allowsNostr"`
	NostrPubkey *string  `json:"nostrPubkey,omitempty"`
}

// NIP-05 response for Nostr identity verification
type NIP05Response struct {
	Names  map[string]string            `json:"names"`
	Relays map[string][]string          `json:"relays,omitempty"`
}

func NewLightningIdentityService(db *sql.DB, domain string) *LightningIdentityService {
	return &LightningIdentityService{
		db:     db,
		domain: domain,
	}
}

// ReserveIdentity reserves a Lightning Address for a user
func (s *LightningIdentityService) ReserveIdentity(userID uuid.UUID, username string, nostrPubkey *string) (*LightningIdentity, error) {
	// Validate username
	if !isValidUsername(username) {
		return nil, fmt.Errorf("invalid username: only lowercase letters, numbers, and hyphens allowed")
	}

	lightningAddr := fmt.Sprintf("%s@%s", username, s.domain)

	// Check availability
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM lightning_identities WHERE username = $1)", username).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("username %s is already taken", username)
	}

	// Create identity
	identity := &LightningIdentity{
		ID:            uuid.New(),
		UserID:        userID,
		Username:      username,
		LightningAddr: lightningAddr,
		NostrPubkey:   nostrPubkey,
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err = s.db.Exec(`
		INSERT INTO lightning_identities (id, user_id, username, lightning_address, nostr_pubkey, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		identity.ID, identity.UserID, identity.Username, identity.LightningAddr,
		identity.NostrPubkey, identity.IsActive, identity.CreatedAt, identity.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create lightning identity: %w", err)
	}

	log.Printf("Reserved Lightning Address: %s for user: %s", lightningAddr, userID.String())
	return identity, nil
}

// ResolveLightningAddress handles LNURL-pay resolution for Lightning Addresses
func (s *LightningIdentityService) ResolveLightningAddress(username string) (*LNURLPayResponse, error) {
	var identity LightningIdentity
	err := s.db.QueryRow(`
		SELECT id, user_id, username, lightning_address, nostr_pubkey, is_active, created_at, updated_at
		FROM lightning_identities
		WHERE username = $1 AND is_active = true`,
		username).Scan(&identity.ID, &identity.UserID, &identity.Username, &identity.LightningAddr,
		&identity.NostrPubkey, &identity.IsActive, &identity.CreatedAt, &identity.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Lightning Address not found: %s@%s", username, s.domain)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Generate LNURL-pay response
	response := &LNURLPayResponse{
		Status:      "OK",
		Tag:         "payRequest",
		Callback:    fmt.Sprintf("https://%s/api/v1/lightning/pay/%s", s.domain, username),
		MinSendable: 1000,       // 1 sat minimum
		MaxSendable: 100000000,  // 100,000 sats maximum
		Metadata:    fmt.Sprintf("[[\"text/plain\",\"Pay to %s\"]]", identity.LightningAddr),
		AllowsNostr: identity.NostrPubkey != nil,
		NostrPubkey: identity.NostrPubkey,
	}

	return response, nil
}

// GetNIP05Data returns NIP-05 verification data for all identities
func (s *LightningIdentityService) GetNIP05Data() (*NIP05Response, error) {
	rows, err := s.db.Query(`
		SELECT username, nostr_pubkey
		FROM lightning_identities
		WHERE nostr_pubkey IS NOT NULL AND is_active = true`)

	if err != nil {
		return nil, fmt.Errorf("failed to query identities: %w", err)
	}
	defer rows.Close()

	names := make(map[string]string)
	relays := make(map[string][]string)

	for rows.Next() {
		var username, nostrPubkey string
		if err := rows.Scan(&username, &nostrPubkey); err != nil {
			continue
		}

		names[username] = nostrPubkey

		// Add default relays for each pubkey
		relays[nostrPubkey] = []string{
			"wss://relay.damus.io",
			"wss://nos.lol",
			"wss://relay.coldforge.xyz", // Our own relay (future)
		}
	}

	return &NIP05Response{
		Names:  names,
		Relays: relays,
	}, nil
}

// GetIdentityByUsername returns identity information for a username
func (s *LightningIdentityService) GetIdentityByUsername(username string) (*LightningIdentity, error) {
	var identity LightningIdentity
	err := s.db.QueryRow(`
		SELECT id, user_id, username, lightning_address, nostr_pubkey, is_active, created_at, updated_at
		FROM lightning_identities
		WHERE username = $1 AND is_active = true`,
		username).Scan(&identity.ID, &identity.UserID, &identity.Username, &identity.LightningAddr,
		&identity.NostrPubkey, &identity.IsActive, &identity.CreatedAt, &identity.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return &identity, nil
}

// GetIdentitiesByUser returns all Lightning identities for a user
func (s *LightningIdentityService) GetIdentitiesByUser(userID uuid.UUID) ([]LightningIdentity, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, username, lightning_address, nostr_pubkey, is_active, created_at, updated_at
		FROM lightning_identities
		WHERE user_id = $1
		ORDER BY created_at DESC`,
		userID)

	if err != nil {
		return nil, fmt.Errorf("failed to query user identities: %w", err)
	}
	defer rows.Close()

	var identities []LightningIdentity
	for rows.Next() {
		var identity LightningIdentity
		err := rows.Scan(&identity.ID, &identity.UserID, &identity.Username, &identity.LightningAddr,
			&identity.NostrPubkey, &identity.IsActive, &identity.CreatedAt, &identity.UpdatedAt)
		if err != nil {
			continue
		}
		identities = append(identities, identity)
	}

	return identities, nil
}

// UpdateIdentity updates an existing Lightning identity
func (s *LightningIdentityService) UpdateIdentity(identityID uuid.UUID, updates map[string]interface{}) error {
	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", key, argIndex))
		args = append(args, value)
		argIndex++
	}

	if len(setParts) == 0 {
		return fmt.Errorf("no updates provided")
	}

	// Add updated_at
	setParts = append(setParts, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now())
	argIndex++

	// Add WHERE clause
	args = append(args, identityID)

	query := fmt.Sprintf("UPDATE lightning_identities SET %s WHERE id = $%d",
		strings.Join(setParts, ", "), argIndex)

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update identity: %w", err)
	}

	return nil
}

// DeactivateIdentity deactivates a Lightning identity
func (s *LightningIdentityService) DeactivateIdentity(identityID uuid.UUID) error {
	return s.UpdateIdentity(identityID, map[string]interface{}{
		"is_active": false,
	})
}

// Helper functions

func isValidUsername(username string) bool {
	// Lightning Address username validation
	// Allow: lowercase letters, numbers, hyphens
	// Disallow: uppercase, special chars, spaces
	pattern := `^[a-z0-9-]+$`
	matched, _ := regexp.MatchString(pattern, username)
	return matched && len(username) >= 2 && len(username) <= 32
}

// HTTP handlers for Lightning Address endpoints

func (s *LightningIdentityService) HandleLNURLPay(w http.ResponseWriter, r *http.Request) {
	// Extract username from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 2 {
		http.Error(w, "Invalid Lightning Address", http.StatusBadRequest)
		return
	}

	username := pathParts[len(pathParts)-1]

	// Resolve Lightning Address
	response, err := s.ResolveLightningAddress(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *LightningIdentityService) HandleNIP05(w http.ResponseWriter, r *http.Request) {
	// Return NIP-05 verification data
	response, err := s.GetNIP05Data()
	if err != nil {
		http.Error(w, "Failed to generate NIP-05 data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

// Database migration for Lightning identities
const LightningIdentityMigration = `
-- Lightning Address identities table
CREATE TABLE IF NOT EXISTS lightning_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username VARCHAR(32) NOT NULL UNIQUE,
    lightning_address VARCHAR(255) NOT NULL UNIQUE,
    nostr_pubkey VARCHAR(64), -- Optional Nostr public key
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_username CHECK (username ~ '^[a-z0-9-]+$'),
    CONSTRAINT username_length CHECK (length(username) >= 2 AND length(username) <= 32)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_lightning_identities_user_id ON lightning_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_lightning_identities_username ON lightning_identities(username);
CREATE INDEX IF NOT EXISTS idx_lightning_identities_nostr_pubkey ON lightning_identities(nostr_pubkey) WHERE nostr_pubkey IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_lightning_identities_active ON lightning_identities(is_active) WHERE is_active = true;

-- Comments
COMMENT ON TABLE lightning_identities IS 'Lightning Address identities for @coldforge.xyz universal Bitcoin identity';
COMMENT ON COLUMN lightning_identities.username IS 'Username part of Lightning Address (alice in alice@coldforge.xyz)';
COMMENT ON COLUMN lightning_identities.lightning_address IS 'Full Lightning Address (alice@coldforge.xyz)';
COMMENT ON COLUMN lightning_identities.nostr_pubkey IS 'Associated Nostr public key for NIP-05 verification';
`