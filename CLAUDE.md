# CLAUDE.md - cloistr-vault

**Zero-knowledge password manager with Nostr integration (Go backend, React frontend)**

**Status:** Production | **Domain:** vault.cloistr.xyz

## Required Reading

| Document | Purpose |
|----------|---------|
| `~/claude/coldforge/cloistr/CLAUDE.md` | Cloistr project rules |
| `~/claude/coldforge/cloistr/services/vault/CLAUDE.md` | Full architecture |
| [docs/reference.md](docs/reference.md) | API, auth methods, deployment |

## Autonomous Work Mode

**Work autonomously. Do NOT stop to ask what to do next.**

- Keep working until task complete or genuine blocker
- Make reasonable decisions - don't ask permission on obvious choices
- If tests fail, fix them. Use reviewer agent. Keep going.

## Agent Usage

| When | Agent |
|------|-------|
| Starting work / need context | `explore` |
| After significant code changes | `reviewer` |
| Writing/running tests | `test-writer` / `tester` |
| Security-sensitive code (auth, crypto) | `security` |

## Quick Commands

```bash
cd backend && go run cmd/server/main.go  # Run backend
cd frontend/web && npm start              # Run web frontend
make test                                  # Run tests
docker-compose up -d                       # Docker
```

## Project Structure

```
backend/
  cmd/server/         Entry point
  internal/
    auth/             Email, Nostr, Lightning, WebAuthn, NIP-05
    crypto/           AES-256-GCM, Scrypt
    vault/            Vault operations
    api/              HTTP handlers (Gin)
frontend/
  web/                React web app
  mobile/             React Native
  desktop/            Electron
  browser-extension/  Chrome/Firefox
```

## Authentication Methods

| Method | Status |
|--------|--------|
| Email/password | Done |
| NIP-07 (browser extension) | Done |
| NIP-46 (remote signer) | Done |
| Lightning (LNURL-auth) | Done |
| WebAuthn/Passkey | Done |
| NIP-05 verification | Done |
| Recovery codes | Done |

## Key Features

| Feature | Status |
|---------|--------|
| Zero-knowledge (AES-256-GCM, Scrypt) | Done |
| Multi-platform (web, mobile, desktop, extension) | Done |
| Vault item CRUD with favorites | Done |
| Search and filtering | Done |
| Relay preferences | Done |
| HA deployment (2 replicas) | Done |
| Prometheus metrics | Done |

## Deployment

- **Namespace:** cloistr
- **Image:** `registry.aegis-hq.xyz/coldforge/cloistr-vault:latest`
- **Database:** `postgres-rw.db.coldforge.xyz` (cloistr database)

```bash
atlas kube apply cloistr-vault
```

## Roadmap

| Item | Priority |
|------|----------|
| Mobile registration flow | P1 |
| Browser extension testing | P2 |
| Folder organization | P3 |

## See Also

- [Security Model](docs/security.md)
- [API Spec](docs/api-spec.yaml)
- Atlas Role: `~/Atlas/roles/kube/cloistr-vault/`

---

**Last Updated:** 2026-03-11
