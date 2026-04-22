package api

import (
	"net/http"

	"github.com/coldforge/vault/internal/security"
	"github.com/gin-gonic/gin"
)

// SecurityHandlers contains handlers for security operations
type SecurityHandlers struct {
	securityService *security.SecurityService
}

// NewSecurityHandlers creates a new security handlers instance
func NewSecurityHandlers(securityService *security.SecurityService) *SecurityHandlers {
	return &SecurityHandlers{
		securityService: securityService,
	}
}

// GetSecurityScore returns overall vault security score
func (h *SecurityHandlers) GetSecurityScore(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	score, err := h.securityService.AnalyzeVaultSecurity(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze security"})
		return
	}

	c.JSON(http.StatusOK, score)
}

// AnalyzePassword analyzes password strength
type AnalyzePasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *SecurityHandlers) AnalyzePassword(c *gin.Context) {
	var req AnalyzePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required"})
		return
	}

	result := security.AnalyzePassword(req.Password)
	c.JSON(http.StatusOK, result)
}

// CheckBreach checks if a password has been compromised
type CheckBreachRequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *SecurityHandlers) CheckBreach(c *gin.Context) {
	var req CheckBreachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required"})
		return
	}

	result := security.CheckHIBP(req.Password)
	c.JSON(http.StatusOK, result)
}

// GetWeakPasswords returns entries with weak passwords
func (h *SecurityHandlers) GetWeakPasswords(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// Query entries with weak passwords
	query := `
		SELECT ve.id, ve.name, ve.entry_type, ve.url, vs.strength_score
		FROM vault_entries ve
		JOIN vault_secrets vs ON ve.id = vs.entry_id
		WHERE ve.user_id = $1 AND vs.secret_type = 'password' AND vs.strength_score < 2
		ORDER BY vs.strength_score ASC, ve.name
	`

	rows, err := h.securityService.DB().Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query weak passwords"})
		return
	}
	defer rows.Close()

	type WeakEntry struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		EntryType     string  `json:"entry_type"`
		URL           *string `json:"url"`
		StrengthScore *int    `json:"strength_score"`
	}

	var entries []WeakEntry
	for rows.Next() {
		var entry WeakEntry
		err := rows.Scan(&entry.ID, &entry.Name, &entry.EntryType, &entry.URL, &entry.StrengthScore)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan results"})
			return
		}
		entries = append(entries, entry)
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// GetBreachedPasswords returns entries with breached passwords
func (h *SecurityHandlers) GetBreachedPasswords(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	query := `
		SELECT id, name, entry_type, url, last_breach_check
		FROM vault_entries
		WHERE user_id = $1 AND has_breach = true
		ORDER BY name
	`

	rows, err := h.securityService.DB().Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query breached passwords"})
		return
	}
	defer rows.Close()

	type BreachedEntry struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		EntryType       string  `json:"entry_type"`
		URL             *string `json:"url"`
		LastBreachCheck *string `json:"last_breach_check"`
	}

	var entries []BreachedEntry
	for rows.Next() {
		var entry BreachedEntry
		err := rows.Scan(&entry.ID, &entry.Name, &entry.EntryType, &entry.URL, &entry.LastBreachCheck)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan results"})
			return
		}
		entries = append(entries, entry)
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// GetReusedPasswords returns entries with reused passwords
func (h *SecurityHandlers) GetReusedPasswords(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	query := `
		SELECT id, name, entry_type, url
		FROM vault_entries
		WHERE user_id = $1 AND has_reused_password = true
		ORDER BY name
	`

	rows, err := h.securityService.DB().Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query reused passwords"})
		return
	}
	defer rows.Close()

	type ReusedEntry struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		EntryType string  `json:"entry_type"`
		URL       *string `json:"url"`
	}

	var entries []ReusedEntry
	for rows.Next() {
		var entry ReusedEntry
		err := rows.Scan(&entry.ID, &entry.Name, &entry.EntryType, &entry.URL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan results"})
			return
		}
		entries = append(entries, entry)
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// GetExpiringSecrets returns secrets expiring within specified days
func (h *SecurityHandlers) GetExpiringSecrets(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// Default to 30 days
	query := `
		SELECT ve.id, ve.name, ve.entry_type, ve.url, vs.name as secret_name, vs.expires_at
		FROM vault_entries ve
		JOIN vault_secrets vs ON ve.id = vs.entry_id
		WHERE ve.user_id = $1 AND vs.expires_at IS NOT NULL
		AND vs.expires_at >= NOW() AND vs.expires_at < NOW() + INTERVAL '30 days'
		ORDER BY vs.expires_at ASC
	`

	rows, err := h.securityService.DB().Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query expiring secrets"})
		return
	}
	defer rows.Close()

	type ExpiringSecret struct {
		EntryID    string `json:"entry_id"`
		EntryName  string `json:"entry_name"`
		EntryType  string `json:"entry_type"`
		URL        *string `json:"url"`
		SecretName string `json:"secret_name"`
		ExpiresAt  string `json:"expires_at"`
	}

	var secrets []ExpiringSecret
	for rows.Next() {
		var secret ExpiringSecret
		err := rows.Scan(&secret.EntryID, &secret.EntryName, &secret.EntryType,
			&secret.URL, &secret.SecretName, &secret.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan results"})
			return
		}
		secrets = append(secrets, secret)
	}

	c.JSON(http.StatusOK, gin.H{"secrets": secrets})
}
