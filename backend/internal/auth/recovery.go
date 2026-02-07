package auth

import (
	"database/sql"
	"fmt"
	"time"
	
	"github.com/coldforge/vault/internal/crypto"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// GenerateRecoveryCodes generates a set of recovery codes for a user
func (a *AuthService) GenerateRecoveryCodes(userID uuid.UUID, count int) ([]string, error) {
	if count <= 0 || count > 10 {
		return nil, fmt.Errorf("invalid recovery code count: must be between 1 and 10")
	}
	
	tx, err := a.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Delete existing recovery codes
	_, err = tx.Exec("DELETE FROM recovery_codes WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete existing recovery codes: %w", err)
	}
	
	var plainCodes []string
	now := time.Now()
	
	for i := 0; i < count; i++ {
		// Generate a random recovery code
		codeBytes, err := crypto.GenerateRandomBytes(16)
		if err != nil {
			return nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		
		// Format as readable string (8-4-4 format)
		plainCode := fmt.Sprintf("%x-%x-%x", 
			codeBytes[0:4], 
			codeBytes[4:8], 
			codeBytes[8:12])
		
		plainCodes = append(plainCodes, plainCode)
		
		// Generate salt and hash the code
		salt, err := crypto.GenerateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		
		codeHash, err := crypto.HashPassword(plainCode, salt)
		if err != nil {
			return nil, fmt.Errorf("failed to hash recovery code: %w", err)
		}
		
		// Store hashed code
		codeID := uuid.New()
		_, err = tx.Exec(`
			INSERT INTO recovery_codes (id, user_id, code_hash, salt, used, created_at) 
			VALUES ($1, $2, $3, $4, $5, $6)`,
			codeID, userID, codeHash, salt, false, now)
		
		if err != nil {
			return nil, fmt.Errorf("failed to store recovery code: %w", err)
		}
	}
	
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return plainCodes, nil
}

// ValidateRecoveryCode validates and marks a recovery code as used
func (a *AuthService) ValidateRecoveryCode(userID uuid.UUID, code string) (bool, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Get all unused recovery codes for the user
	rows, err := tx.Query(`
		SELECT id, code_hash, salt 
		FROM recovery_codes 
		WHERE user_id = $1 AND used = false`,
		userID)
	if err != nil {
		return false, fmt.Errorf("failed to query recovery codes: %w", err)
	}
	defer rows.Close()
	
	// Check each code
	for rows.Next() {
		var codeID uuid.UUID
		var codeHash []byte
		var salt []byte
		
		if err := rows.Scan(&codeID, &codeHash, &salt); err != nil {
			continue
		}
		
		// Verify the provided code against this hash
		if crypto.VerifyPassword(code, salt, codeHash) {
			// Mark as used
			now := time.Now()
			_, err = tx.Exec(`
				UPDATE recovery_codes 
				SET used = true, used_at = $1 
				WHERE id = $2`,
				now, codeID)
			
			if err != nil {
				return false, fmt.Errorf("failed to mark recovery code as used: %w", err)
			}
			
			if err := tx.Commit(); err != nil {
				return false, fmt.Errorf("failed to commit transaction: %w", err)
			}
			
			return true, nil
		}
	}
	
	return false, nil
}

// InitiatePasswordReset starts the password reset process
func (a *AuthService) InitiatePasswordReset(email string) error {
	// Verify user exists
	var userID uuid.UUID
	err := a.db.QueryRow(`
		SELECT u.id 
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.identifier = $1 AND am.type = 'email'`,
		email).Scan(&userID)
	
	if err != nil {
		if err == sql.ErrNoRows {
			// Don't reveal whether user exists for security
			return nil
		}
		return fmt.Errorf("database error: %w", err)
	}
	
	// Generate reset token
	resetToken, err := crypto.GenerateChallenge()
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}
	
	// In a real implementation, this would:
	// 1. Store the reset token with expiration
	// 2. Send email to user with reset link
	// 3. Log the reset attempt for security
	
	// For now, we'll just log it
	fmt.Printf("Password reset requested for user %s, token: %s\n", email, resetToken)
	
	// TODO: Implement email sending
	// TODO: Store reset token in database with expiration
	// TODO: Create audit log entry
	
	return nil
}

