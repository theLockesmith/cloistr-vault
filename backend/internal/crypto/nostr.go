package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/scrypt"
)

var (
	ErrInvalidNostrKey   = errors.New("invalid nostr key")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrInvalidChallenge  = errors.New("invalid challenge")
)

// NostrKeyPair represents a Nostr key pair
type NostrKeyPair struct {
	PrivateKey *secp256k1.PrivateKey
	PublicKey  *secp256k1.PublicKey
}

// GenerateNostrKeyPair generates a new Nostr key pair
func GenerateNostrKeyPair() (*NostrKeyPair, error) {
	privateKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	
	publicKey := privateKey.PubKey()
	
	return &NostrKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// NostrKeyPairFromPrivateKey creates a key pair from a private key hex string
func NostrKeyPairFromPrivateKey(privateKeyHex string) (*NostrKeyPair, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	
	if len(privateKeyBytes) != 32 {
		return nil, ErrInvalidNostrKey
	}
	
	privateKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
	publicKey := privateKey.PubKey()
	
	return &NostrKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// NostrPublicKeyFromHex creates a public key from hex string
func NostrPublicKeyFromHex(publicKeyHex string) (*secp256k1.PublicKey, error) {
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key hex: %w", err)
	}
	
	if len(publicKeyBytes) != 32 {
		return nil, ErrInvalidNostrKey
	}
	
	// Add the 0x02 prefix for compressed public key
	compressedKey := make([]byte, 33)
	compressedKey[0] = 0x02
	copy(compressedKey[1:], publicKeyBytes)
	
	publicKey, err := secp256k1.ParsePubKey(compressedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	
	return publicKey, nil
}

// PrivateKeyHex returns the private key as hex string
func (kp *NostrKeyPair) PrivateKeyHex() string {
	return hex.EncodeToString(kp.PrivateKey.Serialize())
}

// PublicKeyHex returns the public key as hex string (Nostr format - x-only)
func (kp *NostrKeyPair) PublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey.SerializeCompressed()[1:]) // Remove 0x02 prefix
}

// PublicKeyHex returns the public key as hex string (Nostr format - x-only)
func PublicKeyToHex(pubKey *secp256k1.PublicKey) string {
	return hex.EncodeToString(pubKey.SerializeCompressed()[1:]) // Remove 0x02 prefix
}

// SignChallenge signs a challenge string with the private key
func (kp *NostrKeyPair) SignChallenge(challenge string) (string, error) {
	if challenge == "" {
		return "", ErrInvalidChallenge
	}
	
	// Hash the challenge
	hash := sha256.Sum256([]byte(challenge))
	
	// Sign the hash
	signature := ecdsa.Sign(kp.PrivateKey, hash[:])
	
	return hex.EncodeToString(signature.Serialize()), nil
}

// VerifySignature verifies a signature against a challenge and public key
func VerifyNostrSignature(challenge string, signatureHex string, publicKeyHex string) bool {
	if challenge == "" || signatureHex == "" || publicKeyHex == "" {
		return false
	}
	
	// Parse public key
	publicKey, err := NostrPublicKeyFromHex(publicKeyHex)
	if err != nil {
		return false
	}
	
	// Parse signature
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	
	var signature *ecdsa.Signature
	
	// Try parsing as DER signature first
	signature, err = ecdsa.ParseDERSignature(signatureBytes)
	if err != nil {
		// For now, only support DER signatures
		// TODO: Add support for compact signatures if needed
		return false
	}
	
	// Hash the challenge
	hash := sha256.Sum256([]byte(challenge))
	
	// Verify signature
	return signature.Verify(hash[:], publicKey)
}

// GenerateChallenge generates a random challenge for authentication
func GenerateChallenge() (string, error) {
	challengeBytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	
	return hex.EncodeToString(challengeBytes), nil
}

// DeriveKeyFromNostrPrivateKey derives an encryption key from a Nostr private key
func DeriveKeyFromNostrPrivateKey(privateKeyHex string, salt []byte) ([]byte, error) {
	if len(salt) != SaltSize {
		return nil, ErrInvalidSaltSize
	}
	
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	
	if len(privateKeyBytes) != 32 {
		return nil, ErrInvalidNostrKey
	}
	
	// Use the private key bytes as "password" for scrypt
	// This creates a deterministic key derivation from the Nostr private key
	key, err := scrypt.Key(privateKeyBytes, salt, ScryptN, ScryptR, ScryptP, KeySize)
	if err != nil {
		return nil, fmt.Errorf("scrypt key derivation failed: %w", err)
	}
	
	return key, nil
}

// HashNostrPublicKey creates a hash of the public key for storage/lookup
func HashNostrPublicKey(publicKeyHex string) ([]byte, error) {
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key hex: %w", err)
	}
	
	if len(publicKeyBytes) != 32 {
		return nil, ErrInvalidNostrKey
	}
	
	hash := sha256.Sum256(publicKeyBytes)
	return hash[:], nil
}