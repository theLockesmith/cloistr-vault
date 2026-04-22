package vault

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// SecretService handles secret operations within vault entries
type SecretService struct {
	db *database.DB
}

// NewSecretService creates a new secret service
func NewSecretService(db *database.DB) *SecretService {
	return &SecretService{db: db}
}

// AddSecret adds a secret to an entry
func (s *SecretService) AddSecret(entryID, userID uuid.UUID, req *models.CreateSecretInput) (*models.VaultSecret, error) {
	// Verify entry belongs to user
	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify entry ownership: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("entry not found or access denied")
	}

	// Get next position
	position, err := s.getNextPosition(entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next position: %w", err)
	}

	secret := &models.VaultSecret{
		ID:             uuid.New(),
		EntryID:        entryID,
		SecretType:     req.SecretType,
		Name:           req.Name,
		EncryptedValue: req.EncryptedValue,
		Username:       req.Username,
		ExpiresAt:      req.ExpiresAt,
		StrengthScore:  req.StrengthScore,
		Position:       position,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	query := `
		INSERT INTO vault_secrets (id, entry_id, secret_type, name, encrypted_value, username, expires_at, strength_score, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = s.db.Exec(query,
		secret.ID,
		secret.EntryID,
		secret.SecretType,
		secret.Name,
		secret.EncryptedValue,
		secret.Username,
		secret.ExpiresAt,
		secret.StrengthScore,
		secret.Position,
		secret.CreatedAt,
		secret.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	// Record history on entry
	_, _ = s.db.Exec(`
		INSERT INTO vault_entry_history (id, entry_id, user_id, action, created_at)
		VALUES ($1, $2, $3, 'secret_added', $4)
	`, uuid.New(), entryID, userID, time.Now())

	return secret, nil
}

// UpdateSecret updates a secret
func (s *SecretService) UpdateSecret(secretID, userID uuid.UUID, req *UpdateSecretRequest) (*models.VaultSecret, error) {
	// Get secret and verify ownership
	secret, entryID, err := s.getSecretWithEntry(secretID)
	if err != nil {
		return nil, err
	}

	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil || !exists {
		return nil, fmt.Errorf("secret not found or access denied")
	}

	// Build update
	if req.Name != nil {
		secret.Name = *req.Name
	}
	if req.SecretType != nil {
		secret.SecretType = *req.SecretType
	}
	if req.EncryptedValue != nil {
		secret.EncryptedValue = *req.EncryptedValue
		secret.LastRotated = ptrTime(time.Now())
	}
	if req.Username != nil {
		secret.Username = req.Username
	}
	if req.ExpiresAt != nil {
		secret.ExpiresAt = req.ExpiresAt
	}
	if req.StrengthScore != nil {
		secret.StrengthScore = *req.StrengthScore
	}
	if req.Position != nil {
		secret.Position = *req.Position
	}

	secret.UpdatedAt = time.Now()

	query := `
		UPDATE vault_secrets
		SET secret_type = $1, name = $2, encrypted_value = $3, username = $4,
		    expires_at = $5, last_rotated = $6, strength_score = $7, position = $8, updated_at = $9
		WHERE id = $10
	`

	_, err = s.db.Exec(query,
		secret.SecretType,
		secret.Name,
		secret.EncryptedValue,
		secret.Username,
		secret.ExpiresAt,
		secret.LastRotated,
		secret.StrengthScore,
		secret.Position,
		secret.UpdatedAt,
		secretID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	// Record history
	_, _ = s.db.Exec(`
		INSERT INTO vault_entry_history (id, entry_id, user_id, action, created_at)
		VALUES ($1, $2, $3, 'secret_updated', $4)
	`, uuid.New(), entryID, userID, time.Now())

	return secret, nil
}

// DeleteSecret deletes a secret
func (s *SecretService) DeleteSecret(secretID, userID uuid.UUID) error {
	// Get entry for ownership check
	_, entryID, err := s.getSecretWithEntry(secretID)
	if err != nil {
		return err
	}

	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil || !exists {
		return fmt.Errorf("secret not found or access denied")
	}

	_, err = s.db.Exec(`DELETE FROM vault_secrets WHERE id = $1`, secretID)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Record history
	_, _ = s.db.Exec(`
		INSERT INTO vault_entry_history (id, entry_id, user_id, action, created_at)
		VALUES ($1, $2, $3, 'secret_deleted', $4)
	`, uuid.New(), entryID, userID, time.Now())

	return nil
}

// GetSecrets returns all secrets for an entry
func (s *SecretService) GetSecrets(entryID, userID uuid.UUID) ([]models.VaultSecret, error) {
	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil || !exists {
		return nil, fmt.Errorf("entry not found or access denied")
	}

	query := `
		SELECT id, entry_id, secret_type, name, encrypted_value, username, expires_at, last_rotated, strength_score, position, created_at, updated_at
		FROM vault_secrets
		WHERE entry_id = $1
		ORDER BY position
	`

	rows, err := s.db.Query(query, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query secrets: %w", err)
	}
	defer rows.Close()

	var secrets []models.VaultSecret
	for rows.Next() {
		var secret models.VaultSecret
		err := rows.Scan(
			&secret.ID,
			&secret.EntryID,
			&secret.SecretType,
			&secret.Name,
			&secret.EncryptedValue,
			&secret.Username,
			&secret.ExpiresAt,
			&secret.LastRotated,
			&secret.StrengthScore,
			&secret.Position,
			&secret.CreatedAt,
			&secret.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// ReorderSecrets updates the positions of secrets
func (s *SecretService) ReorderSecrets(entryID, userID uuid.UUID, positions map[uuid.UUID]int) error {
	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil || !exists {
		return fmt.Errorf("entry not found or access denied")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for secretID, position := range positions {
		// Verify secret belongs to entry
		var count int
		err := tx.QueryRow(`SELECT COUNT(*) FROM vault_secrets WHERE id = $1 AND entry_id = $2`, secretID, entryID).Scan(&count)
		if err != nil || count == 0 {
			return fmt.Errorf("secret %s not found in entry", secretID)
		}

		_, err = tx.Exec(`UPDATE vault_secrets SET position = $1, updated_at = $2 WHERE id = $3`,
			position, time.Now(), secretID)
		if err != nil {
			return fmt.Errorf("failed to update secret position: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// UpdateSecretRequest is the request to update a secret
type UpdateSecretRequest struct {
	SecretType     *string    `json:"secret_type,omitempty"`
	Name           *string    `json:"name,omitempty"`
	EncryptedValue *string    `json:"encrypted_value,omitempty"`
	Username       *string    `json:"username,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	StrengthScore  *int       `json:"strength_score,omitempty"`
	Position       *int       `json:"position,omitempty"`
}

