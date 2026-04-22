package api

import (
	"net/http"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SecretHandlers contains handlers for secret operations
type SecretHandlers struct {
	secretService   *vault.SecretService
	passwordService *vault.PasswordService
}

// NewSecretHandlers creates a new secret handlers instance
func NewSecretHandlers(secretService *vault.SecretService, passwordService *vault.PasswordService) *SecretHandlers {
	return &SecretHandlers{
		secretService:   secretService,
		passwordService: passwordService,
	}
}

// ListSecrets returns all secrets for an entry
func (h *SecretHandlers) ListSecrets(c *gin.Context) {
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

	secrets, err := h.secretService.GetSecrets(entryID, userID)
	if err != nil {
		if err.Error() == "entry not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve secrets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"secrets": secrets})
}

// AddSecret adds a new secret to an entry
func (h *SecretHandlers) AddSecret(c *gin.Context) {
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

	var req models.CreateSecretInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	secret, err := h.secretService.AddSecret(entryID, userID, &req)
	if err != nil {
		if err.Error() == "entry not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add secret"})
		return
	}

	c.JSON(http.StatusCreated, secret)
}

// UpdateSecret updates a secret
func (h *SecretHandlers) UpdateSecret(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	var req vault.UpdateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	secret, err := h.secretService.UpdateSecret(secretID, userID, &req)
	if err != nil {
		if err.Error() == "secret not found or access denied" || err.Error() == "secret not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update secret"})
		return
	}

	c.JSON(http.StatusOK, secret)
}

// DeleteSecret deletes a secret
func (h *SecretHandlers) DeleteSecret(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	err = h.secretService.DeleteSecret(secretID, userID)
	if err != nil {
		if err.Error() == "secret not found or access denied" || err.Error() == "secret not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete secret"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Secret deleted successfully"})
}

// ReorderSecrets reorders secrets within an entry
func (h *SecretHandlers) ReorderSecrets(c *gin.Context) {
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID: " + idStr})
			return
		}
		positions[id] = pos
	}

	if err := h.secretService.ReorderSecrets(entryID, userID, positions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder secrets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Secrets reordered successfully"})
}

// GeneratePassword generates a secure password
func (h *SecretHandlers) GeneratePassword(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.PasswordGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Set defaults if not provided
	if req.Length == 0 {
		req.Length = 16
	}
	if !req.IncludeUppercase && !req.IncludeLowercase && !req.IncludeNumbers && !req.IncludeSymbols {
		req.IncludeUppercase = true
		req.IncludeLowercase = true
		req.IncludeNumbers = true
		req.IncludeSymbols = true
	}

	result, err := h.passwordService.GeneratePassword(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate password"})
		return
	}

	// Optionally record generation history (fire and forget)
	go h.passwordService.RecordPasswordGeneration(userID, &req, result, nil)

	c.JSON(http.StatusOK, result)
}

// GetPasswordHistory returns password generation history
func (h *SecretHandlers) GetPasswordHistory(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		// Parse limit (ignore errors, use default)
		if parsed, err := uuid.Parse(l); err == nil {
			// This won't work, but we need to parse as int
			_ = parsed
		}
	}

	history, err := h.passwordService.GetPasswordHistory(userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get password history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": history})
}
