package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

func TestExtractLightningUsername(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "standard lightning address",
			address:  "alice@coldforge.xyz",
			expected: "alice",
		},
		{
			name:     "address with numbers",
			address:  "user123@domain.com",
			expected: "user123",
		},
		{
			name:     "no @ symbol",
			address:  "invalidaddress",
			expected: "invalidaddre", // returns first 12 chars
		},
		{
			name:     "short string no @",
			address:  "short",
			expected: "short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLightningUsername(tt.address)
			if got != tt.expected {
				t.Errorf("extractLightningUsername(%q) = %q, want %q", tt.address, got, tt.expected)
			}
		})
	}
}

func TestVerifyLNURLAuthSignature(t *testing.T) {
	// Generate a test key pair
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()
	pubKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

	// Create a k1 challenge
	k1Bytes := make([]byte, 32)
	for i := range k1Bytes {
		k1Bytes[i] = byte(i)
	}
	k1Hex := hex.EncodeToString(k1Bytes)

	// Sign the k1 (LNURL-auth signs sha256(k1))
	msgHash := sha256.Sum256(k1Bytes)
	sig := ecdsa.Sign(privKey, msgHash[:])

	// Serialize signature to compact format (64 bytes)
	// The Serialize method returns DER format, so we use SerializeCompact which returns R || S
	sigBytes := sig.Serialize()
	// DER format needs to be converted to compact format
	// Instead, we'll sign using the SignCompact method
	sigCompact := ecdsa.SignCompact(privKey, msgHash[:], false)
	// SignCompact returns [v (1 byte)][r (32 bytes)][s (32 bytes)]
	// We need to skip the first byte (recovery flag) for LNURL-auth
	sigHex := hex.EncodeToString(sigCompact[1:]) // Skip the recovery byte
	_ = sigBytes                                   // Silence unused variable

	t.Run("valid signature", func(t *testing.T) {
		valid, err := verifyLNURLAuthSignature(k1Hex, sigHex, pubKeyHex)
		if err != nil {
			t.Fatalf("verifyLNURLAuthSignature() error = %v", err)
		}
		if !valid {
			t.Errorf("verifyLNURLAuthSignature() = false, want true")
		}
	})

	t.Run("invalid k1 hex", func(t *testing.T) {
		_, err := verifyLNURLAuthSignature("invalid_hex", sigHex, pubKeyHex)
		if err == nil {
			t.Errorf("verifyLNURLAuthSignature() should error on invalid k1 hex")
		}
	})

	t.Run("invalid signature hex", func(t *testing.T) {
		_, err := verifyLNURLAuthSignature(k1Hex, "invalid_hex", pubKeyHex)
		if err == nil {
			t.Errorf("verifyLNURLAuthSignature() should error on invalid signature hex")
		}
	})

	t.Run("wrong signature length", func(t *testing.T) {
		_, err := verifyLNURLAuthSignature(k1Hex, "abcd", pubKeyHex)
		if err == nil {
			t.Errorf("verifyLNURLAuthSignature() should error on wrong signature length")
		}
	})

	t.Run("invalid public key", func(t *testing.T) {
		_, err := verifyLNURLAuthSignature(k1Hex, sigHex, "invalid_pubkey_hex")
		if err == nil {
			t.Errorf("verifyLNURLAuthSignature() should error on invalid public key")
		}
	})

	t.Run("wrong public key", func(t *testing.T) {
		// Generate a different key pair
		otherPrivKey, _ := secp256k1.GeneratePrivateKey()
		otherPubKeyHex := hex.EncodeToString(otherPrivKey.PubKey().SerializeCompressed())

		valid, err := verifyLNURLAuthSignature(k1Hex, sigHex, otherPubKeyHex)
		if err != nil {
			t.Fatalf("verifyLNURLAuthSignature() error = %v", err)
		}
		if valid {
			t.Errorf("verifyLNURLAuthSignature() = true, want false (wrong key)")
		}
	})
}

func TestGenerateLightningChallenge(t *testing.T) {
	// Create an auth service for testing (without DB connection for unit tests)
	authService := &AuthService{}

	t.Run("valid lightning address", func(t *testing.T) {
		challenge, err := authService.GenerateLightningChallenge("alice@coldforge.xyz")
		if err != nil {
			t.Fatalf("GenerateLightningChallenge() error = %v", err)
		}

		// Check challenge format
		if len(challenge.Value) != 64 { // 32 bytes = 64 hex chars
			t.Errorf("Challenge value length = %d, want 64", len(challenge.Value))
		}

		// Check metadata
		if challenge.Metadata["lightning_address"] != "alice@coldforge.xyz" {
			t.Errorf("Challenge metadata lightning_address = %v, want alice@coldforge.xyz", challenge.Metadata["lightning_address"])
		}

		if challenge.Metadata["auth_type"] != "lnurl_auth" {
			t.Errorf("Challenge metadata auth_type = %v, want lnurl_auth", challenge.Metadata["auth_type"])
		}

		// Check expiration (should be ~10 minutes from now)
		expectedExpiry := time.Now().Add(10 * time.Minute)
		if challenge.ExpiresAt.Before(time.Now().Add(9*time.Minute)) || challenge.ExpiresAt.After(expectedExpiry.Add(1*time.Minute)) {
			t.Errorf("Challenge ExpiresAt = %v, expected around %v", challenge.ExpiresAt, expectedExpiry)
		}
	})

	t.Run("empty lightning address", func(t *testing.T) {
		_, err := authService.GenerateLightningChallenge("")
		if err == nil {
			t.Error("GenerateLightningChallenge() should error on empty address")
		}
	})
}

func TestLNURLAuthChallengeStore(t *testing.T) {
	authService := &AuthService{}

	// Generate a challenge
	challenge, err := authService.GenerateLightningChallenge("test@example.com")
	if err != nil {
		t.Fatalf("GenerateLightningChallenge() error = %v", err)
	}

	// Challenge should be stored
	stored, exists := challengeStore[challenge.ID]
	if !exists {
		t.Error("Challenge should be stored in challengeStore")
	}

	if stored.Value != challenge.Value {
		t.Errorf("Stored challenge value = %v, want %v", stored.Value, challenge.Value)
	}

	// Clean up
	delete(challengeStore, challenge.ID)
}
