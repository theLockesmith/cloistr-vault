package models

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID `json:"id" db:"id"`
	Email            string    `json:"email" db:"email"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
	// Extended fields for display (not stored in users table, populated from auth_methods)
	AuthMethod       string    `json:"auth_method,omitempty"`
	DisplayName      string    `json:"display_name,omitempty"`
	NostrPubkey      string    `json:"nostr_pubkey,omitempty"`
	NIP05Address     string    `json:"nip05_address,omitempty"`
	LightningAddress string    `json:"lightning_address,omitempty"`
}

type AuthMethod struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	Type         string     `json:"type" db:"type"` // "email", "nostr"
	Identifier   string     `json:"identifier" db:"identifier"` // email or nostr pubkey
	Salt         []byte     `json:"-" db:"salt"`
	PasswordHash []byte     `json:"-" db:"password_hash"`
	NostrPubkey  *string    `json:"nostr_pubkey,omitempty" db:"nostr_pubkey"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

type Vault struct {
	ID               uuid.UUID `json:"id" db:"id"`
	UserID           uuid.UUID `json:"user_id" db:"user_id"`
	EncryptedData    []byte    `json:"-" db:"encrypted_data"`
	EncryptionSalt   []byte    `json:"-" db:"encryption_salt"`
	EncryptionNonce  []byte    `json:"-" db:"encryption_nonce"`
	Version          int       `json:"version" db:"version"`
	LastModified     time.Time `json:"last_modified" db:"last_modified"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

type RecoveryCode struct {
	ID           uuid.UUID `json:"id" db:"id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	CodeHash     []byte    `json:"-" db:"code_hash"`
	Salt         []byte    `json:"-" db:"salt"`
	Used         bool      `json:"used" db:"used"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UsedAt       *time.Time `json:"used_at,omitempty" db:"used_at"`
}

type Session struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type VaultEntry struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // "login", "note", "card", "identity"
	Name        string            `json:"name"`
	Fields      map[string]string `json:"fields"`
	Notes       string            `json:"notes"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Favorite    bool              `json:"favorite"`
	FolderID    *string           `json:"folder_id,omitempty"`
}

type LoginRequest struct {
	Method           string  `json:"method" binding:"required"` // "email", "nostr", or "lightning"
	Email            *string `json:"email,omitempty"`
	Password         *string `json:"password,omitempty"`
	NostrPubkey      *string `json:"nostr_pubkey,omitempty"`
	Signature        *string `json:"signature,omitempty"`
	Challenge        *string `json:"challenge,omitempty"`
	// Lightning LNURL-auth fields
	LightningAddress *string `json:"lightning_address,omitempty"`
	LinkingKey       *string `json:"linking_key,omitempty"` // Public key for LNURL-auth verification
}

type RegisterRequest struct {
	Method      string  `json:"method" binding:"required"`
	Email       *string `json:"email,omitempty"`
	Password    *string `json:"password,omitempty"`
	NostrPubkey *string `json:"nostr_pubkey,omitempty"`
	VaultData   []byte  `json:"vault_data" binding:"required"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

type VaultResponse struct {
	ID               uuid.UUID `json:"id"`
	Version          int       `json:"version"`
	LastModified     time.Time `json:"last_modified"`
	EncryptedData    []byte    `json:"encrypted_data"`
	EncryptionSalt   []byte    `json:"encryption_salt"`
	EncryptionNonce  []byte    `json:"encryption_nonce"`
}

// Recovery-related request/response types

type RecoveryRequest struct {
	Email        string `json:"email" binding:"required"`
	RecoveryCode string `json:"recovery_code" binding:"required"`
	NewPassword  string `json:"new_password" binding:"required"`
	VaultData    []byte `json:"vault_data" binding:"required"` // Re-encrypted vault with new key
}

type RecoveryStatusResponse struct {
	Total     int  `json:"total"`
	Remaining int  `json:"remaining"`
	Used      int  `json:"used"`
}

type RegisterResponse struct {
	Token         string    `json:"token"`
	User          User      `json:"user"`
	ExpiresAt     time.Time `json:"expires_at"`
	RecoveryCodes []string  `json:"recovery_codes"`
	Warning       string    `json:"recovery_warning"`
}

// ============================================
// Enhanced Vault Models
// ============================================

// VaultFolder represents a hierarchical folder for organizing vault entries
type VaultFolder struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	UserID    uuid.UUID    `json:"user_id" db:"user_id"`
	ParentID  *uuid.UUID   `json:"parent_id,omitempty" db:"parent_id"`
	Name      string       `json:"name" db:"name"`
	Icon      string       `json:"icon" db:"icon"`
	Color     string       `json:"color" db:"color"`
	Position  int          `json:"position" db:"position"`
	IsShared  bool         `json:"is_shared" db:"is_shared"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
	// Computed fields
	EntryCount int            `json:"entry_count,omitempty" db:"-"`
	Children   []*VaultFolder `json:"children,omitempty" db:"-"`
}

