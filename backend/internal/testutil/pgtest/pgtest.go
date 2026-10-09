// Package pgtest gives integration tests a real, freshly migrated Postgres.
package pgtest

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/coldforge/vault/internal/database"
)

// DSNEnv names the server to test against, in lib/pq key=value form, e.g.
// "host=localhost port=55432 user=postgres password=x dbname=postgres sslmode=disable".
// The role needs CREATEDB.
const DSNEnv = "VAULT_TEST_PG_DSN"

// FreshDB creates a new database on the server named by VAULT_TEST_PG_DSN,
// applies every migration, and drops it when the test ends. It skips the test
// when the variable is unset.
func FreshDB(t *testing.T) *database.DB {
	t.Helper()
	dsn := os.Getenv(DSNEnv)
	if dsn == "" {
		t.Skip(DSNEnv + " not set")
	}

	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	// A CI service container may still be starting.
	deadline := time.Now().Add(60 * time.Second)
	for err = admin.Ping(); err != nil; err = admin.Ping() {
		if time.Now().After(deadline) {
			t.Fatalf("postgres not reachable: %v", err)
		}
		time.Sleep(time.Second)
	}

	name := "vault_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE IF EXISTS "` + name + `" WITH (FORCE)`); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})

	// lib/pq: a later key overrides an earlier one.
	conn, err := sql.Open("postgres", dsn+" dbname="+name)
	if err != nil {
		t.Fatal(err)
	}
	db := &database.DB{DB: conn}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(migrationsDir()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

// migrationsDir is backend/migrations, wherever the calling test runs from.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
}
