# Runtime service configuration (vault)

Directed by cloistr-orchestrator 2026-10-05 for the staging split. Recipe:
`cloistr-collab-common/docs/runtime-config-adoption.md`. Staging definition:
`architecture/staging-environment.md` in the Cloistr docs tree.

## Goal

One image serves production and staging. The web UI reads every Cloistr
service address at page load from `/config.js`, which the container writes
from its environment. With no environment set, everything resolves to
production, so the live service is unchanged.

## Vault differs from the recipe

Vault's UI is served by the Go API, not nginx. The Go binary serves the same
contract:

- `GET /config.js`, exact path only, `Cache-Control: no-store`,
  `application/javascript`, body `window.__CLOISTR_CONFIG__=<json>;`.
- Values from `CLOISTR_RELAY_URL`, `CLOISTR_SIGNER_URL`, `CLOISTR_BLOSSOM_URL`,
  `CLOISTR_DISCOVERY_URL`, `CLOISTR_APP_URL`, `CLOISTR_ENVIRONMENT`; each
  defaults to its production value in Go (the recipe's Dockerfile `ENV`
  defaults, moved to where this binary reads config).
- JSON-encoded, so no value can break out of the script.

The Content-Security-Policy's `connect-src` names the signer. It must follow
`CLOISTR_SIGNER_URL`, or a staging browser is blocked from the staging signer
and still allowed to reach production's.

## Frontend changes

- `index.html`: `<script src="/config.js"></script>` before the module script.
- Every production hostname in a request path goes through
  `getServiceConfig()` from `@cloistr/collab-common/config`: the signer session
  probe, and the `signerUrl` passed to the shared `Header` (login, register,
  and the main layout, which previously fell back to the package's literal).

## Out of scope (named, not silently skipped)

- Footer links to `cloistr.xyz/privacy` and `/terms`: navigation to the
  marketing site, not a request; the reader has no field for it.
- Shared-package gaps listed in the adoption doc (auth relay prepend, session
  cookie domain, nav catalog) belong to cloistr-tooling.
- Backend NIP-05 fallback relay list and WebAuthn RP ID (already env-driven)
  are server-side.
- Staging deploy values (step 5) live in the staging overlay, owned by
  atlas-ops. Production needs no deploy change.

## Done

Merged, deployed, live `/config.js` reports production; the same image run
locally with staging env reports staging hosts, and a real browser's network
log shows no request to any production `*.cloistr.xyz` host.
