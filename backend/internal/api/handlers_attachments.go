package api

import (
	"net/http"

	"git.aegis-hq.xyz/coldforge/cloistr-common/errors"
	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AttachmentHandlers contains handlers for attachment operations
type AttachmentHandlers struct {
	attachmentService *vault.AttachmentService
}

// NewAttachmentHandlers creates a new attachment handlers instance
func NewAttachmentHandlers(attachmentService *vault.AttachmentService) *AttachmentHandlers {
	return &AttachmentHandlers{
		attachmentService: attachmentService,
	}
}

// ListAttachments returns all attachments for an entry
func (h *AttachmentHandlers) ListAttachments(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid entry ID").Abort(c)
		return
	}

	attachments, err := h.attachmentService.ListAttachments(entryID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "entry not found" || errMsg == "entry not found or access denied" {
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to list attachments").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"attachments": attachments})
}

// AddAttachment uploads a new attachment to an entry
func (h *AttachmentHandlers) AddAttachment(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid entry ID").Abort(c)
		return
	}

	var req models.CreateAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	// Override entry ID from URL
	req.EntryID = entryID

	attachment, err := h.attachmentService.CreateAttachment(userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "entry not found", "entry not found or access denied":
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
		case "maximum attachments per entry reached (10)":
			errors.Conflict(errors.CodeResourceConflict, errMsg).Abort(c)
		case "file size exceeds 10MB limit":
			errors.InsufficientStorage(errors.CodeQuotaExceeded, errMsg).Abort(c)
		default:
			errors.InternalError(errors.CodeInternalError, "Failed to create attachment").Abort(c)
		}
		return
	}

	// Return metadata without encrypted data
	meta := models.AttachmentMetadata{
		ID:        attachment.ID,
		EntryID:   attachment.EntryID,
		Name:      attachment.Name,
		FileType:  attachment.FileType,
		MimeType:  attachment.MimeType,
		FileSize:  attachment.FileSize,
		CreatedAt: attachment.CreatedAt,
	}

	c.JSON(http.StatusCreated, meta)
}

// GetAttachment downloads an attachment
func (h *AttachmentHandlers) GetAttachment(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid attachment ID").Abort(c)
		return
	}

	attachment, err := h.attachmentService.GetAttachment(attachmentID, userID)
	if err != nil {
		if err.Error() == "attachment not found" {
			errors.NotFound(errors.CodeResourceNotFound, "Attachment not found").Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to get attachment").Abort(c)
		return
	}

	c.JSON(http.StatusOK, attachment)
}

// GetAttachmentMetadata returns attachment info without the encrypted data
func (h *AttachmentHandlers) GetAttachmentMetadata(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid attachment ID").Abort(c)
		return
	}

	meta, err := h.attachmentService.GetAttachmentMetadata(attachmentID, userID)
	if err != nil {
		if err.Error() == "attachment not found" {
			errors.NotFound(errors.CodeResourceNotFound, "Attachment not found").Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to get attachment").Abort(c)
		return
	}

	c.JSON(http.StatusOK, meta)
}

// UpdateAttachment updates attachment metadata
func (h *AttachmentHandlers) UpdateAttachment(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid attachment ID").Abort(c)
		return
	}

	var req models.UpdateAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid request format").Abort(c)
		return
	}

	if req.Name == nil {
		errors.BadRequest(errors.CodeValidationFailed, "No update fields provided").Abort(c)
		return
	}

	meta, err := h.attachmentService.UpdateAttachmentName(attachmentID, userID, *req.Name)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "attachment not found" || errMsg == "attachment not found or access denied" {
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to update attachment").Abort(c)
		return
	}

	c.JSON(http.StatusOK, meta)
}

// DeleteAttachment removes an attachment
func (h *AttachmentHandlers) DeleteAttachment(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		errors.BadRequest(errors.CodeInvalidInput, "Invalid attachment ID").Abort(c)
		return
	}

	err = h.attachmentService.DeleteAttachment(attachmentID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "attachment not found" || errMsg == "attachment not found or access denied" {
			errors.NotFound(errors.CodeResourceNotFound, errMsg).Abort(c)
			return
		}
		errors.InternalError(errors.CodeInternalError, "Failed to delete attachment").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
}

// GetStorageUsage returns the user's total attachment storage usage
func (h *AttachmentHandlers) GetStorageUsage(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		errors.Unauthorized(errors.CodeAuthRequired, "User ID not found").Abort(c)
		return
	}

	totalSize, count, err := h.attachmentService.GetUserStorageUsage(userID)
	if err != nil {
		errors.InternalError(errors.CodeInternalError, "Failed to get storage usage").Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_bytes":      totalSize,
		"total_attachments": count,
		"limit_bytes":      100 * 1024 * 1024, // 100MB limit
	})
}
