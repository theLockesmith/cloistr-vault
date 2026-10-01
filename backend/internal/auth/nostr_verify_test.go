package auth

import (
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	btcschnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/coldforge/vault/internal/crypto"
)

func makeTestEvent(t *testing.T, privKey *btcec.PrivateKey, challengeValue string) string {
	t.Helper()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	createdAt := time.Now().Unix()
	tags := [][]string{{"challenge", challengeValue}}

	eventID, err := crypto.ComputeEventID(pubKeyHex, createdAt, 22242, tags, challengeValue)
	if err != nil {
		t.Fatalf("ComputeEventID: %v", err)
	}

	idBytes, _ := hex.DecodeString(eventID)
	sig, err := btcschnorr.Sign(privKey, idBytes)
	if err != nil {
		t.Fatalf("schnorr.Sign: %v", err)
	}

	event := crypto.NostrEvent{
		ID:        eventID,
		PubKey:    pubKeyHex,
		CreatedAt: createdAt,
		Kind:      22242,
		Tags:      tags,
		Content:   challengeValue,
		Sig:       hex.EncodeToString(sig.Serialize()),
	}

	b, _ := json.Marshal(event)
	return string(b)
}

func seedChallenge(pubkey, value string, expiresIn time.Duration) {
	challengeStore["test-challenge-id"] = Challenge{
		ID:        "test-challenge-id",
		Value:     value,
		ExpiresAt: time.Now().Add(expiresIn),
		Metadata: map[string]interface{}{
			"pubkey":    pubkey,
			"auth_type": "nostr",
		},
	}
}

func TestAuthenticateWithNostr_RejectsEmptyInputs(t *testing.T) {
	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr("", "")
	if err == nil {
		t.Fatal("empty inputs should be rejected")
	}
}

func TestAuthenticateWithNostr_RejectsInvalidJSON(t *testing.T) {
	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr("a]", "not-json")
	if err == nil {
		t.Fatal("invalid JSON should be rejected")
	}
}

func TestAuthenticateWithNostr_RejectsWrongKind(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))

	event := crypto.NostrEvent{
		ID:        "0000000000000000000000000000000000000000000000000000000000000000",
		PubKey:    pubKeyHex,
		CreatedAt: time.Now().Unix(),
		Kind:      1, // wrong kind
		Tags:      [][]string{{"challenge", "abc"}},
		Content:   "abc",
		Sig:       "a]",
	}
	b, _ := json.Marshal(event)

	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr(pubKeyHex, string(b))
	if err == nil {
		t.Fatal("wrong kind should be rejected")
	}
}

func TestAuthenticateWithNostr_RejectsMismatchedPubkey(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	otherKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	otherPubHex := hex.EncodeToString(btcschnorr.SerializePubKey(otherKey.PubKey()))

	eventJSON := makeTestEvent(t, privKey, "abc")

	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr(otherPubHex, eventJSON)
	if err == nil {
		t.Fatalf("mismatched pubkey should be rejected (event=%s, claimed=%s)", pubKeyHex, otherPubHex)
	}
}

func TestAuthenticateWithNostr_RejectsUnknownChallenge(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	eventJSON := makeTestEvent(t, privKey, "unknown-challenge-value")

	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr(pubKeyHex, eventJSON)
	if err != ErrInvalidChallenge {
		t.Fatalf("unknown challenge should return ErrInvalidChallenge, got %v", err)
	}
}

func TestAuthenticateWithNostr_RejectsExpiredChallenge(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	challengeVal := "expired-challenge-value"

	seedChallenge(pubKeyHex, challengeVal, -1*time.Minute) // already expired
	defer delete(challengeStore, "test-challenge-id")

	eventJSON := makeTestEvent(t, privKey, challengeVal)

	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr(pubKeyHex, eventJSON)
	if err != ErrChallengeExpired {
		t.Fatalf("expired challenge should return ErrChallengeExpired, got %v", err)
	}
}

func TestAuthenticateWithNostr_RejectsReplayedChallenge(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	challengeVal := "replay-challenge-value"

	seedChallenge(pubKeyHex, challengeVal, 10*time.Minute)

	eventJSON := makeTestEvent(t, privKey, challengeVal)

	a := &AuthService{}
	// First attempt passes all crypto checks, consumes the challenge, then panics on nil db.
	// The challenge is already deleted from the store at that point.
	func() {
		defer func() { recover() }()
		_, _, _ = a.AuthenticateWithNostr(pubKeyHex, eventJSON)
	}()

	// Second attempt: challenge was deleted (single-use)
	_, _, err := a.AuthenticateWithNostr(pubKeyHex, eventJSON)
	if err != ErrInvalidChallenge {
		t.Fatalf("replayed challenge should return ErrInvalidChallenge, got %v", err)
	}
}

func TestAuthenticateWithNostr_RejectsForgedSignature(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	challengeVal := "forged-sig-challenge"

	seedChallenge(pubKeyHex, challengeVal, 10*time.Minute)
	defer delete(challengeStore, "test-challenge-id")

	eventJSON := makeTestEvent(t, privKey, challengeVal)

	// Tamper with the signature in the JSON
	var event crypto.NostrEvent
	json.Unmarshal([]byte(eventJSON), &event)
	sigBytes, _ := hex.DecodeString(event.Sig)
	sigBytes[0] ^= 0xff
	event.Sig = hex.EncodeToString(sigBytes)
	tampered, _ := json.Marshal(event)

	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr(pubKeyHex, string(tampered))
	if err != ErrInvalidCredentials {
		t.Fatalf("forged signature should return ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateWithNostr_RejectsWrongPubkeyChallenge(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	otherKey, _ := btcec.NewPrivateKey()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	otherPubHex := hex.EncodeToString(btcschnorr.SerializePubKey(otherKey.PubKey()))
	challengeVal := "wrong-pubkey-challenge"

	// Challenge was issued for otherKey, not privKey
	seedChallenge(otherPubHex, challengeVal, 10*time.Minute)
	defer delete(challengeStore, "test-challenge-id")

	eventJSON := makeTestEvent(t, privKey, challengeVal)

	a := &AuthService{}
	_, _, err := a.AuthenticateWithNostr(pubKeyHex, eventJSON)
	if err != ErrInvalidChallenge {
		t.Fatalf("challenge for wrong pubkey should return ErrInvalidChallenge, got %v", err)
	}
}
