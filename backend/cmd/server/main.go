package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coldforge/vault/internal/api"
	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/config"
	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/kms"
	"github.com/coldforge/vault/internal/vault"
)

func main() {
	log.Println("Starting Coldforge Vault API Server...")
	
	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
	
	// Connect to database
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Run database migrations
	if err := db.RunMigrations("./migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

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
		log.Printf("Warning: Failed to initialize KMS, using fallback: %v", err)
		// Fall back to file-based KMS
		kmsConfig.Provider = "file"
		kmsInstance, err = kms.NewKMS(kmsConfig)
		if err != nil {
			log.Fatalf("Failed to initialize fallback KMS: %v", err)
		}
	}

	// Initialize default keys
	if err := kms.InitializeDefaultKeys(kmsInstance); err != nil {
		log.Printf("Warning: Failed to initialize default keys: %v", err)
	}

	log.Printf("KMS initialized with provider: %s", kmsConfig.Provider)

	// Initialize services
	authService := auth.NewAuthService(db.DB)
	vaultService := vault.NewService(db)
	
	// Setup router
	router := api.SetupRouter(authService, vaultService)
	
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
		log.Printf("Server listening on http://%s:%s", cfg.Server.Host, cfg.Server.Port)
		log.Printf("API documentation: http://%s:%s/api/v1/info", cfg.Server.Host, cfg.Server.Port)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
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
					log.Printf("Failed to cleanup expired sessions: %v", err)
				}
			}
		}
	}()
	
	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	
	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}

