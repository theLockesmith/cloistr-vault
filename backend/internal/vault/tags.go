package vault

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// TagService handles tag operations for the enhanced vault
type TagService struct {
	db *database.DB
}

// NewTagService creates a new tag service
func NewTagService(db *database.DB) *TagService {
	return &TagService{db: db}
}

// CreateTag creates a new tag
func (s *TagService) CreateTag(userID uuid.UUID, req *models.CreateTagRequest) (*models.VaultTag, error) {
	// Check for duplicate tag name
	exists, err := s.tagNameExists(userID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check tag name: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("tag with this name already exists")
	}

	// Set defaults
	color := req.Color
	if color == "" {
		color = "#6366f1"
	}
	category := req.Category
	if category == "" {
		category = "custom"
	}

	tag := &models.VaultTag{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      req.Name,
		Color:     color,
		Category:  category,
		IsSystem:  false,
		UsageCount: 0,
		CreatedAt: time.Now(),
	}

	query := `
		INSERT INTO vault_tags (id, user_id, name, color, category, is_system, usage_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = s.db.Exec(query,
		tag.ID,
		tag.UserID,
		tag.Name,
		tag.Color,
		tag.Category,
		tag.IsSystem,
		tag.UsageCount,
		tag.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	return tag, nil
}

// GetTags retrieves all tags for a user
func (s *TagService) GetTags(userID uuid.UUID, category *string) ([]models.VaultTag, error) {
	var query string
	var args []interface{}

	if category != nil && *category != "" {
		query = `
			SELECT id, user_id, name, color, category, is_system, usage_count, created_at
			FROM vault_tags
			WHERE user_id = $1 AND category = $2
			ORDER BY usage_count DESC, name
		`
		args = []interface{}{userID, *category}
	} else {
		query = `
			SELECT id, user_id, name, color, category, is_system, usage_count, created_at
			FROM vault_tags
			WHERE user_id = $1
			ORDER BY usage_count DESC, name
		`
		args = []interface{}{userID}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
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
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetTag retrieves a single tag
func (s *TagService) GetTag(tagID, userID uuid.UUID) (*models.VaultTag, error) {
	query := `
		SELECT id, user_id, name, color, category, is_system, usage_count, created_at
		FROM vault_tags
		WHERE id = $1 AND user_id = $2
	`

	var tag models.VaultTag
	err := s.db.QueryRow(query, tagID, userID).Scan(
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}

	return &tag, nil
}

// UpdateTag updates a tag
func (s *TagService) UpdateTag(tagID, userID uuid.UUID, req *UpdateTagRequest) (*models.VaultTag, error) {
	// Verify ownership and get current tag
	tag, err := s.GetTag(tagID, userID)
	if err != nil {
		return nil, err
	}

	// Don't allow updating system tags
	if tag.IsSystem {
		return nil, fmt.Errorf("cannot update system tags")
	}

	// Check name uniqueness if changing name
	if req.Name != nil && *req.Name != tag.Name {
		exists, err := s.tagNameExists(userID, *req.Name)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("tag with this name already exists")
		}
		tag.Name = *req.Name
	}

	if req.Color != nil {
		tag.Color = *req.Color
	}
	if req.Category != nil {
		tag.Category = *req.Category
	}

	query := `
		UPDATE vault_tags
		SET name = $1, color = $2, category = $3
		WHERE id = $4 AND user_id = $5
	`

	_, err = s.db.Exec(query, tag.Name, tag.Color, tag.Category, tagID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update tag: %w", err)
	}

	return tag, nil
}

// DeleteTag deletes a tag
func (s *TagService) DeleteTag(tagID, userID uuid.UUID) error {
	// Check if it's a system tag
	tag, err := s.GetTag(tagID, userID)
	if err != nil {
		return err
	}

	if tag.IsSystem {
		return fmt.Errorf("cannot delete system tags")
	}

	// Delete tag (cascade will remove entry associations)
	_, err = s.db.Exec(`DELETE FROM vault_tags WHERE id = $1 AND user_id = $2`, tagID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	return nil
}

// EnsureSystemTags creates default system tags for a user if they don't exist
func (s *TagService) EnsureSystemTags(userID uuid.UUID) error {
	systemTags := []struct {
		Name     string
		Color    string
		Category string
	}{
		{"weak-password", "#ef4444", "security"},
		{"reused-password", "#f59e0b", "security"},
		{"2fa-enabled", "#10b981", "security"},
		{"breach-detected", "#dc2626", "security"},
		{"password-expired", "#f59e0b", "security"},
	}

	for _, st := range systemTags {
		// Check if exists
		exists, err := s.tagNameExists(userID, st.Name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		// Create system tag
		query := `
			INSERT INTO vault_tags (id, user_id, name, color, category, is_system, usage_count, created_at)
			VALUES ($1, $2, $3, $4, $5, true, 0, $6)
		`
		_, err = s.db.Exec(query, uuid.New(), userID, st.Name, st.Color, st.Category, time.Now())
		if err != nil {
			return fmt.Errorf("failed to create system tag %s: %w", st.Name, err)
		}
	}

	return nil
}

// GetEntriesWithTag returns entries that have a specific tag
func (s *TagService) GetEntriesWithTag(tagID, userID uuid.UUID) ([]uuid.UUID, error) {
	// Verify tag belongs to user
	_, err := s.GetTag(tagID, userID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT entry_id FROM vault_entry_tags
		WHERE tag_id = $1
	`

	rows, err := s.db.Query(query, tagID)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entryIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		entryIDs = append(entryIDs, id)
	}

	return entryIDs, nil
}

// UpdateTagRequest is the request for updating a tag
type UpdateTagRequest struct {
	Name     *string `json:"name,omitempty"`
	Color    *string `json:"color,omitempty"`
	Category *string `json:"category,omitempty"`
}

// Helper functions

func (s *TagService) tagNameExists(userID uuid.UUID, name string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM vault_tags WHERE user_id = $1 AND name = $2)`
	err := s.db.QueryRow(query, userID, name).Scan(&exists)
	return exists, err
}
