package api

import (
	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
	"net/http"
	"strconv"

	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
)

// SearchHandlers contains handlers for search operations
type SearchHandlers struct {
	searchService *vault.SearchService
}

// NewSearchHandlers creates a new search handlers instance
func NewSearchHandlers(searchService *vault.SearchService) *SearchHandlers {
	return &SearchHandlers{
		searchService: searchService,
	}
}

// Search performs unified search across entries, folders, and tags
func (h *SearchHandlers) Search(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	query := c.Query("q")
	if query == "" {
		errors.BadRequest(errors.CodeValidationFailed, "Search query is required").Abort(c)
		return
	}

	var req vault.UnifiedSearchRequest
	req.Query = query
	req.IncludeAll = c.Query("include_all") == "true"

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			req.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			req.Offset = o
		}
	}

	result, err := h.searchService.Search(userID, &req)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to perform search").Abort(c)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetRecentEntries returns recently used entries
func (h *SearchHandlers) GetRecentEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	entries, err := h.searchService.GetRecentEntries(userID, limit)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to get recent entries").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// GetFrequentEntries returns most frequently used entries
func (h *SearchHandlers) GetFrequentEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	entries, err := h.searchService.GetFrequentEntries(userID, limit)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to get frequent entries").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}
