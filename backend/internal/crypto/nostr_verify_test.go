package crypto

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	btcschnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func makeSignedEvent(t *testing.T, privKey *btcec.PrivateKey, kind int, tags [][]string, content string) *NostrEvent {
	t.Helper()
	pubKeyHex := hex.EncodeToString(btcschnorr.SerializePubKey(privKey.PubKey()))
	createdAt := time.Now().Unix()

	eventID, err := ComputeEventID(pubKeyHex, createdAt, kind, tags, content)
	if err != nil {
		t.Fatalf("ComputeEventID: %v", err)
	}

	idBytes, _ := hex.DecodeString(eventID)
	sig, err := btcschnorr.Sign(privKey, idBytes)
	if err != nil {
		t.Fatalf("schnorr.Sign: %v", err)
	}

	return &NostrEvent{
		ID:        eventID,
		PubKey:    pubKeyHex,
		CreatedAt: createdAt,
		Kind:      kind,
		Tags:      tags,
		Content:   content,
		Sig:       hex.EncodeToString(sig.Serialize()),
	}
}

func TestVerifyNostrEventSignature_Valid(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	event := makeSignedEvent(t, privKey, 22242, [][]string{{"challenge", "abc123"}}, "abc123")

	if err := VerifyNostrEventSignature(event); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
}

func TestVerifyNostrEventSignature_ForgedSignature(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	event := makeSignedEvent(t, privKey, 22242, [][]string{{"challenge", "abc123"}}, "abc123")

	// Flip a byte in the signature
	sigBytes, _ := hex.DecodeString(event.Sig)
	sigBytes[0] ^= 0xff
	event.Sig = hex.EncodeToString(sigBytes)

	if err := VerifyNostrEventSignature(event); err == nil {
		t.Fatal("forged signature should be rejected")
	}
}

func TestVerifyNostrEventSignature_EmptySignature(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	event := makeSignedEvent(t, privKey, 22242, [][]string{{"challenge", "abc123"}}, "abc123")
	event.Sig = ""

	if err := VerifyNostrEventSignature(event); err == nil {
		t.Fatal("empty signature should be rejected")
	}
}

func TestVerifyNostrEventSignature_WrongPubkey(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	otherKey, _ := btcec.NewPrivateKey()
	event := makeSignedEvent(t, privKey, 22242, [][]string{{"challenge", "abc123"}}, "abc123")

	// Replace pubkey with a different one (event ID will mismatch)
	event.PubKey = hex.EncodeToString(btcschnorr.SerializePubKey(otherKey.PubKey()))

	if err := VerifyNostrEventSignature(event); err == nil {
		t.Fatal("wrong pubkey should be rejected")
	}
}

func TestVerifyNostrEventSignature_TamperedContent(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	event := makeSignedEvent(t, privKey, 22242, [][]string{{"challenge", "abc123"}}, "abc123")
	event.Content = "tampered"

	if err := VerifyNostrEventSignature(event); err == nil {
		t.Fatal("tampered content should be rejected")
	}
}

func TestVerifyNostrEventSignature_TamperedEventID(t *testing.T) {
	privKey, _ := btcec.NewPrivateKey()
	event := makeSignedEvent(t, privKey, 22242, [][]string{{"challenge", "abc123"}}, "abc123")
	event.ID = "0000000000000000000000000000000000000000000000000000000000000000"

	if err := VerifyNostrEventSignature(event); err == nil {
		t.Fatal("tampered event ID should be rejected")
	}
}

func TestVerifyNostrEventSignature_NilEvent(t *testing.T) {
	if err := VerifyNostrEventSignature(nil); err == nil {
		t.Fatal("nil event should be rejected")
	}
}

func TestComputeEventID_Deterministic(t *testing.T) {
	id1, err := ComputeEventID("aabb", 1000, 1, [][]string{{"p", "cc"}}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := ComputeEventID("aabb", 1000, 1, [][]string{{"p", "cc"}}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("event IDs should be deterministic: %s != %s", id1, id2)
	}
	if len(id1) != 64 {
		t.Fatalf("event ID should be 64 hex chars, got %d", len(id1))
	}
}

func TestComputeEventID_NilTags(t *testing.T) {
	id, err := ComputeEventID("aabb", 1000, 1, nil, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 64 {
		t.Fatalf("event ID should be 64 hex chars, got %d", len(id))
	}
}

func TestTagValue(t *testing.T) {
	event := &NostrEvent{
		Tags: [][]string{{"challenge", "abc"}, {"relay", "wss://example.com"}},
	}
	if v := event.TagValue("challenge"); v != "abc" {
		t.Fatalf("expected abc, got %s", v)
	}
	if v := event.TagValue("missing"); v != "" {
		t.Fatalf("expected empty, got %s", v)
	}
}
