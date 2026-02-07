package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	
	"golang.org/x/crypto/scrypt"
)

const (
	// Scrypt parameters (matching config defaults)
	ScryptN = 32768
	ScryptR = 8
	ScryptP = 1
	
	// Key and salt sizes
	KeySize      = 32 // AES-256
	SaltSize     = 32
	NonceSize    = 12 // GCM standard nonce size
)

var (
	ErrInvalidKeySize   = errors.New("invalid key size")
	ErrInvalidSaltSize  = errors.New("invalid salt size")
	ErrInvalidNonceSize = errors.New("invalid nonce size")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrEmptyPlaintext   = errors.New("plaintext cannot be empty")
)

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(size int) ([]byte, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// GenerateSalt generates a random salt for key derivation
func GenerateSalt() ([]byte, error) {
	return GenerateRandomBytes(SaltSize)
}

// GenerateNonce generates a random nonce for AES-GCM
func GenerateNonce() ([]byte, error) {
	return GenerateRandomBytes(NonceSize)
}

// DeriveKey derives a key from password and salt using scrypt
func DeriveKey(password string, salt []byte, n, r, p int) ([]byte, error) {
	if len(salt) != SaltSize {
		return nil, ErrInvalidSaltSize
	}
	
	key, err := scrypt.Key([]byte(password), salt, n, r, p, KeySize)
	if err != nil {
		return nil, fmt.Errorf("scrypt key derivation failed: %w", err)
	}
	
	return key, nil
}

// DeriveKeyDefault derives a key using default scrypt parameters
func DeriveKeyDefault(password string, salt []byte) ([]byte, error) {
	return DeriveKey(password, salt, ScryptN, ScryptR, ScryptP)
}

// HashPassword creates a hash of the password for storage
func HashPassword(password string, salt []byte) ([]byte, error) {
	key, err := DeriveKeyDefault(password, salt)
	if err != nil {
		return nil, err
	}
	
	// Hash the derived key for additional security
	hash := sha256.Sum256(key)
	return hash[:], nil
}

// VerifyPassword verifies a password against its hash
func VerifyPassword(password string, salt []byte, hash []byte) bool {
	computedHash, err := HashPassword(password, salt)
	if err != nil {
		return false
	}
	
	// Constant-time comparison to prevent timing attacks
	return constantTimeEqual(computedHash, hash)
}

// constantTimeEqual performs constant-time comparison
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	
	diff := byte(0)
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	
	return diff == 0
}

// EncryptAESGCM encrypts data using AES-256-GCM
func EncryptAESGCM(plaintext []byte, key []byte) (ciphertext []byte, nonce []byte, err error) {
	if len(plaintext) == 0 {
		return nil, nil, ErrEmptyPlaintext
	}
	
	if len(key) != KeySize {
		return nil, nil, ErrInvalidKeySize
	}
	
	// Generate random nonce
	nonce, err = GenerateNonce()
	if err != nil {
		return nil, nil, err
	}
	
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Encrypt
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	
	return ciphertext, nonce, nil
}

// DecryptAESGCM decrypts data using AES-256-GCM
func DecryptAESGCM(ciphertext []byte, key []byte, nonce []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}
	
	if len(nonce) != NonceSize {
		return nil, ErrInvalidNonceSize
	}
	
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	
	return plaintext, nil
}

// SecureWipe securely overwrites memory
func SecureWipe(data []byte) {
	for i := range data {
		data[i] = 0
	}
}