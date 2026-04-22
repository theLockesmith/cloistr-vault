package api

import (
	"net/http"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EntryHandlers contains handlers for entry operations
type EntryHandlers struct {
	entryService *vault.EntryService
}

// NewEntryHandlers creates a new entry handlers instance
func NewEntryHandlers(entryService *vault.EntryService) *EntryHandlers {
	return &EntryHandlers{
		entryService: entryService,
	}
}

// ListEntries returns entries for the authenticated user
func (h *EntryHandlers) ListEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	response, err := h.entryService.ListEntries(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entries"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetEntry returns a single entry by ID
func (h *EntryHandlers) GetEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry ID"})
		return
	}

	entry, err := h.entryService.GetEntry(entryID, userID)
	if err != nil {
		if err.Error() == "entry not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entry"})
		return
	}

	c.JSON(http.StatusOK, entry)
}

// CreateEntry creates a new entry
func (h *EntryHandlers) CreateEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.CreateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	entry, err := h.entryService.CreateEntry(userID, &req)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "folder not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entry"})
		return
	}

	c.JSON(http.StatusCreated, entry)
}

// UpdateEntry updates an existing entry
func (h *EntryHandlers) UpdateEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry ID"})
		return
	}

	var req models.UpdateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	entry, err := h.entryService.UpdateEntry(entryID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "entry not found or access denied":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "folder not found or access denied":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update entry"})
		}
		return
	}

	c.JSON(http.StatusOK, entry)
}

// DeleteEntry deletes an entry
func (h *EntryHandlers) DeleteEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry ID"})
		return
	}

	err = h.entryService.DeleteEntry(entryID, userID)
	if err != nil {
		if err.Error() == "entry not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete entry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Entry deleted successfully"})
}

// RecordUsage records that an entry was used (for usage tracking)
func (h *EntryHandlers) RecordUsage(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry ID"})
		return
	}

	err = h.entryService.RecordUsage(entryID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record usage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Usage recorded"})
}
