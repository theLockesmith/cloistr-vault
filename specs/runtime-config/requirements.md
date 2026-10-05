# Requirements: runtime service configuration

Directed by cloistr-orchestrator 2026-10-05 for the staging split
(`architecture/staging-environment.md`).

1. One image serves production and staging; no per-environment build.
2. With no environment set, every service address resolves to production, so
   the live service is unchanged.
3. Every production hostname the web UI makes a request to goes through
   `getServiceConfig()` from `@cloistr/collab-common/config`, including values
   passed into shared-package components.
4. The runtime config is never cached (`no-store`) and is served only at the
   exact path `/config.js`.
5. A staging browser can reach the staging signer and is not permitted to reach
   production's (the CSP follows the configured signer).
6. Done = merged, deployed, live `/config.js` reports production, and the same
   image run with staging env reports staging hosts with no request to any
   production `*.cloistr.xyz` host in a real browser's network log.
