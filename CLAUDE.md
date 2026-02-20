# CLAUDE.md - coldforge-vault

**Zero-knowledge password manager with Nostr integration**

## REQUIRED READING (Before ANY Action)

**Claude MUST read this file at the start of every session:**
- `~/claude/coldforge/cloistr/CLAUDE.md` - Cloistr project rules (contains further required reading)

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
- Lightning Address authentication (LNURL-auth / LUD-04)
- WebAuthn/Passkey authentication (platform + roaming authenticators)
- Recovery codes system
- Multi-platform (web, mobile, desktop, browser extension)

## NIPs and LUDs Referenced

- **NIP-07**: Browser extension signing
- **NIP-19**: Bech32-encoded entities (npub, nsec)
- **NIP-46**: Nostr Connect (nsecbunker authentication)
- **LUD-04**: LNURL-auth (Lightning Network authentication)
- **secp256k1**: Nostr/Lightning key cryptography

## Current Status (Updated 2026-02-18)

### Completed
- **Recovery codes** - Full implementation with secure hashing
- **Prometheus metrics** - Instrumented in `backend/internal/observability/metrics.go`
- **Grafana dashboard** - Deployed via `atlas monitoring apply-dashboards`
- **Kubernetes annotations** - Auto-discovery enabled for Prometheus scraping
- **Nostr signature verification** - Fixed Y-parity issue in secp256k1 public key handling
- **CI pipeline green** - Tests passing, Docker image builds successfully
- **Production deployment** - Running on Kubernetes at vault.coldforge.xyz
- **Nostr user display** - Shows `npub1...` bech32 format instead of `@nostr.local`
- **Lightning auth (LNURL-auth)** - Full implementation with secp256k1 signature verification
- **HA deployment** - Scaled to 3 replicas for high availability
- **NIP-05 verification** - Link and verify NIP-05 addresses for Nostr users
- **Frontend Lightning UI** - Lightning authentication in Login component
- **Frontend NIP-05 UI** - Settings page with NIP-05 verification for Nostr users
- **WebAuthn/Passkey backend** - Full backend implementation with database schema, service, and API handlers

### NIP-05 Verification
NIP-05 allows linking human-readable identifiers (`alice@domain.com`) to Nostr pubkeys:
- `GET /.well-known/nostr.json` - Serves NIP-05 data for domain verification
- `GET /api/v1/nip05/lookup?address=` - Look up a NIP-05 address
- `POST /api/v1/nip05/verify` - Verify and link NIP-05 to user account (authenticated)

Key files:
- `backend/internal/auth/nip05.go` - NIP-05 verification service
- `backend/internal/auth/providers/nip05.go` - NIP-05 provider implementation
- `backend/migrations/002_nip05_lightning.up.sql` - Database migration

Display name priority: NIP-05 > Lightning Address > npub

### Lightning Authentication (LNURL-auth)
New endpoints for Lightning Address authentication:
- `POST /api/v1/auth/lightning/challenge` - Generate k1 challenge for LNURL-auth
- `POST /api/v1/auth/login` with `method: "lightning"` - Authenticate with Lightning signature

Key files:
- `backend/internal/auth/lightning.go` - LNURL-auth service methods
- `backend/internal/auth/providers/lightning.go` - Lightning Address provider
- `backend/internal/api/handlers.go` - LightningChallenge handler

The implementation follows LUD-04 (LNURL-auth) specification:
- 32-byte random k1 challenge generation
- secp256k1 signature verification (compact 64-byte format)
- Auto-creation of user accounts from Lightning Addresses
- Integration with existing session management

### Production Environment
- **Namespace**: `coldforge-vault`
- **Image**: `oci.coldforge.xyz/coldforge/vault:latest`
- **Ingress**: `vault.coldforge.xyz`
- **Database**: PostgreSQL 15 with persistent storage
- **Monitoring**: ServiceMonitor configured for Prometheus scraping

### WebAuthn/Passkey Authentication
Backend implementation complete with database migration and API endpoints:

**Public endpoints (login):**
- `POST /api/v1/auth/webauthn/login/begin` - Start login with email
- `POST /api/v1/auth/webauthn/login/begin/discoverable` - Start usernameless login
- `POST /api/v1/auth/webauthn/login/finish` - Complete login

**Protected endpoints (credential management):**
- `POST /api/v1/user/webauthn/register/begin` - Start passkey registration
- `POST /api/v1/user/webauthn/register/finish` - Complete registration
- `GET /api/v1/user/webauthn/credentials` - List user's passkeys
- `PUT /api/v1/user/webauthn/credentials/:id` - Rename a passkey
- `DELETE /api/v1/user/webauthn/credentials/:id` - Remove a passkey

Key files:
- `backend/internal/auth/webauthn.go` - WebAuthn service implementation
- `backend/internal/api/handlers_webauthn.go` - API handlers
- `backend/migrations/003_webauthn_support.up.sql` - Database schema

### Next Steps (Priority Order)
1. **Frontend WebAuthn UI** - Add passkey registration/login to web app
2. **Integrate component-based frontend** - Wire up Login, Dashboard, Settings with App routing

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
