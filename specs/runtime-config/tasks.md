# Tasks: runtime service configuration

- [x] Go: `ClientConfig` with production defaults; tests for defaults and
      staging override.
- [x] Go: `/config.js` handler, exact path, no-store, JSON-encoded; tests.
- [x] Go: CSP `connect-src` follows the configured signer; test.
- [x] Web: `/config.js` script tag before the bundle.
- [x] Web: signer probe and `Header` `signerUrl` read `getServiceConfig()`;
      test that a staging config makes the probe hit the staging signer only.
- [x] Local proof: one image run twice in Chromium; production reports
      production, staging reports staging with zero production requests.
- [ ] Merge once every job except lint is green.
- [ ] After deploy: live `/config.js` reports production; rerun the staging
      browser proof on the deployed image digest.
- [ ] Report image digest and commit to cloistr-orchestrator.
