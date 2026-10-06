# Requirements: no production defaults for service settings

Directed by cloistr-orchestrator 2026-10-06, from atlas-ops' staging build.

1. Outside development, the server refuses to start when any of these is unset,
   and the error names every missing one at once: `WEBAUTHN_RP_ID`,
   `WEBAUTHN_ORIGIN`, `VAULT_SIGNER_URL`, `CLOISTR_RELAY_URL`,
   `CLOISTR_SIGNER_URL`, `CLOISTR_BLOSSOM_URL`, `CLOISTR_DISCOVERY_URL`,
   `CLOISTR_APP_URL`, `CLOISTR_ENVIRONMENT`.
2. Order: production must set all nine explicitly, confirmed on the LIVE pods,
   before this change is deployed. Live check 2026-10-06 found none set, so the
   config change is routed to cloistr-config first and this branch waits.
3. Local development (ENVIRONMENT=development outside Kubernetes) still starts
   with no settings, using the previous values.
4. `HEAD /config.js` gets the same `no-store` response as `GET`.
5. Proof: production pods start and serve after the change; a container with a
   variable unset exits non-zero naming it.
