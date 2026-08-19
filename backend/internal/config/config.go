package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Security SecurityConfig
	KMS      KMSConfig
	Auth     AuthConfig
	WebAuthn WebAuthnConfig
}

// AuthConfig holds unified-auth settings.
type AuthConfig struct {
	// SignerURL is the base URL of the Cloistr signer service used for unified-auth
	// (slice 3). When set, AuthMiddleware will fall back to validating a signer
	// session cookie / Bearer JWT and provisioning a vault user on first login.
	// Empty string disables the feature. Env: VAULT_SIGNER_URL.
	SignerURL string
}

type ServerConfig struct {
	Port string
	Host string
	Env  string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type SecurityConfig struct {
	JWTSecret       string
	ScryptN         int
	ScryptR         int
	ScryptP         int
	SessionDuration int // hours
}

// WebAuthnConfig holds the relying-party identity for passkeys.
//
// RPID must be the site's registrable domain and Origin must match the exact
// scheme+host+port the browser is on, or the authenticator refuses the
// ceremony. For local development that means RPID "localhost" and an Origin of
// the dev server, e.g. http://localhost:3000 — not the API's own port, since
// the browser is talking to the frontend.
//
// Credentials are scoped to RPID: passkeys registered against localhost will
// not work against vault.cloistr.xyz, and vice versa.
type WebAuthnConfig struct {
	RPID        string
	Origin      string
	DisplayName string
}

type KMSConfig struct {
	Provider     string
	Address      string
	Token        string
	MountPath    string
	KeyDir       string
	AutoRotate   bool
	SkipVerify   bool
}

func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "7700"),
			Host: getEnv("HOST", "localhost"),
			Env:  getEnv("ENVIRONMENT", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "vault_user"),
			Password: getEnv("DB_PASSWORD", "vault_password"),
			DBName:   getEnv("DB_NAME", "vault_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Security: SecurityConfig{
			JWTSecret:       getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			ScryptN:         getEnvInt("SCRYPT_N", 32768),
			ScryptR:         getEnvInt("SCRYPT_R", 8),
			ScryptP:         getEnvInt("SCRYPT_P", 1),
			SessionDuration: getEnvInt("SESSION_DURATION_HOURS", 24),
		},
		KMS: KMSConfig{
			Provider:   getEnv("KMS_PROVIDER", "file"),
			Address:    getEnv("KMS_ADDRESS", "http://localhost:7712"),
			Token:      getEnv("KMS_TOKEN", "coldforge-dev-token"),
			MountPath:  getEnv("KMS_MOUNT_PATH", "secret"),
			KeyDir:     getEnv("KMS_KEY_DIR", "./keys"),
			AutoRotate: getEnvBool("KMS_AUTO_ROTATE", true),
			SkipVerify: getEnvBool("KMS_SKIP_VERIFY", false),
		},
		Auth: AuthConfig{
			// Default: in-cluster signer address. Empty string disables unified-auth.
			SignerURL: getEnv("VAULT_SIGNER_URL", "http://cloistr-signer.cloistr.svc.cluster.local:7777"),
		},
		WebAuthn: WebAuthnConfig{
			RPID:        getEnv("WEBAUTHN_RP_ID", "vault.cloistr.xyz"),
			Origin:      getEnv("WEBAUTHN_ORIGIN", "https://vault.cloistr.xyz"),
			DisplayName: getEnv("WEBAUTHN_DISPLAY_NAME", "Cloistr Vault"),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return fallback
}