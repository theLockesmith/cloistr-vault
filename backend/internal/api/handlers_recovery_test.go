package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coldforge/vault/internal/auth"
	"github.com/coldforge/vault/internal/models"
	"github.com/coldforge/vault/internal/observability"
	"github.com/coldforge/vault/internal/recovery"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Initialize logger for tests
	observability.InitLogger("error")
	os.Exit(m.Run())
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

// TestRecoverAccount_Success tests the POST /api/v1/auth/recover endpoint
// NOTE: Full success test requires integration testing with real DB due to scrypt hash comparison.
// The recovery service itself is fully unit tested in recovery_test.go.
// This test is skipped in unit tests - use integration tests for full flow verification.

func TestRecoverAccount_InvalidRequestFormat(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	router.POST("/api/v1/auth/recover", handlers.RecoverAccount)

	// Send invalid JSON
	req, _ := http.NewRequest("POST", "/api/v1/auth/recover", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid request format", response["error"])
}

func TestRecoverAccount_UserNotFound(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	router.POST("/api/v1/auth/recover", handlers.RecoverAccount)

	email := "nonexistent@example.com"

	// Mock user lookup - no rows
	mock.ExpectQuery(`SELECT u.id, u.email, u.created_at, u.updated_at FROM users u JOIN auth_methods am`).
		WithArgs(email).
		WillReturnError(sql.ErrNoRows)

	reqBody := models.RecoveryRequest{
		Email:        email,
		RecoveryCode: "ABCD-EFGH-IJKL",
		NewPassword:  "new-password",
		VaultData:    []byte("vault-data"),
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/auth/recover", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "User not found", response["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRecoverAccount_InvalidRecoveryCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	router.POST("/api/v1/auth/recover", handlers.RecoverAccount)

	userID := uuid.New()
	email := "test@example.com"

	// Mock user lookup
	userRows := sqlmock.NewRows([]string{"id", "email", "created_at", "updated_at"}).
		AddRow(userID, email, time.Now(), time.Now())

	mock.ExpectQuery(`SELECT u.id, u.email, u.created_at, u.updated_at FROM users u JOIN auth_methods am`).
		WithArgs(email).
		WillReturnRows(userRows)

	// Mock recovery code validation - no matching codes
	codeRows := sqlmock.NewRows([]string{"id", "code_hash", "salt"})

	mock.ExpectQuery(`SELECT id, code_hash, salt FROM recovery_codes WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(codeRows)

	reqBody := models.RecoveryRequest{
		Email:        email,
		RecoveryCode: "WRONG-CODE-HERE",
		NewPassword:  "new-password",
		VaultData:    []byte("vault-data"),
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/auth/recover", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid or expired recovery code", response["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetRecoveryStatus tests the GET /api/v1/recovery/status endpoint
func TestGetRecoveryStatus_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	// Add auth middleware simulation
	userID := uuid.New()
	router.GET("/api/v1/recovery/status", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handlers.GetRecoveryStatus(c)
	})

	now := time.Now()
	usedAt := now.Add(-1 * time.Hour)

	// Mock GetCodeStatus query
	codeRows := sqlmock.NewRows([]string{"id", "user_id", "used", "created_at", "used_at"}).
		AddRow(uuid.New(), userID, false, now, nil).
		AddRow(uuid.New(), userID, false, now, nil).
		AddRow(uuid.New(), userID, false, now, nil).
		AddRow(uuid.New(), userID, true, now, usedAt).
		AddRow(uuid.New(), userID, true, now, usedAt)

	mock.ExpectQuery(`SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(codeRows)

	req, _ := http.NewRequest("GET", "/api/v1/recovery/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.RecoveryStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 5, response.Total)
	assert.Equal(t, 3, response.Remaining)
	assert.Equal(t, 2, response.Used)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRecoveryStatus_NoUserID(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	// No userID in context
	router.GET("/api/v1/recovery/status", handlers.GetRecoveryStatus)

	req, _ := http.NewRequest("GET", "/api/v1/recovery/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "User ID not found", response["error"])
}

func TestGetRecoveryStatus_InvalidUserID(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	// Invalid userID in context
	router.GET("/api/v1/recovery/status", func(c *gin.Context) {
		c.Set("userID", "invalid-uuid")
		handlers.GetRecoveryStatus(c)
	})

	req, _ := http.NewRequest("GET", "/api/v1/recovery/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid user ID", response["error"])
}

func TestGetRecoveryStatus_DatabaseError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	userID := uuid.New()
	router.GET("/api/v1/recovery/status", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handlers.GetRecoveryStatus(c)
	})

	// Mock database error
	mock.ExpectQuery(`SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id`).
		WithArgs(userID).
		WillReturnError(errors.New("database error"))

	req, _ := http.NewRequest("GET", "/api/v1/recovery/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Failed to get recovery status", response["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRecoveryStatus_NoCodes(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	userID := uuid.New()
	router.GET("/api/v1/recovery/status", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handlers.GetRecoveryStatus(c)
	})

	// Mock empty result
	codeRows := sqlmock.NewRows([]string{"id", "user_id", "used", "created_at", "used_at"})

	mock.ExpectQuery(`SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(codeRows)

	req, _ := http.NewRequest("GET", "/api/v1/recovery/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.RecoveryStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 0, response.Total)
	assert.Equal(t, 0, response.Remaining)
	assert.Equal(t, 0, response.Used)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRegenerateRecoveryCodes tests the POST /api/v1/recovery/regenerate endpoint
func TestRegenerateRecoveryCodes_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	userID := uuid.New()
	router.POST("/api/v1/recovery/regenerate", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handlers.RegenerateRecoveryCodes(c)
	})

	// Mock transaction
	mock.ExpectBegin()

	// Mock DELETE of existing codes
	mock.ExpectExec(`DELETE FROM recovery_codes WHERE user_id`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 3))

	// Mock INSERT of new codes
	for i := 0; i < recovery.NumRecoveryCodes; i++ {
		mock.ExpectExec(`INSERT INTO recovery_codes`).
			WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg(), false, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	mock.ExpectCommit()

	req, _ := http.NewRequest("POST", "/api/v1/recovery/regenerate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response["codes"])
	assert.NotEmpty(t, response["warning"])

	codes := response["codes"].([]interface{})
	assert.Len(t, codes, recovery.NumRecoveryCodes)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegenerateRecoveryCodes_NoUserID(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	router.POST("/api/v1/recovery/regenerate", handlers.RegenerateRecoveryCodes)

	req, _ := http.NewRequest("POST", "/api/v1/recovery/regenerate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "User ID not found", response["error"])
}

func TestRegenerateRecoveryCodes_InvalidUserID(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	router.POST("/api/v1/recovery/regenerate", func(c *gin.Context) {
		c.Set("userID", "invalid-uuid")
		handlers.RegenerateRecoveryCodes(c)
	})

	req, _ := http.NewRequest("POST", "/api/v1/recovery/regenerate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid user ID", response["error"])
}

func TestRegenerateRecoveryCodes_DatabaseError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	userID := uuid.New()
	router.POST("/api/v1/recovery/regenerate", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handlers.RegenerateRecoveryCodes(c)
	})

	// Mock transaction begin failure
	mock.ExpectBegin().WillReturnError(errors.New("database error"))

	req, _ := http.NewRequest("POST", "/api/v1/recovery/regenerate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Failed to regenerate recovery codes", response["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Edge case tests

func TestRecoverAccount_MissingFields(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	router.POST("/api/v1/auth/recover", handlers.RecoverAccount)

	tests := []struct {
		name    string
		request models.RecoveryRequest
	}{
		{
			name: "missing email",
			request: models.RecoveryRequest{
				RecoveryCode: "ABCD-EFGH-IJKL",
				NewPassword:  "password",
				VaultData:    []byte("data"),
			},
		},
		{
			name: "missing recovery code",
			request: models.RecoveryRequest{
				Email:       "test@example.com",
				NewPassword: "password",
				VaultData:   []byte("data"),
			},
		},
		{
			name: "missing password",
			request: models.RecoveryRequest{
				Email:        "test@example.com",
				RecoveryCode: "ABCD-EFGH-IJKL",
				VaultData:    []byte("data"),
			},
		},
		{
			name: "missing vault data",
			request: models.RecoveryRequest{
				Email:        "test@example.com",
				RecoveryCode: "ABCD-EFGH-IJKL",
				NewPassword:  "password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req, _ := http.NewRequest("POST", "/api/v1/auth/recover", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Gin's binding should fail with 400
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetRecoveryStatus_AllCodesUsed(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	router := setupTestRouter()
	authService := auth.NewAuthService(db)
	handlers := NewHandlers(authService, nil)

	userID := uuid.New()
	router.GET("/api/v1/recovery/status", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handlers.GetRecoveryStatus(c)
	})

	now := time.Now()
	usedAt := now.Add(-1 * time.Hour)

	// All codes used
	codeRows := sqlmock.NewRows([]string{"id", "user_id", "used", "created_at", "used_at"})
	for i := 0; i < 8; i++ {
		codeRows.AddRow(uuid.New(), userID, true, now, usedAt)
	}

	mock.ExpectQuery(`SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(codeRows)

	req, _ := http.NewRequest("GET", "/api/v1/recovery/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.RecoveryStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 8, response.Total)
	assert.Equal(t, 0, response.Remaining)
	assert.Equal(t, 8, response.Used)

	assert.NoError(t, mock.ExpectationsWereMet())
}