// VaultTag represents a tag for categorizing entries
type VaultTag struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Name       string    `json:"name" db:"name"`
	Color      string    `json:"color" db:"color"`
	Category   string    `json:"category" db:"category"` // "security", "type", "custom"
	IsSystem   bool      `json:"is_system" db:"is_system"`
	UsageCount int       `json:"usage_count" db:"usage_count"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// EnhancedVaultEntry represents an individual vault item with granular structure
type EnhancedVaultEntry struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	FolderID          *uuid.UUID `json:"folder_id,omitempty" db:"folder_id"`
	Name              string     `json:"name" db:"name"`
	EntryType         string     `json:"entry_type" db:"entry_type"`
	URL               *string    `json:"url,omitempty" db:"url"`
	Notes             *string    `json:"notes,omitempty" db:"notes"`
	IsFavorite        bool       `json:"is_favorite" db:"is_favorite"`
	Position          int        `json:"position" db:"position"`
	StrengthScore     int        `json:"strength_score" db:"strength_score"`
	HasWeakPassword   bool       `json:"has_weak_password" db:"has_weak_password"`
	HasReusedPassword bool       `json:"has_reused_password" db:"has_reused_password"`
	HasBreach         bool       `json:"has_breach" db:"has_breach"`
	LastBreachCheck   *time.Time `json:"last_breach_check,omitempty" db:"last_breach_check"`
	LastUsed          *time.Time `json:"last_used,omitempty" db:"last_used"`
	UsageCount        int        `json:"usage_count" db:"usage_count"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
	// Related data (populated by queries)
	Secrets     []VaultSecret     `json:"secrets,omitempty" db:"-"`
	Tags        []VaultTag        `json:"tags,omitempty" db:"-"`
	Attachments []VaultAttachment `json:"attachments,omitempty" db:"-"`
}

// VaultSecret represents an encrypted secret value within an entry
type VaultSecret struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	EntryID        uuid.UUID  `json:"entry_id" db:"entry_id"`
	SecretType     string     `json:"secret_type" db:"secret_type"`
	Name           string     `json:"name" db:"name"`
	EncryptedValue string     `json:"encrypted_value" db:"encrypted_value"` // Client-encrypted
	Username       *string    `json:"username,omitempty" db:"username"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	LastRotated    *time.Time `json:"last_rotated,omitempty" db:"last_rotated"`
	StrengthScore  int        `json:"strength_score" db:"strength_score"`
	Position       int        `json:"position" db:"position"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// VaultAttachment represents an encrypted file attached to an entry
