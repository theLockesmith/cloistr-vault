package api

import (
	"time"

	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/observability"
	"github.com/gin-gonic/gin"
)

func SetupRouter(authService *auth.AuthService, vaultService VaultService) *gin.Engine {
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
	}

	// .well-known endpoint at root level (required for NIP-05)
	router.GET("/.well-known/nostr.json", handlers.GetNostrJSON)
	
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
		
		// Vault routes
		vault := protected.Group("/vault")
		{
			vault.GET("", handlers.GetVault)
			vault.PUT("", ContentTypeMiddleware(), handlers.UpdateVault)
		}

		// Recovery routes (authenticated)
		recovery := protected.Group("/recovery")
		{
			recovery.GET("/status", handlers.GetRecoveryStatus)
			recovery.POST("/regenerate", handlers.RegenerateRecoveryCodes)
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
func SetupTestRouter(authService *auth.AuthService, vaultService VaultService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.Use(ErrorHandlingMiddleware())
	router.Use(CORSMiddleware())
	
	handlers := NewHandlers(authService, vaultService)
	
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
	}

	router.GET("/.well-known/nostr.json", handlers.GetNostrJSON)

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

		recovery := protected.Group("/recovery")
		{
			recovery.GET("/status", handlers.GetRecoveryStatus)
			recovery.POST("/regenerate", handlers.RegenerateRecoveryCodes)
		}
	}

	return router
}