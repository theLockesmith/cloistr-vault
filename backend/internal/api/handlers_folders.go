package api

import (
	"net/http"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FolderHandlers contains handlers for folder operations
type FolderHandlers struct {
	folderService *vault.FolderService
}

// NewFolderHandlers creates a new folder handlers instance
func NewFolderHandlers(folderService *vault.FolderService) *FolderHandlers {
	return &FolderHandlers{
		folderService: folderService,
	}
}

// ListFolders returns all folders for the authenticated user
func (h *FolderHandlers) ListFolders(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// Check if tree format is requested
	tree := c.Query("tree") == "true"

	var folders []*models.VaultFolder
	if tree {
		folders, err = h.folderService.GetFolderTree(userID)
	} else {
		folders, err = h.folderService.GetFolders(userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve folders"})
		return
	}

	c.JSON(http.StatusOK, models.FoldersResponse{Folders: folders})
}

// GetFolder returns a single folder by ID
func (h *FolderHandlers) GetFolder(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder ID"})
		return
	}

	folder, err := h.folderService.GetFolder(folderID, userID)
	if err != nil {
		if err.Error() == "folder not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Folder not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve folder"})
		return
	}

	c.JSON(http.StatusOK, folder)
}

// CreateFolder creates a new folder
func (h *FolderHandlers) CreateFolder(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	folder, err := h.folderService.CreateFolder(userID, &req)
	if err != nil {
		if err.Error() == "parent folder not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create folder"})
		return
	}

	c.JSON(http.StatusCreated, folder)
}

// UpdateFolder updates an existing folder
func (h *FolderHandlers) UpdateFolder(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder ID"})
		return
	}

	var req models.UpdateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	folder, err := h.folderService.UpdateFolder(folderID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch {
		case errMsg == "folder not found or access denied":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case errMsg == "folder cannot be its own parent":
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		case errMsg == "cannot move folder: would create circular reference":
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		case errMsg == "parent folder not found or access denied":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update folder"})
		}
		return
	}

	c.JSON(http.StatusOK, folder)
}

// DeleteFolder deletes a folder
func (h *FolderHandlers) DeleteFolder(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder ID"})
		return
	}

	// Check for recursive flag in query params
	recursive := c.Query("recursive") == "true"

	err = h.folderService.DeleteFolder(folderID, userID, recursive)
	if err != nil {
		errMsg := err.Error()
		switch {
		case errMsg == "folder not found or access denied":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case len(errMsg) > 20 && errMsg[:20] == "cannot delete folder":
			c.JSON(http.StatusConflict, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete folder"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Folder deleted successfully"})
}

// ReorderFolders updates the positions of multiple folders
func (h *FolderHandlers) ReorderFolders(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req struct {
		Positions map[string]int `json:"positions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Convert string keys to UUIDs
	positions := make(map[uuid.UUID]int)
	for idStr, pos := range req.Positions {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder ID: " + idStr})
			return
		}
		positions[id] = pos
	}

	if err := h.folderService.ReorderFolders(userID, positions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder folders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Folders reordered successfully"})
}

// Helper function to extract user ID from context
func getUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		return uuid.UUID{}, gin.Error{Err: nil, Meta: "user ID not found"}
	}

	return uuid.Parse(userIDStr.(string))
}
