package vault

import (
	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// SearchService handles unified search across vault entities
type SearchService struct {
	db            *database.DB
	entryService  *EntryService
	folderService *FolderService
	tagService    *TagService
}

// NewSearchService creates a new search service
func NewSearchService(db *database.DB, entryService *EntryService, folderService *FolderService, tagService *TagService) *SearchService {
	return &SearchService{
		db:            db,
		entryService:  entryService,
		folderService: folderService,
		tagService:    tagService,
	}
}

// SearchResult represents unified search results
type SearchResult struct {
	Entries     []models.EnhancedVaultEntry `json:"entries"`
	Folders     []models.VaultFolder        `json:"folders"`
	Tags        []models.VaultTag           `json:"tags"`
	TotalCount  int                         `json:"total_count"`
	EntryCount  int                         `json:"entry_count"`
	FolderCount int                         `json:"folder_count"`
	TagCount    int                         `json:"tag_count"`
}

// UnifiedSearchRequest contains parameters for unified search
type UnifiedSearchRequest struct {
	Query         string `form:"q" binding:"required,min=1"`
	IncludeAll    bool   `form:"include_all"`    // Include folders and tags
	FolderID      *uuid.UUID                     // Filter entries by folder
	TagIDs        []uuid.UUID                    // Filter entries by tags
	EntryType     *string                        // Filter entries by type
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
}

// Search performs a unified search across entries, folders, and tags
func (s *SearchService) Search(userID uuid.UUID, req *UnifiedSearchRequest) (*SearchResult, error) {
	result := &SearchResult{
		Entries: []models.EnhancedVaultEntry{},
		Folders: []models.VaultFolder{},
		Tags:    []models.VaultTag{},
	}

	// Search entries
	entryReq := &models.SearchRequest{
		Query:     req.Query,
		FolderID:  req.FolderID,
		TagIDs:    req.TagIDs,
		EntryType: req.EntryType,
		Limit:     req.Limit,
		Offset:    req.Offset,
	}
	if entryReq.Limit <= 0 {
		entryReq.Limit = 20
	}

	entriesResp, err := s.entryService.ListEntries(userID, entryReq)
	if err != nil {
		return nil, err
	}
	result.Entries = entriesResp.Entries
	result.EntryCount = entriesResp.TotalCount

	// Search folders and tags if include_all is true or no filters applied
	if req.IncludeAll || (req.FolderID == nil && len(req.TagIDs) == 0 && req.EntryType == nil) {
		// Search folders by name
		folders, err := s.searchFolders(userID, req.Query)
		if err != nil {
			return nil, err
		}
		result.Folders = folders
		result.FolderCount = len(folders)

		// Search tags by name
		tags, err := s.searchTags(userID, req.Query)
		if err != nil {
			return nil, err
		}
		result.Tags = tags
		result.TagCount = len(tags)
	}

	result.TotalCount = result.EntryCount + result.FolderCount + result.TagCount

	return result, nil
}

// searchFolders searches folders by name
func (s *SearchService) searchFolders(userID uuid.UUID, query string) ([]models.VaultFolder, error) {
	queryStr := `
		SELECT id, user_id, name, parent_id, icon, color, position, created_at, updated_at
		FROM vault_folders
		WHERE user_id = $1 AND name ILIKE $2
		ORDER BY name
		LIMIT 10
	`

	rows, err := s.db.Query(queryStr, userID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []models.VaultFolder
	for rows.Next() {
		var folder models.VaultFolder
		err := rows.Scan(
			&folder.ID,
			&folder.UserID,
			&folder.Name,
			&folder.ParentID,
			&folder.Icon,
			&folder.Color,
			&folder.Position,
			&folder.CreatedAt,
			&folder.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}

	return folders, nil
}

// searchTags searches tags by name
func (s *SearchService) searchTags(userID uuid.UUID, query string) ([]models.VaultTag, error) {
	queryStr := `
		SELECT id, user_id, name, color, category, is_system, usage_count, created_at
		FROM vault_tags
		WHERE user_id = $1 AND name ILIKE $2
		ORDER BY usage_count DESC, name
		LIMIT 10
	`

	rows, err := s.db.Query(queryStr, userID, "%"+query+"%")
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

// GetRecentEntries returns recently used entries
func (s *SearchService) GetRecentEntries(userID uuid.UUID, limit int) ([]models.EnhancedVaultEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, user_id, folder_id, name, entry_type, url, notes, is_favorite, position,
		       strength_score, has_weak_password, has_reused_password, has_breach,
		       last_breach_check, last_used, usage_count, created_at, updated_at
		FROM vault_entries
		WHERE user_id = $1 AND last_used IS NOT NULL
		ORDER BY last_used DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetFrequentEntries returns most frequently used entries
func (s *SearchService) GetFrequentEntries(userID uuid.UUID, limit int) ([]models.EnhancedVaultEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, user_id, folder_id, name, entry_type, url, notes, is_favorite, position,
		       strength_score, has_weak_password, has_reused_password, has_breach,
		       last_breach_check, last_used, usage_count, created_at, updated_at
		FROM vault_entries
		WHERE user_id = $1 AND usage_count > 0
		ORDER BY usage_count DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
