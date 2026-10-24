Status: DRAFT (awaiting security review and cloistr-pr review, 2026-10-20)

# Tasks: email sign-in never sends the master password

- [x] Client derivation `deriveAuthKey` in `packages/vault-crypto`.
      Evidence: `npx vitest run src/auth-key.test.ts` → 5 passed; the expected keys were computed independently with Python `hashlib.scrypt`.
- [x] Server: v2 register, auth-key sign-in, legacy upgrade, password refused after upgrade, uniform 401 with equal timing, upper-case hex.
      Evidence: `go test ./internal/api/ ./internal/auth/` with VAULT_TEST_PG_DSN → ok; `zk_login_e2e_test.go` (6 tests, real router and Postgres) all failed before the server change.
- [x] Request bodies never contain the typed password on register or sign-in; a legacy account sends it once to upgrade and never again; a 401 is not retried with the password.
      Evidence: `npx vitest run src/contexts/AuthContext.zkLogin.test.tsx` → 4 passed; the same file against master's AuthContext.tsx → 4 failed.
- [x] Recovery takes `new_auth_key`; unknown email and bad code share one 401.
      Evidence: `go test ./internal/api/` → ok, including `handlers_recovery_test.go` (TestRecoverAccount_UserNotFound now expects 401 "Invalid email or recovery code").
- [x] Security review findings: dummy scrypt before the 409 upgrade answer; email normalised for storage, lookup and the client request; stdlib constant-time compare; random dummy salt.
      Evidence: `go test ./internal/api/ ./internal/auth/ ./internal/crypto/` → ok, including TestZKLogin_EmailCaseDoesNotMatter and TestZKLogin_UpgradeAnswerTakesAsLongAsTheOthers.
- [ ] docs/security.md corrected; sign-in and sign-up wording.
      Written; checked by review, not by a test.
- [x] Full suites green.
      Evidence: full Go suite with Postgres → all packages ok; web `vitest run` → 99 passed; `tsc --noEmit` → 0 errors; `golangci-lint run` → 0 issues.
- [ ] Mobile app and browser extensions derive the auth key (not shipped; recorded in docs/security.md).
- [ ] Review (cloistr-pr); report to cloistr-orchestrator before merge.
- [ ] After deploy: capture the live register and sign-in request bodies and confirm there is no `password` field.
