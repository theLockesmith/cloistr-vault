package kms

import (
	"fmt"
)

// TODO: Implement HashiCorp Vault KMS
// For now, return an error to fall back to file-based KMS
func NewVaultKMS(cfg *Config) (*FileKMS, error) {
	// Return error to trigger fallback to file-based KMS
	return nil, fmt.Errorf("Vault KMS not yet implemented - using file-based KMS")
}

// Placeholder - will be implemented when we add full Vault API support
// This file will contain the full HashiCorp Vault implementation
// including proper API client, key management, and cryptographic operations