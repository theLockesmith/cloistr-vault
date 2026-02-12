package recovery

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coldforge/vault/internal/observability"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Initialize logger for tests
	observability.InitLogger("error")
	os.Exit(m.Run())
}

// setupMockDB creates a mock database connection
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func TestNewService(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	service := NewService(db)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
}

func TestGenerateCodes_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	// Begin transaction (not actually needed for test since we pass tx)
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)

	// Expect DELETE of existing codes
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM recovery_codes WHERE user_id = $1")).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect 8 INSERT statements (NumRecoveryCodes = 8)
	for i := 0; i < NumRecoveryCodes; i++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO recovery_codes")).
			WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg(), false, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	codes, err := service.GenerateCodes(tx, userID)

	require.NoError(t, err)
	assert.NotNil(t, codes)
	assert.Len(t, codes.Codes, NumRecoveryCodes)
	assert.NotEmpty(t, codes.Warning)
	assert.False(t, codes.CreatedAt.IsZero())

	// Verify all codes are unique and properly formatted
	seenCodes := make(map[string]bool)
	for _, code := range codes.Codes {
		assert.NotEmpty(t, code)
		assert.Len(t, code, 14) // XXXX-XXXX-XXXX format (12 chars + 2 dashes)
		assert.False(t, seenCodes[code], "duplicate code found: %s", code)
		seenCodes[code] = true
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateCodes_DeleteError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)

	// Expect DELETE to fail
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM recovery_codes WHERE user_id = $1")).
		WithArgs(userID).
		WillReturnError(errors.New("delete failed"))

	codes, err := service.GenerateCodes(tx, userID)

	assert.Error(t, err)
	assert.Nil(t, codes)
	assert.Contains(t, err.Error(), "failed to clear existing codes")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateCodes_InsertError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)

	// DELETE succeeds
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM recovery_codes WHERE user_id = $1")).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// First INSERT fails
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO recovery_codes")).
		WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg(), false, sqlmock.AnyArg()).
		WillReturnError(errors.New("insert failed"))

	codes, err := service.GenerateCodes(tx, userID)

	assert.Error(t, err)
	assert.Nil(t, codes)
	assert.Contains(t, err.Error(), "failed to store code")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateCode_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID := uuid.New()

	// Generate a code and its hash
	code := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678") // 16 bytes
	codeHash, err := hashCode(code, salt)
	require.NoError(t, err)

	// Mock query to return matching code
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
		AddRow(codeID, codeHash, salt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnRows(rows)

	valid, err := service.ValidateCode(userID, code)

	require.NoError(t, err)
	assert.True(t, valid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateCode_InvalidCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID := uuid.New()

	// Generate a different code hash
	correctCode := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")
	codeHash, err := hashCode(correctCode, salt)
	require.NoError(t, err)

	// Mock query returns codes
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
		AddRow(codeID, codeHash, salt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnRows(rows)

	// Try with wrong code
	valid, err := service.ValidateCode(userID, "WRONG-CODE-HERE")

	require.NoError(t, err)
	assert.False(t, valid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateCode_NoCodesFound(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	// Mock query returns no rows
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnRows(rows)

	valid, err := service.ValidateCode(userID, "ABCD-EFGH-IJKL")

	require.NoError(t, err)
	assert.False(t, valid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateCode_DatabaseError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnError(errors.New("database error"))

	valid, err := service.ValidateCode(userID, "ABCD-EFGH-IJKL")

	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "failed to query codes")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsumeCode_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID := uuid.New()

	// Generate a code and its hash
	code := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")
	codeHash, err := hashCode(code, salt)
	require.NoError(t, err)

	// Mock query to return matching code
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
		AddRow(codeID, codeHash, salt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false FOR UPDATE")).
		WithArgs(userID).
		WillReturnRows(rows)

	// Mock update to mark code as used
	mock.ExpectExec(regexp.QuoteMeta("UPDATE recovery_codes SET used = true, used_at = $1 WHERE id = $2 AND used = false")).
		WithArgs(sqlmock.AnyArg(), codeID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	returnedCodeID, err := service.ConsumeCode(userID, code)

	require.NoError(t, err)
	assert.Equal(t, codeID, returnedCodeID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsumeCode_InvalidCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID := uuid.New()

	// Generate a different code hash
	correctCode := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")
	codeHash, err := hashCode(correctCode, salt)
	require.NoError(t, err)

	// Mock query returns codes
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
		AddRow(codeID, codeHash, salt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false FOR UPDATE")).
		WithArgs(userID).
		WillReturnRows(rows)

	// No UPDATE should be called since code doesn't match

	returnedCodeID, err := service.ConsumeCode(userID, "WRONG-CODE-HERE")

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidCode, err)
	assert.Equal(t, uuid.Nil, returnedCodeID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsumeCode_AlreadyUsed(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID := uuid.New()

	// Generate a code and its hash
	code := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")
	codeHash, err := hashCode(code, salt)
	require.NoError(t, err)

	// Mock query to return matching code
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
		AddRow(codeID, codeHash, salt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false FOR UPDATE")).
		WithArgs(userID).
		WillReturnRows(rows)

	// Mock update with 0 rows affected (code already used)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE recovery_codes SET used = true, used_at = $1 WHERE id = $2 AND used = false")).
		WithArgs(sqlmock.AnyArg(), codeID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	returnedCodeID, err := service.ConsumeCode(userID, code)

	assert.Error(t, err)
	assert.Equal(t, ErrCodeAlreadyUsed, err)
	assert.Equal(t, uuid.Nil, returnedCodeID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsumeCode_UpdateError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID := uuid.New()

	// Generate a code and its hash
	code := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")
	codeHash, err := hashCode(code, salt)
	require.NoError(t, err)

	// Mock query to return matching code
	rows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
		AddRow(codeID, codeHash, salt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code_hash, salt FROM recovery_codes WHERE user_id = $1 AND used = false FOR UPDATE")).
		WithArgs(userID).
		WillReturnRows(rows)

	// Mock update with error
	mock.ExpectExec(regexp.QuoteMeta("UPDATE recovery_codes SET used = true, used_at = $1 WHERE id = $2 AND used = false")).
		WithArgs(sqlmock.AnyArg(), codeID).
		WillReturnError(errors.New("database error"))

	returnedCodeID, err := service.ConsumeCode(userID, code)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to mark code as used")
	assert.Equal(t, uuid.Nil, returnedCodeID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRemainingCount_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	// Mock query to return count
	rows := sqlmock.NewRows([]string{"count"}).AddRow(5)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnRows(rows)

	count, err := service.GetRemainingCount(userID)

	require.NoError(t, err)
	assert.Equal(t, 5, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRemainingCount_Zero(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	// Mock query to return zero count
	rows := sqlmock.NewRows([]string{"count"}).AddRow(0)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnRows(rows)

	count, err := service.GetRemainingCount(userID)

	require.NoError(t, err)
	assert.Equal(t, 0, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRemainingCount_DatabaseError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM recovery_codes WHERE user_id = $1 AND used = false")).
		WithArgs(userID).
		WillReturnError(errors.New("database error"))

	count, err := service.GetRemainingCount(userID)

	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "failed to count codes")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCodeStatus_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()
	codeID1 := uuid.New()
	codeID2 := uuid.New()
	now := time.Now()
	usedAt := now.Add(-1 * time.Hour)

	// Mock query to return status
	rows := sqlmock.NewRows([]string{"id", "user_id", "used", "created_at", "used_at"}).
		AddRow(codeID1, userID, false, now, nil).
		AddRow(codeID2, userID, true, now, usedAt)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id = $1 ORDER BY created_at")).
		WithArgs(userID).
		WillReturnRows(rows)

	codes, err := service.GetCodeStatus(userID)

	require.NoError(t, err)
	assert.Len(t, codes, 2)
	assert.Equal(t, codeID1, codes[0].ID)
	assert.False(t, codes[0].Used)
	assert.Nil(t, codes[0].UsedAt)
	assert.Equal(t, codeID2, codes[1].ID)
	assert.True(t, codes[1].Used)
	assert.NotNil(t, codes[1].UsedAt)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCodeStatus_EmptyResult(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	// Mock query to return no rows
	rows := sqlmock.NewRows([]string{"id", "user_id", "used", "created_at", "used_at"})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id = $1 ORDER BY created_at")).
		WithArgs(userID).
		WillReturnRows(rows)

	codes, err := service.GetCodeStatus(userID)

	require.NoError(t, err)
	assert.Len(t, codes, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCodeStatus_DatabaseError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, used, created_at, used_at FROM recovery_codes WHERE user_id = $1 ORDER BY created_at")).
		WithArgs(userID).
		WillReturnError(errors.New("database error"))

	codes, err := service.GetCodeStatus(userID)

	assert.Error(t, err)
	assert.Nil(t, codes)
	assert.Contains(t, err.Error(), "failed to query codes")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegenerateCodes_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	// Expect transaction
	mock.ExpectBegin()

	// Expect DELETE of existing codes
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM recovery_codes WHERE user_id = $1")).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 3)) // 3 old codes deleted

	// Expect 8 INSERT statements
	for i := 0; i < NumRecoveryCodes; i++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO recovery_codes")).
			WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg(), false, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	mock.ExpectCommit()

	codes, err := service.RegenerateCodes(userID)

	require.NoError(t, err)
	assert.NotNil(t, codes)
	assert.Len(t, codes.Codes, NumRecoveryCodes)
	assert.NotEmpty(t, codes.Warning)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegenerateCodes_BeginError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

	codes, err := service.RegenerateCodes(userID)

	assert.Error(t, err)
	assert.Nil(t, codes)
	assert.Contains(t, err.Error(), "failed to begin transaction")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegenerateCodes_GenerateError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectBegin()

	// DELETE fails
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM recovery_codes WHERE user_id = $1")).
		WithArgs(userID).
		WillReturnError(errors.New("delete failed"))

	mock.ExpectRollback()

	codes, err := service.RegenerateCodes(userID)

	assert.Error(t, err)
	assert.Nil(t, codes)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegenerateCodes_CommitError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	service := NewService(db)
	userID := uuid.New()

	mock.ExpectBegin()

	// DELETE succeeds
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM recovery_codes WHERE user_id = $1")).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// All INSERTs succeed
	for i := 0; i < NumRecoveryCodes; i++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO recovery_codes")).
			WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg(), false, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	// Commit fails
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	codes, err := service.RegenerateCodes(userID)

	assert.Error(t, err)
	assert.Nil(t, codes)
	assert.Contains(t, err.Error(), "failed to commit")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Test helper functions

func TestGenerateCode(t *testing.T) {
	// Test multiple generations for uniqueness
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := generateCode()
		require.NoError(t, err)

		// Check format: XXXX-XXXX-XXXX
		assert.Len(t, code, 14)
		assert.Equal(t, "-", string(code[4]))
		assert.Equal(t, "-", string(code[9]))

		// Check uniqueness
		assert.False(t, codes[code], "duplicate code generated: %s", code)
		codes[code] = true

		// Check characters are valid base32
		normalized := normalizeCode(code)
		assert.Len(t, normalized, 12)
		for _, c := range normalized {
			assert.True(t, (c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7'),
				"invalid character in code: %c", c)
		}
	}
}

func TestHashCode_Consistency(t *testing.T) {
	code := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")

	hash1, err := hashCode(code, salt)
	require.NoError(t, err)

	hash2, err := hashCode(code, salt)
	require.NoError(t, err)

	// Same code and salt should produce same hash
	assert.Equal(t, hash1, hash2)
}

func TestHashCode_DifferentSalts(t *testing.T) {
	code := "ABCD-EFGH-IJKL"
	salt1 := []byte("testsalt12345678")
	salt2 := []byte("othersalt1234567")

	hash1, err := hashCode(code, salt1)
	require.NoError(t, err)

	hash2, err := hashCode(code, salt2)
	require.NoError(t, err)

	// Same code but different salts should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestHashCode_DifferentCodes(t *testing.T) {
	salt := []byte("testsalt12345678")
	code1 := "ABCD-EFGH-IJKL"
	code2 := "MNOP-QRST-UVWX"

	hash1, err := hashCode(code1, salt)
	require.NoError(t, err)

	hash2, err := hashCode(code2, salt)
	require.NoError(t, err)

	// Different codes should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "formatted code",
			input:    "ABCD-EFGH-IJKL",
			expected: "ABCDEFGHIJKL",
		},
		{
			name:     "lowercase code",
			input:    "abcd-efgh-ijkl",
			expected: "ABCDEFGHIJKL",
		},
		{
			name:     "code with spaces",
			input:    "ABCD EFGH IJKL",
			expected: "ABCDEFGHIJKL",
		},
		{
			name:     "mixed case with spaces and dashes",
			input:    "aBcD-eFgH iJkL",
			expected: "ABCDEFGHIJKL",
		},
		{
			name:     "no separators",
			input:    "ABCDEFGHIJKL",
			expected: "ABCDEFGHIJKL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test error constants
func TestErrorConstants(t *testing.T) {
	assert.Equal(t, "invalid recovery code", ErrInvalidCode.Error())
	assert.Equal(t, "recovery code already used", ErrCodeAlreadyUsed.Error())
	assert.Equal(t, "no unused recovery codes remaining", ErrNoCodesLeft.Error())
	assert.Equal(t, "user not found", ErrUserNotFound.Error())
}

// Test constants
func TestConstants(t *testing.T) {
	assert.Equal(t, 8, NumRecoveryCodes)
	assert.Equal(t, 12, CodeLength)
	assert.Equal(t, 16384, scryptN)
	assert.Equal(t, 8, scryptR)
	assert.Equal(t, 1, scryptP)
	assert.Equal(t, 32, keyLen)
	assert.Equal(t, 16, saltLen)
}

// Benchmark tests
func BenchmarkGenerateCode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = generateCode()
	}
}

func BenchmarkHashCode(b *testing.B) {
	code := "ABCD-EFGH-IJKL"
	salt := []byte("testsalt12345678")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hashCode(code, salt)
	}
}

func BenchmarkNormalizeCode(b *testing.B) {
	code := "ABCD-EFGH-IJKL"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeCode(code)
	}
}

// Custom matcher for UUID arguments
type anyUUID struct{}

func (a anyUUID) Match(v driver.Value) bool {
	_, ok := v.(uuid.UUID)
	if !ok {
		// Try parsing as string
		if str, ok := v.(string); ok {
			_, err := uuid.Parse(str)
			return err == nil
		}
	}
	return ok
}
