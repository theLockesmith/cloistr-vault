package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	btcschnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// NostrEvent represents a signed Nostr event (NIP-01).
type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// ComputeEventID computes the NIP-01 event ID: sha256 of the canonical serialization.
func ComputeEventID(pubkey string, createdAt int64, kind int, tags [][]string, content string) (string, error) {
	if tags == nil {
		tags = [][]string{}
	}
	serialized, err := json.Marshal([]interface{}{0, pubkey, createdAt, kind, tags, content})
	if err != nil {
		return "", fmt.Errorf("failed to serialize event: %w", err)
	}
	hash := sha256.Sum256(serialized)
	return hex.EncodeToString(hash[:]), nil
}

// VerifyNostrEventSignature verifies a Nostr event's ID and BIP-340 schnorr signature.
func VerifyNostrEventSignature(event *NostrEvent) error {
	if event == nil {
		return fmt.Errorf("nil event")
	}

	computedID, err := ComputeEventID(event.PubKey, event.CreatedAt, event.Kind, event.Tags, event.Content)
	if err != nil {
		return fmt.Errorf("failed to compute event ID: %w", err)
	}
	if computedID != event.ID {
		return fmt.Errorf("event ID mismatch")
	}

	pubKeyBytes, err := hex.DecodeString(event.PubKey)
	if err != nil || len(pubKeyBytes) != 32 {
		return fmt.Errorf("invalid pubkey: must be 32 bytes hex")
	}
	pubKey, err := btcschnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid pubkey: %w", err)
	}

	sigBytes, err := hex.DecodeString(event.Sig)
	if err != nil || len(sigBytes) != 64 {
		return fmt.Errorf("invalid signature: must be 64 bytes hex")
	}
	sig, err := btcschnorr.ParseSignature(sigBytes)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	idBytes, _ := hex.DecodeString(event.ID)
	if !sig.Verify(idBytes, pubKey) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// TagValue returns the first value for a given tag key, or empty string.
func (e *NostrEvent) TagValue(key string) string {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == key {
			return tag[1]
		}
	}
	return ""
}
