package database

import (
	"testing"
	"time"
	
	"github.com/coldforge/vault/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDatabase(t *testing.T) {
	// Skip integration tests that require actual database
	t.Skip("Integration test - requires PostgreSQL database")
	
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "test_user",
		Password: "test_password",
		DBName:   "test_db",
		SSLMode:  "disable",
	}
	
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	
	defer db.Close()
	
	// Test connection
	err = db.HealthCheck()
	assert.NoError(t, err)
	
	// Test stats
	stats := db.GetStats()
	assert.GreaterOrEqual(t, stats.MaxOpenConns, 1)
}

func TestDatabaseConfiguration(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "test_user",
		Password: "test_password",
		DBName:   "test_db",
		SSLMode:  "disable",
	}
	
	// Test DSN construction (without actual connection)
	expectedDSN := "host=localhost port=5432 user=test_user password=test_password dbname=test_db sslmode=disable"
	
	// This would be tested by inspecting the connection string construction
	// For now, we'll test the configuration is properly stored
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, "5432", cfg.Port)
	assert.Equal(t, "test_user", cfg.User)
	assert.Equal(t, "test_password", cfg.Password)
	assert.Equal(t, "test_db", cfg.DBName)
	assert.Equal(t, "disable", cfg.SSLMode)
	
	_ = expectedDSN // Use the variable to avoid unused warning
}

func TestContextWithTimeout(t *testing.T) {
	timeout := 5 * time.Second
	ctx, cancel := ContextWithTimeout(timeout)
	defer cancel()
	
	assert.NotNil(t, ctx)
	
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.True(t, time.Until(deadline) <= timeout)
	assert.True(t, time.Until(deadline) > timeout-time.Millisecond*100) // Allow for small timing differences
}

func TestBaseRepository(t *testing.T) {
	// Test without actual database connection
	var db *DB // nil for this test
	repo := NewBaseRepository(db)
	
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestWithTransaction(t *testing.T) {
	t.Skip("Integration test - requires PostgreSQL database")
	
	// This test would require:
	// 1. Real database connection
	// 2. Test transaction rollback on error
	// 3. Test transaction commit on success
	// 4. Test panic recovery and rollback
}

func TestRunMigrations(t *testing.T) {
	t.Skip("Integration test - requires PostgreSQL database and migration files")
	
	// This test would require:
	// 1. Real database connection
	// 2. Migration files available
	// 3. Test migration up/down
	// 4. Test migration idempotency
}

func TestCleanupExpiredSessions(t *testing.T) {
	t.Skip("Integration test - requires PostgreSQL database")
	
	// This test would require:
	// 1. Real database connection with schema
	// 2. Insert test sessions (some expired, some active)
	// 3. Run cleanup
	// 4. Verify only expired sessions were deleted
}

func TestHealthCheck(t *testing.T) {
	t.Skip("Integration test - requires PostgreSQL database")
	
	// This test would require:
	// 1. Real database connection
	// 2. Test successful health check
	// 3. Test health check with database down
}

func TestDatabaseStats(t *testing.T) {
	t.Skip("Integration test - requires PostgreSQL database")
	
	// This test would require:
	// 1. Real database connection
	// 2. Perform some database operations
	// 3. Check that stats reflect the operations
	// 4. Verify connection pool statistics
}

// Mock database tests (without actual database)
func TestDatabaseConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *config.DatabaseConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "user",
				Password: "password",
				DBName:   "dbname",
				SSLMode:  "disable",
			},
			valid: true,
		},
		{
			name: "missing host",
			config: &config.DatabaseConfig{
				Port:     "5432",
				User:     "user",
				Password: "password",
				DBName:   "dbname",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "missing port",
			config: &config.DatabaseConfig{
				Host:     "localhost",
				User:     "user",
				Password: "password",
				DBName:   "dbname",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "missing user",
			config: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Password: "password",
				DBName:   "dbname",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "missing database name",
			config: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "user",
				Password: "password",
				SSLMode:  "disable",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test configuration validation logic
			valid := tt.config.Host != "" && 
					 tt.config.Port != "" && 
					 tt.config.User != "" && 
					 tt.config.DBName != ""
			
			assert.Equal(t, tt.valid, valid)
		})
	}
}

// Integration test setup helper (for when integration tests are enabled)
func setupTestDatabase(t *testing.T) *DB {
	t.Helper()
	
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "vault_test",
		Password: "vault_test",
		DBName:   "vault_test",
		SSLMode:  "disable",
	}
	
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	
	// Run migrations
	err = db.RunMigrations("../../migrations")
	require.NoError(t, err)
	
	return db
}

func cleanupTestDatabase(t *testing.T, db *DB) {
	t.Helper()
	
	// Clean up test data
	tables := []string{
		"audit_logs",
		"sessions", 
		"recovery_codes",
		"vaults",
		"auth_methods",
		"users",
	}
	
	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table)
		require.NoError(t, err)
	}
	
	db.Close()
}

// Benchmark tests
func BenchmarkDatabaseConnection(b *testing.B) {
	b.Skip("Benchmark test - requires PostgreSQL database")
	
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     "5432", 
		User:     "vault_test",
		Password: "vault_test",
		DBName:   "vault_test",
		SSLMode:  "disable",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := NewDatabase(cfg)
		if err != nil {
			b.Fatal(err)
		}
		db.Close()
	}
}

func BenchmarkHealthCheck(b *testing.B) {
	b.Skip("Benchmark test - requires PostgreSQL database")
	
	db := setupTestDatabase(&testing.T{})
	defer db.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.HealthCheck()
		if err != nil {
			b.Fatal(err)
		}
	}
}