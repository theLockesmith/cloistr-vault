# Tasks: runtime service configuration

- [x] Go: `ClientConfig` with production defaults. Evidence: `TestLoadClientConfig_DefaultsAreProduction`, `TestRuntimeConfig_StagingEnvironmentReplacesProductionHosts` (pass, `go test ./internal/api/`).
- [x] Go: `/config.js` handler, exact path, no-store, JSON-encoded. Evidence: `TestRuntimeConfig_ServesProductionWithNoEnvironment`, `TestRuntimeConfig_ExactPathOnly`, `TestRuntimeConfig_ValuesAreJSONEncoded`, `TestRuntimeConfig_BeatsStaticFilesAndAssetsStayCached` (pass).
- [x] Go: CSP `connect-src` follows the configured signer. Evidence: CSP assertions in `TestRuntimeConfig_StagingEnvironmentReplacesProductionHosts` (pass); local image on staging env returned `connect-src 'self' https://signer.staging.cloistr.xyz`.
- [x] Web: `/config.js` script tag before the bundle. Evidence: built `dist/index.html` contains `<script src="/config.js"></script>` (clean-room build, node:22.23.3-alpine).
- [x] Web: signer probe and `Header` `signerUrl` read `getServiceConfig()`. Evidence: test "probes the signer named by runtime config, not the production literal" (web suite 95/95).
- [x] Local proof in Chromium, one image run twice. Evidence: no env -> environment=production, only off-origin request signer.cloistr.xyz; staging env -> environment=staging, only off-origin request signer.staging.cloistr.xyz, 0 production *.cloistr.xyz requests.
- [ ] Merge once every job except lint is green.
- [ ] After deploy: live `/config.js` reports production; rerun the staging browser proof on the deployed image digest.
- [ ] Report image digest and commit to cloistr-orchestrator.
