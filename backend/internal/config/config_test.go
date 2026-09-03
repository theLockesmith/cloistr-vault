package config

import (
	"strings"
	"testing"
)

// production is the shape the live deployment actually has, verified in the
// running pod on 2026-09-02: ENVIRONMENT=production, KMS_PROVIDER=vault, all
// three secrets present. Every test that wants a healthy production config
// starts from this and removes one thing.
func production(t *testing.T) {
	t.Helper()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	t.Setenv("KMS_PROVIDER", "vault")
	t.Setenv("DB_PASSWORD", "a-real-database-password")
	t.Setenv("JWT_SECRET", "a-real-signing-key")
	t.Setenv("KMS_TOKEN", "a-real-kms-token")
}

// This is the regression guard for the whole change: the three strings that
// used to be compiled in must not be reachable any more. If someone
// reintroduces a fallback, one of these fires.
func TestNoCompiledInSecretFallbacks(t *testing.T) {
	cases := []struct {
		unset string
		want  string
	}{
		{"DB_PASSWORD", "DB_PASSWORD"},
		{"JWT_SECRET", "JWT_SECRET"},
		{"KMS_TOKEN", "KMS_TOKEN"},
	}
	for _, tc := range cases {
		t.Run(tc.unset, func(t *testing.T) {
			production(t)
			t.Setenv(tc.unset, "")

			cfg, err := LoadConfig()
			if err == nil {
				t.Fatalf("LoadConfig succeeded with %s unset; it must refuse to start", tc.unset)
			}
			if cfg != nil {
				t.Errorf("LoadConfig returned a config alongside its error: %+v", cfg)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the missing variable %q: %v", tc.want, err)
			}
		})
	}
}

func TestProductionWithAllSecretsLoads(t *testing.T) {
	production(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig on a correctly configured production env: %v", err)
	}
	if cfg.Security.JWTSecret != "a-real-signing-key" {
		t.Errorf("JWTSecret = %q, want the value from the environment", cfg.Security.JWTSecret)
	}
	if cfg.Database.Password != "a-real-database-password" {
		t.Error("DB_PASSWORD was not read from the environment")
	}
	if cfg.KMS.Token != "a-real-kms-token" {
		t.Error("KMS_TOKEN was not read from the environment")
	}
}

// All three missing should be reported together, not one per restart.
func TestAllMissingSecretsReportedAtOnce(t *testing.T) {
	production(t)
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("KMS_TOKEN", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig succeeded with no secrets set")
	}
	for _, name := range []string{"DB_PASSWORD", "JWT_SECRET", "KMS_TOKEN"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error omits %s, so an operator fixes one variable per restart: %v", name, err)
		}
	}
}

// The file KMS provider does not use a token, so requiring one there would be
// a false alarm that teaches people to set a dummy value.
func TestFileKMSProviderDoesNotRequireToken(t *testing.T) {
	production(t)
	t.Setenv("KMS_PROVIDER", "file")
	t.Setenv("KMS_TOKEN", "")

	if _, err := LoadConfig(); err != nil {
		t.Fatalf("file provider should not require KMS_TOKEN: %v", err)
	}
}

// Rule 2: dev mode invents credentials, so it must never run in a pod. This is
// the rule that would have caught the class of mistake the old defaults hid.
func TestDevelopmentIsRefusedInsideKubernetes(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("ENVIRONMENT=development was accepted inside Kubernetes")
	}
	if !strings.Contains(err.Error(), "Kubernetes") {
		t.Errorf("error should say why it refused: %v", err)
	}
}

// Rule 2 must not fire outside a cluster, or local development stops working.
func TestDevelopmentOutsideKubernetesLoads(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("KMS_TOKEN", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("development outside a cluster should load: %v", err)
	}
	if cfg.Security.JWTSecret == "" {
		t.Fatal("development left JWTSecret empty; nothing can sign")
	}
	if cfg.Security.JWTSecret == "your-secret-key-change-in-production" {
		t.Fatal("development reintroduced the old published signing key")
	}
	if len(cfg.Security.JWTSecret) != 64 {
		t.Errorf("development JWT secret is %d hex chars, want 64 (32 bytes)", len(cfg.Security.JWTSecret))
	}
}

// A generated dev key is only worth anything if it is not the same key twice.
func TestDevelopmentJWTSecretIsNotFixed(t *testing.T) {
	load := func() string {
		t.Helper()
		t.Setenv("ENVIRONMENT", "development")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		t.Setenv("JWT_SECRET", "")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		return cfg.Security.JWTSecret
	}
	if a, b := load(), load(); a == b {
		t.Errorf("two loads produced the same development signing key %q", a)
	}
}

// An explicitly set JWT_SECRET must win in development too, or sessions cannot
// survive a restart while someone is working on them.
func TestDevelopmentHonoursExplicitJWTSecret(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("JWT_SECRET", "my-stable-dev-key")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Security.JWTSecret != "my-stable-dev-key" {
		t.Errorf("JWTSecret = %q, want the explicit value", cfg.Security.JWTSecret)
	}
}
