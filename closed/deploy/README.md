# Legacy Deploy Stub

`closed/deploy` is a compatibility stub retained during repository layout migration.

Canonical deploy asset location:
- `deploy/helm/*`
- `deploy/docker/*`

This legacy shim is retained for compatibility only and will be removed after 2 minor releases.
Deprecation policy: `docs/architecture/deprecation-policy.md`.

Path resolution is handled by `scripts/lib/paths.sh` and prefers canonical deploy paths when present.

Optional overrides:
- `ANIMUS_DEPLOY_DIR`
- `ANIMUS_CONTRACTS_DIR`
- `ANIMUS_ENTERPRISE_SCRIPTS_DIR`
