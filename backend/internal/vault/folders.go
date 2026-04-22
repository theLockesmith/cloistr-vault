package vault

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// FolderService handles folder operations for the enhanced vault
type FolderService struct {
	db *database.DB
}

// NewFolderService creates a new folder service
func NewFolderService(db *database.DB) *FolderService {
	return &FolderService{db: db}
}

// CreateFolder creates a new folder for a user
func (s *FolderService) CreateFolder(userID uuid.UUID, req *models.CreateFolderRequest) (*models.VaultFolder, error) {
	// Validate parent folder if specified
	if req.ParentID != nil {
		exists, err := s.folderBelongsToUser(*req.ParentID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate parent folder: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("parent folder not found or access denied")
		}
	}

	// Get next position for this folder level
	position, err := s.getNextPosition(userID, req.ParentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next position: %w", err)
	}

	// Set defaults
	icon := req.Icon
	if icon == "" {
		icon = "📁"
	}
	color := req.Color
	if color == "" {
		color = "#6366f1"
	}

	folder := &models.VaultFolder{
		ID:        uuid.New(),
		UserID:    userID,
		ParentID:  req.ParentID,
		Name:      req.Name,
		Icon:      icon,
		Color:     color,
		Position:  position,
		IsShared:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `
		INSERT INTO vault_folders (id, user_id, parent_id, name, icon, color, position, is_shared, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = s.db.Exec(query,
		folder.ID,
		folder.UserID,
		folder.ParentID,
		folder.Name,
		folder.Icon,
		folder.Color,
		folder.Position,
		folder.IsShared,
		folder.CreatedAt,
		folder.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return folder, nil
}

// GetFolder retrieves a single folder by ID
func (s *FolderService) GetFolder(folderID, userID uuid.UUID) (*models.VaultFolder, error) {
	query := `
		SELECT id, user_id, parent_id, name, icon, color, position, is_shared, created_at, updated_at
		FROM vault_folders
		WHERE id = $1 AND user_id = $2
	`

	var folder models.VaultFolder
	err := s.db.QueryRow(query, folderID, userID).Scan(
		&folder.ID,
		&folder.UserID,
		&folder.ParentID,
		&folder.Name,
		&folder.Icon,
		&folder.Color,
		&folder.Position,
		&folder.IsShared,
		&folder.CreatedAt,
		&folder.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("folder not found")
		}
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}

	// Get entry count for this folder
	countQuery := `SELECT COUNT(*) FROM vault_entries WHERE folder_id = $1`
	err = s.db.QueryRow(countQuery, folderID).Scan(&folder.EntryCount)
	if err != nil {
		folder.EntryCount = 0 // Default to 0 if count fails
	}

	return &folder, nil
}

// GetFolders retrieves all folders for a user as a flat list
func (s *FolderService) GetFolders(userID uuid.UUID) ([]*models.VaultFolder, error) {
	query := `
		SELECT f.id, f.user_id, f.parent_id, f.name, f.icon, f.color, f.position, f.is_shared, f.created_at, f.updated_at,
		       COALESCE(e.entry_count, 0) as entry_count
		FROM vault_folders f
		LEFT JOIN (
			SELECT folder_id, COUNT(*) as entry_count
			FROM vault_entries
			GROUP BY folder_id
		) e ON f.id = e.folder_id
		WHERE f.user_id = $1
		ORDER BY f.parent_id NULLS FIRST, f.position, f.name
	`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query folders: %w", err)
	}
	defer rows.Close()

	var folders []*models.VaultFolder
	for rows.Next() {
		var folder models.VaultFolder
		err := rows.Scan(
			&folder.ID,
			&folder.UserID,
			&folder.ParentID,
			&folder.Name,
			&folder.Icon,
			&folder.Color,
			&folder.Position,
			&folder.IsShared,
			&folder.CreatedAt,
			&folder.UpdatedAt,
			&folder.EntryCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan folder: %w", err)
		}
		folders = append(folders, &folder)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating folders: %w", err)
	}

	return folders, nil
}

// GetFolderTree retrieves folders as a hierarchical tree structure
func (s *FolderService) GetFolderTree(userID uuid.UUID) ([]*models.VaultFolder, error) {
	// Get flat list first
	folders, err := s.GetFolders(userID)
	if err != nil {
		return nil, err
	}

	// Build tree structure
	folderMap := make(map[uuid.UUID]*models.VaultFolder)
	var rootFolders []*models.VaultFolder

	// First pass: create map
	for _, f := range folders {
		f.Children = []*models.VaultFolder{}
		folderMap[f.ID] = f
	}

	// Second pass: build tree
	for _, f := range folders {
		if f.ParentID == nil {
			rootFolders = append(rootFolders, f)
		} else {
			if parent, exists := folderMap[*f.ParentID]; exists {
				parent.Children = append(parent.Children, f)
			}
		}
	}

	return rootFolders, nil
}

// UpdateFolder updates an existing folder
func (s *FolderService) UpdateFolder(folderID, userID uuid.UUID, req *models.UpdateFolderRequest) (*models.VaultFolder, error) {
	// Verify folder belongs to user
	exists, err := s.folderBelongsToUser(folderID, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("folder not found or access denied")
	}

	// Validate parent folder if changing it
	if req.ParentID != nil {
		// Prevent circular reference
		if *req.ParentID == folderID {
			return nil, fmt.Errorf("folder cannot be its own parent")
		}

		// Check if new parent exists and belongs to user
		parentExists, err := s.folderBelongsToUser(*req.ParentID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate new parent folder: %w", err)
		}
		if !parentExists {
			return nil, fmt.Errorf("parent folder not found or access denied")
		}

		// Check for circular reference in hierarchy
		if s.wouldCreateCycle(folderID, *req.ParentID) {
			return nil, fmt.Errorf("cannot move folder: would create circular reference")
		}
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if req.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.ParentID != nil {
		updates = append(updates, fmt.Sprintf("parent_id = $%d", argNum))
		args = append(args, *req.ParentID)
		argNum++
	}
	if req.Icon != nil {
		updates = append(updates, fmt.Sprintf("icon = $%d", argNum))
		args = append(args, *req.Icon)
		argNum++
	}
	if req.Color != nil {
		updates = append(updates, fmt.Sprintf("color = $%d", argNum))
		args = append(args, *req.Color)
		argNum++
	}
	if req.Position != nil {
		updates = append(updates, fmt.Sprintf("position = $%d", argNum))
		args = append(args, *req.Position)
		argNum++
	}

	if len(updates) == 0 {
		// Nothing to update, just return current folder
		return s.GetFolder(folderID, userID)
	}

	// updated_at is handled by trigger, but let's set it explicitly too
	updates = append(updates, fmt.Sprintf("updated_at = $%d", argNum))
	args = append(args, time.Now())
	argNum++

	// Add WHERE clause args
	args = append(args, folderID, userID)

	query := fmt.Sprintf(`
		UPDATE vault_folders
		SET %s
		WHERE id = $%d AND user_id = $%d
	`, joinStrings(updates, ", "), argNum, argNum+1)

	_, err = s.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update folder: %w", err)
	}

	return s.GetFolder(folderID, userID)
}

// DeleteFolder deletes a folder and optionally its contents
func (s *FolderService) DeleteFolder(folderID, userID uuid.UUID, recursive bool) error {
	// Verify folder belongs to user
	exists, err := s.folderBelongsToUser(folderID, userID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("folder not found or access denied")
	}

	if recursive {
		// Delete with CASCADE will handle children due to FK constraint
		query := `DELETE FROM vault_folders WHERE id = $1 AND user_id = $2`
		_, err = s.db.Exec(query, folderID, userID)
	} else {
		// Check if folder has children or entries
		var childCount int
		err = s.db.QueryRow(`SELECT COUNT(*) FROM vault_folders WHERE parent_id = $1`, folderID).Scan(&childCount)
		if err != nil {
			return fmt.Errorf("failed to check for child folders: %w", err)
		}
		if childCount > 0 {
			return fmt.Errorf("cannot delete folder: has %d child folder(s), use recursive delete", childCount)
		}

		var entryCount int
		err = s.db.QueryRow(`SELECT COUNT(*) FROM vault_entries WHERE folder_id = $1`, folderID).Scan(&entryCount)
		if err != nil {
			return fmt.Errorf("failed to check for entries: %w", err)
		}
		if entryCount > 0 {
			return fmt.Errorf("cannot delete folder: has %d entry(ies), move or delete them first", entryCount)
		}

		query := `DELETE FROM vault_folders WHERE id = $1 AND user_id = $2`
		_, err = s.db.Exec(query, folderID, userID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	return nil
}

// MoveFolder moves a folder to a new parent (or to root if parentID is nil)
func (s *FolderService) MoveFolder(folderID, userID uuid.UUID, newParentID *uuid.UUID) error {
	req := &models.UpdateFolderRequest{
		ParentID: newParentID,
	}
	_, err := s.UpdateFolder(folderID, userID, req)
	return err
}

// ReorderFolders updates the position of multiple folders at once
func (s *FolderService) ReorderFolders(userID uuid.UUID, folderPositions map[uuid.UUID]int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for folderID, position := range folderPositions {
		// Verify ownership in the same transaction
		var ownerID uuid.UUID
		err := tx.QueryRow(`SELECT user_id FROM vault_folders WHERE id = $1`, folderID).Scan(&ownerID)
		if err != nil {
			return fmt.Errorf("folder %s not found: %w", folderID, err)
		}
		if ownerID != userID {
			return fmt.Errorf("access denied to folder %s", folderID)
		}

		_, err = tx.Exec(`UPDATE vault_folders SET position = $1, updated_at = $2 WHERE id = $3`,
			position, time.Now(), folderID)
		if err != nil {
			return fmt.Errorf("failed to update folder %s position: %w", folderID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit reorder: %w", err)
	}

	return nil
}

// Helper functions

func (s *FolderService) folderBelongsToUser(folderID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM vault_folders WHERE id = $1 AND user_id = $2)`
	err := s.db.QueryRow(query, folderID, userID).Scan(&exists)
	return exists, err
}

func (s *FolderService) getNextPosition(userID uuid.UUID, parentID *uuid.UUID) (int, error) {
	var maxPosition sql.NullInt64
	var query string
	var args []interface{}

	if parentID == nil {
		query = `SELECT MAX(position) FROM vault_folders WHERE user_id = $1 AND parent_id IS NULL`
		args = []interface{}{userID}
	} else {
		query = `SELECT MAX(position) FROM vault_folders WHERE user_id = $1 AND parent_id = $2`
		args = []interface{}{userID, *parentID}
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

func (s *FolderService) wouldCreateCycle(folderID, newParentID uuid.UUID) bool {
	// Check if newParentID is a descendant of folderID
	// This would create a cycle if we set folderID's parent to newParentID
	currentID := newParentID
	visited := make(map[uuid.UUID]bool)

	for {
		if currentID == folderID {
			return true // Found cycle
		}
		if visited[currentID] {
			return false // Already checked this branch, no cycle
		}
		visited[currentID] = true

		var parentID *uuid.UUID
		err := s.db.QueryRow(`SELECT parent_id FROM vault_folders WHERE id = $1`, currentID).Scan(&parentID)
		if err != nil || parentID == nil {
			return false // Reached root or error, no cycle
		}
		currentID = *parentID
	}
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
