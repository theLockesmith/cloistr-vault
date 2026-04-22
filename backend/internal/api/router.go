package api

import (
	"time"

	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/observability"
	"github.com/coldforge/vault/internal/security"
	"github.com/coldforge/vault/internal/vault"
	"github.com/gin-gonic/gin"
)

func SetupRouter(authService *auth.AuthService, vaultService VaultService, folderService *vault.FolderService, entryService *vault.EntryService, secretService *vault.SecretService, passwordService *vault.PasswordService, tagService *vault.TagService, searchService *vault.SearchService, securityService *security.SecurityService, attachmentService *vault.AttachmentService, sharingService *vault.SharingService) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode) // Change to gin.DebugMode for development

	router := gin.New()

	// Global middleware - use observability for logging and metrics
	router.Use(gin.Recovery())
	router.Use(observability.LoggingMiddleware())
	router.Use(observability.MetricsMiddleware())
	router.Use(ErrorHandlingMiddleware())
	router.Use(CORSMiddleware())
	router.Use(SecurityHeadersMiddleware())
	router.Use(RateLimitingMiddleware())
	router.Use(RequestTimeoutMiddleware(30 * time.Second))

	// Initialize handlers
	handlers := NewHandlers(authService, vaultService)
	folderHandlers := NewFolderHandlers(folderService)
	entryHandlers := NewEntryHandlers(entryService)
	secretHandlers := NewSecretHandlers(secretService, passwordService)
	tagHandlers := NewTagHandlers(tagService)
	searchHandlers := NewSearchHandlers(searchService)
	securityHandlers := NewSecurityHandlers(securityService)
	attachmentHandlers := NewAttachmentHandlers(attachmentService)
	sharingHandlers := NewSharingHandlers(sharingService)

	// Metrics endpoint (no auth required, for Prometheus scraping)
	router.GET("/metrics", observability.MetricsHandler())

	// Public routes
	public := router.Group("/api/v1")
	{
		// Health check
		public.GET("/health", handlers.HealthCheck)

		// API info
		public.GET("/info", handlers.GetAPIInfo)
		
		// Authentication routes
		auth := public.Group("/auth")
		{
			auth.POST("/register", ContentTypeMiddleware(), handlers.Register)
			auth.POST("/login", ContentTypeMiddleware(), handlers.Login)
			auth.POST("/nostr/challenge", ContentTypeMiddleware(), handlers.NostrChallenge)
			auth.POST("/lightning/challenge", ContentTypeMiddleware(), handlers.LightningChallenge)
			auth.POST("/recover", ContentTypeMiddleware(), handlers.RecoverAccount)
		}

		// NIP-05 public endpoints
		nip05 := public.Group("/nip05")
		{
			nip05.GET("/lookup", handlers.LookupNIP05)
		}

		// WebAuthn public endpoints (for login)
		webauthn := public.Group("/auth/webauthn")
		{
			webauthn.POST("/login/begin", ContentTypeMiddleware(), handlers.WebAuthnBeginLogin)
			webauthn.POST("/login/begin/discoverable", handlers.WebAuthnBeginDiscoverableLogin)
			webauthn.POST("/login/finish", ContentTypeMiddleware(), handlers.WebAuthnFinishLogin)
		}
	}

	// .well-known endpoints at root level
	router.GET("/.well-known/nostr.json", handlers.GetNostrJSON)

	// Passkey/WebAuthn domain association files
	// iOS: Apple App Site Association (AASA)
	router.GET("/.well-known/apple-app-site-association", handlers.AppleAppSiteAssociation)
	// Android: Digital Asset Links
	router.GET("/.well-known/assetlinks.json", handlers.AssetLinks)
	
	// Protected routes (require authentication)
	protected := router.Group("/api/v1")
	protected.Use(AuthMiddleware(authService))
	{
		// Authentication routes that require being logged in
		auth := protected.Group("/auth")
		{
			auth.POST("/logout", handlers.Logout)
		}
		
		// User routes
		user := protected.Group("/user")
		{
			user.GET("/profile", handlers.GetProfile)
		}

		// NIP-05 routes (authenticated)
		nip05 := protected.Group("/nip05")
		{
			nip05.POST("/verify", ContentTypeMiddleware(), handlers.VerifyNIP05)
		}
		
		// Vault routes (legacy blob-based)
		vault := protected.Group("/vault")
		{
			vault.GET("", handlers.GetVault)
			vault.PUT("", ContentTypeMiddleware(), handlers.UpdateVault)
		}

		// Folder routes (enhanced vault)
		folders := protected.Group("/folders")
		{
			folders.GET("", folderHandlers.ListFolders)
			folders.POST("", ContentTypeMiddleware(), folderHandlers.CreateFolder)
			folders.GET("/:id", folderHandlers.GetFolder)
			folders.PUT("/:id", ContentTypeMiddleware(), folderHandlers.UpdateFolder)
			folders.DELETE("/:id", folderHandlers.DeleteFolder)
			folders.POST("/reorder", ContentTypeMiddleware(), folderHandlers.ReorderFolders)
		}

		// Entry routes (enhanced vault)
		entries := protected.Group("/entries")
		{
			entries.GET("", entryHandlers.ListEntries)
			entries.POST("", ContentTypeMiddleware(), entryHandlers.CreateEntry)
			entries.GET("/:id", entryHandlers.GetEntry)
			entries.PUT("/:id", ContentTypeMiddleware(), entryHandlers.UpdateEntry)
			entries.DELETE("/:id", entryHandlers.DeleteEntry)
			entries.POST("/:id/usage", entryHandlers.RecordUsage)
			// Secret management within entries
			entries.GET("/:id/secrets", secretHandlers.ListSecrets)
			entries.POST("/:id/secrets", ContentTypeMiddleware(), secretHandlers.AddSecret)
			entries.PUT("/:id/secrets/:secretId", ContentTypeMiddleware(), secretHandlers.UpdateSecret)
			entries.DELETE("/:id/secrets/:secretId", secretHandlers.DeleteSecret)
			entries.POST("/:id/secrets/reorder", ContentTypeMiddleware(), secretHandlers.ReorderSecrets)
			// Attachment management within entries
			entries.GET("/:id/attachments", attachmentHandlers.ListAttachments)
			entries.POST("/:id/attachments", ContentTypeMiddleware(), attachmentHandlers.AddAttachment)
			entries.GET("/:id/attachments/:attachmentId", attachmentHandlers.GetAttachment)
			entries.GET("/:id/attachments/:attachmentId/meta", attachmentHandlers.GetAttachmentMetadata)
			entries.PUT("/:id/attachments/:attachmentId", ContentTypeMiddleware(), attachmentHandlers.UpdateAttachment)
			entries.DELETE("/:id/attachments/:attachmentId", attachmentHandlers.DeleteAttachment)
		}

		// Attachment storage usage
		attachments := protected.Group("/attachments")
		{
			attachments.GET("/usage", attachmentHandlers.GetStorageUsage)
		}

		// Password generator
		password := protected.Group("/password")
		{
			password.POST("/generate", ContentTypeMiddleware(), secretHandlers.GeneratePassword)
			password.GET("/history", secretHandlers.GetPasswordHistory)
		}

		// Tag routes
		tags := protected.Group("/tags")
		{
			tags.GET("", tagHandlers.ListTags)
			tags.POST("", ContentTypeMiddleware(), tagHandlers.CreateTag)
			tags.GET("/:id", tagHandlers.GetTag)
			tags.PUT("/:id", ContentTypeMiddleware(), tagHandlers.UpdateTag)
			tags.DELETE("/:id", tagHandlers.DeleteTag)
			tags.GET("/:id/entries", tagHandlers.GetTagEntries)
			tags.POST("/init", tagHandlers.InitializeSystemTags)
		}

		// Search routes
		search := protected.Group("/search")
		{
			search.GET("", searchHandlers.Search)
			search.GET("/recent", searchHandlers.GetRecentEntries)
			search.GET("/frequent", searchHandlers.GetFrequentEntries)
		}

		// Security routes
		sec := protected.Group("/security")
		{
			sec.GET("/score", securityHandlers.GetSecurityScore)
			sec.POST("/analyze-password", ContentTypeMiddleware(), securityHandlers.AnalyzePassword)
			sec.POST("/check-breach", ContentTypeMiddleware(), securityHandlers.CheckBreach)
			sec.GET("/weak-passwords", securityHandlers.GetWeakPasswords)
			sec.GET("/breached-passwords", securityHandlers.GetBreachedPasswords)
			sec.GET("/reused-passwords", securityHandlers.GetReusedPasswords)
			sec.GET("/expiring-secrets", securityHandlers.GetExpiringSecrets)
		}

		// Team routes
		teams := protected.Group("/teams")
		{
			teams.GET("", sharingHandlers.ListTeams)
			teams.POST("", ContentTypeMiddleware(), sharingHandlers.CreateTeam)
			teams.GET("/:id", sharingHandlers.GetTeam)
			teams.GET("/:id/members", sharingHandlers.GetTeamMembers)
			teams.POST("/:id/invite", ContentTypeMiddleware(), sharingHandlers.InviteToTeam)
		}

		// Team invitation routes
		invitations := protected.Group("/invitations")
		{
			invitations.POST("/:id/accept", sharingHandlers.AcceptTeamInvitation)
		}

		// Sharing routes
		sharing := protected.Group("/sharing")
		{
			sharing.POST("/folder", ContentTypeMiddleware(), sharingHandlers.ShareFolder)
			sharing.GET("/folders", sharingHandlers.GetSharedFolders)
			sharing.GET("/folders/:id/key", sharingHandlers.GetFolderKey)
			sharing.DELETE("/:id", sharingHandlers.RevokeShare)
		}

		// Recovery routes (authenticated)
		recovery := protected.Group("/recovery")
		{
			recovery.GET("/status", handlers.GetRecoveryStatus)
			recovery.POST("/regenerate", handlers.RegenerateRecoveryCodes)
		}

		// WebAuthn routes (authenticated - for credential management)
		webauthn := protected.Group("/user/webauthn")
		{
			webauthn.POST("/register/begin", handlers.WebAuthnBeginRegistration)
			webauthn.POST("/register/finish", ContentTypeMiddleware(), handlers.WebAuthnFinishRegistration)
			webauthn.GET("/credentials", handlers.ListWebAuthnCredentials)
			webauthn.DELETE("/credentials/:id", handlers.DeleteWebAuthnCredential)
			webauthn.PUT("/credentials/:id", ContentTypeMiddleware(), handlers.UpdateWebAuthnCredential)
		}
	}
	
	// Add a catch-all route for 404s
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error":   "Not Found",
			"message": "The requested endpoint does not exist",
		})
	})
	
	return router
}

