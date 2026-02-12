package recovery

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/observability"
	"github.com/google/uuid"
	"golang.org/x/crypto/scrypt"
)

const (
	// Number of recovery codes to generate per user
	NumRecoveryCodes = 8

	// Code format: XXXX-XXXX-XXXX (12 chars + 2 dashes)
	CodeLength = 12

	// Scrypt parameters for hashing recovery codes
	// Using lower N than password hashing since codes are high-entropy
	scryptN = 16384
	scryptR = 8
	scryptP = 1
	keyLen  = 32
	saltLen = 16
)

var (
	ErrInvalidCode     = errors.New("invalid recovery code")
	ErrCodeAlreadyUsed = errors.New("recovery code already used")
	ErrNoCodesLeft     = errors.New("no unused recovery codes remaining")
	ErrUserNotFound    = errors.New("user not found")
)

// Service handles recovery code operations
type Service struct {
	db *sql.DB
}

// NewService creates a new recovery service
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GeneratedCodes holds plaintext codes shown to user once
type GeneratedCodes struct {
	Codes     []string  `json:"codes"`
	CreatedAt time.Time `json:"created_at"`
	Warning   string    `json:"warning"`
}

// GenerateCodes creates new recovery codes for a user
// Returns plaintext codes (shown once) and stores hashed versions
func (s *Service) GenerateCodes(tx *sql.Tx, userID uuid.UUID) (*GeneratedCodes, error) {
	// Delete any existing codes for this user
	_, err := tx.Exec("DELETE FROM recovery_codes WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to clear existing codes: %w", err)
	}

	codes := make([]string, NumRecoveryCodes)
	now := time.Now()

	for i := 0; i < NumRecoveryCodes; i++ {
		// Generate random code
		code, err := generateCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code: %w", err)
		}
		codes[i] = code

		// Generate salt
		salt := make([]byte, saltLen)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}

		// Hash the code
		codeHash, err := hashCode(code, salt)
		if err != nil {
			return nil, fmt.Errorf("failed to hash code: %w", err)
		}

		// Store hashed code
		codeID := uuid.New()
		_, err = tx.Exec(`
			INSERT INTO recovery_codes (id, user_id, code_hash, salt, used, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			codeID, userID, codeHash, salt, false, now)
		if err != nil {
			return nil, fmt.Errorf("failed to store code: %w", err)
		}
	}

	observability.Info("recovery codes generated",
		"user_id", userID.String(),
		"count", NumRecoveryCodes,
	)

	return &GeneratedCodes{
		Codes:     codes,
		CreatedAt: now,
		Warning:   "Store these codes safely. Each code can only be used once. You will not be able to see them again.",
	}, nil
}

// ValidateCode checks if a recovery code is valid for a user
// Does NOT consume the code - use ConsumeCode for that
func (s *Service) ValidateCode(userID uuid.UUID, code string) (bool, error) {
	code = normalizeCode(code)

	// Get all unused codes for this user
	rows, err := s.db.Query(`
		SELECT id, code_hash, salt
		FROM recovery_codes
		WHERE user_id = $1 AND used = false`,
		userID)
	if err != nil {
		return false, fmt.Errorf("failed to query codes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var codeID uuid.UUID
		var storedHash, salt []byte

		if err := rows.Scan(&codeID, &storedHash, &salt); err != nil {
			continue
		}

		// Hash the provided code with the stored salt
		computedHash, err := hashCode(code, salt)
		if err != nil {
			continue
		}

		// Compare hashes (constant time comparison via sha256)
		if sha256.Sum256(computedHash) == sha256.Sum256(storedHash) {
			return true, nil
		}
	}

	return false, nil
}

// ConsumeCode validates and marks a recovery code as used
// Returns the code ID if successful
func (s *Service) ConsumeCode(userID uuid.UUID, code string) (uuid.UUID, error) {
	code = normalizeCode(code)

	// Get all unused codes for this user
	rows, err := s.db.Query(`
		SELECT id, code_hash, salt
		FROM recovery_codes
		WHERE user_id = $1 AND used = false
		FOR UPDATE`,
		userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to query codes: %w", err)
	}
	defer rows.Close()

	var matchedCodeID uuid.UUID
	found := false

	for rows.Next() {
		var codeID uuid.UUID
		var storedHash, salt []byte

		if err := rows.Scan(&codeID, &storedHash, &salt); err != nil {
			continue
		}

		// Hash the provided code with the stored salt
		computedHash, err := hashCode(code, salt)
		if err != nil {
			continue
		}

		// Compare hashes
		if sha256.Sum256(computedHash) == sha256.Sum256(storedHash) {
			matchedCodeID = codeID
			found = true
			break
		}
	}
	rows.Close()

	if !found {
		observability.Warn("invalid recovery code attempt",
			"user_id", userID.String(),
		)
		return uuid.Nil, ErrInvalidCode
	}

	// Mark the code as used
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE recovery_codes
		SET used = true, used_at = $1
		WHERE id = $2 AND used = false`,
		now, matchedCodeID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to mark code as used: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return uuid.Nil, ErrCodeAlreadyUsed
	}

	observability.Info("recovery code consumed",
		"user_id", userID.String(),
		"code_id", matchedCodeID.String(),
	)

	return matchedCodeID, nil
}

// GetRemainingCount returns the number of unused recovery codes for a user
func (s *Service) GetRemainingCount(userID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM recovery_codes
		WHERE user_id = $1 AND used = false`,
		userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count codes: %w", err)
	}
	return count, nil
}

// GetCodeStatus returns status of all recovery codes for a user
func (s *Service) GetCodeStatus(userID uuid.UUID) ([]models.RecoveryCode, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, used, created_at, used_at
		FROM recovery_codes
		WHERE user_id = $1
		ORDER BY created_at`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query codes: %w", err)
	}
	defer rows.Close()

	var codes []models.RecoveryCode
	for rows.Next() {
		var code models.RecoveryCode
		if err := rows.Scan(&code.ID, &code.UserID, &code.Used, &code.CreatedAt, &code.UsedAt); err != nil {
			continue
		}
		codes = append(codes, code)
	}

	return codes, nil
}

// RegenerateCodes deletes existing codes and generates new ones
// Requires a valid session or other proof of account ownership
func (s *Service) RegenerateCodes(userID uuid.UUID) (*GeneratedCodes, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	codes, err := s.GenerateCodes(tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	observability.Info("recovery codes regenerated",
		"user_id", userID.String(),
	)

	return codes, nil
}

// generateCode creates a random recovery code in format XXXX-XXXX-XXXX
func generateCode() (string, error) {
	// Generate 8 random bytes (gives us more than enough entropy)
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Encode to base32 (no padding, uppercase) and take first 12 chars
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	encoded = strings.ToUpper(encoded)[:CodeLength]

	// Format as XXXX-XXXX-XXXX
	return fmt.Sprintf("%s-%s-%s", encoded[0:4], encoded[4:8], encoded[8:12]), nil
}

// hashCode hashes a recovery code using scrypt
func hashCode(code string, salt []byte) ([]byte, error) {
	// Normalize the code (remove dashes, uppercase)
	normalized := normalizeCode(code)

	hash, err := scrypt.Key([]byte(normalized), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, err
	}

	return hash, nil
}

// normalizeCode removes dashes and converts to uppercase
func normalizeCode(code string) string {
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return strings.ToUpper(code)
}
