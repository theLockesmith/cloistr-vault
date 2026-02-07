package providers

import (
	"context"
	"time"
	
	"github.com/google/uuid"
)

// AuthProvider defines the interface that all authentication providers must implement
type AuthProvider interface {
	// GetType returns the authentication type (e.g., "email", "nostr", "webauthn", "ethereum")
	GetType() string
	
	// ValidateCredentials validates the provided credentials
	ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*AuthResult, error)
	
	// PrepareRegistration prepares data needed for registration
	PrepareRegistration(ctx context.Context, data map[string]interface{}) (*RegistrationData, error)
	
	// CompleteRegistration finalizes the registration process
	CompleteRegistration(ctx context.Context, userID uuid.UUID, data *RegistrationData) error
	
	// GenerateChallenge creates a challenge for challenge-response auth (optional)
	GenerateChallenge(ctx context.Context, identifier string) (*Challenge, error)
	
	// SupportsChallenge indicates if this provider uses challenge-response auth
	SupportsChallenge() bool
	
	// GetRequiredFields returns the fields required for this auth method
	GetRequiredFields() []string
	
	// GetOptionalFields returns optional fields for this auth method
	GetOptionalFields() []string
}

// AuthResult contains the result of a successful authentication
type AuthResult struct {
	UserID        uuid.UUID `json:"user_id"`
	Identifier    string    `json:"identifier"`    // email, pubkey, etc.
	DisplayName   string    `json:"display_name"`  // user-friendly name
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	RequiresMFA   bool      `json:"requires_mfa"`
	TrustScore    float64   `json:"trust_score"`   // 0.0-1.0, for risk assessment
}

// RegistrationData contains data needed to complete registration
type RegistrationData struct {
	Identifier   string                 `json:"identifier"`
	AuthData     map[string]interface{} `json:"auth_data"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	RequiresVerification bool           `json:"requires_verification"`
}

// Challenge represents an authentication challenge
type Challenge struct {
	ID        string    `json:"id"`
	Value     string    `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MFAProvider defines multi-factor authentication capabilities
type MFAProvider interface {
	// GetMFAType returns the MFA type (e.g., "totp", "sms", "hardware_key")
	GetMFAType() string
	
	// SetupMFA initiates MFA setup for a user
	SetupMFA(ctx context.Context, userID uuid.UUID) (*MFASetupData, error)
	
	// CompleteMFASetup finalizes MFA setup
	CompleteMFASetup(ctx context.Context, userID uuid.UUID, setupData *MFASetupData, verification string) error
	
	// VerifyMFA verifies an MFA token/code
	VerifyMFA(ctx context.Context, userID uuid.UUID, token string) (bool, error)
	
	// DisableMFA removes MFA for a user
	DisableMFA(ctx context.Context, userID uuid.UUID) error
}

// MFASetupData contains data needed for MFA setup
type MFASetupData struct {
	Secret      string                 `json:"secret,omitempty"`
	QRCode      string                 `json:"qr_code,omitempty"`
	BackupCodes []string               `json:"backup_codes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BiometricProvider defines biometric authentication capabilities
type BiometricProvider interface {
	// RegisterBiometric registers biometric data for a user
	RegisterBiometric(ctx context.Context, userID uuid.UUID, biometricData []byte) error
	
	// VerifyBiometric verifies biometric authentication
	VerifyBiometric(ctx context.Context, userID uuid.UUID, biometricData []byte) (bool, error)
	
	// GetSupportedBiometrics returns supported biometric types
	GetSupportedBiometrics() []string
}

// RecoveryProvider defines account recovery capabilities
type RecoveryProvider interface {
	// GenerateRecoveryCodes creates recovery codes for a user
	GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID, count int) ([]string, error)
	
	// VerifyRecoveryCode validates a recovery code
	VerifyRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (bool, error)
	
	// InitiateRecovery starts the account recovery process
	InitiateRecovery(ctx context.Context, identifier string, method string) error
	
	// CompleteRecovery finalizes account recovery
	CompleteRecovery(ctx context.Context, token string, newCredentials map[string]interface{}) error
}

// AuthProviderManager manages all authentication providers
type AuthProviderManager struct {
	providers     map[string]AuthProvider
	mfaProviders  map[string]MFAProvider
	recoveryProvider RecoveryProvider
}

// NewAuthProviderManager creates a new provider manager
func NewAuthProviderManager() *AuthProviderManager {
	return &AuthProviderManager{
		providers:    make(map[string]AuthProvider),
		mfaProviders: make(map[string]MFAProvider),
	}
}

// RegisterProvider registers an authentication provider
func (m *AuthProviderManager) RegisterProvider(provider AuthProvider) {
	m.providers[provider.GetType()] = provider
}

// RegisterMFAProvider registers an MFA provider
func (m *AuthProviderManager) RegisterMFAProvider(provider MFAProvider) {
	m.mfaProviders[provider.GetMFAType()] = provider
}

// GetProvider returns a provider by type
func (m *AuthProviderManager) GetProvider(authType string) (AuthProvider, bool) {
	provider, exists := m.providers[authType]
	return provider, exists
}

// GetMFAProvider returns an MFA provider by type
func (m *AuthProviderManager) GetMFAProvider(mfaType string) (MFAProvider, bool) {
	provider, exists := m.mfaProviders[mfaType]
	return provider, exists
}

// GetSupportedMethods returns all supported authentication methods
func (m *AuthProviderManager) GetSupportedMethods() []string {
	methods := make([]string, 0, len(m.providers))
	for authType := range m.providers {
		methods = append(methods, authType)
	}
	return methods
}

// GetSupportedMFAMethods returns all supported MFA methods
func (m *AuthProviderManager) GetSupportedMFAMethods() []string {
	methods := make([]string, 0, len(m.mfaProviders))
	for mfaType := range m.mfaProviders {
		methods = append(methods, mfaType)
	}
	return methods
}