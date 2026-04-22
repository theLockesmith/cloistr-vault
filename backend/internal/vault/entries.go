package vault

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// EntryService handles entry operations for the enhanced vault
type EntryService struct {
	db *database.DB
}

// NewEntryService creates a new entry service
func NewEntryService(db *database.DB) *EntryService {
	return &EntryService{db: db}
}

// CreateEntry creates a new vault entry with optional secrets
func (s *EntryService) CreateEntry(userID uuid.UUID, req *models.CreateEntryRequest) (*models.EnhancedVaultEntry, error) {
	// Validate folder if specified
	if req.FolderID != nil {
		exists, err := s.folderBelongsToUser(*req.FolderID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate folder: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("folder not found or access denied")
		}
	}

	// Get next position
	position, err := s.getNextPosition(userID, req.FolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next position: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	entry := &models.EnhancedVaultEntry{
		ID:         uuid.New(),
		UserID:     userID,
		FolderID:   req.FolderID,
		Name:       req.Name,
		EntryType:  req.EntryType,
		URL:        req.URL,
		Notes:      req.Notes,
		IsFavorite: req.IsFavorite,
		Position:   position,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Insert entry
	query := `
		INSERT INTO vault_entries (id, user_id, folder_id, name, entry_type, url, notes, is_favorite, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = tx.Exec(query,
		entry.ID,
		entry.UserID,
		entry.FolderID,
		entry.Name,
		entry.EntryType,
		entry.URL,
		entry.Notes,
		entry.IsFavorite,
		entry.Position,
		entry.CreatedAt,
		entry.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create entry: %w", err)
	}

	// Insert secrets
	if len(req.Secrets) > 0 {
		for i, secretReq := range req.Secrets {
			secret := models.VaultSecret{
				ID:             uuid.New(),
				EntryID:        entry.ID,
				SecretType:     secretReq.SecretType,
				Name:           secretReq.Name,
				EncryptedValue: secretReq.EncryptedValue,
				Username:       secretReq.Username,
				ExpiresAt:      secretReq.ExpiresAt,
				StrengthScore:  secretReq.StrengthScore,
				Position:       i,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			secretQuery := `
				INSERT INTO vault_secrets (id, entry_id, secret_type, name, encrypted_value, username, expires_at, strength_score, position, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`
			_, err = tx.Exec(secretQuery,
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
			entry.Secrets = append(entry.Secrets, secret)
		}
	}

	// Associate tags
	if len(req.TagIDs) > 0 {
		for _, tagID := range req.TagIDs {
			_, err = tx.Exec(`INSERT INTO vault_entry_tags (entry_id, tag_id) VALUES ($1, $2)`,
				entry.ID, tagID)
			if err != nil {
				return nil, fmt.Errorf("failed to associate tag: %w", err)
			}
		}
	}

	// Record history
	_, err = tx.Exec(`
		INSERT INTO vault_entry_history (id, entry_id, user_id, action, created_at)
		VALUES ($1, $2, $3, 'created', $4)
	`, uuid.New(), entry.ID, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	// Load tags for response
	entry.Tags, _ = s.getEntryTags(entry.ID)

	return entry, nil
}

// GetEntry retrieves a single entry by ID with all related data
func (s *EntryService) GetEntry(entryID, userID uuid.UUID) (*models.EnhancedVaultEntry, error) {
	query := `
		SELECT id, user_id, folder_id, name, entry_type, url, notes, is_favorite, position,
		       strength_score, has_weak_password, has_reused_password, has_breach,
		       last_breach_check, last_used, usage_count, created_at, updated_at
		FROM vault_entries
		WHERE id = $1 AND user_id = $2
	`

	var entry models.EnhancedVaultEntry
	err := s.db.QueryRow(query, entryID, userID).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.FolderID,
		&entry.Name,
		&entry.EntryType,
		&entry.URL,
		&entry.Notes,
		&entry.IsFavorite,
		&entry.Position,
		&entry.StrengthScore,
		&entry.HasWeakPassword,
		&entry.HasReusedPassword,
		&entry.HasBreach,
		&entry.LastBreachCheck,
		&entry.LastUsed,
		&entry.UsageCount,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entry not found")
		}
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	// Load secrets
	entry.Secrets, _ = s.getEntrySecrets(entry.ID)

	// Load tags
	entry.Tags, _ = s.getEntryTags(entry.ID)

	// Load attachments (without data)
	entry.Attachments, _ = s.getEntryAttachments(entry.ID)

	return &entry, nil
}

// ListEntries retrieves entries for a user with optional filtering
func (s *EntryService) ListEntries(userID uuid.UUID, req *models.SearchRequest) (*models.EntriesResponse, error) {
	// Build query with filters
	whereConditions := []string{"user_id = $1"}
	args := []interface{}{userID}
	argNum := 2

	if req.FolderID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("folder_id = $%d", argNum))
		args = append(args, *req.FolderID)
		argNum++
	}

	if req.EntryType != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("entry_type = $%d", argNum))
		args = append(args, *req.EntryType)
		argNum++
	}

	if req.Favorite != nil && *req.Favorite {
		whereConditions = append(whereConditions, "is_favorite = true")
	}

	if req.Query != "" {
		whereConditions = append(whereConditions, fmt.Sprintf(
			"(name ILIKE $%d OR url ILIKE $%d OR notes ILIKE $%d)",
			argNum, argNum, argNum))
		args = append(args, "%"+req.Query+"%")
		argNum++
	}

	// Handle tag filtering
	if len(req.TagIDs) > 0 {
		tagPlaceholders := make([]string, len(req.TagIDs))
		for i, tagID := range req.TagIDs {
			tagPlaceholders[i] = fmt.Sprintf("$%d", argNum)
			args = append(args, tagID)
			argNum++
		}
		whereConditions = append(whereConditions, fmt.Sprintf(
			"id IN (SELECT entry_id FROM vault_entry_tags WHERE tag_id IN (%s))",
			strings.Join(tagPlaceholders, ", ")))
	}

	whereClause := strings.Join(whereConditions, " AND ")

	// Count total
	var totalCount int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM vault_entries WHERE %s", whereClause)
	err := s.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count entries: %w", err)
	}

	// Apply pagination
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := req.Offset

	query := fmt.Sprintf(`
		SELECT id, user_id, folder_id, name, entry_type, url, notes, is_favorite, position,
		       strength_score, has_weak_password, has_reused_password, has_breach,
		       last_breach_check, last_used, usage_count, created_at, updated_at
		FROM vault_entries
		WHERE %s
		ORDER BY is_favorite DESC, position, name
		LIMIT $%d OFFSET $%d
	`, whereClause, argNum, argNum+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entries []models.EnhancedVaultEntry
	for rows.Next() {
		var entry models.EnhancedVaultEntry
		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.FolderID,
			&entry.Name,
			&entry.EntryType,
			&entry.URL,
			&entry.Notes,
			&entry.IsFavorite,
			&entry.Position,
			&entry.StrengthScore,
			&entry.HasWeakPassword,
			&entry.HasReusedPassword,
			&entry.HasBreach,
			&entry.LastBreachCheck,
			&entry.LastUsed,
			&entry.UsageCount,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Load secrets and tags for each entry (could be optimized with batch loading)
		entry.Secrets, _ = s.getEntrySecrets(entry.ID)
		entry.Tags, _ = s.getEntryTags(entry.ID)

		entries = append(entries, entry)
	}

	return &models.EntriesResponse{
		Entries:    entries,
		TotalCount: totalCount,
		HasMore:    offset+limit < totalCount,
	}, nil
}

// UpdateEntry updates an existing entry
func (s *EntryService) UpdateEntry(entryID, userID uuid.UUID, req *models.UpdateEntryRequest) (*models.EnhancedVaultEntry, error) {
	// Verify ownership
	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("entry not found or access denied")
	}

	// Validate folder if changing
	if req.FolderID != nil {
		folderExists, err := s.folderBelongsToUser(*req.FolderID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate folder: %w", err)
		}
		if !folderExists {
			return nil, fmt.Errorf("folder not found or access denied")
		}
	}

	// Build dynamic update
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if req.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.FolderID != nil {
		updates = append(updates, fmt.Sprintf("folder_id = $%d", argNum))
		args = append(args, *req.FolderID)
		argNum++
	}
	if req.EntryType != nil {
		updates = append(updates, fmt.Sprintf("entry_type = $%d", argNum))
		args = append(args, *req.EntryType)
		argNum++
	}
	if req.URL != nil {
		updates = append(updates, fmt.Sprintf("url = $%d", argNum))
		args = append(args, *req.URL)
		argNum++
	}
	if req.Notes != nil {
		updates = append(updates, fmt.Sprintf("notes = $%d", argNum))
		args = append(args, *req.Notes)
		argNum++
	}
	if req.IsFavorite != nil {
		updates = append(updates, fmt.Sprintf("is_favorite = $%d", argNum))
		args = append(args, *req.IsFavorite)
		argNum++
	}
	if req.Position != nil {
		updates = append(updates, fmt.Sprintf("position = $%d", argNum))
		args = append(args, *req.Position)
		argNum++
	}
	if req.StrengthScore != nil {
		updates = append(updates, fmt.Sprintf("strength_score = $%d", argNum))
		args = append(args, *req.StrengthScore)
		argNum++
	}

	if len(updates) == 0 && len(req.TagIDs) == 0 {
		return s.GetEntry(entryID, userID)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if len(updates) > 0 {
		updates = append(updates, fmt.Sprintf("updated_at = $%d", argNum))
		args = append(args, time.Now())
		argNum++

		args = append(args, entryID, userID)

		query := fmt.Sprintf(`
			UPDATE vault_entries
			SET %s
			WHERE id = $%d AND user_id = $%d
		`, strings.Join(updates, ", "), argNum, argNum+1)

		_, err = tx.Exec(query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to update entry: %w", err)
		}
	}

	// Update tags if provided
	if req.TagIDs != nil {
		// Remove existing tags
		_, err = tx.Exec(`DELETE FROM vault_entry_tags WHERE entry_id = $1`, entryID)
		if err != nil {
			return nil, fmt.Errorf("failed to clear tags: %w", err)
		}

		// Add new tags
		for _, tagID := range req.TagIDs {
			_, err = tx.Exec(`INSERT INTO vault_entry_tags (entry_id, tag_id) VALUES ($1, $2)`,
				entryID, tagID)
			if err != nil {
				return nil, fmt.Errorf("failed to associate tag: %w", err)
			}
		}
	}

	// Record history
	_, err = tx.Exec(`
		INSERT INTO vault_entry_history (id, entry_id, user_id, action, created_at)
		VALUES ($1, $2, $3, 'updated', $4)
	`, uuid.New(), entryID, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return s.GetEntry(entryID, userID)
}

// DeleteEntry deletes an entry and all associated data
func (s *EntryService) DeleteEntry(entryID, userID uuid.UUID) error {
	exists, err := s.entryBelongsToUser(entryID, userID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("entry not found or access denied")
	}

	// Cascade delete will handle secrets, tags, attachments, history
	_, err = s.db.Exec(`DELETE FROM vault_entries WHERE id = $1 AND user_id = $2`, entryID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	return nil
}

// RecordUsage updates usage statistics for an entry
func (s *EntryService) RecordUsage(entryID, userID uuid.UUID) error {
	_, err := s.db.Exec(`
		UPDATE vault_entries
		SET last_used = $1, usage_count = usage_count + 1
		WHERE id = $2 AND user_id = $3
	`, time.Now(), entryID, userID)

	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Record history
	_, _ = s.db.Exec(`
		INSERT INTO vault_entry_history (id, entry_id, user_id, action, created_at)
		VALUES ($1, $2, $3, 'accessed', $4)
	`, uuid.New(), entryID, userID, time.Now())

	return nil
}

// Helper functions

func (s *EntryService) entryBelongsToUser(entryID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM vault_entries WHERE id = $1 AND user_id = $2)`
	err := s.db.QueryRow(query, entryID, userID).Scan(&exists)
	return exists, err
}

func (s *EntryService) folderBelongsToUser(folderID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM vault_folders WHERE id = $1 AND user_id = $2)`
	err := s.db.QueryRow(query, folderID, userID).Scan(&exists)
	return exists, err
}

func (s *EntryService) getNextPosition(userID uuid.UUID, folderID *uuid.UUID) (int, error) {
	var maxPosition sql.NullInt64
	var query string
	var args []interface{}

	if folderID == nil {
		query = `SELECT MAX(position) FROM vault_entries WHERE user_id = $1 AND folder_id IS NULL`
		args = []interface{}{userID}
	} else {
		query = `SELECT MAX(position) FROM vault_entries WHERE user_id = $1 AND folder_id = $2`
		args = []interface{}{userID, *folderID}
	}

	err := s.db.QueryRow(query, args...).Scan(&maxPosition)
	if err != nil {
		return 0, err
	}

	if maxPosition.Valid {
		return int(maxPosition.Int64) + 1, nil
	}
	return 0, nil
}

func (s *EntryService) getEntrySecrets(entryID uuid.UUID) ([]models.VaultSecret, error) {
	query := `
		SELECT id, entry_id, secret_type, name, encrypted_value, username, expires_at, last_rotated, strength_score, position, created_at, updated_at
		FROM vault_secrets
		WHERE entry_id = $1
		ORDER BY position
	`

	rows, err := s.db.Query(query, entryID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

func (s *EntryService) getEntryTags(entryID uuid.UUID) ([]models.VaultTag, error) {
	query := `
		SELECT t.id, t.user_id, t.name, t.color, t.category, t.is_system, t.usage_count, t.created_at
		FROM vault_tags t
		JOIN vault_entry_tags et ON t.id = et.tag_id
		WHERE et.entry_id = $1
	`

	rows, err := s.db.Query(query, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []models.VaultTag
	for rows.Next() {
		var tag models.VaultTag
		err := rows.Scan(
			&tag.ID,
			&tag.UserID,
			&tag.Name,
			&tag.Color,
			&tag.Category,
			&tag.IsSystem,
			&tag.UsageCount,
			&tag.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (s *EntryService) getEntryAttachments(entryID uuid.UUID) ([]models.VaultAttachment, error) {
	query := `
		SELECT id, entry_id, name, file_type, mime_type, file_size, created_at
		FROM vault_attachments
		WHERE entry_id = $1
	`

	rows, err := s.db.Query(query, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []models.VaultAttachment
	for rows.Next() {
		var att models.VaultAttachment
		err := rows.Scan(
			&att.ID,
			&att.EntryID,
			&att.Name,
			&att.FileType,
			&att.MimeType,
			&att.FileSize,
			&att.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, att)
	}

	return attachments, nil
}
