# Recovery Codes Testing Guide

Comprehensive test suite for the recovery codes feature in coldforge-vault backend.

## Overview

This test suite covers the complete recovery codes functionality including:
- Recovery code generation and storage
- Code validation and consumption
- Account recovery via recovery codes
- Recovery status and regeneration APIs

## Test Files

| File | Location | Description |
|------|----------|-------------|
| `recovery_test.go` | `/internal/recovery/` | Service layer unit tests |
| `handlers_recovery_test.go` | `/internal/api/` | HTTP handler tests |
| `test-recovery.sh` | `/scripts/` | Test runner script |

## Quick Start

### 1. Install Dependencies

```bash
cd /home/forgemaster/Development/coldforge-vault/backend
go get github.com/DATA/go-sqlmock
go get github.com/gin-gonic/gin
go get github.com/stretchr/testify
go mod tidy
```

### 2. Run All Tests

```bash
# Using the test script (recommended)
bash scripts/test-recovery.sh

# Or manually
go test ./internal/recovery -v
go test ./internal/api -v -run Recovery
```

### 3. View Coverage

```bash
go test ./internal/recovery -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Coverage

### Recovery Service Tests (`internal/recovery/recovery_test.go`)

#### Core Functions
- ✅ `NewService()` - Service initialization
- ✅ `GenerateCodes()` - Success, DELETE error, INSERT error
- ✅ `ValidateCode()` - Success, invalid code, no codes, DB error
- ✅ `ConsumeCode()` - Success, invalid code, already used, update error
- ✅ `GetRemainingCount()` - Success, zero codes, DB error
- ✅ `GetCodeStatus()` - Success, empty result, DB error
- ✅ `RegenerateCodes()` - Success, begin error, generate error, commit error

#### Helper Functions
- ✅ `generateCode()` - Uniqueness, format validation, character validation
- ✅ `hashCode()` - Consistency, different salts, different codes
- ✅ `normalizeCode()` - Various formats (dashes, spaces, case)

#### Benchmarks
- ✅ `BenchmarkGenerateCode`
- ✅ `BenchmarkHashCode`
- ✅ `BenchmarkNormalizeCode`

**Total Tests**: 30+
**Expected Coverage**: >85%

### API Handler Tests (`internal/api/handlers_recovery_test.go`)

#### `POST /api/v1/auth/recover`
- ✅ Success case with valid recovery code
- ✅ Invalid request format (malformed JSON)
- ✅ User not found
- ✅ Invalid recovery code
- ✅ Missing fields (email, code, password, vault data)

#### `GET /api/v1/recovery/status`
- ✅ Success with mixed used/unused codes
- ✅ No user ID in context
- ✅ Invalid user ID format
- ✅ Database error
- ✅ No codes exist
- ✅ All codes used

#### `POST /api/v1/recovery/regenerate`
- ✅ Success case
- ✅ No user ID in context
- ✅ Invalid user ID format
- ✅ Database error

**Total Tests**: 15+
**Expected Coverage**: >90%

## Test Architecture

```
┌─────────────────────────────────────────────┐
│          HTTP Handler Tests                 │
│  (handlers_recovery_test.go)               │
│  - HTTP request/response                    │
│  - Status codes                             │
│  - JSON serialization                       │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│          Service Layer Tests                │
│  (recovery_test.go)                         │
│  - Business logic                           │
│  - Database operations                      │
│  - Error handling                           │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│          Mock Database                      │
│  (go-sqlmock)                               │
│  - No real DB required                      │
│  - Fast execution                           │
│  - Deterministic results                    │
└─────────────────────────────────────────────┘
```

## Mock Database Pattern

We use `go-sqlmock` to mock database operations:

```go
// Setup
db, mock := setupMockDB(t)
defer db.Close()

// Expect a query
mock.ExpectQuery("SELECT ...").
    WithArgs(userID).
    WillReturnRows(sqlmock.NewRows(...).AddRow(...))

// Expect an execution
mock.ExpectExec("INSERT INTO ...").
    WithArgs(sqlmock.AnyArg()).
    WillReturnResult(sqlmock.NewResult(1, 1))

// Verify all expectations met
assert.NoError(t, mock.ExpectationsWereMet())
```

## Test Data

### Recovery Code Format
```
Format: XXXX-XXXX-XXXX
Length: 14 characters (12 + 2 dashes)
Characters: A-Z, 2-7 (base32)
Example: ABCD-EFGH-IJKL
```

### Sample Test Data
```go
userID := uuid.New()
email := "test@example.com"
recoveryCode := "ABCD-EFGH-IJKL"
salt := []byte("testsalt12345678") // 16 bytes
```

## Running Specific Tests

```bash
# Run single test
go test ./internal/recovery -v -run TestGenerateCodes_Success

