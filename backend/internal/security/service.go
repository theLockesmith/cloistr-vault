package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/google/uuid"
)

// SecurityService provides security analysis for the vault
type SecurityService struct {
	db *database.DB
}

// NewSecurityService creates a new security service
func NewSecurityService(db *database.DB) *SecurityService {
	return &SecurityService{db: db}
}

// DB returns the database connection for handlers
func (s *SecurityService) DB() *database.DB {
	return s.db
}

// VaultSecurityScore represents overall vault security health
type VaultSecurityScore struct {
	OverallScore       int       `json:"overall_score"`       // 0-100
	TotalEntries       int       `json:"total_entries"`
	EntriesWithSecrets int       `json:"entries_with_secrets"`
	WeakPasswords      int       `json:"weak_passwords"`
	ReusedPasswords    int       `json:"reused_passwords"`
	BreachedPasswords  int       `json:"breached_passwords"`
	ExpiringSecrets    int       `json:"expiring_secrets"`    // within 30 days
	ExpiredSecrets     int       `json:"expired_secrets"`
	MissingMFA         int       `json:"missing_mfa"`         // entries without 2FA noted
	LastAnalyzed       time.Time `json:"last_analyzed"`
	Recommendations    []string  `json:"recommendations"`
}

// AnalyzeVaultSecurity performs comprehensive security analysis
func (s *SecurityService) AnalyzeVaultSecurity(userID uuid.UUID) (*VaultSecurityScore, error) {
	score := &VaultSecurityScore{
		LastAnalyzed:    time.Now(),
		Recommendations: []string{},
	}

	// Count total entries
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM vault_entries WHERE user_id = $1
	`, userID).Scan(&score.TotalEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to count entries: %w", err)
	}

	// Count entries with secrets
	err = s.db.QueryRow(`
		SELECT COUNT(DISTINCT entry_id) FROM vault_secrets vs
		JOIN vault_entries ve ON vs.entry_id = ve.id
		WHERE ve.user_id = $1
	`, userID).Scan(&score.EntriesWithSecrets)
	if err != nil {
		return nil, fmt.Errorf("failed to count entries with secrets: %w", err)
	}

	// Count weak passwords (strength_score < 2)
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM vault_secrets vs
		JOIN vault_entries ve ON vs.entry_id = ve.id
		WHERE ve.user_id = $1 AND vs.secret_type = 'password' AND vs.strength_score < 2
	`, userID).Scan(&score.WeakPasswords)
	if err != nil {
		return nil, fmt.Errorf("failed to count weak passwords: %w", err)
	}

	// Count expired and expiring secrets
	now := time.Now()
	thirtyDays := now.AddDate(0, 0, 30)

	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM vault_secrets vs
		JOIN vault_entries ve ON vs.entry_id = ve.id
		WHERE ve.user_id = $1 AND vs.expires_at IS NOT NULL AND vs.expires_at < $2
	`, userID, now).Scan(&score.ExpiredSecrets)
	if err != nil {
		return nil, fmt.Errorf("failed to count expired secrets: %w", err)
	}

	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM vault_secrets vs
		JOIN vault_entries ve ON vs.entry_id = ve.id
		WHERE ve.user_id = $1 AND vs.expires_at IS NOT NULL
		AND vs.expires_at >= $2 AND vs.expires_at < $3
	`, userID, now, thirtyDays).Scan(&score.ExpiringSecrets)
	if err != nil {
		return nil, fmt.Errorf("failed to count expiring secrets: %w", err)
	}

	// Get reused password count from entry flags
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM vault_entries
		WHERE user_id = $1 AND has_reused_password = true
	`, userID).Scan(&score.ReusedPasswords)
	if err != nil {
		return nil, fmt.Errorf("failed to count reused passwords: %w", err)
	}

	// Get breached password count from entry flags
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM vault_entries
		WHERE user_id = $1 AND has_breach = true
	`, userID).Scan(&score.BreachedPasswords)
	if err != nil {
		return nil, fmt.Errorf("failed to count breached passwords: %w", err)
	}

	// Calculate overall score (0-100)
	score.OverallScore = s.calculateOverallScore(score)

	// Generate recommendations
	score.Recommendations = s.generateRecommendations(score)

	return score, nil
}

