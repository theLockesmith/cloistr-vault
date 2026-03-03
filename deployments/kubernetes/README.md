# Kubernetes Deployment

**DO NOT USE THESE MANIFESTS DIRECTLY.**

Production deployment is managed by:

1. **Atlas** - Infrastructure automation
   - Role: `~/Atlas/roles/kube/cloistr-vault/`
   - Database setup, secrets, configuration

2. **cloistr-config** - Kustomize overlays + ArgoCD
   - Base: `~/Development/cloistr-config/base/vault/`
   - Production: Uses shared PostgreSQL cluster at `postgres-rw.db.coldforge.xyz`

## Deploy

```bash
# Via Atlas (idempotent)
atlas kube apply cloistr-vault --kube-context atlantis

# ArgoCD handles image updates automatically after GitLab CI builds
```

## Database

Uses shared Cloistr PostgreSQL cluster:
- Host: `postgres-rw.db.coldforge.xyz`
- Database: `cloistr`
- User: `cloistr`

## Namespace

All Cloistr services deploy to the `cloistr` namespace.
