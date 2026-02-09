package observability

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var Logger *slog.Logger

// InitLogger initializes the global structured logger
func InitLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Use JSON handler for structured logging (stdout for k8s/loki)
	handler := slog.NewJSONHandler(os.Stdout, opts)
	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

// LoggingMiddleware returns a Gin middleware that logs requests using slog
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		// Build log attributes
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("body_size", c.Writer.Size()),
		}

		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		// Get user ID from context if available
		if userID, exists := c.Get("user_id"); exists {
			attrs = append(attrs, slog.Any("user_id", userID))
		}

		// Log based on status code
		msg := "request"
		if status >= 500 {
			Logger.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
		} else if status >= 400 {
			Logger.LogAttrs(context.Background(), slog.LevelWarn, msg, attrs...)
		} else {
			Logger.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
		}
	}
}

// Info logs an info message with structured fields
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// Error logs an error message with structured fields
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// Warn logs a warning message with structured fields
func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

// Debug logs a debug message with structured fields
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

// WithContext returns a logger with context values
func WithContext(ctx context.Context) *slog.Logger {
	return Logger.With()
}

// WithFields returns a logger with additional fields
func WithFields(args ...any) *slog.Logger {
	return Logger.With(args...)
}
