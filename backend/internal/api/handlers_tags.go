package api

import (
	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
	"net/http"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TagHandlers contains handlers for tag operations
type TagHandlers struct {
	tagService *vault.TagService
}

// NewTagHandlers creates a new tag handlers instance
func NewTagHandlers(tagService *vault.TagService) *TagHandlers {
	return &TagHandlers{
		tagService: tagService,
	}
}

// ListTags returns all tags for the authenticated user
func (h *TagHandlers) ListTags(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	// Optional category filter
	category := c.Query("category")
	var categoryPtr *string
	if category != "" {
		categoryPtr = &category
	}

	tags, err := h.tagService.GetTags(userID, categoryPtr)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to retrieve tags").Abort(c)
		return
	}

	c.JSON(http.StatusOK, models.TagsResponse{Tags: tags})
}

// GetTag returns a single tag by ID
func (h *TagHandlers) GetTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid tag ID").Abort(c)
		return
	}

	tag, err := h.tagService.GetTag(tagID, userID)
	if err != nil {
		if err.Error() == "tag not found" {
			errors.NotFound(errors.CodeResourceNotFound, "Tag not found").Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to retrieve tag").Abort(c)
		return
	}

	c.JSON(http.StatusOK, tag)
}

// CreateTag creates a new tag
func (h *TagHandlers) CreateTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	var req models.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	tag, err := h.tagService.CreateTag(userID, &req)
	if err != nil {
		if err.Error() == "tag with this name already exists" {
			errors.Conflict(errors.CodeResourceConflict, err.Error()).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to create tag").Abort(c)
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// UpdateTag updates an existing tag
func (h *TagHandlers) UpdateTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid tag ID").Abort(c)
		return
	}

	var req vault.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	tag, err := h.tagService.UpdateTag(tagID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "tag not found":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "cannot update system tags":
			errors.Forbidden(errors.CodeAccessDenied, errMsg).Abort(c)
		case "tag with this name already exists":
			errors.Conflict(errors.CodeResourceConflict, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to update tag").Abort(c)
		}
		return
	}

	c.JSON(http.StatusOK, tag)
}

// DeleteTag deletes a tag
func (h *TagHandlers) DeleteTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid tag ID").Abort(c)
		return
	}

	err = h.tagService.DeleteTag(tagID, userID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "tag not found":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "cannot delete system tags":
			errors.Forbidden(errors.CodeAccessDenied, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to delete tag").Abort(c)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted successfully"})
}

// GetTagEntries returns entry IDs that have a specific tag
func (h *TagHandlers) GetTagEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid tag ID").Abort(c)
		return
	}

	entryIDs, err := h.tagService.GetEntriesWithTag(tagID, userID)
	if err != nil {
		if err.Error() == "tag not found" {
			errors.NotFound(errors.CodeResourceNotFound, "Tag not found").Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to get entries").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"entry_ids": entryIDs})
}

// InitializeSystemTags ensures system tags exist for the user
func (h *TagHandlers) InitializeSystemTags(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	err = h.tagService.EnsureSystemTags(userID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to initialize system tags").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "System tags initialized"})
}
