package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// LocalStorage implements local SQLite-based vault storage
type LocalStorage struct {
	db       *sql.DB
	basePath string
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	
	// Open SQLite database
	dbPath := filepath.Join(basePath, "vault.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Initialize schema
	if err := initializeLocalSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	
	return &LocalStorage{
		db:       db,
		basePath: basePath,
	}, nil
}

// Store saves encrypted vault data locally
func (s *LocalStorage) Store(ctx context.Context, userID uuid.UUID, encryptedData []byte, version int) (*StoreResult, error) {
	// Calculate checksum
	checksum := calculateChecksum(encryptedData)
	now := time.Now()
	
	// Check for version conflicts
	var currentVersion int
	err := s.db.QueryRowContext(ctx, "SELECT version FROM vaults WHERE user_id = ?", userID.String()).Scan(&currentVersion)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check current version: %w", err)
	}
	
	if err == sql.ErrNoRows {
		// First vault creation
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO vaults (id, user_id, encrypted_data, checksum, version, last_modified, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), userID.String(), encryptedData, checksum, version, now, now)
	} else {
		// Update existing vault
		if version <= currentVersion {
			return nil, fmt.Errorf("version conflict: expected version > %d, got %d", currentVersion, version)
		}
		
		_, err = s.db.ExecContext(ctx, `
			UPDATE vaults 
			SET encrypted_data = ?, checksum = ?, version = ?, last_modified = ?
			WHERE user_id = ?`,
			encryptedData, checksum, version, now, userID.String())
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to store vault: %w", err)
	}
	
	return &StoreResult{
		Version:      version,
		LastModified: now,
		Checksum:     checksum,
		StorageInfo:  fmt.Sprintf("local:%s", s.basePath),
	}, nil
}

// Retrieve gets encrypted vault data from local storage
func (s *LocalStorage) Retrieve(ctx context.Context, userID uuid.UUID) (*RetrieveResult, error) {
	var encryptedData []byte
	var version int
	var lastModified time.Time
	var checksum string
	
	err := s.db.QueryRowContext(ctx, `
		SELECT encrypted_data, version, last_modified, checksum
		FROM vaults WHERE user_id = ?`,
		userID.String()).Scan(&encryptedData, &version, &lastModified, &checksum)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vault not found")
		}
		return nil, fmt.Errorf("failed to retrieve vault: %w", err)
	}
	
	// Verify data integrity
	if calculateChecksum(encryptedData) != checksum {
		return nil, fmt.Errorf("vault data integrity check failed")
	}
	
	return &RetrieveResult{
		EncryptedData: encryptedData,
		Version:       version,
		LastModified:  lastModified,
		Checksum:      checksum,
	}, nil
}

// Delete removes vault data from local storage
func (s *LocalStorage) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM vaults WHERE user_id = ?", userID.String())
	if err != nil {
		return fmt.Errorf("failed to delete vault: %w", err)
	}
	
	return nil
}

// GetMetadata returns vault metadata without sensitive data
func (s *LocalStorage) GetMetadata(ctx context.Context, userID uuid.UUID) (*VaultMetadata, error) {
	var metadata VaultMetadata
	var size int64
	
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, version, last_modified, LENGTH(encrypted_data), checksum, created_at
		FROM vaults WHERE user_id = ?`,
		userID.String()).Scan(
		&metadata.UserID, &metadata.Version, &metadata.LastModified,
		&size, &metadata.Checksum, &metadata.CreatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vault not found")
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	
	metadata.Size = size
	metadata.StorageType = "local"
	
	return &metadata, nil
}

// ListVersions returns vault version history
func (s *LocalStorage) ListVersions(ctx context.Context, userID uuid.UUID) ([]VaultVersion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT version, last_modified, LENGTH(encrypted_data), checksum
		FROM vault_history 
		WHERE user_id = ? 
		ORDER BY version DESC 
		LIMIT 10`,
		userID.String())
	
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer rows.Close()
	
	var versions []VaultVersion
	for rows.Next() {
		var version VaultVersion
		var size int64
		
		err := rows.Scan(&version.Version, &version.Timestamp, &size, &version.Checksum)
		if err != nil {
			continue
		}
		
		version.Size = size
		versions = append(versions, version)
	}
	
	return versions, nil
}

// CreateBackup creates a backup of the vault
func (s *LocalStorage) CreateBackup(ctx context.Context, userID uuid.UUID) (*BackupInfo, error) {
	// Get current vault data
	result, err := s.Retrieve(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vault for backup: %w", err)
	}
	
	// Create backup file
	backupID := uuid.New().String()
	backupPath := filepath.Join(s.basePath, "backups", fmt.Sprintf("%s_%s.backup", userID.String(), backupID))
	
	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(backupPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}
	
	// Write backup file
	if err := os.WriteFile(backupPath, result.EncryptedData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write backup: %w", err)
	}
	
	backup := &BackupInfo{
		ID:          backupID,
		UserID:      userID,
		Version:     result.Version,
		CreatedAt:   time.Now(),
		Size:        int64(len(result.EncryptedData)),
		Checksum:    result.Checksum,
		StorageType: "local_file",
		Encrypted:   true,
	}
	
	// Store backup metadata
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO backups (id, user_id, version, created_at, size, checksum, file_path)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		backupID, userID.String(), result.Version, backup.CreatedAt, backup.Size, backup.Checksum, backupPath)
	
	if err != nil {
		return nil, fmt.Errorf("failed to store backup metadata: %w", err)
	}
	
	return backup, nil
}

