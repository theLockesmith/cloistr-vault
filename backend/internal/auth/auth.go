package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/crypto"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrInvalidAuthMethod = errors.New("invalid authentication method")
	ErrChallengeExpired  = errors.New("challenge expired")
	ErrInvalidChallenge  = errors.New("invalid challenge")
)

type AuthService struct {
	db *sql.DB
}

type Challenge struct {
	ID        string                 `json:"id"`
	Value     string                 `json:"value"`
	UserID    uuid.UUID             `json:"user_id,omitempty"`
	ExpiresAt time.Time             `json:"expires_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

var challengeStore = make(map[string]Challenge) // In production, use Redis or database

func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db}
}

// RegisterUser creates a new user account
func (a *AuthService) RegisterUser(req *models.RegisterRequest) (*models.User, error) {
	switch req.Method {
	case "email":
		return a.registerEmailUser(req)
	case "nostr":
		return a.registerNostrUser(req)
	default:
		return nil, ErrInvalidAuthMethod
	}
}

// LoginUser authenticates a user and returns a session
func (a *AuthService) LoginUser(req *models.LoginRequest) (*models.AuthResponse, error) {
	switch req.Method {
	case "email":
		return a.loginEmailUser(req)
	case "nostr":
		return a.loginNostrUser(req)
	default:
		return nil, ErrInvalidAuthMethod
	}
}

// registerEmailUser handles email/password registration
func (a *AuthService) registerEmailUser(req *models.RegisterRequest) (*models.User, error) {
	if req.Email == nil || req.Password == nil {
		return nil, errors.New("email and password are required")
	}
	
	// Check if user already exists
	var exists bool
	err := a.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users u JOIN auth_methods am ON u.id = am.user_id WHERE am.identifier = $1 AND am.type = 'email')", *req.Email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if exists {
		return nil, ErrUserExists
	}
	
	// Start transaction
	tx, err := a.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Create user
	userID := uuid.New()
	now := time.Now()
	
	_, err = tx.Exec("INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		userID, *req.Email, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	// Generate salt and hash password
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	
	passwordHash, err := crypto.HashPassword(*req.Password, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	
	// Create auth method
	authMethodID := uuid.New()
	_, err = tx.Exec("INSERT INTO auth_methods (id, user_id, type, identifier, salt, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		authMethodID, userID, "email", *req.Email, salt, passwordHash, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth method: %w", err)
	}
	
	// Create initial vault with provided data
	err = a.createInitialVault(tx, userID, req.VaultData)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial vault: %w", err)
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return &models.User{
		ID:        userID,
		Email:     *req.Email,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// registerNostrUser handles Nostr keypair registration
func (a *AuthService) registerNostrUser(req *models.RegisterRequest) (*models.User, error) {
	if req.NostrPubkey == nil {
		return nil, errors.New("nostr public key is required")
	}
	
	// Validate Nostr public key format
	_, err := crypto.NostrPublicKeyFromHex(*req.NostrPubkey)
	if err != nil {
		return nil, fmt.Errorf("invalid nostr public key: %w", err)
	}
	
	// Check if user already exists
	var exists bool
	err = a.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users u JOIN auth_methods am ON u.id = am.user_id WHERE am.nostr_pubkey = $1 AND am.type = 'nostr')", *req.NostrPubkey).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if exists {
		return nil, ErrUserExists
	}
	
	// Start transaction
	tx, err := a.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Create user
	userID := uuid.New()
	now := time.Now()
	email := fmt.Sprintf("%s@nostr.local", (*req.NostrPubkey)[:16]) // Pseudo email for compatibility
	
	_, err = tx.Exec("INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		userID, email, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	// Create auth method
	authMethodID := uuid.New()
	_, err = tx.Exec("INSERT INTO auth_methods (id, user_id, type, identifier, nostr_pubkey, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		authMethodID, userID, "nostr", *req.NostrPubkey, *req.NostrPubkey, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth method: %w", err)
	}
	
	// Create initial vault
	err = a.createInitialVault(tx, userID, req.VaultData)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial vault: %w", err)
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return &models.User{
		ID:        userID,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// loginEmailUser handles email/password login
func (a *AuthService) loginEmailUser(req *models.LoginRequest) (*models.AuthResponse, error) {
	if req.Email == nil || req.Password == nil {
		return nil, errors.New("email and password are required")
	}
	
	// Get user and auth method
	var user models.User
	var authMethod models.AuthMethod
	
	query := `
		SELECT u.id, u.email, u.created_at, u.updated_at,
		       am.salt, am.password_hash
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.identifier = $1 AND am.type = 'email'
	`
	
	err := a.db.QueryRow(query, *req.Email).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt,
		&authMethod.Salt, &authMethod.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Verify password
	if !crypto.VerifyPassword(*req.Password, authMethod.Salt, authMethod.PasswordHash) {
		return nil, ErrInvalidCredentials
	}
	
	// Create session
	return a.createSession(user)
}

// loginNostrUser handles Nostr keypair login
func (a *AuthService) loginNostrUser(req *models.LoginRequest) (*models.AuthResponse, error) {
	if req.NostrPubkey == nil || req.Signature == nil || req.Challenge == nil {
		return nil, errors.New("nostr public key, signature, and challenge are required")
	}
	
	// Verify challenge
	challenge, exists := challengeStore[*req.Challenge]
	if !exists {
		return nil, ErrInvalidChallenge
	}
	
	if time.Now().After(challenge.ExpiresAt) {
		delete(challengeStore, *req.Challenge)
		return nil, ErrChallengeExpired
	}
	
	// Verify signature
	if !crypto.VerifyNostrSignature(*req.Challenge, *req.Signature, *req.NostrPubkey) {
		return nil, ErrInvalidCredentials
	}
	
	// Clean up challenge
	delete(challengeStore, *req.Challenge)
	
	// Get user
	var user models.User
	query := `
		SELECT u.id, u.email, u.created_at, u.updated_at
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.nostr_pubkey = $1 AND am.type = 'nostr'
	`
	
	err := a.db.QueryRow(query, *req.NostrPubkey).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Verify challenge belongs to this user
	if challenge.UserID != user.ID {
		return nil, ErrInvalidChallenge
	}
	
	// Create session
	return a.createSession(user)
}

// GenerateNostrChallenge creates a challenge for Nostr authentication
func (a *AuthService) GenerateNostrChallenge(publicKeyHex string) (*Challenge, error) {
	// Verify public key format
	_, err := crypto.NostrPublicKeyFromHex(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	
	// Get user by public key
	var userID uuid.UUID
	query := `
		SELECT u.id
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.nostr_pubkey = $1 AND am.type = 'nostr'
	`
	
	err = a.db.QueryRow(query, publicKeyHex).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Generate challenge
	challengeValue, err := crypto.GenerateChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}
	
	challenge := Challenge{
		Value:     challengeValue,
		UserID:    userID,
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5 minute expiry
	}
	
	// Store challenge
	challengeStore[challengeValue] = challenge
	
	return &challenge, nil
}

// createSession creates a new session for the user
func (a *AuthService) createSession(user models.User) (*models.AuthResponse, error) {
	sessionID := uuid.New()
	token := uuid.New().String() // In production, use proper JWT
	expiresAt := time.Now().Add(24 * time.Hour)
	now := time.Now()
	
	// Store session in database
	_, err := a.db.Exec("INSERT INTO sessions (id, user_id, token, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)",
		sessionID, user.ID, token, expiresAt, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	return &models.AuthResponse{
		Token:     token,
		User:      user,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateSession validates a session token
func (a *AuthService) ValidateSession(token string) (*models.User, error) {
	var user models.User
	var expiresAt time.Time
	
	query := `
		SELECT u.id, u.email, u.created_at, u.updated_at, s.expires_at
		FROM users u 
		JOIN sessions s ON u.id = s.user_id 
		WHERE s.token = $1
	`
	
	err := a.db.QueryRow(query, token).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Check if session is expired
	if time.Now().After(expiresAt) {
		// Clean up expired session
		a.db.Exec("DELETE FROM sessions WHERE token = $1", token)
		return nil, ErrInvalidCredentials
	}
	
	return &user, nil
}

// RevokeSession revokes a session token
func (a *AuthService) RevokeSession(token string) error {
	_, err := a.db.Exec("DELETE FROM sessions WHERE token = $1", token)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

// createInitialVault creates the initial encrypted vault for a new user
func (a *AuthService) createInitialVault(tx *sql.Tx, userID uuid.UUID, encryptedData []byte) error {
	vaultID := uuid.New()
	now := time.Now()
	
	// For initial vault, we assume the client has already encrypted the data
	// In a real implementation, you might want additional validation
	
	_, err := tx.Exec(`
		INSERT INTO vaults (id, user_id, encrypted_data, encryption_salt, encryption_nonce, version, last_modified, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		vaultID, userID, encryptedData, []byte{}, []byte{}, 1, now, now)
	
	if err != nil {
		return fmt.Errorf("failed to create vault: %w", err)
	}
	
	return nil
}