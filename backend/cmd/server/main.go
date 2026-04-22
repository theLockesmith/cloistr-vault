package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.coldforge.xyz/coldforge/cloistr-common/relayprefs"
	"github.com/coldforge/vault/internal/api"
	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/config"
	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/kms"
	"github.com/coldforge/vault/internal/observability"
	"github.com/coldforge/vault/internal/security"
	"github.com/coldforge/vault/internal/vault"
)

func main() {
	// Initialize structured logger (JSON to stdout for k8s/loki)
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	observability.InitLogger(logLevel)

	observability.Info("starting coldforge vault api server",
		"version", "1.0.0",
		"service", "coldforge-vault",
	)

	// Load configuration
	cfg := config.LoadConfig()
	observability.Info("configuration loaded",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"environment", cfg.Server.Env,
	)

	// Connect to database
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		observability.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	observability.Info("database connected", "host", cfg.Database.Host)

	// Run database migrations
	if err := db.RunMigrations("./migrations"); err != nil {
		observability.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	observability.Info("database migrations completed")

	// Initialize KMS
	kmsConfig := &kms.Config{
		Provider:  cfg.KMS.Provider,
		Address:   cfg.KMS.Address,
		Token:     cfg.KMS.Token,
		MountPath: cfg.KMS.MountPath,
		Options: map[string]string{
			"key_dir": cfg.KMS.KeyDir,
		},
		AutoRotate: cfg.KMS.AutoRotate,
	}

	kmsInstance, err := kms.NewKMS(kmsConfig)
	if err != nil {
		observability.Warn("kms initialization failed, using fallback",
			"error", err,
			"fallback", "file",
		)
		// Fall back to file-based KMS
		kmsConfig.Provider = "file"
		kmsInstance, err = kms.NewKMS(kmsConfig)
		if err != nil {
			observability.Error("failed to initialize fallback kms", "error", err)
			os.Exit(1)
		}
	}

	// Initialize default keys
	if err := kms.InitializeDefaultKeys(kmsInstance); err != nil {
		observability.Warn("failed to initialize default keys", "error", err)
	}

	observability.Info("kms initialized", "provider", kmsConfig.Provider)

	// Initialize relay preferences client (for user relay preferences in NIP-05)
	relayPrefsClient := relayprefs.NewClientFromEnv()
	if err := relayPrefsClient.Validate(); err != nil {
		observability.Warn("relay prefs client validation warning", "error", err)
	}
	observability.Info("relay preferences client initialized",
		"use_cloistr_fallback", relayPrefsClient.Config().UseCloistrFallback,
	)

	// Initialize services
	authService := auth.NewAuthService(db.DB, relayPrefsClient)
	vaultService := vault.NewService(db)
	folderService := vault.NewFolderService(db)
	entryService := vault.NewEntryService(db)
	secretService := vault.NewSecretService(db)
	passwordService := vault.NewPasswordService(db)
	tagService := vault.NewTagService(db)
	searchService := vault.NewSearchService(db, entryService, folderService, tagService)
	securityService := security.NewSecurityService(db)
	attachmentService := vault.NewAttachmentService(db)
	sharingService := vault.NewSharingService(db)

	// Setup router
	router := api.SetupRouter(authService, vaultService, folderService, entryService, secretService, passwordService, tagService, searchService, securityService, attachmentService, sharingService)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		observability.Info("server listening",
			"addr", fmt.Sprintf("http://%s:%s", cfg.Server.Host, cfg.Server.Port),
			"metrics", fmt.Sprintf("http://%s:%s/metrics", cfg.Server.Host, cfg.Server.Port),
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			observability.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Start cleanup routine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := db.CleanupExpiredSessions(); err != nil {
					observability.Error("failed to cleanup expired sessions", "error", err)
				} else {
					observability.Debug("expired sessions cleaned up")
				}
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	observability.Info("shutdown signal received, gracefully stopping server")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		observability.Error("server forced to shutdown", "error", err)
	} else {
		observability.Info("server gracefully stopped")
	}
}
