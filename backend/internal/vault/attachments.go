package vault

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// AttachmentService handles file attachment operations
type AttachmentService struct {
	db *database.DB
}

// NewAttachmentService creates a new attachment service
func NewAttachmentService(db *database.DB) *AttachmentService {
	return &AttachmentService{db: db}
}

// CreateAttachment adds a new attachment to an entry
func (s *AttachmentService) CreateAttachment(userID uuid.UUID, req *models.CreateAttachmentRequest) (*models.VaultAttachment, error) {
	// Verify entry belongs to user
	var entryUserID uuid.UUID
	err := s.db.QueryRow(`SELECT user_id FROM vault_entries WHERE id = $1`, req.EntryID).Scan(&entryUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entry not found")
		}
		return nil, fmt.Errorf("failed to verify entry: %w", err)
	}
	if entryUserID != userID {
		return nil, fmt.Errorf("entry not found or access denied")
	}

	// Check attachment count limit (max 10 per entry)
	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM vault_attachments WHERE entry_id = $1`, req.EntryID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count attachments: %w", err)
	}
	if count >= 10 {
		return nil, fmt.Errorf("maximum attachments per entry reached (10)")
	}

	// Check file size limit (10MB)
	if req.FileSize > 10*1024*1024 {
		return nil, fmt.Errorf("file size exceeds 10MB limit")
	}

	attachment := &models.VaultAttachment{
		ID:            uuid.New(),
		EntryID:       req.EntryID,
		Name:          req.Name,
		FileType:      req.FileType,
		MimeType:      req.MimeType,
		FileSize:      req.FileSize,
		EncryptedData: req.EncryptedData,
		CreatedAt:     time.Now(),
	}

	query := `
		INSERT INTO vault_attachments (id, entry_id, name, file_type, mime_type, file_size, encrypted_data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = s.db.Exec(query,
		attachment.ID,
		attachment.EntryID,
		attachment.Name,
		attachment.FileType,
		attachment.MimeType,
		attachment.FileSize,
		attachment.EncryptedData,
		attachment.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create attachment: %w", err)
	}

	return attachment, nil
}

// GetAttachment retrieves an attachment by ID
func (s *AttachmentService) GetAttachment(attachmentID, userID uuid.UUID) (*models.VaultAttachment, error) {
	query := `
		SELECT va.id, va.entry_id, va.name, va.file_type, va.mime_type, va.file_size, va.encrypted_data, va.created_at
		FROM vault_attachments va
		JOIN vault_entries ve ON va.entry_id = ve.id
		WHERE va.id = $1 AND ve.user_id = $2
	`

	var attachment models.VaultAttachment
	err := s.db.QueryRow(query, attachmentID, userID).Scan(
		&attachment.ID,
		&attachment.EntryID,
		&attachment.Name,
		&attachment.FileType,
		&attachment.MimeType,
		&attachment.FileSize,
		&attachment.EncryptedData,
		&attachment.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment not found")
		}
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}

	return &attachment, nil
}

// GetAttachmentMetadata retrieves attachment metadata without the encrypted data
func (s *AttachmentService) GetAttachmentMetadata(attachmentID, userID uuid.UUID) (*models.AttachmentMetadata, error) {
	query := `
		SELECT va.id, va.entry_id, va.name, va.file_type, va.mime_type, va.file_size, va.created_at
		FROM vault_attachments va
		JOIN vault_entries ve ON va.entry_id = ve.id
		WHERE va.id = $1 AND ve.user_id = $2
	`

	var meta models.AttachmentMetadata
	err := s.db.QueryRow(query, attachmentID, userID).Scan(
		&meta.ID,
		&meta.EntryID,
		&meta.Name,
		&meta.FileType,
		&meta.MimeType,
		&meta.FileSize,
		&meta.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment not found")
		}
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}

	return &meta, nil
}

// ListAttachments returns all attachments for an entry
func (s *AttachmentService) ListAttachments(entryID, userID uuid.UUID) ([]models.AttachmentMetadata, error) {
	// Verify entry belongs to user
	var entryUserID uuid.UUID
	err := s.db.QueryRow(`SELECT user_id FROM vault_entries WHERE id = $1`, entryID).Scan(&entryUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entry not found")
		}
		return nil, fmt.Errorf("failed to verify entry: %w", err)
	}
	if entryUserID != userID {
		return nil, fmt.Errorf("entry not found or access denied")
	}

	query := `
		SELECT id, entry_id, name, file_type, mime_type, file_size, created_at
		FROM vault_attachments
		WHERE entry_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments: %w", err)
	}
	defer rows.Close()

	var attachments []models.AttachmentMetadata
	for rows.Next() {
		var meta models.AttachmentMetadata
		err := rows.Scan(
			&meta.ID,
			&meta.EntryID,
			&meta.Name,
			&meta.FileType,
			&meta.MimeType,
			&meta.FileSize,
			&meta.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}
		attachments = append(attachments, meta)
	}

	return attachments, nil
}

// DeleteAttachment removes an attachment
func (s *AttachmentService) DeleteAttachment(attachmentID, userID uuid.UUID) error {
	// Verify attachment belongs to user's entry
	var entryUserID uuid.UUID
	err := s.db.QueryRow(`
		SELECT ve.user_id FROM vault_attachments va
		JOIN vault_entries ve ON va.entry_id = ve.id
		WHERE va.id = $1
	`, attachmentID).Scan(&entryUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("attachment not found")
		}
		return fmt.Errorf("failed to verify attachment: %w", err)
	}
	if entryUserID != userID {
		return fmt.Errorf("attachment not found or access denied")
	}

	_, err = s.db.Exec(`DELETE FROM vault_attachments WHERE id = $1`, attachmentID)
	if err != nil {
		return fmt.Errorf("failed to delete attachment: %w", err)
	}

	return nil
}

// UpdateAttachmentName updates the name of an attachment
func (s *AttachmentService) UpdateAttachmentName(attachmentID, userID uuid.UUID, name string) (*models.AttachmentMetadata, error) {
	// Verify attachment belongs to user's entry
	var entryUserID uuid.UUID
	err := s.db.QueryRow(`
		SELECT ve.user_id FROM vault_attachments va
		JOIN vault_entries ve ON va.entry_id = ve.id
		WHERE va.id = $1
	`, attachmentID).Scan(&entryUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment not found")
		}
		return nil, fmt.Errorf("failed to verify attachment: %w", err)
	}
	if entryUserID != userID {
		return nil, fmt.Errorf("attachment not found or access denied")
	}

	_, err = s.db.Exec(`UPDATE vault_attachments SET name = $1 WHERE id = $2`, name, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update attachment: %w", err)
	}

	return s.GetAttachmentMetadata(attachmentID, userID)
}

// GetUserStorageUsage returns total storage used by a user's attachments
func (s *AttachmentService) GetUserStorageUsage(userID uuid.UUID) (int64, int, error) {
	var totalSize int64
	var count int

	query := `
		SELECT COALESCE(SUM(va.file_size), 0), COUNT(*)
		FROM vault_attachments va
		JOIN vault_entries ve ON va.entry_id = ve.id
		WHERE ve.user_id = $1
	`

	err := s.db.QueryRow(query, userID).Scan(&totalSize, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get storage usage: %w", err)
	}

	return totalSize, count, nil
}