// CompletePasswordReset completes the password reset process
func (a *AuthService) CompletePasswordReset(resetToken string, newPassword string) error {
	// In a real implementation, this would:
	// 1. Validate the reset token
	// 2. Ensure token hasn't expired
	// 3. Update the user's password
	// 4. Invalidate all existing sessions
	// 5. Generate new recovery codes
	// 6. Log the password change
	
	// TODO: Implement full password reset flow
	return fmt.Errorf("password reset not fully implemented yet")
}

// RecoveryCodeLogin allows login with a recovery code
func (a *AuthService) RecoveryCodeLogin(email string, recoveryCode string) (*models.AuthResponse, error) {
	// Get user
	var user models.User
	err := a.db.QueryRow(`
		SELECT u.id, u.email, u.created_at, u.updated_at
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.identifier = $1 AND am.type = 'email'`,
		email).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Validate recovery code
	valid, err := a.ValidateRecoveryCode(user.ID, recoveryCode)
	if err != nil {
		return nil, fmt.Errorf("failed to validate recovery code: %w", err)
	}
	
	if !valid {
		return nil, ErrInvalidCredentials
	}
	
	// Create session
	return a.createSession(user)
}

// GetRecoveryCodeCount returns the number of unused recovery codes for a user
func (a *AuthService) GetRecoveryCodeCount(userID uuid.UUID) (int, error) {
	var count int
	err := a.db.QueryRow(`
		SELECT COUNT(*) 
		FROM recovery_codes 
		WHERE user_id = $1 AND used = false`,
		userID).Scan(&count)
	
	if err != nil {
		return 0, fmt.Errorf("failed to count recovery codes: %w", err)
	}
	
	return count, nil
}

// RevokeAllSessions revokes all sessions for a user (for security)
func (a *AuthService) RevokeAllSessions(userID uuid.UUID) error {
	_, err := a.db.Exec("DELETE FROM sessions WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to revoke sessions: %w", err)
	}
	return nil
}

// DeviceRegistration represents a trusted device for recovery
type DeviceRegistration struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	DeviceName  string    `json:"device_name"`
	DeviceID    string    `json:"device_id"`
	PublicKey   []byte    `json:"public_key"`
	Trusted     bool      `json:"trusted"`
	LastSeen    time.Time `json:"last_seen"`
	CreatedAt   time.Time `json:"created_at"`
}

// RegisterTrustedDevice registers a device for recovery purposes
func (a *AuthService) RegisterTrustedDevice(userID uuid.UUID, deviceName string, deviceID string, publicKey []byte) error {
	deviceRegID := uuid.New()
	now := time.Now()
	
	_, err := a.db.Exec(`
		INSERT INTO trusted_devices (id, user_id, device_name, device_id, public_key, trusted, last_seen, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		deviceRegID, userID, deviceName, deviceID, publicKey, true, now, now)
	
	if err != nil {
		return fmt.Errorf("failed to register trusted device: %w", err)
	}
	
	return nil
}

// GetTrustedDevices returns all trusted devices for a user
func (a *AuthService) GetTrustedDevices(userID uuid.UUID) ([]DeviceRegistration, error) {
	rows, err := a.db.Query(`
		SELECT id, user_id, device_name, device_id, public_key, trusted, last_seen, created_at
		FROM trusted_devices 
		WHERE user_id = $1 AND trusted = true 
		ORDER BY last_seen DESC`,
		userID)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query trusted devices: %w", err)
	}
	defer rows.Close()
	
	var devices []DeviceRegistration
	for rows.Next() {
		var device DeviceRegistration
		err := rows.Scan(
			&device.ID,
			&device.UserID,
			&device.DeviceName,
			&device.DeviceID,
			&device.PublicKey,
			&device.Trusted,
			&device.LastSeen,
			&device.CreatedAt,
		)
		if err != nil {
			continue
		}
		devices = append(devices, device)
	}
	
	return devices, nil
}