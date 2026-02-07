package kms

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileKMS implements a simple file-based KMS for development/fallback
type FileKMS struct {
	keyDir string
	config *Config
}

// NewFileKMS creates a new file-based KMS instance
func NewFileKMS(cfg *Config) (*FileKMS, error) {
	keyDir := cfg.Options["key_dir"]
	if keyDir == "" {
		keyDir = "./keys"
	}

	// Create key directory if it doesn't exist
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	return &FileKMS{
		keyDir: keyDir,
		config: cfg,
	}, nil
}

// GenerateKey generates a new key and stores it as a file
func (f *FileKMS) GenerateKey(ctx context.Context, keyType KeyType, keySize int) (*KeyInfo, error) {
	// Generate cryptographically secure random key
	keyBytes := make([]byte, keySize/8)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Create key info
	now := time.Now()
	version := fmt.Sprintf("v%d", now.Unix())
	keyInfo := &KeyInfo{
		ID:        fmt.Sprintf("%s-%s", keyType, version),
		Type:      keyType,
		Version:   version,
		Algorithm: f.getAlgorithmForKeyType(keyType),
		KeySize:   keySize,
		CreatedAt: now,
		Status:    KeyStatusActive,
		Metadata: map[string]string{
			"created_by": "coldforge-vault",
			"key_usage":  string(keyType),
		},
	}

	// Store key file
	keyPath := filepath.Join(f.keyDir, fmt.Sprintf("%s-%s.json", keyType, version))
	keyData := map[string]interface{}{
		"id":           keyInfo.ID,
		"type":         string(keyInfo.Type),
		"version":      keyInfo.Version,
		"algorithm":    keyInfo.Algorithm,
		"key_size":     keyInfo.KeySize,
		"created_at":   keyInfo.CreatedAt.Format(time.RFC3339),
		"status":       string(keyInfo.Status),
		"metadata":     keyInfo.Metadata,
		"key_material": keyBytes,
	}

	data, err := json.Marshal(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key data: %w", err)
	}

	if err := os.WriteFile(keyPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Update latest key symlink
	latestPath := filepath.Join(f.keyDir, fmt.Sprintf("%s-latest.json", keyType))
	os.Remove(latestPath) // Remove existing symlink
	if err := os.Symlink(filepath.Base(keyPath), latestPath); err != nil {
		// If symlink fails, just copy the file
		if err := os.WriteFile(latestPath, data, 0600); err != nil {
			return nil, fmt.Errorf("failed to create latest key pointer: %w", err)
		}
	}

	keyInfo.KeyMaterial = keyBytes
	return keyInfo, nil
}

// GetKey retrieves a specific key version from file
func (f *FileKMS) GetKey(ctx context.Context, keyType KeyType, version string) (*KeyInfo, error) {
	keyPath := filepath.Join(f.keyDir, fmt.Sprintf("%s-%s.json", keyType, version))

	data, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewKeyNotFoundError(keyType)
		}
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var keyData map[string]interface{}
	if err := json.Unmarshal(data, &keyData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key data: %w", err)
	}

	return f.parseKeyInfo(keyData)
}

// GetLatestKey retrieves the latest version of a key
func (f *FileKMS) GetLatestKey(ctx context.Context, keyType KeyType) (*KeyInfo, error) {
	latestPath := filepath.Join(f.keyDir, fmt.Sprintf("%s-latest.json", keyType))

	data, err := os.ReadFile(latestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewKeyNotFoundError(keyType)
		}
		return nil, fmt.Errorf("failed to read latest key file: %w", err)
	}

	var keyData map[string]interface{}
	if err := json.Unmarshal(data, &keyData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key data: %w", err)
	}

	return f.parseKeyInfo(keyData)
}

// Sign data using the specified key (simple implementation)
func (f *FileKMS) Sign(ctx context.Context, keyType KeyType, data []byte) ([]byte, error) {
	// Simple implementation for development
	return data[:min(len(data), 32)], nil
}

// Verify signature using the specified key
func (f *FileKMS) Verify(ctx context.Context, keyType KeyType, data []byte, signature []byte) error {
	expectedSig, err := f.Sign(ctx, keyType, data)
	if err != nil {
		return err
	}

	if len(signature) != len(expectedSig) {
		return fmt.Errorf("signature verification failed")
	}

	for i := range signature {
		if signature[i] != expectedSig[i] {
			return fmt.Errorf("signature verification failed")
		}
	}

	return nil
}

// Encrypt data using the specified key
func (f *FileKMS) Encrypt(ctx context.Context, keyType KeyType, plaintext []byte) ([]byte, error) {
	// Simple XOR encryption for development
	key, err := f.GetLatestKey(ctx, keyType)
	if err != nil {
		return nil, err
	}

	encrypted := make([]byte, len(plaintext))
	for i := range plaintext {
		encrypted[i] = plaintext[i] ^ key.KeyMaterial[i%len(key.KeyMaterial)]
	}

	return encrypted, nil
}

// Decrypt data using the specified key
func (f *FileKMS) Decrypt(ctx context.Context, keyType KeyType, ciphertext []byte) ([]byte, error) {
	// XOR decryption (same as encryption)
	return f.Encrypt(ctx, keyType, ciphertext)
}