# Run test pattern
go test ./internal/recovery -v -run TestValidate

# Run with race detector
go test ./internal/recovery -v -race

# Run with coverage
go test ./internal/recovery -v -cover

# Run benchmarks only
go test ./internal/recovery -bench=. -run=^$
```

## Continuous Integration

### GitHub Actions

```yaml
name: Recovery Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Install dependencies
        run: |
          cd backend
          go mod download

      - name: Run recovery tests
        run: |
          cd backend
          bash scripts/test-recovery.sh

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./backend/coverage-recovery.out,./backend/coverage-api.out
```

## Coverage Goals

| Component | Target | Current |
|-----------|--------|---------|
| Recovery Service | >85% | Run tests to check |
| API Handlers | >90% | Run tests to check |
| Overall | >85% | Run tests to check |

## Best Practices Followed

### Test Independence
- Each test is independent
- No shared state between tests
- Uses `t.Cleanup()` where appropriate

### Clear Test Names
```go
func TestFeature_Scenario_ExpectedResult(t *testing.T)
// Examples:
// TestGenerateCodes_Success
// TestValidateCode_InvalidCode
// TestConsumeCode_AlreadyUsed
```

### Arrange-Act-Assert Pattern
```go
// Arrange - Setup
db, mock := setupMockDB(t)
service := NewService(db)

// Act - Execute
result, err := service.Function()

// Assert - Verify
require.NoError(t, err)
assert.Equal(t, expected, result)
```

### Comprehensive Error Testing
- Success paths
- Error paths
- Edge cases
- Boundary conditions

### Mock Verification
Always verify all mock expectations were met:
```go
assert.NoError(t, mock.ExpectationsWereMet())
```

## Troubleshooting

### Common Errors

#### "all expectations were already fulfilled"
**Problem**: More mocks set up than used.
**Solution**: Remove unused `mock.Expect*()` calls.

#### "there is a remaining expectation"
**Problem**: Missing database mock for a query/exec.
**Solution**: Add `mock.Expect*()` for all DB calls.

#### "arguments do not match"
**Problem**: `WithArgs()` doesn't match actual arguments.
**Solution**: Use `sqlmock.AnyArg()` for dynamic values like UUIDs.

#### Tests are slow
**Problem**: Scrypt hashing is intentionally CPU-intensive.
**Solution**:
- Use `-parallel` flag: `go test -parallel=4`
- Run fewer benchmark iterations
- Consider mocking crypto in some tests

### Debug Mode

Run tests with verbose output and race detection:
```bash
go test ./internal/recovery -v -race -count=1
```

The `-count=1` flag disables test caching, useful for debugging.

## Performance Benchmarks

Expected performance (run on local machine):

```
BenchmarkGenerateCode-8     10000    ~150000 ns/op
BenchmarkHashCode-8          1000   ~1000000 ns/op  (scrypt is slow)
BenchmarkNormalizeCode-8  1000000       ~1000 ns/op
```

Run benchmarks with memory profiling:
```bash
go test ./internal/recovery -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof
go tool pprof cpu.prof
```

## Security Considerations

These tests verify security-critical functionality:

1. **Password Reset**: Recovery codes enable password reset
2. **One-Time Use**: Codes can only be used once
3. **Hashing**: Codes are hashed with scrypt (not plaintext)
4. **Salting**: Each code has unique salt
5. **Transaction Safety**: Atomic operations prevent race conditions

## Future Enhancements

- [ ] Add integration tests with real database
- [ ] Add concurrency tests (race conditions)
- [ ] Add fuzz testing for code generation
- [ ] Add performance regression tests
- [ ] Mock observability package for log verification
- [ ] Add property-based tests
- [ ] Test database transaction rollback scenarios
- [ ] Add load testing scenarios

## Related Documentation

- [Recovery Service README](/home/forgemaster/Development/coldforge-vault/backend/internal/recovery/TEST_README.md)
- [API Handler Tests README](/home/forgemaster/Development/coldforge-vault/backend/internal/api/TEST_README.md)
- [Security Model](../docs/security.md)
- [API Specification](../docs/api-spec.yaml)

## Contributing

When adding new recovery features:

1. Write tests first (TDD)
2. Follow existing test patterns
3. Aim for >85% coverage
4. Test success, error, and edge cases
5. Update this documentation
6. Run full test suite before committing

```bash
# Pre-commit checklist
go test ./internal/recovery -v -race -cover
go test ./internal/api -v -race -run Recovery
go vet ./...
golangci-lint run ./internal/recovery
```

## Support

For issues with tests:
1. Check troubleshooting section above
2. Review test documentation in each package
3. Run with `-v` flag for verbose output
4. Check mock expectations match actual calls

## License

These tests are part of the coldforge-vault project and follow the same license.