type VaultAttachment struct {
	ID            uuid.UUID `json:"id" db:"id"`
	EntryID       uuid.UUID `json:"entry_id" db:"entry_id"`
	Name          string    `json:"name" db:"name"`
	FileType      string    `json:"file_type" db:"file_type"`
	MimeType      string    `json:"mime_type" db:"mime_type"`
	FileSize      int       `json:"file_size" db:"file_size"`
	EncryptedData string    `json:"encrypted_data,omitempty" db:"encrypted_data"` // Client-encrypted, omit in list responses
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// VaultEntryHistory represents an audit log entry for vault modifications
type VaultEntryHistory struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	EntryID   uuid.UUID              `json:"entry_id" db:"entry_id"`
	UserID    uuid.UUID              `json:"user_id" db:"user_id"`
	Action    string                 `json:"action" db:"action"`
	Changes   map[string]interface{} `json:"changes,omitempty" db:"changes"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
}

// PasswordGenerationHistory tracks generated passwords
type PasswordGenerationHistory struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	Length           int        `json:"length" db:"length"`
	IncludeUppercase bool       `json:"include_uppercase" db:"include_uppercase"`
	IncludeLowercase bool       `json:"include_lowercase" db:"include_lowercase"`
	IncludeNumbers   bool       `json:"include_numbers" db:"include_numbers"`
	IncludeSymbols   bool       `json:"include_symbols" db:"include_symbols"`
	StrengthScore    int        `json:"strength_score" db:"strength_score"`
	EntropyBits      float64    `json:"entropy_bits" db:"entropy_bits"`
	UsedForEntryID   *uuid.UUID `json:"used_for_entry_id,omitempty" db:"used_for_entry_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}

// ============================================
// Request/Response types for enhanced vault
// ============================================

// CreateFolderRequest is the request body for creating a folder
type CreateFolderRequest struct {
	Name     string     `json:"name" binding:"required,min=1,max=255"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Icon     string     `json:"icon"`
	Color    string     `json:"color"`
}

// UpdateFolderRequest is the request body for updating a folder
type UpdateFolderRequest struct {
	Name     *string    `json:"name,omitempty"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Icon     *string    `json:"icon,omitempty"`
	Color    *string    `json:"color,omitempty"`
	Position *int       `json:"position,omitempty"`
}

// CreateEntryRequest is the request body for creating a vault entry
type CreateEntryRequest struct {
	FolderID   *uuid.UUID           `json:"folder_id,omitempty"`
	Name       string               `json:"name" binding:"required,min=1,max=255"`
	EntryType  string               `json:"entry_type" binding:"required"`
	URL        *string              `json:"url,omitempty"`
	Notes      *string              `json:"notes,omitempty"`
	IsFavorite bool                 `json:"is_favorite"`
	Secrets    []CreateSecretInput  `json:"secrets,omitempty"`
	TagIDs     []uuid.UUID          `json:"tag_ids,omitempty"`
}

// CreateSecretInput is input for creating a secret within an entry
type CreateSecretInput struct {
	SecretType     string     `json:"secret_type" binding:"required"`
	Name           string     `json:"name" binding:"required"`
	EncryptedValue string     `json:"encrypted_value" binding:"required"`
	Username       *string    `json:"username,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	StrengthScore  int        `json:"strength_score"`
}

// UpdateEntryRequest is the request body for updating a vault entry
type UpdateEntryRequest struct {
	FolderID      *uuid.UUID `json:"folder_id,omitempty"`
	Name          *string    `json:"name,omitempty"`
	EntryType     *string    `json:"entry_type,omitempty"`
	URL           *string    `json:"url,omitempty"`
	Notes         *string    `json:"notes,omitempty"`
	IsFavorite    *bool      `json:"is_favorite,omitempty"`
	Position      *int       `json:"position,omitempty"`
	StrengthScore *int       `json:"strength_score,omitempty"`
	TagIDs        []uuid.UUID `json:"tag_ids,omitempty"`
}

// CreateTagRequest is the request body for creating a tag
type CreateTagRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Color    string `json:"color"`
	Category string `json:"category"`
}

// SearchRequest contains search/filter parameters
type SearchRequest struct {
	Query     string      `form:"q"`
	FolderID  *uuid.UUID  `form:"folder_id"`
	TagIDs    []uuid.UUID `form:"tag_ids"`
	EntryType *string     `form:"entry_type"`
	Favorite  *bool       `form:"favorite"`
	Limit     int         `form:"limit"`
	Offset    int         `form:"offset"`
}

// PasswordGenerateRequest is the request for generating a password
type PasswordGenerateRequest struct {
	Length           int    `json:"length" binding:"min=8,max=128"`
	IncludeUppercase bool   `json:"include_uppercase"`
	IncludeLowercase bool   `json:"include_lowercase"`
	IncludeNumbers   bool   `json:"include_numbers"`
	IncludeSymbols   bool   `json:"include_symbols"`
	ExcludeSimilar   bool   `json:"exclude_similar"`   // avoid 0, O, l, 1
	ExcludeAmbiguous bool   `json:"exclude_ambiguous"` // avoid {}[]()\/~,;.<>
	CustomSymbols    string `json:"custom_symbols"`
}

// PasswordGenerateResponse is the response from password generation
type PasswordGenerateResponse struct {
	Password      string  `json:"password"`
	StrengthScore int     `json:"strength_score"`
	EntropyBits   float64 `json:"entropy_bits"`
	TimeToCrack   string  `json:"time_to_crack"`
}

// FoldersResponse is the response containing folder tree
type FoldersResponse struct {
	Folders []*VaultFolder `json:"folders"`
}

// EntriesResponse is the response containing entries list
type EntriesResponse struct {
	Entries    []EnhancedVaultEntry `json:"entries"`
	TotalCount int                  `json:"total_count"`
	HasMore    bool                 `json:"has_more"`
}

// TagsResponse is the response containing tags list
type TagsResponse struct {
	Tags []VaultTag `json:"tags"`
}

// AttachmentMetadata contains attachment info without the encrypted data
type AttachmentMetadata struct {
	ID        uuid.UUID `json:"id"`
	EntryID   uuid.UUID `json:"entry_id"`
	Name      string    `json:"name"`
	FileType  string    `json:"file_type"`
	MimeType  string    `json:"mime_type"`
	FileSize  int       `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateAttachmentRequest is the request for creating an attachment
type CreateAttachmentRequest struct {
	EntryID       uuid.UUID `json:"entry_id" binding:"required"`
	Name          string    `json:"name" binding:"required"`
	FileType      string    `json:"file_type" binding:"required"`
	MimeType      string    `json:"mime_type" binding:"required"`
	FileSize      int       `json:"file_size" binding:"required,min=1"`
	EncryptedData string    `json:"encrypted_data" binding:"required"` // Base64 encoded, client-encrypted
}

// UpdateAttachmentRequest is the request for updating an attachment
type UpdateAttachmentRequest struct {
	Name *string `json:"name,omitempty"`
}

// Team represents a team/organization for sharing
type Team struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Description *string    `json:"description,omitempty" db:"description"`
	OwnerID     uuid.UUID  `json:"owner_id" db:"owner_id"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	Members     []TeamMember `json:"members,omitempty" db:"-"`
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TeamID   uuid.UUID `json:"team_id" db:"team_id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	Role     string    `json:"role" db:"role"` // owner, admin, member, viewer
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
	Username string    `json:"username,omitempty" db:"-"` // Populated from join
	Email    string    `json:"email,omitempty" db:"-"`
}

// SharedFolder represents a folder shared with a team or user
type SharedFolder struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	FolderID        uuid.UUID  `json:"folder_id" db:"folder_id"`
	TeamID          *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	SharedBy        uuid.UUID  `json:"shared_by" db:"shared_by"`
	SharedWith      *uuid.UUID `json:"shared_with,omitempty" db:"shared_with"`
	PermissionLevel string     `json:"permission_level" db:"permission_level"` // view, edit, admin
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	FolderName      string     `json:"folder_name,omitempty" db:"-"`
	SharedByName    string     `json:"shared_by_name,omitempty" db:"-"`
}

