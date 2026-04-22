package api

import (
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve tags"})
		return
	}

	c.JSON(http.StatusOK, models.TagsResponse{Tags: tags})
}

// GetTag returns a single tag by ID
func (h *TagHandlers) GetTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	tag, err := h.tagService.GetTag(tagID, userID)
	if err != nil {
		if err.Error() == "tag not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve tag"})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// CreateTag creates a new tag
func (h *TagHandlers) CreateTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	tag, err := h.tagService.CreateTag(userID, &req)
	if err != nil {
		if err.Error() == "tag with this name already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tag"})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// UpdateTag updates an existing tag
func (h *TagHandlers) UpdateTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	var req vault.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	tag, err := h.tagService.UpdateTag(tagID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "tag not found":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "cannot update system tags":
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
		case "tag with this name already exists":
			c.JSON(http.StatusConflict, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tag"})
		}
		return
	}

	c.JSON(http.StatusOK, tag)
}

// DeleteTag deletes a tag
func (h *TagHandlers) DeleteTag(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	err = h.tagService.DeleteTag(tagID, userID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "tag not found":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "cannot delete system tags":
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tag"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted successfully"})
}

// GetTagEntries returns entry IDs that have a specific tag
func (h *TagHandlers) GetTagEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	entryIDs, err := h.tagService.GetEntriesWithTag(tagID, userID)
	if err != nil {
		if err.Error() == "tag not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entry_ids": entryIDs})
}

// InitializeSystemTags ensures system tags exist for the user
func (h *TagHandlers) InitializeSystemTags(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	err = h.tagService.EnsureSystemTags(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize system tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "System tags initialized"})
}
