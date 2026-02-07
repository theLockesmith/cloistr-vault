package crypto

import (
	"testing"
)

func TestNostrAuthenticationFlowFixed(t *testing.T) {
	// 1. Generate key pair
	kp, err := GenerateNostrKeyPairFixed()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	publicKeyHex := kp.PublicKeyHex()
	if len(publicKeyHex) != 64 {
		t.Fatalf("Invalid public key length: expected 64, got %d", len(publicKeyHex))
	}
	
	// 2. Create authentication challenge
	challengeHex, _, err := CreateNostrAuthChallenge(publicKeyHex)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}
	
	if len(challengeHex) != 64 {
		t.Fatalf("Invalid challenge length: expected 64, got %d", len(challengeHex))
	}
	
	// 3. Sign the challenge
	signature, err := kp.SignChallenge(challengeHex)
	if err != nil {
		t.Fatalf("Failed to sign challenge: %v", err)
	}
	
	if signature == "" {
		t.Fatal("Empty signature returned")
	}
	
	// 4. Verify the signature
	valid := VerifyNostrAuthResponse(publicKeyHex, challengeHex, signature)
	if !valid {
		t.Fatal("Signature verification failed - this should work!")
	}
	
	// 5. Test with wrong challenge
	wrongChallenge := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	invalidValid := VerifyNostrAuthResponse(publicKeyHex, wrongChallenge, signature)
	if invalidValid {
		t.Fatal("Signature verification succeeded with wrong challenge - this should fail!")
	}
	
	t.Log("✅ Fixed Nostr authentication flow working perfectly!")
}

func TestNostrEventSigning(t *testing.T) {
	kp, err := GenerateNostrKeyPairFixed()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	eventData := []byte("test-event-data")
	
	// Sign event
	signature, err := SignNostrEvent(kp.PrivateKeyHex(), eventData)
	if err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}
	
	// Verify event
	valid := VerifyNostrEvent(kp.PublicKeyHex(), eventData, signature)
	if !valid {
		t.Fatal("Event signature verification failed")
	}
	
	// Test with different data
	differentData := []byte("different-event-data")
	invalidValid := VerifyNostrEvent(kp.PublicKeyHex(), differentData, signature)
	if invalidValid {
		t.Fatal("Signature verified with wrong data")
	}
	
	t.Log("✅ Nostr event signing working correctly!")
}

func BenchmarkNostrSigning(b *testing.B) {
	kp, _ := GenerateNostrKeyPairFixed()
	eventData := []byte("benchmark-event-data")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SignNostrEvent(kp.PrivateKeyHex(), eventData)
	}
}

func BenchmarkNostrVerification(b *testing.B) {
	kp, _ := GenerateNostrKeyPairFixed()
	eventData := []byte("benchmark-event-data")
	signature, _ := SignNostrEvent(kp.PrivateKeyHex(), eventData)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyNostrEvent(kp.PublicKeyHex(), eventData, signature)
	}
}