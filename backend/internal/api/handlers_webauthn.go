package api

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/coldforge/vault/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/google/uuid"
)

// WebAuthnBeginRegistration starts the WebAuthn registration ceremony for an authenticated user
func (h *Handlers) WebAuthnBeginRegistration(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	options, err := h.authService.BeginWebAuthnRegistration(userID)
	if err != nil {
		switch err {
		case auth.ErrWebAuthnNotConfigured:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WebAuthn not configured"})
		case auth.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin registration"})
		}
		return
	}

	c.JSON(http.StatusOK, options)
}

// WebAuthnFinishRegistration completes the WebAuthn registration ceremony
func (h *Handlers) WebAuthnFinishRegistration(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get credential name from query param or body
	credName := c.Query("name")
	if credName == "" {
		credName = "Passkey"
	}

	// Parse the credential creation response
	response, err := protocol.ParseCredentialCreationResponseBody(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential response: " + err.Error()})
		return
	}

	// Finish registration
	credInfo, err := h.authService.FinishWebAuthnRegistration(userID, credName, response)
	if err != nil {
		switch err {
		case auth.ErrSessionNotFound:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration session not found - please start again"})
		case auth.ErrSessionExpired:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration session expired - please start again"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration failed: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Passkey registered successfully",
		"credential": credInfo,
	})
}

// WebAuthnBeginLogin starts the WebAuthn authentication ceremony (username-based)
func (h *Handlers) WebAuthnBeginLogin(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	options, err := h.authService.BeginWebAuthnLogin(req.Email)
	if err != nil {
		switch err {
		case auth.ErrWebAuthnNotConfigured:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WebAuthn not configured"})
		case auth.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		case auth.ErrNoCredentialsForUser:
			c.JSON(http.StatusNotFound, gin.H{"error": "No passkeys registered for this user"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin login"})
		}
		return
	}

	c.JSON(http.StatusOK, options)
}

// WebAuthnBeginDiscoverableLogin starts a discoverable credential login (usernameless)
func (h *Handlers) WebAuthnBeginDiscoverableLogin(c *gin.Context) {
	options, sessionID, err := h.authService.BeginWebAuthnDiscoverableLogin()
	if err != nil {
		switch err {
		case auth.ErrWebAuthnNotConfigured:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WebAuthn not configured"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin login"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"options":    options,
		"session_id": sessionID,
	})
}

// WebAuthnFinishLogin completes the WebAuthn authentication ceremony
func (h *Handlers) WebAuthnFinishLogin(c *gin.Context) {
	var req struct {
		Email     string `json:"email"`
		SessionID string `json:"session_id"`
	}

	// Try to bind JSON for email/session_id
	c.ShouldBindJSON(&req)

	// Parse the credential assertion response
	response, err := protocol.ParseCredentialRequestResponseBody(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential response: " + err.Error()})
		return
	}

	var user interface{}
	var token string

	if req.SessionID != "" {
		// Discoverable login
		u, t, err := h.authService.FinishWebAuthnDiscoverableLogin(req.SessionID, response)
		if err != nil {
			handleWebAuthnLoginError(c, err)
			return
		}
		user = u
		token = t
	} else if req.Email != "" {
		// Username-based login
		u, t, err := h.authService.FinishWebAuthnLogin(req.Email, response)
		if err != nil {
			handleWebAuthnLoginError(c, err)
			return
		}
		user = u
		token = t
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either email or session_id is required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"user":       user,
		"expires_at": time.Now().Add(24 * time.Hour),
	})
}

func handleWebAuthnLoginError(c *gin.Context, err error) {
	switch err {
	case auth.ErrSessionNotFound:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Login session not found - please start again"})
	case auth.ErrSessionExpired:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Login session expired - please start again"})
	case auth.ErrCredentialNotFound:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Credential not recognized"})
	case auth.ErrUserNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed: " + err.Error()})
	}
}

// ListWebAuthnCredentials returns all registered passkeys for the authenticated user
func (h *Handlers) ListWebAuthnCredentials(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	credentials, err := h.authService.ListWebAuthnCredentials(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list credentials"})
		return
	}

	if credentials == nil {
		credentials = []auth.WebAuthnCredentialInfo{}
	}

	c.JSON(http.StatusOK, gin.H{
		"credentials": credentials,
	})
}

// DeleteWebAuthnCredential removes a passkey from the authenticated user's account
func (h *Handlers) DeleteWebAuthnCredential(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	credentialID := c.Param("id")
	if credentialID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Credential ID is required"})
		return
	}

	// URL decode the credential ID (it's base64url encoded)
	_, err = base64.URLEncoding.DecodeString(credentialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID format"})
		return
	}

	err = h.authService.DeleteWebAuthnCredential(userID, credentialID)
	if err != nil {
		switch err {
		case auth.ErrCredentialNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete credential"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Passkey deleted successfully",
	})
}

// UpdateWebAuthnCredential updates a passkey's name
func (h *Handlers) UpdateWebAuthnCredential(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	credentialID := c.Param("id")
	if credentialID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Credential ID is required"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	err = h.authService.UpdateWebAuthnCredentialName(userID, credentialID, req.Name)
	if err != nil {
		switch err {
		case auth.ErrCredentialNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update credential"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Passkey updated successfully",
	})
}
