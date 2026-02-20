package auth

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/observability"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

var (
	ErrWebAuthnNotConfigured  = errors.New("WebAuthn not configured")
	ErrCredentialNotFound     = errors.New("credential not found")
	ErrNoCredentialsForUser   = errors.New("user has no registered credentials")
	ErrSessionExpired         = errors.New("WebAuthn session expired")
	ErrSessionNotFound        = errors.New("WebAuthn session not found")
	ErrCredentialIDMismatch   = errors.New("credential ID mismatch")
)

// WebAuthnUser implements the webauthn.User interface
type WebAuthnUser struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Credentials []webauthn.Credential
}

// WebAuthnID returns the user's unique identifier as bytes
func (u *WebAuthnUser) WebAuthnID() []byte {
	return u.ID[:]
}

// WebAuthnName returns the user's identifier (email)
func (u *WebAuthnUser) WebAuthnName() string {
	return u.Email
}

// WebAuthnDisplayName returns a human-readable name
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Email
}

// WebAuthnCredentials returns the user's registered credentials
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// WebAuthnIcon returns an icon URL (deprecated but required by interface)
func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

// WebAuthnCredentialInfo contains credential metadata for API responses
type WebAuthnCredentialInfo struct {
	ID             string     `json:"id"`
	CredentialID   string     `json:"credential_id"`
	Name           string     `json:"name"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	BackupEligible bool       `json:"backup_eligible"`
	BackupState    bool       `json:"backup_state"`
}

// InitWebAuthn initializes the WebAuthn configuration for the service
func (a *AuthService) InitWebAuthn(rpID, rpOrigin, rpDisplayName string) error {
	config := &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpDisplayName,
		RPOrigins:             []string{rpOrigin},
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
			UserVerification:        protocol.VerificationPreferred,
			AuthenticatorAttachment: protocol.Platform,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Minute * 5,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Minute * 5,
			},
		},
	}

	wa, err := webauthn.New(config)
	if err != nil {
		return fmt.Errorf("failed to initialize WebAuthn: %w", err)
	}

	a.webauthn = wa
	return nil
}

// BeginWebAuthnRegistration starts the WebAuthn registration ceremony
func (a *AuthService) BeginWebAuthnRegistration(userID uuid.UUID) (*protocol.CredentialCreation, error) {
	if a.webauthn == nil {
		return nil, ErrWebAuthnNotConfigured
	}

	// Get user info
	user, err := a.getUserForWebAuthn(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get existing credentials to exclude them
	credentials, err := a.getWebAuthnCredentials(userID)
	if err != nil && err != ErrNoCredentialsForUser {
		return nil, fmt.Errorf("failed to get existing credentials: %w", err)
	}
	user.Credentials = credentials

	// Begin registration
	options, session, err := a.webauthn.BeginRegistration(user)
	if err != nil {
		return nil, fmt.Errorf("failed to begin registration: %w", err)
	}

	// Store session data in database
	err = a.storeWebAuthnSession(userID, session, "registration")
	if err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return options, nil
}

// FinishWebAuthnRegistration completes the WebAuthn registration ceremony
func (a *AuthService) FinishWebAuthnRegistration(userID uuid.UUID, credName string, response *protocol.ParsedCredentialCreationData) (*WebAuthnCredentialInfo, error) {
	if a.webauthn == nil {
		return nil, ErrWebAuthnNotConfigured
	}

	// Get user
	user, err := a.getUserForWebAuthn(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get existing credentials
	credentials, err := a.getWebAuthnCredentials(userID)
	if err != nil && err != ErrNoCredentialsForUser {
		return nil, fmt.Errorf("failed to get existing credentials: %w", err)
	}
	user.Credentials = credentials

	// Get session data
	session, err := a.getWebAuthnSession(userID, "registration")
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Finish registration
	credential, err := a.webauthn.CreateCredential(user, *session, response)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// Store credential
	credInfo, err := a.storeWebAuthnCredential(userID, credential, credName)
	if err != nil {
		return nil, fmt.Errorf("failed to store credential: %w", err)
	}

	// Clean up session
	a.deleteWebAuthnSession(userID, "registration")

	observability.Info("WebAuthn credential registered",
		"user_id", userID.String(),
		"credential_name", credName,
	)

	return credInfo, nil
}

// BeginWebAuthnLogin starts the WebAuthn authentication ceremony
func (a *AuthService) BeginWebAuthnLogin(email string) (*protocol.CredentialAssertion, error) {
	if a.webauthn == nil {
		return nil, ErrWebAuthnNotConfigured
	}

	// Get user by email
	var userID uuid.UUID
	err := a.db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Get user with credentials
	user, err := a.getUserForWebAuthn(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	credentials, err := a.getWebAuthnCredentials(userID)
	if err != nil {
		return nil, err
	}
	if len(credentials) == 0 {
		return nil, ErrNoCredentialsForUser
	}
	user.Credentials = credentials

	// Begin login
	options, session, err := a.webauthn.BeginLogin(user)
	if err != nil {
		return nil, fmt.Errorf("failed to begin login: %w", err)
	}

	// Store session
	err = a.storeWebAuthnSession(userID, session, "authentication")
	if err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return options, nil
}

// BeginWebAuthnDiscoverableLogin starts a discoverable credential login (usernameless)
func (a *AuthService) BeginWebAuthnDiscoverableLogin() (*protocol.CredentialAssertion, string, error) {
	if a.webauthn == nil {
		return nil, "", ErrWebAuthnNotConfigured
	}

	// Begin discoverable login (no user specified)
	options, session, err := a.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin discoverable login: %w", err)
	}

	// Store session with a temporary ID
	sessionID := uuid.New().String()
	err = a.storeDiscoverableSession(sessionID, session)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store session: %w", err)
	}

	return options, sessionID, nil
}

// FinishWebAuthnLogin completes the WebAuthn authentication ceremony
func (a *AuthService) FinishWebAuthnLogin(email string, response *protocol.ParsedCredentialAssertionData) (*models.User, string, error) {
	if a.webauthn == nil {
		return nil, "", ErrWebAuthnNotConfigured
	}

	// Get user by email
	var userID uuid.UUID
	err := a.db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Get user with credentials
	user, err := a.getUserForWebAuthn(userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	credentials, err := a.getWebAuthnCredentials(userID)
	if err != nil {
		return nil, "", err
	}
	user.Credentials = credentials

	// Get session
	session, err := a.getWebAuthnSession(userID, "authentication")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get session: %w", err)
	}

	// Finish login
	credential, err := a.webauthn.ValidateLogin(user, *session, response)
	if err != nil {
		return nil, "", fmt.Errorf("failed to validate login: %w", err)
	}

	// Update credential sign count and last used
	err = a.updateCredentialUsage(credential.ID, credential.Authenticator.SignCount)
	if err != nil {
		observability.Warn("failed to update credential usage",
			"credential_id", base64.StdEncoding.EncodeToString(credential.ID),
			"error", err.Error(),
		)
	}

	// Clean up session
	a.deleteWebAuthnSession(userID, "authentication")

	// Get full user model
	fullUser, err := a.getUserByID(userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	fullUser.AuthMethod = "webauthn"

	// Create session
	authResp, err := a.createSession(*fullUser)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	observability.Info("WebAuthn login successful",
		"user_id", userID.String(),
	)

	return &authResp.User, authResp.Token, nil
}

// FinishWebAuthnDiscoverableLogin completes a discoverable credential login
func (a *AuthService) FinishWebAuthnDiscoverableLogin(sessionID string, response *protocol.ParsedCredentialAssertionData) (*models.User, string, error) {
	if a.webauthn == nil {
		return nil, "", ErrWebAuthnNotConfigured
	}

	// Get session
	session, err := a.getDiscoverableSession(sessionID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get session: %w", err)
	}

	// Handler to look up user by credential ID
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		// Find user by credential ID
		var userID uuid.UUID
		err := a.db.QueryRow(`
			SELECT user_id FROM webauthn_credentials
			WHERE credential_id = $1`,
			rawID).Scan(&userID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrCredentialNotFound
			}
			return nil, err
		}

		user, err := a.getUserForWebAuthn(userID)
		if err != nil {
			return nil, err
		}

		credentials, err := a.getWebAuthnCredentials(userID)
		if err != nil {
			return nil, err
		}
		user.Credentials = credentials

		return user, nil
	}

	// Validate discoverable login
	credential, err := a.webauthn.ValidateDiscoverableLogin(handler, *session, response)
	if err != nil {
		return nil, "", fmt.Errorf("failed to validate discoverable login: %w", err)
	}

	// Find user ID from credential
	var userID uuid.UUID
	err = a.db.QueryRow(`
		SELECT user_id FROM webauthn_credentials
		WHERE credential_id = $1`,
		response.RawID).Scan(&userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find user: %w", err)
	}

	// Update credential usage
	err = a.updateCredentialUsage(credential.ID, credential.Authenticator.SignCount)
	if err != nil {
		observability.Warn("failed to update credential usage",
			"error", err.Error(),
		)
	}

	// Clean up session
	a.deleteDiscoverableSession(sessionID)

	// Get full user
	user, err := a.getUserByID(userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	user.AuthMethod = "webauthn"

	// Create session
	authResp, err := a.createSession(*user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	observability.Info("WebAuthn discoverable login successful",
		"user_id", userID.String(),
	)

	return &authResp.User, authResp.Token, nil
}

// ListWebAuthnCredentials returns all credentials for a user
func (a *AuthService) ListWebAuthnCredentials(userID uuid.UUID) ([]WebAuthnCredentialInfo, error) {
	rows, err := a.db.Query(`
		SELECT id, credential_id, credential_name, created_at, last_used_at,
		       flags_backup_eligible, flags_backup_state
		FROM webauthn_credentials
		WHERE user_id = $1
		ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	defer rows.Close()

	var credentials []WebAuthnCredentialInfo
	for rows.Next() {
		var cred WebAuthnCredentialInfo
		var credID []byte
		var lastUsed sql.NullTime

		err := rows.Scan(&cred.ID, &credID, &cred.Name, &cred.CreatedAt, &lastUsed,
			&cred.BackupEligible, &cred.BackupState)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		cred.CredentialID = base64.URLEncoding.EncodeToString(credID)
		if lastUsed.Valid {
			cred.LastUsedAt = &lastUsed.Time
		}

		credentials = append(credentials, cred)
	}

	return credentials, nil
}

