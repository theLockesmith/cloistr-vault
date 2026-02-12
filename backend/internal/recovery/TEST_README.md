# Recovery Codes Testing Documentation

This directory contains comprehensive unit tests for the recovery codes feature in the coldforge-vault backend.

## Test Files

### `recovery_test.go`
Tests for the recovery service (`recovery.go`), covering all public and private functions.

## Test Coverage

### Service Functions

#### `GenerateCodes()`
- **Success case**: Generates 8 unique recovery codes and stores hashed versions
- **Error cases**:
  - Database error on DELETE
  - Database error on INSERT
- **Edge cases**:
  - Ensures all codes are unique
  - Validates code format (XXXX-XXXX-XXXX)

#### `ValidateCode()`
- **Success case**: Validates a correct recovery code
- **Error cases**:
  - Invalid/wrong code
  - No codes found for user
  - Database connection error
- **Edge cases**:
  - Case insensitive validation
  - Handles codes with/without dashes

#### `ConsumeCode()`
- **Success case**: Validates and marks code as used
- **Error cases**:
  - Invalid code (returns `ErrInvalidCode`)
  - Code already used (returns `ErrCodeAlreadyUsed`)
  - Database update error
- **Edge cases**:
  - Uses row locking (FOR UPDATE)
  - Returns the consumed code ID

#### `GetRemainingCount()`
- **Success case**: Returns count of unused codes
- **Error cases**: Database error
- **Edge cases**: Returns 0 when no codes exist

#### `GetCodeStatus()`
- **Success case**: Returns all codes with used/unused status
- **Error cases**: Database error
- **Edge cases**: Returns empty array when no codes exist

#### `RegenerateCodes()`
- **Success case**: Deletes old codes and generates new ones
- **Error cases**:
  - Transaction begin error
  - GenerateCodes error (propagated)
  - Transaction commit error
- **Edge cases**: Uses transaction to ensure atomicity

### Helper Functions

#### `generateCode()`
- Tests uniqueness over 100 iterations
- Validates format (XXXX-XXXX-XXXX)
- Ensures valid base32 characters

#### `hashCode()`
- Tests consistency (same input = same output)
- Tests different salts produce different hashes
- Tests different codes produce different hashes

#### `normalizeCode()`
- Tests various formats (dashes, spaces, mixed case)
- Tests already normalized codes

## Running Tests

### Install dependencies
```bash
cd /home/forgemaster/Development/coldforge-vault/backend
go get github.com/DATA/go-sqlmock
go mod tidy
```

### Run all recovery tests
```bash
go test ./internal/recovery -v
```

### Run specific test
```bash
go test ./internal/recovery -v -run TestGenerateCodes_Success
```

### Run with coverage
```bash
go test ./internal/recovery -v -cover
go test ./internal/recovery -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run benchmarks
```bash
go test ./internal/recovery -bench=. -benchmem
```

## Test Structure

All tests follow the **Arrange-Act-Assert** pattern:

```go
func TestFeature_Scenario(t *testing.T) {
    // Arrange - Setup test data and mocks
    db, mock := setupMockDB(t)
    defer db.Close()
    service := NewService(db)

    // Mock database expectations
    mock.ExpectQuery("...").WillReturnRows(...)

    // Act - Execute the function
    result, err := service.SomeFunction(params)

    // Assert - Verify results
    require.NoError(t, err)
    assert.Equal(t, expected, result)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

## Mock Database

We use `go-sqlmock` to mock database interactions without requiring a real database:

```go
db, mock := setupMockDB(t)
defer db.Close()

// Expect specific queries
mock.ExpectQuery("SELECT ...").
    WithArgs(userID).
    WillReturnRows(sqlmock.NewRows(...).AddRow(...))

// Expect specific executions
mock.ExpectExec("INSERT INTO ...").
    WithArgs(sqlmock.AnyArg(), ...).
    WillReturnResult(sqlmock.NewResult(1, 1))
```

## Coverage Goals

- **Line coverage**: > 85%
- **Branch coverage**: > 80%
- **Function coverage**: 100% of public functions

## Current Coverage

Run `go test -cover` to see current coverage:

```bash
go test ./internal/recovery -cover
```

Expected output:
```
ok      github.com/coldforge/vault/internal/recovery    0.XYZs  coverage: XX.X% of statements
```

## Test Data

### Sample Recovery Code
```
Format: XXXX-XXXX-XXXX
Example: ABCD-EFGH-IJKL
Characters: A-Z, 2-7 (base32)
```

### Sample User IDs
All tests use `uuid.New()` to generate unique user IDs.

### Sample Salts
Tests use fixed 16-byte salts for deterministic hashing:
```go
salt := []byte("testsalt12345678") // 16 bytes
```

## Known Limitations

1. **Observability calls**: Tests don't verify `observability.Info()` and `observability.Warn()` calls
2. **Time-based tests**: Some tests use `time.Now()` which could be flaky; consider using a time mock
3. **Scrypt parameters**: Tests use actual scrypt which may be slow; consider mocking for unit tests

## Continuous Integration

These tests are designed to run in CI environments without external dependencies:

```yaml
# .github/workflows/test.yml
- name: Run Recovery Tests
  run: |
    cd backend
    go test ./internal/recovery -v -cover
```

## Troubleshooting

### "sqlmock: all expectations were already fulfilled"
This means you set up more mocks than were used. Remove unused `mock.Expect*()` calls.

### "there is a remaining expectation which was not matched"
You didn't set up all required mocks. Add `mock.Expect*()` for all database calls.

### "arguments do not match"
Check that `WithArgs()` matches the actual function arguments. Use `sqlmock.AnyArg()` for dynamic values.

### Tests are slow
Scrypt hashing is intentionally slow. If tests take too long, consider:
- Running fewer iterations in benchmarks
- Using parallel test execution: `go test -parallel=4`

## Future Improvements

1. Add integration tests with real database
2. Add concurrency tests (multiple users accessing codes simultaneously)
3. Add stress tests (many codes, many users)
4. Mock observability package to verify logging
5. Add property-based testing with `go-fuzz` or similar