// Helper functions

func (s *SecretService) entryBelongsToUser(entryID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM vault_entries WHERE id = $1 AND user_id = $2)`
	err := s.db.QueryRow(query, entryID, userID).Scan(&exists)
	return exists, err
}

func (s *SecretService) getNextPosition(entryID uuid.UUID) (int, error) {
	var maxPosition sql.NullInt64
	query := `SELECT MAX(position) FROM vault_secrets WHERE entry_id = $1`
	err := s.db.QueryRow(query, entryID).Scan(&maxPosition)
	if err != nil {
		return 0, err
	}
	if maxPosition.Valid {
		return int(maxPosition.Int64) + 1, nil
	}
	return 0, nil
}

func (s *SecretService) getSecretWithEntry(secretID uuid.UUID) (*models.VaultSecret, uuid.UUID, error) {
	var secret models.VaultSecret
	query := `
		SELECT id, entry_id, secret_type, name, encrypted_value, username, expires_at, last_rotated, strength_score, position, created_at, updated_at
		FROM vault_secrets
		WHERE id = $1
	`

	err := s.db.QueryRow(query, secretID).Scan(
		&secret.ID,
		&secret.EntryID,
		&secret.SecretType,
		&secret.Name,
		&secret.EncryptedValue,
		&secret.Username,
		&secret.ExpiresAt,
		&secret.LastRotated,
		&secret.StrengthScore,
		&secret.Position,
		&secret.CreatedAt,
		&secret.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, uuid.UUID{}, fmt.Errorf("secret not found")
		}
		return nil, uuid.UUID{}, fmt.Errorf("failed to get secret: %w", err)
	}

	return &secret, secret.EntryID, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