// SharedFolderKey stores the encrypted folder key for a user
type SharedFolderKey struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	FolderID           uuid.UUID `json:"folder_id" db:"folder_id"`
	UserID             uuid.UUID `json:"user_id" db:"user_id"`
	EncryptedFolderKey string    `json:"encrypted_folder_key" db:"encrypted_folder_key"`
	KeyVersion         int       `json:"key_version" db:"key_version"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

// TeamInvitation represents a pending invitation to join a team
type TeamInvitation struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	TeamID       uuid.UUID  `json:"team_id" db:"team_id"`
	InvitedBy    uuid.UUID  `json:"invited_by" db:"invited_by"`
	InvitedEmail *string    `json:"invited_email,omitempty" db:"invited_email"`
	InvitedPubkey *string   `json:"invited_pubkey,omitempty" db:"invited_pubkey"`
	Role         string     `json:"role" db:"role"`
	Status       string     `json:"status" db:"status"` // pending, accepted, declined, expired
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
	AcceptedAt   *time.Time `json:"accepted_at,omitempty" db:"accepted_at"`
	TeamName     string     `json:"team_name,omitempty" db:"-"`
}

// CreateTeamRequest is the request for creating a team
type CreateTeamRequest struct {
	Name        string  `json:"name" binding:"required,min=1,max=100"`
	Description *string `json:"description,omitempty"`
}

// UpdateTeamRequest is the request for updating a team
type UpdateTeamRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// InviteToTeamRequest is the request for inviting someone to a team
type InviteToTeamRequest struct {
	Email  *string `json:"email,omitempty"`
	Pubkey *string `json:"pubkey,omitempty"` // Nostr pubkey
	Role   string  `json:"role" binding:"required,oneof=admin member viewer"`
}

// ShareFolderRequest is the request for sharing a folder
type ShareFolderRequest struct {
	FolderID           uuid.UUID  `json:"folder_id" binding:"required"`
	TeamID             *uuid.UUID `json:"team_id,omitempty"`
	UserID             *uuid.UUID `json:"user_id,omitempty"`
	PermissionLevel    string     `json:"permission_level" binding:"required,oneof=view edit admin"`
	EncryptedFolderKey string     `json:"encrypted_folder_key" binding:"required"` // Folder key encrypted with recipient's pubkey
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
}

// AcceptShareRequest is the request for accepting a share (provides the user's encrypted key)
type AcceptShareRequest struct {
	SharedFolderID     uuid.UUID `json:"shared_folder_id" binding:"required"`
	EncryptedFolderKey string    `json:"encrypted_folder_key" binding:"required"`
}