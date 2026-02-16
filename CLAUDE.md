# CLAUDE.md - coldforge-vault

**Zero-knowledge password manager with Nostr integration**

## Documentation

Full documentation is maintained at:
`~/claude/coldforge/services/vault/CLAUDE.md`

This file exists to help Claude Code find context when working in this repository.

## Autonomous Work Mode (CRITICAL)

**Work autonomously. Do NOT stop to ask what to do next.**

- Keep working until the task is complete or you hit a genuine blocker
- Use the "Next Steps" section in the service docs to know what to work on
- Make reasonable decisions - don't ask for permission on obvious choices
- Only stop to ask if there's a true ambiguity that affects architecture
- If tests fail, fix them. If code needs review, use the reviewer agent. Keep going.
- Update this CLAUDE.md and the service docs as you make progress

## Agent Usage (IMPORTANT)

**Use agents proactively. Do not wait for explicit instructions.**

| When... | Use agent... |
|---------|-------------|
| Starting new work or need context | `explore` |
| Need to research NIPs or protocols | `explore` |
| Writing or modifying code | `reviewer` after significant changes |
| Writing tests | `test-writer` |
| Running tests | `tester` |
| Investigating bugs | `debugger` |
| Updating documentation | `documenter` |
| Creating Dockerfiles | `docker` |
| Setting up Kubernetes deployment | `atlas-deploy` |
| Security-sensitive code (auth, crypto) | `security` |

## Workflow

1. **Before coding:** Use `explore` to read the service documentation and understand requirements
2. **While coding:** Write code, then use `reviewer` to check it
3. **Testing:** Use `test-writer` to create tests, `tester` to run them
4. **Before committing:** Use `security` for auth/crypto code
5. **Deployment:** Use `docker` for containers, `atlas-deploy` for Kubernetes

## Project Structure

```
coldforge-vault/
├── backend/
│   ├── cmd/server/main.go      # Application entry point
│   ├── internal/               # Private packages
│   │   ├── auth/               # Authentication (email, Nostr)
│   │   ├── crypto/             # Encryption (AES-256-GCM, Scrypt)
│   │   ├── vault/              # Vault operations
│   │   └── api/                # HTTP handlers (Gin)
│   ├── migrations/             # Database migrations
│   └── go.mod
├── frontend/
│   ├── web/                    # React web app
│   ├── mobile/                 # React Native
│   ├── mobile-expo/            # Expo version
│   ├── desktop/                # Electron app
│   └── browser-extension/      # Chrome/Firefox
├── deployments/kubernetes/     # K8s manifests
├── docs/                       # Project documentation
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

## Quick Commands

- **Start backend:** `cd backend && go run cmd/server/main.go`
- **Start web frontend:** `cd frontend/web && npm start`
- **Run tests:** `make test`
- **Docker Compose:** `docker-compose up -d`
- **Kubernetes:** `kubectl apply -f deployments/kubernetes/`

## Key Features

- Zero-knowledge architecture (server never sees unencrypted data)
- Client-side AES-256-GCM encryption
- Scrypt key derivation (N=32768, r=8, p=1)
- Email/password authentication
- Nostr keypair authentication (NIP-07, NIP-46)
- Recovery codes system
- Multi-platform (web, mobile, desktop, browser extension)

## NIPs Referenced

- **NIP-07**: Browser extension signing
- **NIP-46**: Nostr Connect (nsecbunker authentication)
- **secp256k1**: Nostr key cryptography

## Current Status (Updated 2026-02-16)

### Completed
- **Recovery codes** - Full implementation with secure hashing
- **Prometheus metrics** - Instrumented in `backend/internal/observability/metrics.go`
- **Grafana dashboard** - Deployed via `atlas monitoring apply-dashboards`
- **Kubernetes annotations** - Auto-discovery enabled for Prometheus scraping

### Blockers
- **CI test failure**: `TestNostrAuthenticationFlowFixed` in `internal/crypto` - signature verification issue
- **Container image**: Not built (blocked by failing tests)
- **Deployment**: Pods can't start without image

### Next Steps (Priority Order)
1. **Fix failing test** - `backend/internal/crypto/nostr_fixed_test.go:42`
2. **Get CI green** - Tests passing, image built and pushed
3. **Deploy to production** - Pods running with new Prometheus annotations
4. **Fix Nostr user display** - Show `npub1...` instead of `@nostr.local`
5. **Complete Lightning auth** - LNURL-auth flow

## Monitoring

Metrics available at `/metrics` endpoint:
- `coldforge_vault_requests_total` - HTTP request counts
- `coldforge_vault_request_duration_seconds` - Latency histogram
- `coldforge_vault_auth_attempts_total` - Auth attempts by method
- `coldforge_vault_operations_total` - Vault CRUD operations
- `coldforge_vault_sessions_active` - Active session gauge

Dashboard: `atlas monitoring dashboards` → `coldforge-vault`

## See Also

- Service Documentation: `~/claude/coldforge/services/vault/CLAUDE.md`
- Coldforge Overview: `~/claude/coldforge/CLAUDE.md`
- Security Model: `docs/security.md`
- API Spec: `docs/api-spec.yaml`
