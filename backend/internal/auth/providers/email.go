package providers

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
	
	"github.com/coldforge/vault/internal/crypto"
	"github.com/google/uuid"
)

// EmailProvider implements traditional email/password authentication
type EmailProvider struct {
	db *sql.DB
}

// NewEmailProvider creates a new email authentication provider
func NewEmailProvider(db *sql.DB) *EmailProvider {
	return &EmailProvider{db: db}
}

// GetType returns the authentication type
func (p *EmailProvider) GetType() string {
	return "email"
}

// GetRequiredFields returns required fields for email auth
func (p *EmailProvider) GetRequiredFields() []string {
	return []string{"email", "password"}
}

// GetOptionalFields returns optional fields
func (p *EmailProvider) GetOptionalFields() []string {
	return []string{"display_name", "profile_picture"}
}

// SupportsChallenge indicates this provider doesn't use challenge-response
func (p *EmailProvider) SupportsChallenge() bool {
	return false
}

// GenerateChallenge is not supported for email auth
func (p *EmailProvider) GenerateChallenge(ctx context.Context, identifier string) (*Challenge, error) {
	return nil, fmt.Errorf("email provider does not support challenge generation")
}

// ValidateCredentials validates email/password authentication
func (p *EmailProvider) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*AuthResult, error) {
	// Extract credentials
	email, ok := credentials["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email is required")
	}
	
	password, ok := credentials["password"].(string)
	if !ok || password == "" {
		return nil, fmt.Errorf("password is required")
	}
	
	// Validate email format
	if !p.isValidEmail(email) {
		return nil, fmt.Errorf("invalid email format")
	}
	
	// Get user and auth data
	var user struct {
		ID           uuid.UUID
		Email        string
		CreatedAt    time.Time
		Salt         []byte
		PasswordHash []byte
	}
	
	query := `
		SELECT u.id, u.email, u.created_at, am.salt, am.password_hash
		FROM users u 
		JOIN auth_methods am ON u.id = am.user_id 
		WHERE am.identifier = $1 AND am.type = 'email'
	`
	
	err := p.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.Salt, &user.PasswordHash)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Verify password
	if !crypto.VerifyPassword(password, user.Salt, user.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Calculate trust score
	trustScore := p.calculateTrustScore(user.CreatedAt, email)
	
	return &AuthResult{
		UserID:      user.ID,
		Identifier:  email,
		DisplayName: email,
		Metadata: map[string]interface{}{
			"email":        email,
			"auth_method":  "email",
			"account_age":  time.Since(user.CreatedAt).Hours() / 24,
			"domain":       p.extractDomain(email),
		},
		RequiresMFA: p.shouldRequireMFA(email, trustScore),
		TrustScore:  trustScore,
	}, nil
}

// PrepareRegistration prepares email registration data
func (p *EmailProvider) PrepareRegistration(ctx context.Context, data map[string]interface{}) (*RegistrationData, error) {
	email, ok := data["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email is required")
	}
	
	password, ok := data["password"].(string)
	if !ok || password == "" {
		return nil, fmt.Errorf("password is required")
	}
	
	// Validate email format
	if !p.isValidEmail(email) {
		return nil, fmt.Errorf("invalid email format")
	}
	
	// Validate password strength
	if err := p.validatePasswordStrength(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}
	
	// Check if email is already registered
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth_methods 
			WHERE identifier = $1 AND type = 'email'
		)`, email).Scan(&exists)
	
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	if exists {
		return nil, fmt.Errorf("email already registered")
	}
	
	// Generate salt and hash password
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	
	passwordHash, err := crypto.HashPassword(password, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	
	return &RegistrationData{
		Identifier: email,
		AuthData: map[string]interface{}{
			"salt":          salt,
			"password_hash": passwordHash,
			"email":         email,
		},
		Metadata: map[string]interface{}{
			"domain":     p.extractDomain(email),
			"registered": time.Now().Unix(),
			"method":     "email",
		},
		RequiresVerification: true, // Email verification recommended
	}, nil
}

// CompleteRegistration finalizes email registration
func (p *EmailProvider) CompleteRegistration(ctx context.Context, userID uuid.UUID, data *RegistrationData) error {
	salt := data.AuthData["salt"].([]byte)
	passwordHash := data.AuthData["password_hash"].([]byte)
	email := data.AuthData["email"].(string)
	
	// Create auth method record
	authMethodID := uuid.New()
	now := time.Now()
	
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO auth_methods (
			id, user_id, type, identifier, salt, password_hash, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		authMethodID, userID, "email", email, salt, passwordHash, now, now)
	
	if err != nil {
		return fmt.Errorf("failed to create auth method: %w", err)
	}
	
	return nil
}

// Helper methods
func (p *EmailProvider) isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func (p *EmailProvider) extractDomain(email string) string {
	parts := regexp.MustCompile(`@([^@]+)$`).FindStringSubmatch(email)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func (p *EmailProvider) validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	
	// Check for required character types
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{}|;:,.<>?]`).MatchString(password)
	
	requiredTypes := 0
	if hasLower { requiredTypes++ }
	if hasUpper { requiredTypes++ }
	if hasDigit { requiredTypes++ }
	if hasSpecial { requiredTypes++ }
	
	if requiredTypes < 3 {
		return fmt.Errorf("password must contain at least 3 of: lowercase, uppercase, digits, special characters")
	}
	
	return nil
}

func (p *EmailProvider) calculateTrustScore(createdAt time.Time, email string) float64 {
	// Base score for email auth
	score := 0.6
	
	// Account age bonus
	accountAgeDays := time.Since(createdAt).Hours() / 24
	ageBonus := min(accountAgeDays/180*0.2, 0.2) // Max bonus for 6+ month old account
	
	// Domain reputation bonus (simplified)
	domain := p.extractDomain(email)
	domainBonus := 0.0
	trustedDomains := []string{"gmail.com", "outlook.com", "protonmail.com", "fastmail.com"}
	for _, trusted := range trustedDomains {
		if domain == trusted {
			domainBonus = 0.1
			break
		}
	}
	
	return min(score+ageBonus+domainBonus, 1.0)
}

func (p *EmailProvider) shouldRequireMFA(email string, trustScore float64) bool {
	// Require MFA for low trust scores or high-value domains
	if trustScore < 0.7 {
		return true
	}
	
	// Could add domain-specific rules here
	return false
}