// DeleteWebAuthnCredential removes a credential
func (a *AuthService) DeleteWebAuthnCredential(userID uuid.UUID, credentialID string) error {
	credIDBytes, err := base64.URLEncoding.DecodeString(credentialID)
	if err != nil {
		return fmt.Errorf("invalid credential ID: %w", err)
	}

	result, err := a.db.Exec(`
		DELETE FROM webauthn_credentials
		WHERE user_id = $1 AND credential_id = $2`,
		userID, credIDBytes)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrCredentialNotFound
	}

	observability.Info("WebAuthn credential deleted",
		"user_id", userID.String(),
	)

	return nil
}

// UpdateWebAuthnCredentialName updates a credential's display name
func (a *AuthService) UpdateWebAuthnCredentialName(userID uuid.UUID, credentialID, newName string) error {
	credIDBytes, err := base64.URLEncoding.DecodeString(credentialID)
	if err != nil {
		return fmt.Errorf("invalid credential ID: %w", err)
	}

	result, err := a.db.Exec(`
		UPDATE webauthn_credentials
		SET credential_name = $1
		WHERE user_id = $2 AND credential_id = $3`,
		newName, userID, credIDBytes)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrCredentialNotFound
	}

	return nil
}

// Helper methods

func (a *AuthService) getUserForWebAuthn(userID uuid.UUID) (*WebAuthnUser, error) {
	var user WebAuthnUser
	user.ID = userID

	err := a.db.QueryRow(`
		SELECT email FROM users WHERE id = $1`,
		userID).Scan(&user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Get display name from NIP-05, Lightning, or use email
	a.db.QueryRow(`
		SELECT COALESCE(nip05_address, identifier, $2)
		FROM auth_methods
		WHERE user_id = $1
		LIMIT 1`,
		userID, user.Email).Scan(&user.DisplayName)

	return &user, nil
}

func (a *AuthService) getUserByID(userID uuid.UUID) (*models.User, error) {
	var user models.User
	err := a.db.QueryRow(`
		SELECT id, email, created_at, updated_at
		FROM users WHERE id = $1`,
		userID).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (a *AuthService) getWebAuthnCredentials(userID uuid.UUID) ([]webauthn.Credential, error) {
	rows, err := a.db.Query(`
		SELECT credential_id, public_key, sign_count, aaguid, transports,
		       flags_user_present, flags_user_verified, flags_backup_eligible, flags_backup_state
		FROM webauthn_credentials
		WHERE user_id = $1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	defer rows.Close()

	var credentials []webauthn.Credential
	for rows.Next() {
		var cred webauthn.Credential
		var transportsJSON sql.NullString
		var aaguid []byte
		var userPresent, userVerified, backupEligible, backupState bool

		err := rows.Scan(&cred.ID, &cred.PublicKey, &cred.Authenticator.SignCount,
			&aaguid, &transportsJSON,
			&userPresent, &userVerified, &backupEligible, &backupState)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		// Parse transports
		if transportsJSON.Valid && transportsJSON.String != "" {
			var transports []string
			json.Unmarshal([]byte(transportsJSON.String), &transports)
			for _, t := range transports {
				cred.Transport = append(cred.Transport, protocol.AuthenticatorTransport(t))
			}
		}

		// Set AAGUID
		if len(aaguid) == 16 {
			copy(cred.Authenticator.AAGUID[:], aaguid)
		}

		// Set flags
		cred.Flags.UserPresent = userPresent
		cred.Flags.UserVerified = userVerified
		cred.Flags.BackupEligible = backupEligible
		cred.Flags.BackupState = backupState

		credentials = append(credentials, cred)
	}

	if len(credentials) == 0 {
		return nil, ErrNoCredentialsForUser
	}

	return credentials, nil
}

func (a *AuthService) storeWebAuthnCredential(userID uuid.UUID, cred *webauthn.Credential, name string) (*WebAuthnCredentialInfo, error) {
	credID := uuid.New()
	now := time.Now()

	// Serialize transports
	var transports []string
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	transportsJSON, _ := json.Marshal(transports)

	_, err := a.db.Exec(`
		INSERT INTO webauthn_credentials (
			id, user_id, credential_id, public_key, credential_name,
			attestation_type, transports, sign_count, aaguid,
			flags_user_present, flags_user_verified, flags_backup_eligible, flags_backup_state,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		credID, userID, cred.ID, cred.PublicKey, name,
		cred.AttestationType, string(transportsJSON), cred.Authenticator.SignCount, cred.Authenticator.AAGUID[:],
		cred.Flags.UserPresent, cred.Flags.UserVerified, cred.Flags.BackupEligible, cred.Flags.BackupState,
		now)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &WebAuthnCredentialInfo{
		ID:             credID.String(),
		CredentialID:   base64.URLEncoding.EncodeToString(cred.ID),
		Name:           name,
		CreatedAt:      now,
		BackupEligible: cred.Flags.BackupEligible,
		BackupState:    cred.Flags.BackupState,
	}, nil
}

func (a *AuthService) updateCredentialUsage(credentialID []byte, signCount uint32) error {
	_, err := a.db.Exec(`
		UPDATE webauthn_credentials
		SET sign_count = $1, last_used_at = $2
		WHERE credential_id = $3`,
		signCount, time.Now(), credentialID)
	return err
}

func (a *AuthService) storeWebAuthnSession(userID uuid.UUID, session *webauthn.SessionData, ceremonyType string) error {
	// Serialize session data
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(session); err != nil {
		return fmt.Errorf("failed to encode session: %w", err)
	}

	// Delete any existing session for this user and ceremony type
	a.db.Exec(`DELETE FROM webauthn_sessions WHERE user_id = $1 AND ceremony_type = $2`,
		userID, ceremonyType)

	// Store new session
	_, err := a.db.Exec(`
		INSERT INTO webauthn_sessions (id, user_id, session_data, ceremony_type, challenge, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), userID, buf.Bytes(), ceremonyType, session.Challenge, time.Now().Add(5*time.Minute))

	return err
}

func (a *AuthService) getWebAuthnSession(userID uuid.UUID, ceremonyType string) (*webauthn.SessionData, error) {
	var sessionData []byte
	var expiresAt time.Time

	err := a.db.QueryRow(`
		SELECT session_data, expires_at
		FROM webauthn_sessions
		WHERE user_id = $1 AND ceremony_type = $2`,
		userID, ceremonyType).Scan(&sessionData, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if time.Now().After(expiresAt) {
		a.deleteWebAuthnSession(userID, ceremonyType)
		return nil, ErrSessionExpired
	}

	// Decode session
	var session webauthn.SessionData
	dec := gob.NewDecoder(bytes.NewReader(sessionData))
	if err := dec.Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

func (a *AuthService) deleteWebAuthnSession(userID uuid.UUID, ceremonyType string) {
	a.db.Exec(`DELETE FROM webauthn_sessions WHERE user_id = $1 AND ceremony_type = $2`,
		userID, ceremonyType)
}

func (a *AuthService) storeDiscoverableSession(sessionID string, session *webauthn.SessionData) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(session); err != nil {
		return fmt.Errorf("failed to encode session: %w", err)
	}

	_, err := a.db.Exec(`
		INSERT INTO webauthn_sessions (id, session_data, ceremony_type, challenge, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		sessionID, buf.Bytes(), "discoverable", session.Challenge, time.Now().Add(5*time.Minute))

	return err
}

func (a *AuthService) getDiscoverableSession(sessionID string) (*webauthn.SessionData, error) {
	var sessionData []byte
	var expiresAt time.Time

	err := a.db.QueryRow(`
		SELECT session_data, expires_at
		FROM webauthn_sessions
		WHERE id::text = $1 AND ceremony_type = 'discoverable'`,
		sessionID).Scan(&sessionData, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if time.Now().After(expiresAt) {
		a.deleteDiscoverableSession(sessionID)
		return nil, ErrSessionExpired
	}

	var session webauthn.SessionData
	dec := gob.NewDecoder(bytes.NewReader(sessionData))
	if err := dec.Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

func (a *AuthService) deleteDiscoverableSession(sessionID string) {
	a.db.Exec(`DELETE FROM webauthn_sessions WHERE id::text = $1`, sessionID)
}
