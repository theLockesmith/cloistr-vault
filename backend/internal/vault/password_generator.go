package vault

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// PasswordService handles password generation and history
type PasswordService struct {
	db *database.DB
}

// NewPasswordService creates a new password service
func NewPasswordService(db *database.DB) *PasswordService {
	return &PasswordService{db: db}
}

// Character sets for password generation
const (
	uppercase    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowercase    = "abcdefghijklmnopqrstuvwxyz"
	numbers      = "0123456789"
	symbols      = "!@#$%^&*()-_=+[]{}|;:,.<>?"
	similar      = "il1Lo0O"
	ambiguous    = "{}[]()/\\~,;.<>"
)

// GeneratePassword generates a secure password based on requirements
func (s *PasswordService) GeneratePassword(req *models.PasswordGenerateRequest) (*models.PasswordGenerateResponse, error) {
	// Validate length
	length := req.Length
	if length < 8 {
		length = 8
	}
	if length > 128 {
		length = 128
	}

	// Build character set
	var charset string

	if req.IncludeUppercase {
		charset += uppercase
	}
	if req.IncludeLowercase {
		charset += lowercase
	}
	if req.IncludeNumbers {
		charset += numbers
	}
	if req.IncludeSymbols {
		if req.CustomSymbols != "" {
			charset += req.CustomSymbols
		} else {
			charset += symbols
		}
	}

	// Default to alphanumeric if nothing selected
	if charset == "" {
		charset = uppercase + lowercase + numbers
	}

	// Remove similar characters if requested
	if req.ExcludeSimilar {
		charset = removeChars(charset, similar)
	}

	// Remove ambiguous characters if requested
	if req.ExcludeAmbiguous {
		charset = removeChars(charset, ambiguous)
	}

	// Generate password
	password, err := generateSecurePassword(charset, length)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	// Ensure password contains at least one character from each selected category
	password = enforceCharacterTypes(password, req, charset)

	// Calculate strength metrics
	entropyBits := calculateEntropy(len(charset), length)
	strengthScore := calculateStrengthScore(password, entropyBits)
	timeToCrack := estimateTimeToCrack(entropyBits)

	return &models.PasswordGenerateResponse{
		Password:      password,
		StrengthScore: strengthScore,
		EntropyBits:   entropyBits,
		TimeToCrack:   timeToCrack,
	}, nil
}

