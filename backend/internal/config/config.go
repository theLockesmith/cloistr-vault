package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	Provider   string
	Address    string
	Token      string
	MountPath  string
	KeyDir     string
	AutoRotate bool
	SkipVerify bool
}

// Secrets have NO compiled-in fallback.
//
// Until 2026-09-02 this file shipped three of them: DB_PASSWORD defaulted to
// "vault_password", JWT_SECRET to "your-secret-key-change-in-production" and
// KMS_TOKEN to "coldforge-dev-token". A compiled-in default for a secret is
// worse than a missing one, because a misconfigured deployment starts happily
// and signs real sessions with a value that is published in a public AGPL
// repository. Anyone could mint a valid vault JWT from the source tree.
//
// So the three are now read with no fallback and the process refuses to start
// without them. See LoadConfig for the two rules, and devFallback for the only
// place a value is ever invented (local development, never in a pod, and never
// a fixed string).

// LoadConfig reads configuration from the environment.
//
// It returns an error rather than a half-configured process. Two rules:
//
//  1. Outside development, DB_PASSWORD and JWT_SECRET must be set, and
//     KMS_TOKEN must be set whenever KMS_PROVIDER is "vault" (the file
//     provider does not use a token).
//  2. ENVIRONMENT=development is refused inside Kubernetes. Dev mode invents
//     local credentials; a pod must never take that path, whatever its
//     ConfigMap says.
//
// Verified against the live deployment before this landed: ENVIRONMENT is
// "production", KMS_PROVIDER is "vault", and all three secrets are present in
// the pod with non-fallback values. Neither rule fires on it.
func LoadConfig() (*Config, error) {
	env := getEnv("ENVIRONMENT", "development")
	inKubernetes := os.Getenv("KUBERNETES_SERVICE_HOST") != ""
	development := strings.EqualFold(env, "development")

	if development && inKubernetes {
		return nil, fmt.Errorf(
			"config: ENVIRONMENT=%q is refused inside Kubernetes: development mode "+
				"invents local credentials and must never run in a cluster; set "+
				"ENVIRONMENT=production (or staging) on this deployment", env)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "7700"),
			Host: getEnv("HOST", "localhost"),
			Env:  env,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "vault_user"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   getEnv("DB_NAME", "vault_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Security: SecurityConfig{
			JWTSecret:       os.Getenv("JWT_SECRET"),
			ScryptN:         getEnvInt("SCRYPT_N", 32768),
			ScryptR:         getEnvInt("SCRYPT_R", 8),
			ScryptP:         getEnvInt("SCRYPT_P", 1),
			SessionDuration: getEnvInt("SESSION_DURATION_HOURS", 24),
		},
		KMS: KMSConfig{
			Provider:   getEnv("KMS_PROVIDER", "file"),
			Address:    getEnv("KMS_ADDRESS", "http://localhost:7712"),
			Token:      os.Getenv("KMS_TOKEN"),
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

	// KMS_TOKEN is only a credential when we actually talk to a KMS server.
	// The "file" provider keys off KMS_KEY_DIR and ignores the token.
	tokenRequired := !strings.EqualFold(cfg.KMS.Provider, "file")

	if development {
		return cfg, devFallback(cfg, tokenRequired)
	}

	var missing []string
	if cfg.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if cfg.Security.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if tokenRequired && cfg.KMS.Token == "" {
		missing = append(missing, fmt.Sprintf("KMS_TOKEN (required by KMS_PROVIDER=%q)", cfg.KMS.Provider))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"config: refusing to start with ENVIRONMENT=%q: missing required secret(s): %s",
			env, strings.Join(missing, ", "))
	}

	return cfg, nil
}

// devFallback fills local-development values for anything unset.
//
// DB_PASSWORD and KMS_TOKEN get the old local-only strings back, because they
// only ever address a developer's own postgres and dev KMS. JWT_SECRET does
// NOT: a fixed signing key is the one value that stays dangerous even in
// development, since a dev token minted from a published constant is a real
// token. It is generated per process instead, so no shared key exists to leak.
func devFallback(cfg *Config, tokenRequired bool) error {
	if cfg.Database.Password == "" {
		cfg.Database.Password = "vault_password"
	}
	if tokenRequired && cfg.KMS.Token == "" {
		cfg.KMS.Token = "coldforge-dev-token"
	}
	if cfg.Security.JWTSecret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("config: generating a development JWT secret: %w", err)
		}
		cfg.Security.JWTSecret = hex.EncodeToString(buf)
		fmt.Fprintln(os.Stderr,
			"config: JWT_SECRET unset; generated a random one for this process. "+
				"Sessions will not survive a restart. Set JWT_SECRET to keep them.")
	}
	return nil
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
