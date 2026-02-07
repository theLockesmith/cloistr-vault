package kms

import (
	"context"
	"fmt"
	"time"
)

// NewKMS creates a new KMS instance based on the configuration
func NewKMS(cfg *Config) (KMS, error) {
	switch cfg.Provider {
	case "vault", "hashicorp_vault":
		return NewVaultKMS(cfg)
	case "file":
		return NewFileKMS(cfg)
	case "aws":
		// TODO: Implement AWS KMS
		return nil, fmt.Errorf("AWS KMS not yet implemented")
	case "azure":
		// TODO: Implement Azure Key Vault
		return nil, fmt.Errorf("Azure Key Vault not yet implemented")
	case "gcp":
		// TODO: Implement Google Cloud KMS
		return nil, fmt.Errorf("Google Cloud KMS not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported KMS provider: %s", cfg.Provider)
	}
}

// DefaultConfig returns a default configuration for development
func DefaultConfig() *Config {
	return &Config{
		Provider:  "vault",
		Address:   "http://localhost:7712",
		Token:     "coldforge-dev-token",
		MountPath: "secret",
		AutoRotate: true,
		RotationSchedule: map[KeyType]time.Duration{
			KeyTypeJWT:         30 * 24 * time.Hour, // 30 days
			KeyTypeDatabase:    90 * 24 * time.Hour, // 90 days
			KeyTypeRedis:       30 * 24 * time.Hour, // 30 days
			KeyTypeLightning:   365 * 24 * time.Hour, // 1 year
			KeyTypeNostrRelay:  180 * 24 * time.Hour, // 6 months
			KeyTypeAPISignature: 60 * 24 * time.Hour, // 60 days
		},
	}
}

// InitializeDefaultKeys creates default keys for all key types if they don't exist
func InitializeDefaultKeys(kms KMS) error {
	keyTypes := []KeyType{
		KeyTypeJWT,
		KeyTypeDatabase,
		KeyTypeRedis,
		KeyTypeAPISignature,
	}

	for _, keyType := range keyTypes {
		// Check if key already exists
		_, err := kms.GetLatestKey(context.Background(), keyType)
		if err == nil {
			continue // Key already exists
		}

		// Generate default key
		keySize := getDefaultKeySizeForType(keyType)
		_, err = kms.GenerateKey(context.Background(), keyType, keySize)
		if err != nil {
			return fmt.Errorf("failed to generate default key for %s: %w", keyType, err)
		}

		fmt.Printf("Generated default %s key\n", keyType)
	}

	return nil
}

func getDefaultKeySizeForType(keyType KeyType) int {
	switch keyType {
	case KeyTypeJWT:
		return 256 // 32 bytes
	case KeyTypeDatabase:
		return 256 // 32 bytes for AES-256
	case KeyTypeRedis:
		return 256 // 32 bytes for AES-256
	case KeyTypeLightning:
		return 256 // secp256k1 private key
	case KeyTypeNostrRelay:
		return 256 // secp256k1 private key
	case KeyTypeAPISignature:
		return 256 // 32 bytes for HMAC
	default:
		return 256
	}
}