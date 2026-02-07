package kms

import (
	"context"
	"time"
)

// KeyType represents different types of keys managed by KMS
type KeyType string

const (
	KeyTypeJWT         KeyType = "jwt"
	KeyTypeDatabase    KeyType = "database"
	KeyTypeRedis       KeyType = "redis"
	KeyTypeLightning   KeyType = "lightning"
	KeyTypeNostrRelay  KeyType = "nostr_relay"
	KeyTypeAPISignature KeyType = "api_signature"
)

// KMS defines the interface for key management operations
type KMS interface {
	// Key Generation
	GenerateKey(ctx context.Context, keyType KeyType, keySize int) (*KeyInfo, error)

	// Key Retrieval
	GetKey(ctx context.Context, keyType KeyType, version string) (*KeyInfo, error)
	GetLatestKey(ctx context.Context, keyType KeyType) (*KeyInfo, error)

	// Cryptographic Operations
	Sign(ctx context.Context, keyType KeyType, data []byte) ([]byte, error)
	Verify(ctx context.Context, keyType KeyType, data []byte, signature []byte) error
	Encrypt(ctx context.Context, keyType KeyType, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, keyType KeyType, ciphertext []byte) ([]byte, error)

	// Key Management
	RotateKey(ctx context.Context, keyType KeyType) (*KeyInfo, error)
	DisableKey(ctx context.Context, keyType KeyType, version string) error
	ListKeys(ctx context.Context, keyType KeyType) ([]*KeyInfo, error)

	// Health & Status
	HealthCheck(ctx context.Context) error
	GetStatus(ctx context.Context) (*Status, error)
}

// KeyInfo contains metadata about a managed key
type KeyInfo struct {
	ID          string            `json:"id"`
	Type        KeyType           `json:"type"`
	Version     string            `json:"version"`
	Algorithm   string            `json:"algorithm"`
	KeySize     int               `json:"key_size"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Status      KeyStatus         `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	// Actual key material (only populated when needed)
	KeyMaterial []byte `json:"-"` // Never serialize
}

// KeyStatus represents the current status of a key
type KeyStatus string

const (
	KeyStatusActive    KeyStatus = "active"
	KeyStatusRotating  KeyStatus = "rotating"
	KeyStatusDisabled  KeyStatus = "disabled"
	KeyStatusExpired   KeyStatus = "expired"
)

// Status represents the overall KMS status
type Status struct {
	Healthy       bool              `json:"healthy"`
	Provider      string            `json:"provider"`
	Version       string            `json:"version"`
	KeyCount      map[KeyType]int   `json:"key_count"`
	LastRotation  map[KeyType]time.Time `json:"last_rotation"`
	NextRotation  map[KeyType]time.Time `json:"next_rotation"`
	Errors        []string          `json:"errors,omitempty"`
}

// Config represents KMS configuration
type Config struct {
	Provider     string            `json:"provider"`      // "vault", "aws", "azure", "gcp", "file"
	Address      string            `json:"address"`       // KMS endpoint
	Token        string            `json:"token"`         // Authentication token
	MountPath    string            `json:"mount_path"`    // Vault mount path
	Region       string            `json:"region"`        // Cloud provider region
	Options      map[string]string `json:"options"`       // Provider-specific options

	// Rotation policy
	AutoRotate      bool                    `json:"auto_rotate"`
	RotationSchedule map[KeyType]time.Duration `json:"rotation_schedule"`
}

// Error types
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	KeyType KeyType `json:"key_type,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// Common error codes
const (
	ErrCodeKeyNotFound     = "KEY_NOT_FOUND"
	ErrCodeInvalidKeyType  = "INVALID_KEY_TYPE"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeProviderError   = "PROVIDER_ERROR"
	ErrCodeEncryptionError = "ENCRYPTION_ERROR"
	ErrCodeDecryptionError = "DECRYPTION_ERROR"
)

// Helper functions for creating standard errors
func NewKeyNotFoundError(keyType KeyType) *Error {
	return &Error{
		Code:    ErrCodeKeyNotFound,
		Message: "key not found",
		KeyType: keyType,
	}
}

func NewProviderError(message string) *Error {
	return &Error{
		Code:    ErrCodeProviderError,
		Message: message,
	}
}