package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coldforge/vault/internal/identity"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// NIP05Response is the standard NIP-05 JSON response format
type NIP05Response struct {
	Names  map[string]string   `json:"names"`
	Relays map[string][]string `json:"relays,omitempty"`
}

// VerifyNIP05 verifies a NIP-05 address and links it to a user
func (a *AuthService) VerifyNIP05(userID uuid.UUID, nip05Address string) error {
	// Parse NIP-05 address
	parts := strings.Split(nip05Address, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid NIP-05 format: expected username@domain")
	}

	// Get user's Nostr pubkey
	var nostrPubkey sql.NullString
	err := a.db.QueryRow(`
		SELECT am.nostr_pubkey
		FROM auth_methods am
		WHERE am.user_id = $1 AND am.type = 'nostr'`,
		userID).Scan(&nostrPubkey)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user must have a Nostr pubkey to verify NIP-05")
		}
		return fmt.Errorf("database error: %w", err)
	}

	if !nostrPubkey.Valid || nostrPubkey.String == "" {
		return fmt.Errorf("user must have a Nostr pubkey to verify NIP-05")
	}

	// Fetch NIP-05 data from the domain
	verifiedPubkey, relays, err := fetchNIP05(nip05Address)
	if err != nil {
		return fmt.Errorf("NIP-05 verification failed: %w", err)
	}

	// Check if the verified pubkey matches the user's pubkey
	if verifiedPubkey != nostrPubkey.String {
		return fmt.Errorf("NIP-05 pubkey mismatch: %s resolves to a different pubkey", nip05Address)
	}

	// Store the verified NIP-05 address
	now := time.Now()
	_, err = a.db.Exec(`
		UPDATE auth_methods
		SET nip05_address = $1, nip05_verified_at = $2, nip05_relays = $3, updated_at = $4
		WHERE user_id = $5 AND type = 'nostr'`,
		nip05Address, now, strings.Join(relays, ","), now, userID)

	if err != nil {
		return fmt.Errorf("failed to store NIP-05 address: %w", err)
	}

	log.Printf("NIP-05 verified for user %s: %s -> %s", userID, nip05Address, nostrPubkey.String[:16]+"...")
	return nil
}

// GetNIP05ForUser returns the verified NIP-05 address for a user
func (a *AuthService) GetNIP05ForUser(userID uuid.UUID) (string, error) {
	var nip05Address sql.NullString
	err := a.db.QueryRow(`
		SELECT nip05_address
		FROM auth_methods
		WHERE user_id = $1 AND type = 'nostr' AND nip05_address IS NOT NULL`,
		userID).Scan(&nip05Address)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No NIP-05 address
		}
		return "", fmt.Errorf("database error: %w", err)
	}

	if !nip05Address.Valid {
		return "", nil
	}

	return nip05Address.String, nil
}

// LookupNIP05 looks up a NIP-05 address and returns the pubkey
func (a *AuthService) LookupNIP05(nip05Address string) (string, []string, error) {
	return fetchNIP05(nip05Address)
}

// GetNostrJSON returns the NIP-05 JSON data for serving at /.well-known/nostr.json
func (a *AuthService) GetNostrJSON(ctx context.Context, domain string) (*NIP05Response, error) {
	// Query all users with Nostr auth who have registered NIP-05 addresses on this domain
	rows, err := a.db.QueryContext(ctx, `
		SELECT am.nostr_pubkey, am.nip05_address
		FROM auth_methods am
		WHERE am.type = 'nostr'
		AND am.nostr_pubkey IS NOT NULL
		AND am.nip05_address LIKE '%@' || $1`,
		domain)

	if err != nil {
		return nil, fmt.Errorf("failed to query NIP-05 data: %w", err)
	}
	defer rows.Close()

	names := make(map[string]string)
	relays := make(map[string][]string)

	for rows.Next() {
		var pubkey, nip05Addr string
		if err := rows.Scan(&pubkey, &nip05Addr); err != nil {
			continue
		}

		// Extract username from address
		parts := strings.Split(nip05Addr, "@")
		if len(parts) == 2 {
			username := parts[0]
			names[username] = pubkey

			// Add default relays
			relays[pubkey] = []string{
				"wss://relay.cloistr.xyz",
				"wss://relay.damus.io",
				"wss://nos.lol",
			}
		}
	}

	return &NIP05Response{
		Names:  names,
		Relays: relays,
	}, nil
}