// RestoreFromBackup restores vault from a backup
func (s *LocalStorage) RestoreFromBackup(ctx context.Context, userID uuid.UUID, backupID string) error {
	// Get backup metadata
	var backupPath string
	var backupVersion int
	
	err := s.db.QueryRowContext(ctx, `
		SELECT file_path, version
		FROM backups 
		WHERE id = ? AND user_id = ?`,
		backupID, userID.String()).Scan(&backupPath, &backupVersion)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("backup not found")
		}
		return fmt.Errorf("failed to get backup info: %w", err)
	}
	
	// Read backup file
	encryptedData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	
	// Restore vault (increment version to avoid conflicts)
	newVersion := backupVersion + 1000 // Large increment to avoid conflicts
	_, err = s.Store(ctx, userID, encryptedData, newVersion)
	if err != nil {
		return fmt.Errorf("failed to restore vault: %w", err)
	}
	
	return nil
}

// GetStorageInfo returns information about the local storage
func (s *LocalStorage) GetStorageInfo() StorageInfo {
	return StorageInfo{
		Type:     "local",
		Location: s.basePath,
		Capabilities: []string{
			"offline",
			"versioning", 
			"backup",
			"export",
			"zero_network_dependency",
		},
		Limits: map[string]interface{}{
			"max_vault_size": "1GB",
			"max_versions":   100,
			"max_backups":    50,
		},
		Health: "healthy",
	}
}

// HealthCheck verifies local storage is accessible
func (s *LocalStorage) HealthCheck(ctx context.Context) error {
	// Check database connection
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	
	// Check write permissions
	testFile := filepath.Join(s.basePath, ".health_check")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return fmt.Errorf("write permission test failed: %w", err)
	}
	os.Remove(testFile)
	
	return nil
}

// Close closes the local storage
func (s *LocalStorage) Close() error {
	return s.db.Close()
}

// Export creates a portable vault export
func (s *LocalStorage) Export(ctx context.Context, userID uuid.UUID, format string) ([]byte, error) {
	result, err := s.Retrieve(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vault for export: %w", err)
	}
	
	switch format {
	case "raw":
		return result.EncryptedData, nil
	case "json":
		export := map[string]interface{}{
			"version":        result.Version,
			"encrypted_data": hex.EncodeToString(result.EncryptedData),
			"checksum":       result.Checksum,
			"exported_at":    time.Now().Format(time.RFC3339),
			"storage_type":   "local",
		}
		return json.Marshal(export)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// Import imports vault data from an export
func (s *LocalStorage) Import(ctx context.Context, userID uuid.UUID, data []byte, format string) error {
	var encryptedData []byte
	var version int = 1
	
	switch format {
	case "raw":
		encryptedData = data
	case "json":
		var importData map[string]interface{}
		if err := json.Unmarshal(data, &importData); err != nil {
			return fmt.Errorf("invalid import format: %w", err)
		}
		
		hexData, ok := importData["encrypted_data"].(string)
		if !ok {
			return fmt.Errorf("missing encrypted_data in import")
		}
		
		var err error
		encryptedData, err = hex.DecodeString(hexData)
		if err != nil {
			return fmt.Errorf("invalid encrypted_data format: %w", err)
		}
		
		if v, ok := importData["version"].(float64); ok {
			version = int(v)
		}
	default:
		return fmt.Errorf("unsupported import format: %s", format)
	}
	
	// Store imported data
	_, err := s.Store(ctx, userID, encryptedData, version)
	if err != nil {
		return fmt.Errorf("failed to store imported data: %w", err)
	}
	
	return nil
}

// Helper functions

func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func initializeLocalSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS vaults (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		encrypted_data BLOB NOT NULL,
		checksum TEXT NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		last_modified DATETIME NOT NULL,
		created_at DATETIME NOT NULL,
		UNIQUE(user_id)
	);
	
	CREATE TABLE IF NOT EXISTS vault_history (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		encrypted_data BLOB NOT NULL,
		checksum TEXT NOT NULL,
		version INTEGER NOT NULL,
		created_at DATETIME NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS backups (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		version INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		size INTEGER NOT NULL,
		checksum TEXT NOT NULL,
		file_path TEXT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS sync_state (
		user_id TEXT PRIMARY KEY,
		last_sync DATETIME,
		remote_version INTEGER,
		local_version INTEGER,
		conflicts INTEGER DEFAULT 0
	);
	
	CREATE INDEX IF NOT EXISTS idx_vaults_user_id ON vaults(user_id);
	CREATE INDEX IF NOT EXISTS idx_vault_history_user_id ON vault_history(user_id);
	CREATE INDEX IF NOT EXISTS idx_backups_user_id ON backups(user_id);
	`
	
	_, err := db.Exec(schema)
	return err
}

import "encoding/json"