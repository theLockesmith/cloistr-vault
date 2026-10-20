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
- [ ] docs/security.md corrected; sign-in and sign-up wording.
      Written; checked by review, not by a test.
- [ ] Full suites (web tests and typecheck, vault-crypto, Go lint, full Go suite) green.
- [ ] Review (cloistr-pr); report to cloistr-orchestrator before merge.
- [ ] After deploy: capture the live register and sign-in request bodies and confirm there is no `password` field.
