package models

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
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
	Method     string  `json:"method" binding:"required"` // "email" or "nostr"
	Email      *string `json:"email,omitempty"`
	Password   *string `json:"password,omitempty"`
	NostrPubkey *string `json:"nostr_pubkey,omitempty"`
	Signature  *string `json:"signature,omitempty"`
	Challenge  *string `json:"challenge,omitempty"`
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