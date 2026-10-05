package api

import (
	"encoding/json"
	"net/http"

	"github.com/coldforge/vault/internal/config"
	"github.com/gin-gonic/gin"
)

// RuntimeConfigHandler serves /config.js: the service addresses the web UI
// reads through @cloistr/collab-common's getServiceConfig(). The body is
// computed once; it cannot change without a restart.
//
// no-store matters: a cached copy pins a browser (or an edge cache) to
// whichever environment it saw first.
func RuntimeConfigHandler(cc config.ClientConfig) gin.HandlerFunc {
	payload, _ := json.Marshal(map[string]string{
		"relayUrl":     cc.RelayURL,
		"blossomUrl":   cc.BlossomURL,
		"discoveryUrl": cc.DiscoveryURL,
		"signerUrl":    cc.SignerURL,
		"appUrl":       cc.AppURL,
		"environment":  cc.Environment,
	})
	body := []byte("window.__CLOISTR_CONFIG__=" + string(payload) + ";")
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", body)
	}
}