// SetupTestRouter creates a router for testing with minimal middleware
func SetupTestRouter(authService *auth.AuthService, vaultService VaultService, folderService *vault.FolderService, entryService *vault.EntryService, secretService *vault.SecretService, passwordService *vault.PasswordService, tagService *vault.TagService, searchService *vault.SearchService, securityService *security.SecurityService, attachmentService *vault.AttachmentService, sharingService *vault.SharingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.Use(ErrorHandlingMiddleware())
	router.Use(CORSMiddleware())
	
	handlers := NewHandlers(authService, vaultService)
	folderHandlers := NewFolderHandlers(folderService)
	entryHandlers := NewEntryHandlers(entryService)
	secretHandlers := NewSecretHandlers(secretService, passwordService)
	tagHandlers := NewTagHandlers(tagService)
	searchHandlers := NewSearchHandlers(searchService)
	securityHandlers := NewSecurityHandlers(securityService)
	attachmentHandlers := NewAttachmentHandlers(attachmentService)
	sharingHandlers := NewSharingHandlers(sharingService)

	// Public routes
	public := router.Group("/api/v1")
	{
		public.GET("/health", handlers.HealthCheck)
		public.GET("/info", handlers.GetAPIInfo)

		auth := public.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.POST("/nostr/challenge", handlers.NostrChallenge)
			auth.POST("/lightning/challenge", handlers.LightningChallenge)
			auth.POST("/recover", handlers.RecoverAccount)
		}

		nip05 := public.Group("/nip05")
		{
			nip05.GET("/lookup", handlers.LookupNIP05)
		}

		webauthn := public.Group("/auth/webauthn")
		{
			webauthn.POST("/login/begin", handlers.WebAuthnBeginLogin)
			webauthn.POST("/login/begin/discoverable", handlers.WebAuthnBeginDiscoverableLogin)
			webauthn.POST("/login/finish", handlers.WebAuthnFinishLogin)
		}
	}

	router.GET("/.well-known/nostr.json", handlers.GetNostrJSON)
	router.GET("/.well-known/apple-app-site-association", handlers.AppleAppSiteAssociation)
	router.GET("/.well-known/assetlinks.json", handlers.AssetLinks)

	// Protected routes
	protected := router.Group("/api/v1")
	protected.Use(AuthMiddleware(authService))
	{
		auth := protected.Group("/auth")
		{
			auth.POST("/logout", handlers.Logout)
		}

		user := protected.Group("/user")
		{
			user.GET("/profile", handlers.GetProfile)
		}

		nip05 := protected.Group("/nip05")
		{
			nip05.POST("/verify", handlers.VerifyNIP05)
		}

		vault := protected.Group("/vault")
		{
			vault.GET("", handlers.GetVault)
			vault.PUT("", handlers.UpdateVault)
		}

		folders := protected.Group("/folders")
		{
			folders.GET("", folderHandlers.ListFolders)
			folders.POST("", folderHandlers.CreateFolder)
			folders.GET("/:id", folderHandlers.GetFolder)
			folders.PUT("/:id", folderHandlers.UpdateFolder)
			folders.DELETE("/:id", folderHandlers.DeleteFolder)
			folders.POST("/reorder", folderHandlers.ReorderFolders)
		}

		entries := protected.Group("/entries")
		{
			entries.GET("", entryHandlers.ListEntries)
			entries.POST("", entryHandlers.CreateEntry)
			entries.GET("/:id", entryHandlers.GetEntry)
			entries.PUT("/:id", entryHandlers.UpdateEntry)
			entries.DELETE("/:id", entryHandlers.DeleteEntry)
			entries.POST("/:id/usage", entryHandlers.RecordUsage)
			entries.GET("/:id/secrets", secretHandlers.ListSecrets)
			entries.POST("/:id/secrets", secretHandlers.AddSecret)
			entries.PUT("/:id/secrets/:secretId", secretHandlers.UpdateSecret)
			entries.DELETE("/:id/secrets/:secretId", secretHandlers.DeleteSecret)
			entries.POST("/:id/secrets/reorder", secretHandlers.ReorderSecrets)
			entries.GET("/:id/attachments", attachmentHandlers.ListAttachments)
			entries.POST("/:id/attachments", attachmentHandlers.AddAttachment)
			entries.GET("/:id/attachments/:attachmentId", attachmentHandlers.GetAttachment)
			entries.GET("/:id/attachments/:attachmentId/meta", attachmentHandlers.GetAttachmentMetadata)
			entries.PUT("/:id/attachments/:attachmentId", attachmentHandlers.UpdateAttachment)
			entries.DELETE("/:id/attachments/:attachmentId", attachmentHandlers.DeleteAttachment)
		}

		attachments := protected.Group("/attachments")
		{
			attachments.GET("/usage", attachmentHandlers.GetStorageUsage)
		}

		password := protected.Group("/password")
		{
			password.POST("/generate", secretHandlers.GeneratePassword)
			password.GET("/history", secretHandlers.GetPasswordHistory)
		}

		tags := protected.Group("/tags")
		{
			tags.GET("", tagHandlers.ListTags)
			tags.POST("", tagHandlers.CreateTag)
			tags.GET("/:id", tagHandlers.GetTag)
			tags.PUT("/:id", tagHandlers.UpdateTag)
			tags.DELETE("/:id", tagHandlers.DeleteTag)
			tags.GET("/:id/entries", tagHandlers.GetTagEntries)
			tags.POST("/init", tagHandlers.InitializeSystemTags)
		}

		search := protected.Group("/search")
		{
			search.GET("", searchHandlers.Search)
			search.GET("/recent", searchHandlers.GetRecentEntries)
			search.GET("/frequent", searchHandlers.GetFrequentEntries)
		}

		sec := protected.Group("/security")
		{
			sec.GET("/score", securityHandlers.GetSecurityScore)
			sec.POST("/analyze-password", securityHandlers.AnalyzePassword)
			sec.POST("/check-breach", securityHandlers.CheckBreach)
			sec.GET("/weak-passwords", securityHandlers.GetWeakPasswords)
			sec.GET("/breached-passwords", securityHandlers.GetBreachedPasswords)
			sec.GET("/reused-passwords", securityHandlers.GetReusedPasswords)
			sec.GET("/expiring-secrets", securityHandlers.GetExpiringSecrets)
		}

		teams := protected.Group("/teams")
		{
			teams.GET("", sharingHandlers.ListTeams)
			teams.POST("", sharingHandlers.CreateTeam)
			teams.GET("/:id", sharingHandlers.GetTeam)
			teams.GET("/:id/members", sharingHandlers.GetTeamMembers)
			teams.POST("/:id/invite", sharingHandlers.InviteToTeam)
		}

		invitations := protected.Group("/invitations")
		{
			invitations.POST("/:id/accept", sharingHandlers.AcceptTeamInvitation)
		}

		sharing := protected.Group("/sharing")
		{
			sharing.POST("/folder", sharingHandlers.ShareFolder)
			sharing.GET("/folders", sharingHandlers.GetSharedFolders)
			sharing.GET("/folders/:id/key", sharingHandlers.GetFolderKey)
			sharing.DELETE("/:id", sharingHandlers.RevokeShare)
		}

		recovery := protected.Group("/recovery")
		{
			recovery.GET("/status", handlers.GetRecoveryStatus)
			recovery.POST("/regenerate", handlers.RegenerateRecoveryCodes)
		}

		webauthn := protected.Group("/user/webauthn")
		{
			webauthn.POST("/register/begin", handlers.WebAuthnBeginRegistration)
			webauthn.POST("/register/finish", handlers.WebAuthnFinishRegistration)
			webauthn.GET("/credentials", handlers.ListWebAuthnCredentials)
			webauthn.DELETE("/credentials/:id", handlers.DeleteWebAuthnCredential)
			webauthn.PUT("/credentials/:id", handlers.UpdateWebAuthnCredential)
		}
	}

	return router
}