// calculateOverallScore computes 0-100 security score
func (s *SecurityService) calculateOverallScore(score *VaultSecurityScore) int {
	if score.TotalEntries == 0 {
		return 100 // No entries = technically secure
	}

	baseScore := 100

	// Deduct for weak passwords (up to 30 points)
	if score.EntriesWithSecrets > 0 {
		weakPct := float64(score.WeakPasswords) / float64(score.EntriesWithSecrets)
		baseScore -= int(weakPct * 30)
	}

	// Deduct for reused passwords (up to 25 points)
	if score.EntriesWithSecrets > 0 {
		reusedPct := float64(score.ReusedPasswords) / float64(score.EntriesWithSecrets)
		baseScore -= int(reusedPct * 25)
	}

	// Deduct for breached passwords (up to 30 points)
	if score.EntriesWithSecrets > 0 {
		breachedPct := float64(score.BreachedPasswords) / float64(score.EntriesWithSecrets)
		baseScore -= int(breachedPct * 30)
	}

	// Deduct for expired secrets (up to 10 points)
	if score.EntriesWithSecrets > 0 {
		expiredPct := float64(score.ExpiredSecrets) / float64(score.EntriesWithSecrets)
		baseScore -= int(expiredPct * 10)
	}

	// Deduct for expiring secrets (up to 5 points)
	if score.EntriesWithSecrets > 0 {
		expiringPct := float64(score.ExpiringSecrets) / float64(score.EntriesWithSecrets)
		baseScore -= int(expiringPct * 5)
	}

	if baseScore < 0 {
		baseScore = 0
	}

	return baseScore
}

// generateRecommendations creates actionable security advice
func (s *SecurityService) generateRecommendations(score *VaultSecurityScore) []string {
	recs := []string{}

	if score.BreachedPasswords > 0 {
		recs = append(recs, fmt.Sprintf(
			"URGENT: %d password(s) found in data breaches - change immediately",
			score.BreachedPasswords))
	}

	if score.WeakPasswords > 0 {
		recs = append(recs, fmt.Sprintf(
			"Strengthen %d weak password(s) using the password generator",
			score.WeakPasswords))
	}

	if score.ReusedPasswords > 0 {
		recs = append(recs, fmt.Sprintf(
			"Replace %d reused password(s) with unique passwords",
			score.ReusedPasswords))
	}

	if score.ExpiredSecrets > 0 {
		recs = append(recs, fmt.Sprintf(
			"Update %d expired credential(s)",
			score.ExpiredSecrets))
	}

	if score.ExpiringSecrets > 0 {
		recs = append(recs, fmt.Sprintf(
			"Plan to rotate %d credential(s) expiring soon",
			score.ExpiringSecrets))
	}

	if len(recs) == 0 {
		recs = append(recs, "Your vault security is excellent!")
	}

	return recs
}

// PasswordHashInfo stores hashed passwords for reuse detection
type PasswordHashInfo struct {
	Hash      string
	EntryID   uuid.UUID
	SecretID  uuid.UUID
	EntryName string
}

// DetectReusedPasswords identifies passwords used multiple times
func (s *SecurityService) DetectReusedPasswords(userID uuid.UUID, passwords map[uuid.UUID]string) (map[string][]PasswordHashInfo, error) {
	// Group by hash to find reused passwords
	hashGroups := make(map[string][]PasswordHashInfo)

	for secretID, password := range passwords {
		hash := hashPassword(password)

		// Get entry info for this secret
		var entryID uuid.UUID
		var entryName string
		err := s.db.QueryRow(`
			SELECT ve.id, ve.name FROM vault_entries ve
			JOIN vault_secrets vs ON ve.id = vs.entry_id
			WHERE vs.id = $1 AND ve.user_id = $2
		`, secretID, userID).Scan(&entryID, &entryName)

		if err != nil {
			continue // Skip secrets we can't find
		}

		info := PasswordHashInfo{
			Hash:      hash,
			EntryID:   entryID,
			SecretID:  secretID,
			EntryName: entryName,
		}

		hashGroups[hash] = append(hashGroups[hash], info)
	}

	// Filter to only groups with 2+ entries (actual reuse)
	reused := make(map[string][]PasswordHashInfo)
	for hash, infos := range hashGroups {
		if len(infos) > 1 {
			reused[hash] = infos
		}
	}

	return reused, nil
}

// UpdateEntrySecurityFlags updates security flags on entries
func (s *SecurityService) UpdateEntrySecurityFlags(entryID uuid.UUID, hasWeak, hasReused, hasBreach bool) error {
	_, err := s.db.Exec(`
		UPDATE vault_entries
		SET has_weak_password = $1, has_reused_password = $2, has_breach = $3
		WHERE id = $4
	`, hasWeak, hasReused, hasBreach, entryID)
	return err
}

// RecordBreachCheck stores breach check timestamp
func (s *SecurityService) RecordBreachCheck(entryID uuid.UUID) error {
	_, err := s.db.Exec(`
		UPDATE vault_entries SET last_breach_check = $1 WHERE id = $2
	`, time.Now(), entryID)
	return err
}

// GetEntriesNeedingBreachCheck returns entries not checked recently
func (s *SecurityService) GetEntriesNeedingBreachCheck(userID uuid.UUID, olderThan time.Duration) ([]uuid.UUID, error) {
	threshold := time.Now().Add(-olderThan)

	rows, err := s.db.Query(`
		SELECT id FROM vault_entries
		WHERE user_id = $1
		AND (last_breach_check IS NULL OR last_breach_check < $2)
	`, userID, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// hashPassword creates a consistent hash for password comparison
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}
