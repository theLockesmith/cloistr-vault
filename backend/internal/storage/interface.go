package storage

import (
	"context"
	"time"
	
	"github.com/google/uuid"
)

// VaultStorage defines the interface for vault data storage
type VaultStorage interface {
	// Core operations
	Store(ctx context.Context, userID uuid.UUID, encryptedData []byte, version int) (*StoreResult, error)
	Retrieve(ctx context.Context, userID uuid.UUID) (*RetrieveResult, error)
	Delete(ctx context.Context, userID uuid.UUID) error
	
	// Metadata operations
	GetMetadata(ctx context.Context, userID uuid.UUID) (*VaultMetadata, error)
	ListVersions(ctx context.Context, userID uuid.UUID) ([]VaultVersion, error)
	
	// Backup operations
	CreateBackup(ctx context.Context, userID uuid.UUID) (*BackupInfo, error)
	RestoreFromBackup(ctx context.Context, userID uuid.UUID, backupID string) error
	
	// Storage info
	GetStorageInfo() StorageInfo
	HealthCheck(ctx context.Context) error
}

// RecoveryStorage defines the interface for recovery data storage
type RecoveryStorage interface {
	// Store encrypted recovery data (separate from main vault)
	StoreRecoveryData(ctx context.Context, userID uuid.UUID, encryptedRecoveryData []byte) error
	RetrieveRecoveryData(ctx context.Context, userID uuid.UUID, recoveryProof []byte) ([]byte, error)
	DeleteRecoveryData(ctx context.Context, userID uuid.UUID) error
	
	// Recovery attempt tracking
	LogRecoveryAttempt(ctx context.Context, userID uuid.UUID, attempt RecoveryAttempt) error
	GetRecoveryAttempts(ctx context.Context, userID uuid.UUID, since time.Time) ([]RecoveryAttempt, error)
}

// SyncManager handles synchronization between storage backends
type SyncManager interface {
	// Sync operations
	SyncVault(ctx context.Context, userID uuid.UUID) (*SyncResult, error)
	ResolveConflict(ctx context.Context, userID uuid.UUID, strategy ConflictStrategy) error
	
	// Sync status
	GetSyncStatus(ctx context.Context, userID uuid.UUID) (*SyncStatus, error)
	EnableSync(ctx context.Context, userID uuid.UUID, config SyncConfig) error
	DisableSync(ctx context.Context, userID uuid.UUID) error
}

// Data structures
type StoreResult struct {
	Version       int       `json:"version"`
	LastModified  time.Time `json:"last_modified"`
	Checksum      string    `json:"checksum"`
	StorageInfo   string    `json:"storage_info"`
}

type RetrieveResult struct {
	EncryptedData []byte    `json:"encrypted_data"`
	Version       int       `json:"version"`
	LastModified  time.Time `json:"last_modified"`
	Checksum      string    `json:"checksum"`
}

type VaultMetadata struct {
	UserID       uuid.UUID `json:"user_id"`
	Version      int       `json:"version"`
	LastModified time.Time `json:"last_modified"`
	CreatedAt    time.Time `json:"created_at"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	StorageType  string    `json:"storage_type"`
	BackupCount  int       `json:"backup_count"`
}

type VaultVersion struct {
	Version      int       `json:"version"`
	Timestamp    time.Time `json:"timestamp"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	Description  string    `json:"description,omitempty"`
}

type BackupInfo struct {
	ID           string    `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	StorageType  string    `json:"storage_type"`
	Encrypted    bool      `json:"encrypted"`
}

type StorageInfo struct {
	Type         string                 `json:"type"`
	Location     string                 `json:"location"`
	Capabilities []string               `json:"capabilities"`
	Limits       map[string]interface{} `json:"limits"`
	Health       string                 `json:"health"`
}

type RecoveryAttempt struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	Success      bool      `json:"success"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	RiskScore    float64   `json:"risk_score"`
}

