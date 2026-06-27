package api

import (
	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
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
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	var req models.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	team, err := h.sharingService.CreateTeam(userID, &req)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to create team").Abort(c)
		return
	}

	c.JSON(http.StatusCreated, team)
}

// GetTeam returns a team by ID
func (h *SharingHandlers) GetTeam(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid team ID").Abort(c)
		return
	}

	team, err := h.sharingService.GetTeam(teamID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "team not found" || errMsg == "team not found or access denied" {
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to get team").Abort(c)
		return
	}

	c.JSON(http.StatusOK, team)
}

// ListTeams returns all teams the user belongs to
func (h *SharingHandlers) ListTeams(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	teams, err := h.sharingService.ListUserTeams(userID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to list teams").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

// GetTeamMembers returns all members of a team
func (h *SharingHandlers) GetTeamMembers(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid team ID").Abort(c)
		return
	}

	// Verify membership
	_, err = h.sharingService.GetTeam(teamID, userID)
	if err != nil {
		errors.NotFound(errors.CodeAccessDenied, "Team not found or access denied").Abort(c)
		return
	}

	members, err := h.sharingService.GetTeamMembers(teamID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to get team members").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

// InviteToTeam creates an invitation to join a team
func (h *SharingHandlers) InviteToTeam(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid team ID").Abort(c)
		return
	}

	var req models.InviteToTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	invitation, err := h.sharingService.InviteToTeam(teamID, userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "not a team member":
			errors.Forbidden(errors.CodeAccessDenied, errMsg).Abort(c)
		case "insufficient permissions to invite":
			errors.Forbidden(errors.CodeAccessDenied, errMsg).Abort(c)
		case "email or pubkey required":
			errors.BadRequest(errors.CodeValidationFailed, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to create invitation").Abort(c)
		}
		return
	}

	c.JSON(http.StatusCreated, invitation)
}

// AcceptTeamInvitation accepts a team invitation
func (h *SharingHandlers) AcceptTeamInvitation(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	invitationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid invitation ID").Abort(c)
		return
	}

	err = h.sharingService.AcceptTeamInvitation(invitationID, userID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "invitation not found":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "invitation expired":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		default:
			if len(errMsg) > 18 && errMsg[:18] == "invitation already" {
				errors.Conflict(errors.CodeResourceConflict, errMsg).Abort(c)
			} else {
				errors.InternalError(errors.CodeInternalError, "Failed to accept invitation").Abort(c)
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
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	var req models.ShareFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	shared, err := h.sharingService.ShareFolder(userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "folder not found":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "not the folder owner":
			errors.Forbidden(errors.CodeAccessDenied, errMsg).Abort(c)
		case "team_id or user_id required":
			errors.BadRequest(errors.CodeValidationFailed, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to share folder").Abort(c)
		}
		return
	}

	c.JSON(http.StatusCreated, shared)
}

// GetSharedFolders returns folders shared with the user
func (h *SharingHandlers) GetSharedFolders(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	folders, err := h.sharingService.GetSharedFolders(userID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to get shared folders").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"shared_folders": folders})
}

// GetFolderKey retrieves the encrypted folder key for decryption
func (h *SharingHandlers) GetFolderKey(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid folder ID").Abort(c)
		return
	}

	key, err := h.sharingService.GetFolderKey(folderID, userID)
	if err != nil {
		if err.Error() == "folder key not found" {
			errors.Forbidden(errors.CodeAccessDenied, "No access to this folder").Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to get folder key").Abort(c)
		return
	}

	c.JSON(http.StatusOK, key)
}

// RevokeShare removes a folder share
func (h *SharingHandlers) RevokeShare(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	shareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid share ID").Abort(c)
		return
	}

	err = h.sharingService.RevokeShare(shareID, userID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "shared folder not found":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "not authorized to revoke this share":
			errors.Forbidden(errors.CodeAccessDenied, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to revoke share").Abort(c)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Share revoked"})
}
