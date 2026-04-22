package api

import (
	"net/http"

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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry ID"})
		return
	}

	attachments, err := h.attachmentService.ListAttachments(entryID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "entry not found" || errMsg == "entry not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list attachments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"attachments": attachments})
}

// AddAttachment uploads a new attachment to an entry
func (h *AttachmentHandlers) AddAttachment(c *gin.Context) {
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

	var req models.CreateAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Override entry ID from URL
	req.EntryID = entryID

	attachment, err := h.attachmentService.CreateAttachment(userID, &req)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "entry not found", "entry not found or access denied":
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		case "maximum attachments per entry reached (10)":
			c.JSON(http.StatusConflict, gin.H{"error": errMsg})
		case "file size exceeds 10MB limit":
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": errMsg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create attachment"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	attachment, err := h.attachmentService.GetAttachment(attachmentID, userID)
	if err != nil {
		if err.Error() == "attachment not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get attachment"})
		return
	}

	c.JSON(http.StatusOK, attachment)
}

// GetAttachmentMetadata returns attachment info without the encrypted data
func (h *AttachmentHandlers) GetAttachmentMetadata(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	meta, err := h.attachmentService.GetAttachmentMetadata(attachmentID, userID)
	if err != nil {
		if err.Error() == "attachment not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get attachment"})
		return
	}

	c.JSON(http.StatusOK, meta)
}

// UpdateAttachment updates attachment metadata
func (h *AttachmentHandlers) UpdateAttachment(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	var req models.UpdateAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No update fields provided"})
		return
	}

	meta, err := h.attachmentService.UpdateAttachmentName(attachmentID, userID, *req.Name)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "attachment not found" || errMsg == "attachment not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update attachment"})
		return
	}

	c.JSON(http.StatusOK, meta)
}

// DeleteAttachment removes an attachment
func (h *AttachmentHandlers) DeleteAttachment(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	attachmentID, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	err = h.attachmentService.DeleteAttachment(attachmentID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "attachment not found" || errMsg == "attachment not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attachment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
}

// GetStorageUsage returns the user's total attachment storage usage
func (h *AttachmentHandlers) GetStorageUsage(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	totalSize, count, err := h.attachmentService.GetUserStorageUsage(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get storage usage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_bytes":      totalSize,
		"total_attachments": count,
		"limit_bytes":      100 * 1024 * 1024, // 100MB limit
	})
}