// fetchNIP05 fetches and verifies a NIP-05 address from its domain
func fetchNIP05(nip05Address string) (string, []string, error) {
	parts := strings.Split(nip05Address, "@")
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid NIP-05 format")
	}
	username := parts[0]
	domain := parts[1]

	// Construct NIP-05 verification URL
	url := fmt.Sprintf("https://%s/.well-known/nostr.json?name=%s", domain, username)

	// Make HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch NIP-05 data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("NIP-05 verification failed: HTTP %d", resp.StatusCode)
	}

	// Parse response
	var nip05Data NIP05Response
	if err := json.NewDecoder(resp.Body).Decode(&nip05Data); err != nil {
		return "", nil, fmt.Errorf("invalid NIP-05 response format: %w", err)
	}

	// Look up username in names mapping
	pubkey, exists := nip05Data.Names[username]
	if !exists {
		return "", nil, fmt.Errorf("username not found in NIP-05 data: %s", username)
	}

	// Validate public key format
	if len(pubkey) != 64 {
		return "", nil, fmt.Errorf("invalid public key length in NIP-05 data")
	}

	// Get relays if available
	var relays []string
	if r, ok := nip05Data.Relays[pubkey]; ok {
		relays = r
	}

	return pubkey, relays, nil
}

// GetDisplayNameForUser returns the best display name for a user
// Priority: NIP-05 > Lightning Address > npub
func (a *AuthService) GetDisplayNameForUser(userID uuid.UUID) string {
	var nostrPubkey, nip05Address, lightningAddress sql.NullString

	err := a.db.QueryRow(`
		SELECT am.nostr_pubkey, am.nip05_address
		FROM auth_methods am
		WHERE am.user_id = $1 AND am.type = 'nostr'`,
		userID).Scan(&nostrPubkey, &nip05Address)

	if err != nil && err != sql.ErrNoRows {
		return ""
	}

	// Check for Lightning address
	a.db.QueryRow(`
		SELECT am.identifier
		FROM auth_methods am
		WHERE am.user_id = $1 AND am.type = 'lightning_address'`,
		userID).Scan(&lightningAddress)

	// Priority: NIP-05 > Lightning > npub
	if nip05Address.Valid && nip05Address.String != "" {
		return nip05Address.String
	}

	if lightningAddress.Valid && lightningAddress.String != "" {
		return lightningAddress.String
	}

	if nostrPubkey.Valid && nostrPubkey.String != "" {
		return identity.FormatNpubShort(nostrPubkey.String)
	}

	return ""
}

// PopulateUserDisplayInfo populates extended display fields for a user
func (a *AuthService) PopulateUserDisplayInfo(user *models.User) error {
	var nostrPubkey, nip05Address sql.NullString
	var authType sql.NullString

	// Get Nostr auth method info
	err := a.db.QueryRow(`
		SELECT am.nostr_pubkey, am.nip05_address, am.type
		FROM auth_methods am
		WHERE am.user_id = $1
		ORDER BY CASE am.type
			WHEN 'nostr' THEN 1
			WHEN 'lightning_address' THEN 2
			ELSE 3
		END
		LIMIT 1`,
		user.ID).Scan(&nostrPubkey, &nip05Address, &authType)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get user auth info: %w", err)
	}

	if authType.Valid {
		user.AuthMethod = authType.String
	}

	if nostrPubkey.Valid && nostrPubkey.String != "" {
		user.NostrPubkey = nostrPubkey.String
	}

	if nip05Address.Valid && nip05Address.String != "" {
		user.NIP05Address = nip05Address.String
	}

	// Check for Lightning address
	var lightningAddr sql.NullString
	a.db.QueryRow(`
		SELECT am.identifier
		FROM auth_methods am
		WHERE am.user_id = $1 AND am.type = 'lightning_address'`,
		user.ID).Scan(&lightningAddr)

	if lightningAddr.Valid && lightningAddr.String != "" {
		user.LightningAddress = lightningAddr.String
	}

	// Set display name with priority: NIP-05 > Lightning > npub
	user.DisplayName = identity.GetDisplayNameForNostrUser(
		user.NostrPubkey,
		&user.LightningAddress,
		&user.NIP05Address,
	)

	return nil
}
