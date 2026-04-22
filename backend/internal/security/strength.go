package security

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// StrengthScore represents password strength levels
type StrengthScore int

const (
	ScoreVeryWeak   StrengthScore = 0
	ScoreWeak       StrengthScore = 1
	ScoreFair       StrengthScore = 2
	ScoreStrong     StrengthScore = 3
	ScoreVeryStrong StrengthScore = 4
)

// StrengthResult contains detailed password analysis
type StrengthResult struct {
	Score           StrengthScore `json:"score"`            // 0-4 score
	Entropy         float64       `json:"entropy"`          // bits of entropy
	CrackTimeDisplay string       `json:"crack_time"`       // human readable
	CrackTimeSeconds float64      `json:"crack_time_secs"`  // raw seconds
	Feedback        []string      `json:"feedback"`         // suggestions
	HasLowercase    bool          `json:"has_lowercase"`
	HasUppercase    bool          `json:"has_uppercase"`
	HasNumbers      bool          `json:"has_numbers"`
	HasSymbols      bool          `json:"has_symbols"`
	Length          int           `json:"length"`
}

// AnalyzePassword performs comprehensive password strength analysis
func AnalyzePassword(password string) *StrengthResult {
	result := &StrengthResult{
		Length:   len(password),
		Feedback: []string{},
	}

	// Character class analysis
	for _, r := range password {
		if unicode.IsLower(r) {
			result.HasLowercase = true
		} else if unicode.IsUpper(r) {
			result.HasUppercase = true
		} else if unicode.IsDigit(r) {
			result.HasNumbers = true
		} else {
			result.HasSymbols = true
		}
	}

	// Calculate character pool size
	poolSize := 0
	if result.HasLowercase {
		poolSize += 26
	}
	if result.HasUppercase {
		poolSize += 26
	}
	if result.HasNumbers {
		poolSize += 10
	}
	if result.HasSymbols {
		poolSize += 32 // common symbols
	}

	// Calculate entropy
	if poolSize > 0 && len(password) > 0 {
		result.Entropy = float64(len(password)) * math.Log2(float64(poolSize))
	}

	// Apply penalties for common patterns
	result.Entropy = applyPatternPenalties(password, result.Entropy)

	// Calculate crack time (10B guesses/sec for offline attack)
	combinations := math.Pow(2, result.Entropy)
	result.CrackTimeSeconds = combinations / 10_000_000_000
	result.CrackTimeDisplay = formatCrackTime(result.CrackTimeSeconds)

	// Determine score based on entropy
	switch {
	case result.Entropy < 28:
		result.Score = ScoreVeryWeak
	case result.Entropy < 35:
		result.Score = ScoreWeak
	case result.Entropy < 60:
		result.Score = ScoreFair
	case result.Entropy < 80:
		result.Score = ScoreStrong
	default:
		result.Score = ScoreVeryStrong
	}

	// Generate feedback
	result.Feedback = generateFeedback(result, password)

	return result
}

// applyPatternPenalties reduces entropy for common patterns
func applyPatternPenalties(password string, entropy float64) float64 {
	lower := strings.ToLower(password)

	// Common patterns to check
	commonPatterns := []string{
		"123456", "password", "qwerty", "abc123",
		"letmein", "welcome", "admin", "login",
		"passw0rd", "12345678", "sunshine", "princess",
	}

	for _, pattern := range commonPatterns {
		if strings.Contains(lower, pattern) {
			entropy *= 0.5 // Heavy penalty for common patterns
			break
		}
	}

	// Sequential characters penalty
	if hasSequential(password, 4) {
		entropy *= 0.8
	}

	// Repeated characters penalty
	if hasRepeated(password, 3) {
		entropy *= 0.9
	}

	// Keyboard patterns penalty
	if hasKeyboardPattern(password) {
		entropy *= 0.7
	}

	return entropy
}

// hasSequential checks for sequential characters
func hasSequential(s string, minLen int) bool {
	if len(s) < minLen {
		return false
	}

	count := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1]+1 || s[i] == s[i-1]-1 {
			count++
			if count >= minLen {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

// hasRepeated checks for repeated characters
func hasRepeated(s string, minLen int) bool {
	if len(s) < minLen {
		return false
	}

	count := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			count++
			if count >= minLen {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

// hasKeyboardPattern checks for keyboard patterns
func hasKeyboardPattern(s string) bool {
	patterns := []string{
		"qwert", "asdf", "zxcv", "qazws",
		"!@#$%", "yuiop", "hjkl", "nm,.",
	}
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// generateFeedback provides actionable suggestions
func generateFeedback(result *StrengthResult, password string) []string {
	feedback := []string{}

	if result.Length < 12 {
		feedback = append(feedback, "Use at least 12 characters")
	}
	if !result.HasUppercase {
		feedback = append(feedback, "Add uppercase letters")
	}
	if !result.HasLowercase {
		feedback = append(feedback, "Add lowercase letters")
	}
	if !result.HasNumbers {
		feedback = append(feedback, "Add numbers")
	}
	if !result.HasSymbols {
		feedback = append(feedback, "Add special characters")
	}

	lower := strings.ToLower(password)
	if containsCommonWord(lower) {
		feedback = append(feedback, "Avoid common words or phrases")
	}

	if hasSequential(password, 4) {
		feedback = append(feedback, "Avoid sequential characters (abc, 123)")
	}

	if hasRepeated(password, 3) {
		feedback = append(feedback, "Avoid repeated characters")
	}

	return feedback
}

// containsCommonWord checks for dictionary words
func containsCommonWord(s string) bool {
	commonWords := []string{
		"password", "admin", "login", "welcome",
		"hello", "user", "guest", "test",
		"love", "dragon", "master", "monkey",
	}
	for _, word := range commonWords {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

// formatCrackTime converts seconds to human-readable time
func formatCrackTime(seconds float64) string {
	if seconds < 1 {
		return "instant"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0f seconds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.0f minutes", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%.0f hours", seconds/3600)
	}
	if seconds < 31536000 {
		return fmt.Sprintf("%.0f days", seconds/86400)
	}
	if seconds < 31536000*100 {
		return fmt.Sprintf("%.0f years", seconds/31536000)
	}
	if seconds < 31536000*1000 {
		return fmt.Sprintf("%.0f centuries", seconds/(31536000*100))
	}
	return "millennia+"
}

// BreachResult contains HIBP check results
type BreachResult struct {
	Compromised bool   `json:"compromised"`
	Count       int    `json:"count"` // Number of times seen in breaches
	CheckedAt   string `json:"checked_at"`
	Error       string `json:"error,omitempty"`
}

// CheckHIBP checks if a password has been compromised using k-anonymity
func CheckHIBP(password string) *BreachResult {
	result := &BreachResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Hash password with SHA-1
	hash := sha1.Sum([]byte(password))
	hashHex := strings.ToUpper(hex.EncodeToString(hash[:]))

	// k-anonymity: send first 5 chars, server returns matching hashes
	prefix := hashHex[:5]
	suffix := hashHex[5:]

	// Query HIBP API
	url := fmt.Sprintf("https://api.pwnedpasswords.com/range/%s", prefix)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		result.Error = "Unable to check breach database"
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = "Breach check service unavailable"
		return result
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = "Failed to read breach response"
		return result
	}

	// Parse response: each line is "SUFFIX:COUNT"
	lines := strings.Split(string(body), "\r\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) == 2 && parts[0] == suffix {
			result.Compromised = true
			fmt.Sscanf(parts[1], "%d", &result.Count)
			break
		}
	}

	return result
}
