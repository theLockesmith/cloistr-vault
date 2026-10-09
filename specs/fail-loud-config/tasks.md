# Tasks: no production defaults for service settings

- [x] Live production pods carry all nine settings (evidence 2026-10-09: configmap write 14:18:10Z; both pods started 14:27Z; in-pod sha256 9/9 match on both pods).
- [x] Tests (evidence: TestNoProductionDefaultsForServiceSettings, TestAllMissingServiceSettingsReportedAtOnce, TestDevelopmentFillsServiceSettings pass): each setting unset in production refuses to start naming it; all set loads; development loads with none.
- [x] Code (evidence: go test ./... all pass): remove the defaults; development fallback keeps old values.
- [x] (evidence: TestRuntimeConfig_HeadMatchesGet passes) HEAD /config.js returns no-store; test.
- [x] (evidence: local image, CLOISTR_ENVIRONMENT unset -> exit 1 naming it; none set -> exit 1 naming all nine) Container with a setting unset exits non-zero naming it (local run).
- [ ] Merge only after the first task is done; after deploy, pods start and serve.
