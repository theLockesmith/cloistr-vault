package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handlers struct {
	authService  *auth.AuthService
	vaultService VaultService // We'll define this interface
}

// VaultService interface for vault operations
type VaultService interface {
	GetVault(userID uuid.UUID) (*models.VaultResponse, error)
	UpdateVault(userID uuid.UUID, data []byte, version int) (*models.VaultResponse, error)
}

func NewHandlers(authService *auth.AuthService, vaultService VaultService) *Handlers {
	return &Handlers{
		authService:  authService,
		vaultService: vaultService,
	}
}

// Health check endpoint
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": gin.H{"unix": gin.H{"seconds": 1}}["unix"],
		"version":   "1.0.0",
	})
}

// Register new user
func (h *Handlers) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}
	
	// Validate request
	if req.Method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Method is required"})
		return
	}
	
	if len(req.VaultData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vault data is required"})
		return
	}
	
	// Register user
	user, err := h.authService.RegisterUser(&req)
	if err != nil {
		switch err {
		case auth.ErrUserExists:
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		case auth.ErrInvalidAuthMethod:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authentication method"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		}
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"user":    user,
		"message": "User registered successfully",
	})
}

// Login user
func (h *Handlers) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if req.Method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Method is required"})
		return
	}

	// Handle different authentication methods
	switch req.Method {
	case "email":
		response, err := h.authService.LoginUser(&req)
		if err != nil {
			switch err {
			case auth.ErrUserNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			case auth.ErrInvalidCredentials:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
			}
			return
		}
		c.JSON(http.StatusOK, response)

	case "nostr":
		// Handle Nostr authentication
		if req.NostrPubkey == nil || req.Signature == nil || req.Challenge == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Nostr authentication requires pubkey, signature, and challenge"})
			return
		}

		user, token, err := h.authService.AuthenticateWithNostr(*req.NostrPubkey, *req.Signature, *req.Challenge)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Nostr authentication failed: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":     token,
			"user":      user,
			"expires_at": time.Now().Add(24 * time.Hour),
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authentication method"})
	}
}

// Logout user
func (h *Handlers) Logout(c *gin.Context) {
	// Extract token from authorization header
	token := extractTokenFromHeader(c.GetHeader("Authorization"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorization header"})
		return
	}
	
	// Revoke session
	err := h.authService.RevokeSession(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logout failed"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// Generate Nostr challenge
func (h *Handlers) NostrChallenge(c *gin.Context) {
	var req struct {
		PublicKey string `json:"public_key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate public key format
	if len(req.PublicKey) != 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid public key format"})
		return
	}

	// For Nostr authentication, we don't require existing user - create challenge for any valid pubkey
	challenge, err := h.authService.GenerateNostrChallengePublic(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Challenge generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"challenge":  challenge.Value,
		"expires_at": challenge.ExpiresAt,
	})
}

// Get user profile
func (h *Handlers) GetProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// Get user's vault
func (h *Handlers) GetVault(c *gin.Context) {
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
	
	vault, err := h.vaultService.GetVault(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve vault"})
		return
	}
	
	c.JSON(http.StatusOK, vault)
}

// Update user's vault
func (h *Handlers) UpdateVault(c *gin.Context) {
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
	
	var req struct {
		EncryptedData []byte `json:"encrypted_data" binding:"required"`
		Version       int    `json:"version" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}
	
	vault, err := h.vaultService.UpdateVault(userID, req.EncryptedData, req.Version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vault"})
		return
	}
	
	c.JSON(http.StatusOK, vault)
}

// Get API info
func (h *Handlers) GetAPIInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":    "Coldforge Vault API",
		"version": "1.0.0",
		"auth_methods": []string{"email", "nostr"},
		"endpoints": gin.H{
			"auth": gin.H{
				"register":        "/api/v1/auth/register",
				"login":          "/api/v1/auth/login",
				"logout":         "/api/v1/auth/logout",
				"nostr_challenge": "/api/v1/auth/nostr/challenge",
			},
			"user": gin.H{
				"profile": "/api/v1/user/profile",
			},
			"vault": gin.H{
				"get":    "/api/v1/vault",
				"update": "/api/v1/vault",
			},
		},
	})
}

// Helper functions
func extractTokenFromHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	
	return parts[1]
}