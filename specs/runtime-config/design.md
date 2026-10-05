# Design: runtime service configuration

Recipe: `cloistr-collab-common/docs/runtime-config-adoption.md`. Vault differs
in one way: its UI is served by the Go API, not nginx.

## Server (Go)

- `config.ClientConfig` / `LoadClientConfig()` read `CLOISTR_RELAY_URL`,
  `CLOISTR_SIGNER_URL`, `CLOISTR_BLOSSOM_URL`, `CLOISTR_DISCOVERY_URL`,
  `CLOISTR_APP_URL`, `CLOISTR_ENVIRONMENT`, each defaulting to production. This
  replaces the recipe's Dockerfile `ENV` defaults: the binary is where config is
  read, so the default lives there.
- `GET /config.js` (registered before the SPA catch-all): `no-store`,
  `application/javascript`, body `window.__CLOISTR_CONFIG__=<json>;`, JSON
  encoded so no value can break out of the script. Body computed once.
- `SecurityHeadersMiddleware(signerURL)`: CSP `connect-src 'self' <signer>`.

## Client

- `index.html` loads `<script src="/config.js">` before the module bundle (a
  script tag, not a fetch, so modules see it when they import).
- Signer session probe and the `signerUrl` given to the shared `Header` (login,
  register, main layout) come from `getServiceConfig()`.

## Out of scope

- Footer links to the marketing site's privacy/terms pages: navigation, not a
  request; the reader has no field for it.
- Shared-package gaps (auth relay prepend, session cookie domain, nav catalog):
  cloistr-tooling.
- Backend NIP-05 fallback relays; WebAuthn RP ID is already env-driven.
- Staging deploy values: the staging overlay, owned by atlas-ops.
