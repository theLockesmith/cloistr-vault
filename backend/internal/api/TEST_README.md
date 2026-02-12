# API Handler Tests Documentation

This directory contains comprehensive tests for the recovery-related API endpoints.

## Test Files

### `handlers_recovery_test.go`
Tests for recovery-related HTTP endpoints in the coldforge-vault API.

## Endpoints Tested

### `POST /api/v1/auth/recover`
Account recovery using a recovery code.

**Test Cases:**
- **Success**: Valid recovery with proper code
- **Invalid request format**: Malformed JSON
- **User not found**: Non-existent email
- **Invalid recovery code**: Wrong or expired code
- **Missing fields**: Email, recovery code, password, or vault data missing

**Request Body:**
```json
{
  "email": "user@example.com",
  "recovery_code": "ABCD-EFGH-IJKL",
  "new_password": "new-secure-password",
  "vault_data": "base64-encoded-encrypted-vault"
}
```

**Success Response (200 OK):**
```json
{
  "message": "Account recovered successfully",
  "token": "session-token",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "created_at": "timestamp",
    "updated_at": "timestamp"
  },
  "expires_at": "timestamp"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request format or missing fields
- `404 Not Found`: User not found
- `401 Unauthorized`: Invalid recovery code
- `500 Internal Server Error`: Server error

---

### `GET /api/v1/recovery/status`
Get recovery codes status for authenticated user.

**Test Cases:**
- **Success**: Returns total, remaining, and used counts
- **No user ID**: Missing authentication
- **Invalid user ID**: Malformed UUID in context
- **Database error**: Database connection failure
- **No codes**: User has no recovery codes
- **All codes used**: All 8 codes have been consumed

**Authentication:** Required (Bearer token)

**Success Response (200 OK):**
```json
{
  "total": 8,
  "remaining": 5,
  "used": 3
}
```

**Error Responses:**
- `401 Unauthorized`: Missing or invalid authentication
- `400 Bad Request`: Invalid user ID format
- `500 Internal Server Error`: Database error

---

### `POST /api/v1/recovery/regenerate`
Regenerate all recovery codes for authenticated user.

**Test Cases:**
- **Success**: Generates 8 new codes
- **No user ID**: Missing authentication
- **Invalid user ID**: Malformed UUID
- **Database error**: Transaction failure

**Authentication:** Required (Bearer token)

**Success Response (200 OK):**
```json
{
  "codes": [
    "ABCD-EFGH-IJKL",
    "MNOP-QRST-UVWX",
    ...
  ],
  "warning": "Store these codes safely. Each code can only be used once. You will not be able to see them again."
}
```

**Error Responses:**
- `401 Unauthorized`: Missing or invalid authentication
- `400 Bad Request`: Invalid user ID format
- `500 Internal Server Error`: Failed to regenerate codes

---

## Running Tests

### Install dependencies
```bash
cd /home/forgemaster/Development/coldforge-vault/backend
go get github.com/DATA/go-sqlmock
go get github.com/gin-gonic/gin
go get github.com/stretchr/testify
go mod tidy
```

### Run all API tests
```bash
go test ./internal/api -v
```

### Run only recovery tests
```bash
go test ./internal/api -v -run Recovery
```

### Run specific test
```bash
go test ./internal/api -v -run TestRecoverAccount_Success
```

### Run with coverage
```bash
go test ./internal/api -v -cover
go test ./internal/api -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Structure

### Setup Functions

```go
// setupTestRouter creates a test Gin router
func setupTestRouter() *gin.Engine {
    gin.SetMode(gin.TestMode)
    return gin.New()
}

// setupMockDB creates a mock database connection
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    return db, mock
}
```

### Test Pattern

