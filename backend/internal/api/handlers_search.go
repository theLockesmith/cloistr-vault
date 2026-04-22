package api

import (
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to perform search"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetRecentEntries returns recently used entries
func (h *SearchHandlers) GetRecentEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recent entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// GetFrequentEntries returns most frequently used entries
func (h *SearchHandlers) GetFrequentEntries(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get frequent entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}