// RecordPasswordGeneration records a password generation event (optional, for analytics)
func (s *PasswordService) RecordPasswordGeneration(userID uuid.UUID, req *models.PasswordGenerateRequest, result *models.PasswordGenerateResponse, usedForEntryID *uuid.UUID) error {
	query := `
		INSERT INTO password_generation_history (id, user_id, length, include_uppercase, include_lowercase, include_numbers, include_symbols, strength_score, entropy_bits, used_for_entry_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := s.db.Exec(query,
		uuid.New(),
		userID,
		req.Length,
		req.IncludeUppercase,
		req.IncludeLowercase,
		req.IncludeNumbers,
		req.IncludeSymbols,
		result.StrengthScore,
		result.EntropyBits,
		usedForEntryID,
		time.Now(),
	)

	return err
}

// GetPasswordHistory retrieves password generation history for a user
func (s *PasswordService) GetPasswordHistory(userID uuid.UUID, limit int) ([]models.PasswordGenerationHistory, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, user_id, length, include_uppercase, include_lowercase, include_numbers, include_symbols, strength_score, entropy_bits, used_for_entry_id, created_at
		FROM password_generation_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query password history: %w", err)
	}
	defer rows.Close()

	var history []models.PasswordGenerationHistory
	for rows.Next() {
		var h models.PasswordGenerationHistory
		err := rows.Scan(
			&h.ID,
			&h.UserID,
			&h.Length,
			&h.IncludeUppercase,
			&h.IncludeLowercase,
			&h.IncludeNumbers,
			&h.IncludeSymbols,
			&h.StrengthScore,
			&h.EntropyBits,
			&h.UsedForEntryID,
			&h.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan password history: %w", err)
		}
		history = append(history, h)
	}

	return history, nil
}

// Helper functions

func generateSecurePassword(charset string, length int) (string, error) {
	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		password[i] = charset[randomIndex.Int64()]
	}

	return string(password), nil
}

func removeChars(s, chars string) string {
	for _, c := range chars {
		s = strings.ReplaceAll(s, string(c), "")
	}
	return s
}

func enforceCharacterTypes(password string, req *models.PasswordGenerateRequest, charset string) string {
	// Check if password needs characters from each type
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSymbol := false

	for _, c := range password {
		switch {
		case strings.ContainsRune(uppercase, c):
			hasUpper = true
		case strings.ContainsRune(lowercase, c):
			hasLower = true
		case strings.ContainsRune(numbers, c):
			hasNumber = true
		default:
			hasSymbol = true
		}
	}

	// Replace characters if needed (modify password to ensure diversity)
	pwBytes := []byte(password)
	pos := 0

	if req.IncludeUppercase && !hasUpper && len(pwBytes) > pos {
		char, _ := getRandomChar(uppercase)
		pwBytes[pos] = char
		pos++
	}
	if req.IncludeLowercase && !hasLower && len(pwBytes) > pos {
		char, _ := getRandomChar(lowercase)
		pwBytes[pos] = char
		pos++
	}
	if req.IncludeNumbers && !hasNumber && len(pwBytes) > pos {
		char, _ := getRandomChar(numbers)
		pwBytes[pos] = char
		pos++
	}
	if req.IncludeSymbols && !hasSymbol && len(pwBytes) > pos {
		symbolSet := req.CustomSymbols
		if symbolSet == "" {
			symbolSet = symbols
		}
		char, _ := getRandomChar(symbolSet)
		pwBytes[pos] = char
	}

	// Shuffle to randomize positions of enforced chars
	shuffle(pwBytes)

	return string(pwBytes)
}

func getRandomChar(charset string) (byte, error) {
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return 0, err
	}
	return charset[idx.Int64()], nil
}

func shuffle(s []byte) {
	for i := len(s) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		s[i], s[j.Int64()] = s[j.Int64()], s[i]
	}
}

func calculateEntropy(charsetSize, length int) float64 {
	// Entropy = log2(charsetSize^length) = length * log2(charsetSize)
	return float64(length) * math.Log2(float64(charsetSize))
}

func calculateStrengthScore(password string, entropy float64) int {
	// Score from 0-100 based on entropy and character diversity
	score := int(entropy * 1.5) // Base score from entropy

	// Bonus for character diversity
	hasUpper := strings.ContainsAny(password, uppercase)
	hasLower := strings.ContainsAny(password, lowercase)
	hasNumber := strings.ContainsAny(password, numbers)
	hasSymbol := false
	for _, c := range password {
		if !strings.ContainsRune(uppercase+lowercase+numbers, c) {
			hasSymbol = true
			break
		}
	}

	diversity := 0
	if hasUpper {
		diversity++
	}
	if hasLower {
		diversity++
	}
	if hasNumber {
		diversity++
	}
	if hasSymbol {
		diversity++
	}

	score += diversity * 5

	// Bonus for length
	if len(password) >= 16 {
		score += 10
	} else if len(password) >= 12 {
		score += 5
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

func estimateTimeToCrack(entropy float64) string {
	// Assume 10 billion guesses per second (modern GPU cluster)
	guessesPerSecond := 10e9
	combinations := math.Pow(2, entropy)
	seconds := combinations / guessesPerSecond / 2 // Average case

	switch {
	case seconds < 1:
		return "instant"
	case seconds < 60:
		return fmt.Sprintf("%.0f seconds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%.0f minutes", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%.0f hours", seconds/3600)
	case seconds < 31536000:
		return fmt.Sprintf("%.0f days", seconds/86400)
	case seconds < 31536000*100:
		return fmt.Sprintf("%.0f years", seconds/31536000)
	case seconds < 31536000*1000:
		return fmt.Sprintf("%.0f centuries", seconds/31536000/100)
	default:
		return "millions of years"
	}
}
