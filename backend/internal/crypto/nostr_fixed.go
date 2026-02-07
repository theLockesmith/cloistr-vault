package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// NostrSignatureFixed implements proper Nostr signature handling
// Based on NIP-01 specification for event signing

// SignNostrEvent signs a Nostr event with the private key
func SignNostrEvent(privateKeyHex string, eventData []byte) (string, error) {
	// Parse private key
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key hex: %w", err)
	}
	
	if len(privateKeyBytes) != 32 {
		return "", fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privateKeyBytes))
	}
	
	privateKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
	
	// Hash the event data (Nostr uses SHA-256)
	hash := sha256.Sum256(eventData)
	
	// Sign with deterministic k (RFC 6979)
	signature := ecdsa.Sign(privateKey, hash[:])
	
	// Return as hex string (DER encoding)
	return hex.EncodeToString(signature.Serialize()), nil
}

// VerifyNostrEvent verifies a Nostr event signature
func VerifyNostrEvent(publicKeyHex string, eventData []byte, signatureHex string) bool {
	// Parse public key
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false
	}
	
	if len(publicKeyBytes) != 32 {
		return false
	}
	
	// Create compressed public key (add 0x02 prefix)
	compressedKey := make([]byte, 33)
	compressedKey[0] = 0x02
	copy(compressedKey[1:], publicKeyBytes)
	
	publicKey, err := secp256k1.ParsePubKey(compressedKey)
	if err != nil {
		return false
	}
	
	// Parse signature
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	
	signature, err := ecdsa.ParseDERSignature(signatureBytes)
	if err != nil {
		return false
	}
	
	// Hash the event data
	hash := sha256.Sum256(eventData)
	
	// Verify signature
	return signature.Verify(hash[:], publicKey)
}

// CreateNostrAuthChallenge creates a proper Nostr authentication challenge
func CreateNostrAuthChallenge(publicKeyHex string) (string, []byte, error) {
	// Generate random challenge
	challengeBytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate challenge: %w", err)
	}
	
	// Create challenge string
	challengeHex := hex.EncodeToString(challengeBytes)
	
	// Create event data for signing (simplified Nostr event structure)
	eventData := []byte(fmt.Sprintf("coldforge-auth:%s:%s", publicKeyHex, challengeHex))
	
	return challengeHex, eventData, nil
}

// VerifyNostrAuthResponse verifies the response to an auth challenge
func VerifyNostrAuthResponse(publicKeyHex, challengeHex, signatureHex string) bool {
	// Recreate the event data that should have been signed
	eventData := []byte(fmt.Sprintf("coldforge-auth:%s:%s", publicKeyHex, challengeHex))
	
	// Verify the signature
	return VerifyNostrEvent(publicKeyHex, eventData, signatureHex)
}

// NostrKeyPairFixed represents a working Nostr key pair
type NostrKeyPairFixed struct {
	privateKey *secp256k1.PrivateKey
	publicKey  *secp256k1.PublicKey
}

// GenerateNostrKeyPairFixed generates a new Nostr key pair
func GenerateNostrKeyPairFixed() (*NostrKeyPairFixed, error) {
	privateKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	
	return &NostrKeyPairFixed{
		privateKey: privateKey,
		publicKey:  privateKey.PubKey(),
	}, nil
}

// PrivateKeyHex returns the private key as hex (32 bytes)
func (kp *NostrKeyPairFixed) PrivateKeyHex() string {
	return hex.EncodeToString(kp.privateKey.Serialize())
}

// PublicKeyHex returns the public key as hex (32 bytes, x-only)
func (kp *NostrKeyPairFixed) PublicKeyHex() string {
	return hex.EncodeToString(kp.publicKey.SerializeCompressed()[1:])
}

// SignChallenge signs an authentication challenge
func (kp *NostrKeyPairFixed) SignChallenge(challengeHex string) (string, error) {
	// Create event data
	eventData := []byte(fmt.Sprintf("coldforge-auth:%s:%s", kp.PublicKeyHex(), challengeHex))
	
	// Sign the event
	return SignNostrEvent(kp.PrivateKeyHex(), eventData)
}