// RotateKey creates a new version of the key
func (f *FileKMS) RotateKey(ctx context.Context, keyType KeyType) (*KeyInfo, error) {
	// Get current key to determine key size
	currentKey, err := f.GetLatestKey(ctx, keyType)
	if err != nil {
		// If no current key exists, create with default size
		return f.GenerateKey(ctx, keyType, f.getDefaultKeySize(keyType))
	}

	// Generate new key with same size
	return f.GenerateKey(ctx, keyType, currentKey.KeySize)
}

// DisableKey marks a key version as disabled
func (f *FileKMS) DisableKey(ctx context.Context, keyType KeyType, version string) error {
	keyPath := filepath.Join(f.keyDir, fmt.Sprintf("%s-%s.json", keyType, version))

	data, err := os.ReadFile(keyPath)
	if err != nil {
		return NewKeyNotFoundError(keyType)
	}

	var keyData map[string]interface{}
	if err := json.Unmarshal(data, &keyData); err != nil {
		return fmt.Errorf("failed to unmarshal key data: %w", err)
	}

	keyData["status"] = string(KeyStatusDisabled)

	updatedData, err := json.Marshal(keyData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated key data: %w", err)
	}

	return os.WriteFile(keyPath, updatedData, 0600)
}

// ListKeys returns all versions of a key type
func (f *FileKMS) ListKeys(ctx context.Context, keyType KeyType) ([]*KeyInfo, error) {
	pattern := filepath.Join(f.keyDir, fmt.Sprintf("%s-v*.json", keyType))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list key files: %w", err)
	}

	var keyInfos []*KeyInfo
	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			continue
		}

		var keyData map[string]interface{}
		if err := json.Unmarshal(data, &keyData); err != nil {
			continue
		}

		keyInfo, err := f.parseKeyInfo(keyData)
		if err != nil {
			continue
		}

		keyInfos = append(keyInfos, keyInfo)
	}

	return keyInfos, nil
}

// HealthCheck always returns nil for file-based KMS
func (f *FileKMS) HealthCheck(ctx context.Context) error {
	// Check if key directory is accessible
	_, err := os.Stat(f.keyDir)
	return err
}

// GetStatus returns the current KMS status
func (f *FileKMS) GetStatus(ctx context.Context) (*Status, error) {
	status := &Status{
		Provider:     "file",
		Healthy:      true,
		KeyCount:     make(map[KeyType]int),
		LastRotation: make(map[KeyType]time.Time),
		NextRotation: make(map[KeyType]time.Time),
	}

	// Check health
	if err := f.HealthCheck(ctx); err != nil {
		status.Healthy = false
		status.Errors = append(status.Errors, err.Error())
	}

	// Count keys for each type
	keyTypes := []KeyType{KeyTypeJWT, KeyTypeDatabase, KeyTypeRedis, KeyTypeLightning}
	for _, keyType := range keyTypes {
		keys, err := f.ListKeys(ctx, keyType)
		if err == nil {
			status.KeyCount[keyType] = len(keys)
		}
	}

	return status, nil
}

// Helper functions

func (f *FileKMS) parseKeyInfo(data map[string]interface{}) (*KeyInfo, error) {
	keyInfo := &KeyInfo{}

	if id, ok := data["id"].(string); ok {
		keyInfo.ID = id
	}

	if keyType, ok := data["type"].(string); ok {
		keyInfo.Type = KeyType(keyType)
	}

	if version, ok := data["version"].(string); ok {
		keyInfo.Version = version
	}

	if algorithm, ok := data["algorithm"].(string); ok {
		keyInfo.Algorithm = algorithm
	}

	if keySize, ok := data["key_size"].(float64); ok {
		keyInfo.KeySize = int(keySize)
	}

	if createdAtStr, ok := data["created_at"].(string); ok {
		if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			keyInfo.CreatedAt = createdAt
		}
	}

	if status, ok := data["status"].(string); ok {
		keyInfo.Status = KeyStatus(status)
	}

	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		keyInfo.Metadata = make(map[string]string)
		for k, v := range metadata {
			if s, ok := v.(string); ok {
				keyInfo.Metadata[k] = s
			}
		}
	}

	if keyMaterial, ok := data["key_material"].([]interface{}); ok {
		keyInfo.KeyMaterial = make([]byte, len(keyMaterial))
		for i, b := range keyMaterial {
			if byteVal, ok := b.(float64); ok {
				keyInfo.KeyMaterial[i] = byte(byteVal)
			}
		}
	}

	return keyInfo, nil
}

func (f *FileKMS) getAlgorithmForKeyType(keyType KeyType) string {
	switch keyType {
	case KeyTypeJWT:
		return "HMAC-SHA256"
	case KeyTypeDatabase:
		return "AES-256-GCM"
	case KeyTypeRedis:
		return "AES-256-GCM"
	case KeyTypeLightning:
		return "secp256k1"
	case KeyTypeNostrRelay:
		return "secp256k1"
	case KeyTypeAPISignature:
		return "HMAC-SHA256"
	default:
		return "AES-256-GCM"
	}
}

func (f *FileKMS) getDefaultKeySize(keyType KeyType) int {
	switch keyType {
	case KeyTypeJWT:
		return 256
	case KeyTypeDatabase:
		return 256
	case KeyTypeRedis:
		return 256
	case KeyTypeLightning:
		return 256
	case KeyTypeNostrRelay:
		return 256
	case KeyTypeAPISignature:
		return 256
	default:
		return 256
	}
}