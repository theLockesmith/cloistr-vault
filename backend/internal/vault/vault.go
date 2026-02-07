package vault

import (
	"database/sql"
	"fmt"
	"time"
	
	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

type Service struct {
	db *database.DB
}

func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// GetVault retrieves the user's encrypted vault
func (s *Service) GetVault(userID uuid.UUID) (*models.VaultResponse, error) {
	query := `
		SELECT id, encrypted_data, encryption_salt, encryption_nonce, version, last_modified
		FROM vaults 
		WHERE user_id = $1
	`
	
	var vault models.VaultResponse
	err := s.db.QueryRow(query, userID).Scan(
		&vault.ID,
		&vault.EncryptedData,
		&vault.EncryptionSalt,
		&vault.EncryptionNonce,
		&vault.Version,
		&vault.LastModified,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vault not found for user")
		}
		return nil, fmt.Errorf("failed to retrieve vault: %w", err)
	}
	
	return &vault, nil
}

// UpdateVault updates the user's encrypted vault with version checking
func (s *Service) UpdateVault(userID uuid.UUID, data []byte, version int) (*models.VaultResponse, error) {
	// Start transaction for atomic update
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Check current version for optimistic concurrency control
	var currentVersion int
	var vaultID uuid.UUID
	err = tx.QueryRow("SELECT id, version FROM vaults WHERE user_id = $1", userID).Scan(&vaultID, &currentVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vault not found for user")
		}
		return nil, fmt.Errorf("failed to check vault version: %w", err)
	}
	
	// Version check for conflict detection
	if currentVersion != version {
		return nil, fmt.Errorf("vault conflict: expected version %d, got %d", currentVersion, version)
	}
	
	// Update vault with incremented version
	now := time.Now()
	newVersion := version + 1
	
	_, err = tx.Exec(`
		UPDATE vaults 
		SET encrypted_data = $1, version = $2, last_modified = $3 
		WHERE user_id = $4`,
		data, newVersion, now, userID)
	
	if err != nil {
		return nil, fmt.Errorf("failed to update vault: %w", err)
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	// Return updated vault info
	return &models.VaultResponse{
		ID:           vaultID,
		Version:      newVersion,
		LastModified: now,
		EncryptedData: data,
		// Note: We don't return salt/nonce for security
	}, nil
}

// CreateVault creates a new vault for a user (used during registration)
func (s *Service) CreateVault(userID uuid.UUID, data []byte) (*models.VaultResponse, error) {
	vaultID := uuid.New()
	now := time.Now()
	version := 1
	
	_, err := s.db.Exec(`
		INSERT INTO vaults (id, user_id, encrypted_data, encryption_salt, encryption_nonce, version, last_modified, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		vaultID, userID, data, []byte{}, []byte{}, version, now, now)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create vault: %w", err)
	}
	
	return &models.VaultResponse{
		ID:           vaultID,
		Version:      version,
		LastModified: now,
		EncryptedData: data,
	}, nil
}

// GetVaultMetadata returns vault metadata without sensitive data
func (s *Service) GetVaultMetadata(userID uuid.UUID) (*VaultMetadata, error) {
	query := `
		SELECT id, version, last_modified, created_at
		FROM vaults 
		WHERE user_id = $1
	`
	
	var metadata VaultMetadata
	err := s.db.QueryRow(query, userID).Scan(
		&metadata.ID,
		&metadata.Version,
		&metadata.LastModified,
		&metadata.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vault not found for user")
		}
		return nil, fmt.Errorf("failed to retrieve vault metadata: %w", err)
	}
	
	return &metadata, nil
}

// VaultMetadata represents non-sensitive vault information
type VaultMetadata struct {
	ID           uuid.UUID `json:"id"`
	Version      int       `json:"version"`
	LastModified time.Time `json:"last_modified"`
	CreatedAt    time.Time `json:"created_at"`
}

// DeleteVault removes a user's vault (for account deletion)
func (s *Service) DeleteVault(userID uuid.UUID) error {
	_, err := s.db.Exec("DELETE FROM vaults WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete vault: %w", err)
	}
	return nil
}

// VaultExists checks if a user has a vault
func (s *Service) VaultExists(userID uuid.UUID) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM vaults WHERE user_id = $1)"
	err := s.db.QueryRow(query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check vault existence: %w", err)
	}
	return exists, nil
}