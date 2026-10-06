# Tasks: no production defaults for service settings

- [ ] Live production pods carry all nine settings (routed to cloistr-config via cloistr-orchestrator 2026-10-06; confirm configmap has them AND pods started after the sync).
- [x] Tests (evidence: TestNoProductionDefaultsForServiceSettings, TestAllMissingServiceSettingsReportedAtOnce, TestDevelopmentFillsServiceSettings pass): each setting unset in production refuses to start naming it; all set loads; development loads with none.
- [x] Code (evidence: go test ./... all pass): remove the defaults; development fallback keeps old values.
- [x] (evidence: TestRuntimeConfig_HeadMatchesGet passes) HEAD /config.js returns no-store; test.
- [ ] Container with a setting unset exits non-zero naming it (local run).
- [ ] Merge only after the first task is done; after deploy, pods start and serve.
