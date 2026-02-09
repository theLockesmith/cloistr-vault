package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestsTotal counts total HTTP requests
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coldforge_vault_requests_total",
			Help: "Total HTTP requests processed",
		},
		[]string{"method", "path", "status"},
	)

	// RequestDuration measures HTTP request duration
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coldforge_vault_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// ErrorsTotal counts errors by type
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coldforge_vault_errors_total",
			Help: "Total errors by type",
		},
		[]string{"type"},
	)

	// ActiveSessions tracks current active sessions
	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "coldforge_vault_sessions_active",
			Help: "Current number of active sessions",
		},
	)

	// VaultOperationsTotal counts vault operations
	VaultOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coldforge_vault_operations_total",
			Help: "Total vault operations",
		},
		[]string{"operation", "status"},
	)

	// AuthAttemptsTotal counts authentication attempts
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coldforge_vault_auth_attempts_total",
			Help: "Total authentication attempts",
		},
		[]string{"method", "status"},
	)

	// DatabaseQueryDuration measures database query latency
	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coldforge_vault_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"query_type"},
	)
)

// MetricsHandler returns the Prometheus metrics handler for Gin
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// MetricsMiddleware records request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Process request
		c.Next()

		// Skip metrics endpoint itself
		if path == "/metrics" {
			return
		}

		duration := time.Since(start)
		status := strconv.Itoa(c.Writer.Status())

		// Record metrics
		RequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		RequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration.Seconds())

		// Record errors
		if c.Writer.Status() >= 400 {
			errorType := "client_error"
			if c.Writer.Status() >= 500 {
				errorType = "server_error"
			}
			ErrorsTotal.WithLabelValues(errorType).Inc()
		}
	}
}

// RecordAuthAttempt records an authentication attempt
func RecordAuthAttempt(method string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	AuthAttemptsTotal.WithLabelValues(method, status).Inc()
}

// RecordVaultOperation records a vault operation
func RecordVaultOperation(operation string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	VaultOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordDBQuery records a database query duration
func RecordDBQuery(queryType string, duration time.Duration) {
	DatabaseQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}
