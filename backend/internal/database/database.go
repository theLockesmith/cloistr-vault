package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
	
	"github.com/coldforge/vault/internal/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// DB holds the database connection and provides methods for database operations
type DB struct {
	*sql.DB
	config *config.DatabaseConfig
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.DatabaseConfig) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	log.Printf("Connected to database: %s:%s/%s", cfg.Host, cfg.Port, cfg.DBName)
	
	return &DB{
		DB:     db,
		config: cfg,
	}, nil
}

// RunMigrations runs database migrations
func (db *DB) RunMigrations(migrationPath string) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}
	
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	
	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	
	if err == migrate.ErrNoChange {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Database migrations completed successfully")
	}
	
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// HealthCheck performs a health check on the database
func (db *DB) HealthCheck() error {
	ctx, cancel := ContextWithTimeout(5 * time.Second)
	defer cancel()
	
	return db.PingContext(ctx)
}

// CleanupExpiredSessions removes expired sessions from the database
func (db *DB) CleanupExpiredSessions() error {
	query := "SELECT cleanup_expired_sessions()"
	var deletedCount int
	
	err := db.QueryRow(query).Scan(&deletedCount)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	
	if deletedCount > 0 {
		log.Printf("Cleaned up %d expired sessions", deletedCount)
	}
	
	return nil
}

// GetStats returns database connection statistics
func (db *DB) GetStats() sql.DBStats {
	return db.Stats()
}

// WithTransaction executes a function within a database transaction
func (db *DB) WithTransaction(fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction: %v (original error: %w)", rollbackErr, err)
		}
		return err
	}
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

func ContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// Repository pattern base struct
type BaseRepository struct {
	db *DB
}

func NewBaseRepository(db *DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// Common database operations
func (r *BaseRepository) Exists(query string, args ...interface{}) (bool, error) {
	var exists bool
	err := r.db.QueryRow(query, args...).Scan(&exists)
	return exists, err
}

func (r *BaseRepository) Count(query string, args ...interface{}) (int64, error) {
	var count int64
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}