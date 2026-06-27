package api

import (
	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
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
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid user ID").Abort(c)
		return
	}

	options, err := h.authService.BeginWebAuthnRegistration(userID)
	if err != nil {
		switch err {
		case auth.ErrWebAuthnNotConfigured:
			errors.ServiceUnavailable(errors.CodeServiceUnavailable, "WebAuthn not configured", 0).Abort(c)
		case auth.ErrUserNotFound:
			errors.NotFound(errors.CodeResourceNotFound, "User not found").Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to begin registration").Abort(c)
		}
		return
	}

	c.JSON(http.StatusOK, options)
}

// WebAuthnFinishRegistration completes the WebAuthn registration ceremony
func (h *Handlers) WebAuthnFinishRegistration(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid user ID").Abort(c)
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
		errors.BadRequest(errors.CodeInvalidInput, "Invalid credential response: " + err.Error()).Abort(c)
		return
	}

	// Finish registration
	credInfo, err := h.authService.FinishWebAuthnRegistration(userID, credName, response)
	if err != nil {
		switch err {
		case auth.ErrSessionNotFound:
			errors.BadRequest(errors.CodeValidationFailed, "Registration session not found - please start again").Abort(c)
		case auth.ErrSessionExpired:
			errors.BadRequest(errors.CodeValidationFailed, "Registration session expired - please start again").Abort(c)
		default:
			errors.BadRequest(errors.CodeInvalidInput, "Registration failed: " + err.Error()).Abort(c)
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
		errors.BadRequest(errors.CodeValidationFailed, "Email is required").Abort(c)
		return
	}

	options, err := h.authService.BeginWebAuthnLogin(req.Email)
	if err != nil {
		switch err {
		case auth.ErrWebAuthnNotConfigured:
			errors.ServiceUnavailable(errors.CodeServiceUnavailable, "WebAuthn not configured", 0).Abort(c)
		case auth.ErrUserNotFound:
			errors.NotFound(errors.CodeResourceNotFound, "User not found").Abort(c)
		case auth.ErrNoCredentialsForUser:
			errors.NotFound(errors.CodeResourceNotFound, "No passkeys registered for this user").Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to begin login").Abort(c)
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
			errors.ServiceUnavailable(errors.CodeServiceUnavailable, "WebAuthn not configured", 0).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to begin login").Abort(c)
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
		errors.BadRequest(errors.CodeInvalidInput, "Invalid credential response: " + err.Error()).Abort(c)
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
		errors.BadRequest(errors.CodeValidationFailed, "Either email or session_id is required").Abort(c)
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
		errors.BadRequest(errors.CodeValidationFailed, "Login session not found - please start again").Abort(c)
	case auth.ErrSessionExpired:
		errors.BadRequest(errors.CodeValidationFailed, "Login session expired - please start again").Abort(c)
	case auth.ErrCredentialNotFound:
		errors.Unauthorized(errors.CodeAuthInvalid, "Credential not recognized").Abort(c)
	case auth.ErrUserNotFound:
		errors.NotFound(errors.CodeResourceNotFound, "User not found").Abort(c)
	default:
		errors.BadRequest(errors.CodeInvalidInput, "Authentication failed: " + err.Error()).Abort(c)
	}
}

// ListWebAuthnCredentials returns all registered passkeys for the authenticated user
func (h *Handlers) ListWebAuthnCredentials(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid user ID").Abort(c)
		return
	}

	credentials, err := h.authService.ListWebAuthnCredentials(userID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to list credentials").Abort(c)
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
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid user ID").Abort(c)
		return
	}

	credentialID := c.Param("id")
	if credentialID == "" {
		errors.BadRequest(errors.CodeValidationFailed, "Credential ID is required").Abort(c)
		return
	}

	// URL decode the credential ID (it's base64url encoded)
	_, err = base64.URLEncoding.DecodeString(credentialID)
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid credential ID format").Abort(c)
		return
	}

	err = h.authService.DeleteWebAuthnCredential(userID, credentialID)
	if err != nil {
		switch err {
		case auth.ErrCredentialNotFound:
			errors.NotFound(errors.CodeResourceNotFound, "Credential not found").Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to delete credential").Abort(c)
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
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid user ID").Abort(c)
		return
	}

	credentialID := c.Param("id")
	if credentialID == "" {
		errors.BadRequest(errors.CodeValidationFailed, "Credential ID is required").Abort(c)
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeValidationFailed, "Name is required").Abort(c)
		return
	}

	err = h.authService.UpdateWebAuthnCredentialName(userID, credentialID, req.Name)
	if err != nil {
		switch err {
		case auth.ErrCredentialNotFound:
			errors.NotFound(errors.CodeResourceNotFound, "Credential not found").Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to update credential").Abort(c)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Passkey updated successfully",
	})
}
