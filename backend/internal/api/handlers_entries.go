package api

import (
	"net/http"

	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
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
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	var req models.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid query parameters").Abort(c)
		return
	}

	response, err := h.entryService.ListEntries(userID, &req)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to retrieve entries").Abort(c)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetEntry returns a single entry by ID
func (h *EntryHandlers) GetEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid entry ID").Abort(c)
		return
	}

	entry, err := h.entryService.GetEntry(entryID, userID)
	if err != nil {
		if err.Error() == "entry not found" {
			errors.NotFound(errors.CodeResourceNotFound, "Entry not found").Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to retrieve entry").Abort(c)
		return
	}

	c.JSON(http.StatusOK, entry)
}

// CreateEntry creates a new entry
func (h *EntryHandlers) CreateEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	var req models.CreateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	entry, err := h.entryService.CreateEntry(userID, &req)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "folder not found or access denied" {
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to create entry").Abort(c)
		return
	}

	c.JSON(http.StatusCreated, entry)
}

// UpdateEntry updates an existing entry
func (h *EntryHandlers) UpdateEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid entry ID").Abort(c)
		return
	}

	var req models.UpdateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	entry, err := h.entryService.UpdateEntry(entryID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "entry not found or access denied":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "folder not found or access denied":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to update entry").Abort(c)
		}
		return
	}

	c.JSON(http.StatusOK, entry)
}

// DeleteEntry deletes an entry
func (h *EntryHandlers) DeleteEntry(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid entry ID").Abort(c)
		return
	}

	err = h.entryService.DeleteEntry(entryID, userID)
	if err != nil {
		if err.Error() == "entry not found or access denied" {
			errors.NotFound(errors.CodeResourceNotFound, err.Error()).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to delete entry").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Entry deleted successfully"})
}

// RecordUsage records that an entry was used (for usage tracking)
func (h *EntryHandlers) RecordUsage(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid entry ID").Abort(c)
		return
	}

	err = h.entryService.RecordUsage(entryID, userID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to record usage").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Usage recorded"})
}