```go
func TestEndpoint_Scenario(t *testing.T) {
    // Setup
    db, mock := setupMockDB(t)
    defer db.Close()

    router := setupTestRouter()
    authService := auth.NewAuthService(db)
    handlers := NewHandlers(authService, nil)

    router.POST("/endpoint", handlers.HandlerFunc)

    // Mock database expectations
    mock.ExpectQuery("...").WillReturnRows(...)

    // Create request
    reqBody := RequestStruct{...}
    body, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", "/endpoint", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")

    // Execute
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)

    var response ResponseStruct
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)

    assert.Equal(t, expected, response.Field)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

## Authentication Simulation

For authenticated endpoints, tests simulate the auth middleware:

```go
router.GET("/api/v1/recovery/status", func(c *gin.Context) {
    c.Set("userID", userID.String())
    handlers.GetRecoveryStatus(c)
})
```

This sets the `userID` in the Gin context as the real auth middleware would.

## Mocking Patterns

### User Lookup
```go
userRows := sqlmock.NewRows([]string{"id", "email", "created_at", "updated_at"}).
    AddRow(userID, email, time.Now(), time.Now())

mock.ExpectQuery(`SELECT u.id, u.email FROM users u JOIN auth_methods am`).
    WithArgs(email).
    WillReturnRows(userRows)
```

### Recovery Code Validation
```go
codeRows := sqlmock.NewRows([]string{"id", "code_hash", "salt"}).
    AddRow(codeID, codeHash, salt)

mock.ExpectQuery(`SELECT id, code_hash, salt FROM recovery_codes`).
    WithArgs(userID).
    WillReturnRows(codeRows)
```

### Transaction Mocking
```go
mock.ExpectBegin()
mock.ExpectExec("UPDATE ...").WillReturnResult(...)
mock.ExpectCommit()
```

## Edge Cases Covered

1. **Missing authentication** - Endpoints requiring auth fail appropriately
2. **Invalid UUIDs** - Malformed user IDs return 400
3. **Database errors** - Connection failures return 500
4. **Empty results** - No codes return empty arrays/zero counts
5. **All codes used** - Status shows 0 remaining
6. **Concurrent access** - Uses FOR UPDATE locking (tested via service layer)

## HTTP Status Codes

- `200 OK` - Success
- `400 Bad Request` - Invalid request format or parameters
- `401 Unauthorized` - Missing or invalid authentication/authorization
- `404 Not Found` - Resource not found (user)
- `500 Internal Server Error` - Server/database errors

## Test Coverage Goals

- **Handler coverage**: 100% of recovery endpoints
- **Status codes**: All possible status codes tested
- **Error paths**: All error conditions covered
- **Edge cases**: Boundary conditions and special cases

## Integration with Service Layer

These tests mock the database but use the real service layer:

```
HTTP Handler → Auth Service → Recovery Service → Mock Database
```

This ensures:
- Handlers correctly call service methods
- Request/response serialization works
- Error propagation is correct
- Business logic is integrated

## Common Issues

### "User ID not found"
Ensure you set `userID` in the context for authenticated endpoints:
```go
router.GET("/endpoint", func(c *gin.Context) {
    c.Set("userID", userID.String())
    handlers.Handler(c)
})
```

### "Invalid request format"
Check JSON marshaling and ensure all required fields have `binding:"required"` tags.

### "Failed to get recovery status"
Ensure all database mocks are set up correctly with proper column names and types.

## Future Enhancements

1. Add rate limiting tests
2. Add CORS tests
3. Add request validation tests (max length, etc.)
4. Add concurrent request tests
5. Add authentication middleware tests
6. Add request/response logging verification

## CI/CD Integration

```yaml
# .github/workflows/api-test.yml
- name: Run API Tests
  run: |
    cd backend
    go test ./internal/api -v -cover -race
```

The `-race` flag enables the race detector to catch concurrency issues.

## Manual Testing

While unit tests are comprehensive, manual testing with curl is also useful:

```bash
# Recover account
curl -X POST http://localhost:8080/api/v1/auth/recover \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "recovery_code": "ABCD-EFGH-IJKL",
    "new_password": "new-password",
    "vault_data": "encrypted-data"
  }'

# Get recovery status (requires auth token)
curl -X GET http://localhost:8080/api/v1/recovery/status \
  -H "Authorization: Bearer YOUR_TOKEN"

# Regenerate codes (requires auth token)
curl -X POST http://localhost:8080/api/v1/recovery/regenerate \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Postman Collection

A Postman collection for manual testing is available at:
`docs/postman/recovery-endpoints.json`

(Note: Create this file if needed for manual testing)
