# Recovery Tests - Quick Start

One-page guide to running recovery code tests.

## Installation (One-Time Setup)

```bash
cd /home/forgemaster/Development/coldforge-vault/backend
go get github.com/DATA/go-sqlmock
go mod tidy
chmod +x scripts/test-recovery.sh
```

## Running Tests

### Option 1: Test Script (Recommended)
```bash
bash scripts/test-recovery.sh
```

### Option 2: Manual Commands
```bash
# Recovery service tests
go test ./internal/recovery -v

# API handler tests
go test ./internal/api -v -run Recovery

# Both with coverage
go test ./internal/recovery -v -cover
go test ./internal/api -v -run Recovery -cover
```

## Quick Commands

| Command | Description |
|---------|-------------|
| `go test ./internal/recovery -v` | Run recovery service tests |
| `go test ./internal/api -v -run Recovery` | Run API handler tests |
| `go test ./internal/recovery -v -cover` | Run with coverage report |
| `go test ./internal/recovery -v -race` | Run with race detector |
| `go test ./internal/recovery -bench=.` | Run benchmarks |
| `go test ./internal/recovery -v -run TestGenerateCodes` | Run specific test pattern |
| `go test -coverprofile=cov.out ./internal/recovery && go tool cover -html=cov.out` | Generate HTML coverage |

## Test Files

| File | Tests |
|------|-------|
| `/internal/recovery/recovery_test.go` | Service layer (30+ tests) |
| `/internal/api/handlers_recovery_test.go` | HTTP handlers (15+ tests) |

## What's Tested

### Recovery Service
- ✅ Generate 8 unique codes
- ✅ Validate recovery codes
- ✅ Consume codes (one-time use)
- ✅ Get remaining count
- ✅ Get code status
- ✅ Regenerate codes

### API Endpoints
- ✅ `POST /api/v1/auth/recover` - Account recovery
- ✅ `GET /api/v1/recovery/status` - Check status
- ✅ `POST /api/v1/recovery/regenerate` - New codes

## Expected Output

```
=== RUN   TestGenerateCodes_Success
--- PASS: TestGenerateCodes_Success (0.15s)
=== RUN   TestValidateCode_Success
--- PASS: TestValidateCode_Success (0.12s)
...
PASS
coverage: 87.5% of statements
ok      github.com/coldforge/vault/internal/recovery    2.145s
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `package github.com/DATA/go-sqlmock not found` | Run `go get github.com/DATA/go-sqlmock` |
| Tests fail with SQL errors | Check mock expectations match actual queries |
| Tests are slow | Use `-parallel=4` flag or reduce benchmark iterations |
| "expectations not met" | Add missing `mock.Expect*()` calls |

## Coverage Target

- **Recovery Service**: >85%
- **API Handlers**: >90%
- **Overall**: >85%

## Next Steps

1. ✅ Install dependencies
2. ✅ Run tests
3. ✅ Check coverage
4. ✅ Fix any failures
5. 📚 Read full documentation in `TESTING_RECOVERY.md`

## File Locations

```
backend/
├── internal/
│   ├── recovery/
│   │   ├── recovery.go             # Implementation
│   │   ├── recovery_test.go        # Service tests ⭐
│   │   └── TEST_README.md          # Detailed docs
│   └── api/
│       ├── handlers.go             # Implementation
│       ├── handlers_recovery_test.go  # API tests ⭐
│       └── TEST_README.md          # Detailed docs
├── scripts/
│   └── test-recovery.sh            # Test runner ⭐
├── TESTING_RECOVERY.md             # Full guide ⭐
└── RECOVERY_TESTS_QUICKSTART.md    # This file ⭐
```

## CI/CD

Tests run automatically on:
- Pull requests
- Commits to main
- Nightly builds

## Success Criteria

All tests passing means:
- ✅ Recovery codes generate correctly
- ✅ Codes validate and consume properly
- ✅ API endpoints work as expected
- ✅ Error handling is correct
- ✅ Edge cases are covered
- ✅ No race conditions

## Get Help

- 📖 Full docs: `TESTING_RECOVERY.md`
- 📖 Service tests: `internal/recovery/TEST_README.md`
- 📖 API tests: `internal/api/TEST_README.md`
- 🐛 Issues: Check troubleshooting sections
- 💬 Questions: Review test comments for explanation

---

**Happy Testing! 🧪**
