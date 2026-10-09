# Design: no production defaults for service settings

- `LoadConfig` reads the nine settings with no default. In non-development it
  adds each unset one to the same `missing` list the secrets already use, so
  one error names everything; the message says "required setting(s)" rather
  than "secret(s)" since most are not secret.
- `devFallback` fills the nine with the values that used to be compiled-in
  defaults, so a developer's local run behaves as before. Development is
  already refused inside Kubernetes, so this path cannot reach a pod.
- `LoadClientConfig` reads `CLOISTR_*` without defaults; the production values
  now live only in the deployment config (cloistr-config `base/vault`).
- `HEAD /config.js` is registered on the same handler; net/http drops the body
  for HEAD, headers stay identical.