type SyncResult struct {
	Conflicts    []ConflictInfo `json:"conflicts"`
	Merged       bool          `json:"merged"`
	NewVersion   int           `json:"new_version"`
	SyncedAt     time.Time     `json:"synced_at"`
}

type ConflictInfo struct {
	Field        string    `json:"field"`
	LocalValue   string    `json:"local_value"`
	RemoteValue  string    `json:"remote_value"`
	LocalTime    time.Time `json:"local_time"`
	RemoteTime   time.Time `json:"remote_time"`
}

type SyncStatus struct {
	Enabled      bool      `json:"enabled"`
	LastSync     time.Time `json:"last_sync"`
	Conflicts    int       `json:"conflicts"`
	PendingSync  bool      `json:"pending_sync"`
	StorageTypes []string  `json:"storage_types"`
}

type SyncConfig struct {
	PrimaryStorage   string            `json:"primary_storage"`
	BackupStorages   []string          `json:"backup_storages"`
	SyncInterval     time.Duration     `json:"sync_interval"`
	ConflictStrategy ConflictStrategy  `json:"conflict_strategy"`
	EncryptionLevel  string           `json:"encryption_level"`
}

type ConflictStrategy string

const (
	ConflictStrategyNewest    ConflictStrategy = "newest"
	ConflictStrategyManual    ConflictStrategy = "manual"
	ConflictStrategyMerge     ConflictStrategy = "merge"
	ConflictStrategyLocal     ConflictStrategy = "local_wins"
	ConflictStrategyRemote    ConflictStrategy = "remote_wins"
)

// StorageManager coordinates multiple storage backends
type StorageManager struct {
	primary   VaultStorage
	backups   []VaultStorage
	recovery  RecoveryStorage
	sync      SyncManager
	config    *StorageConfig
}

type StorageConfig struct {
	Mode            StorageMode           `json:"mode"`
	LocalPath       string               `json:"local_path,omitempty"`
	CloudEndpoint   string               `json:"cloud_endpoint,omitempty"`
	SelfHostedURL   string               `json:"self_hosted_url,omitempty"`
	P2PConfig       *P2PConfig           `json:"p2p_config,omitempty"`
	BackupTargets   []BackupTarget       `json:"backup_targets"`
	RecoveryEnabled bool                 `json:"recovery_enabled"`
	SyncSettings    *SyncSettings        `json:"sync_settings"`
}

type StorageMode string

const (
	StorageModeLocal      StorageMode = "local"
	StorageModeCloud      StorageMode = "cloud" 
	StorageModeSelfHosted StorageMode = "self_hosted"
	StorageModeHybrid     StorageMode = "hybrid"
	StorageModeP2P        StorageMode = "p2p"
)

type BackupTarget struct {
	Type     string                 `json:"type"`
	Config   map[string]interface{} `json:"config"`
	Enabled  bool                   `json:"enabled"`
	Schedule string                 `json:"schedule"` // cron format
}

type P2PConfig struct {
	NetworkID    string   `json:"network_id"`
	TrustedPeers []string `json:"trusted_peers"`
	DHT          bool     `json:"dht"`
	Encryption   bool     `json:"encryption"`
}

type SyncSettings struct {
	Interval         time.Duration    `json:"interval"`
	Strategy         ConflictStrategy `json:"strategy"`
	BackgroundSync   bool            `json:"background_sync"`
	WiFiOnly         bool            `json:"wifi_only"`
	BatteryOptimized bool            `json:"battery_optimized"`
}

// LocalConfig holds configuration for local SQLite storage
type LocalConfig struct {
	BasePath string `json:"base_path"`
}

// CloudConfig holds configuration for cloud storage
type CloudConfig struct {
	Endpoint  string `json:"endpoint"`
	APIKey    string `json:"api_key"`
	Region    string `json:"region"`
}

// SelfHostedConfig holds configuration for self-hosted storage
type SelfHostedConfig struct {
	URL      string `json:"url"`
	APIKey   string `json:"api_key"`
	TLS      bool   `json:"tls"`
}