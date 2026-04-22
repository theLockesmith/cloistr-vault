package api

import (
	"net/http"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SharingHandlers contains handlers for sharing and team operations
type SharingHandlers struct {
	sharingService *vault.SharingService
}

// NewSharingHandlers creates a new sharing handlers instance
func NewSharingHandlers(sharingService *vault.SharingService) *SharingHandlers {
	return &SharingHandlers{
		sharingService: sharingService,
	}
}

// --- Team Handlers ---

// CreateTeam creates a new team
func (h *SharingHandlers) CreateTeam(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	team, err := h.sharingService.CreateTeam(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	c.JSON(http.StatusCreated, team)
}

// GetTeam returns a team by ID
func (h *SharingHandlers) GetTeam(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	team, err := h.sharingService.GetTeam(teamID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "team not found" || errMsg == "team not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}

	c.JSON(http.StatusOK, team)
}

// ListTeams returns all teams the user belongs to
func (h *SharingHandlers) ListTeams(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	teams, err := h.sharingService.ListUserTeams(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teams"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

// GetTeamMembers returns all members of a team
func (h *SharingHandlers) GetTeamMembers(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	// Verify membership
	_, err = h.sharingService.GetTeam(teamID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found or access denied"})
		return
	}

	members, err := h.sharingService.GetTeamMembers(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team members"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

// InviteToTeam creates an invitation to join a team
func (h *SharingHandlers) InviteToTeam(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	var req models.InviteToTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	invitation, err := h.sharingService.InviteToTeam(teamID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "not a team member":
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
		case "insufficient permissions to invite":
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
		case "email or pubkey required":
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		}
		return
	}

	c.JSON(http.StatusCreated, invitation)
}

// AcceptTeamInvitation accepts a team invitation
func (h *SharingHandlers) AcceptTeamInvitation(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	invitationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation ID"})
		return
	}

	err = h.sharingService.AcceptTeamInvitation(invitationID, userID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "invitation not found":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "invitation expired":
			c.JSON(http.StatusGone, gin.H{"error": errMsg})
		default:
			if len(errMsg) > 18 && errMsg[:18] == "invitation already" {
				c.JSON(http.StatusConflict, gin.H{"error": errMsg})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept invitation"})
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation accepted"})
}

// --- Folder Sharing Handlers ---

// ShareFolder shares a folder with a team or user
func (h *SharingHandlers) ShareFolder(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req models.ShareFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	shared, err := h.sharingService.ShareFolder(userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "folder not found":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "not the folder owner":
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
		case "team_id or user_id required":
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share folder"})
		}
		return
	}

	c.JSON(http.StatusCreated, shared)
}

// GetSharedFolders returns folders shared with the user
func (h *SharingHandlers) GetSharedFolders(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	folders, err := h.sharingService.GetSharedFolders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get shared folders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shared_folders": folders})
}

// GetFolderKey retrieves the encrypted folder key for decryption
func (h *SharingHandlers) GetFolderKey(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder ID"})
		return
	}

	key, err := h.sharingService.GetFolderKey(folderID, userID)
	if err != nil {
		if err.Error() == "folder key not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "No access to this folder"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get folder key"})
		return
	}

	c.JSON(http.StatusOK, key)
}

// RevokeShare removes a folder share
func (h *SharingHandlers) RevokeShare(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	shareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid share ID"})
		return
	}

	err = h.sharingService.RevokeShare(shareID, userID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "shared folder not found":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "not authorized to revoke this share":
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke share"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Share revoked"})
}
