package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AppleAppSiteAssociation serves the Apple App Site Association file for iOS passkeys
// This file is required for iOS to recognize our domain as associated with the app
func (h *Handlers) AppleAppSiteAssociation(c *gin.Context) {
	// Get app IDs from environment or use defaults
	// Format: TEAMID.bundleID (e.g., "ABCD1234.xyz.cloistr.vault")
	appIDs := os.Getenv("IOS_APP_IDS")
	if appIDs == "" {
		// Default app ID - update when iOS app is published
		appIDs = "TEAMID.xyz.cloistr.vault"
	}

	// Parse comma-separated app IDs
	appIDList := strings.Split(appIDs, ",")
	for i := range appIDList {
		appIDList[i] = strings.TrimSpace(appIDList[i])
	}

	// Build the AASA response
	// webcredentials is for passkeys/WebAuthn
	response := gin.H{
		"webcredentials": gin.H{
			"apps": appIDList,
		},
	}

	// Apple requires specific content type
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, response)
}

// AssetLinks serves the Android Digital Asset Links file for passkeys
// This file is required for Android to recognize our domain as associated with the app
func (h *Handlers) AssetLinks(c *gin.Context) {
	// Get package name and SHA-256 fingerprints from environment
	packageName := os.Getenv("ANDROID_PACKAGE_NAME")
	if packageName == "" {
		packageName = "xyz.cloistr.vault"
	}

	// SHA-256 fingerprints of the signing certificate(s)
	// Can be comma-separated for multiple certificates (debug, release, etc.)
	fingerprints := os.Getenv("ANDROID_CERT_FINGERPRINTS")
	if fingerprints == "" {
		// Placeholder - must be replaced with actual certificate fingerprints
		fingerprints = "00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00"
	}

	// Parse comma-separated fingerprints
	fingerprintList := strings.Split(fingerprints, ",")
	for i := range fingerprintList {
		fingerprintList[i] = strings.TrimSpace(fingerprintList[i])
	}

	// Build the asset links response
	// Each fingerprint gets its own entry
	var statements []gin.H
	for _, fp := range fingerprintList {
		statements = append(statements, gin.H{
			"relation": []string{
				"delegate_permission/common.handle_all_urls",
				"delegate_permission/common.get_login_creds",
			},
			"target": gin.H{
				"namespace":                "android_app",
				"package_name":             packageName,
				"sha256_cert_fingerprints": []string{fp},
			},
		})
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, statements)
}
