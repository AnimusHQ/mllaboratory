# Legacy Contracts Stub

`open/api` is a compatibility stub kept during repository layout migration.

Canonical contracts location:
- `core/contracts/openapi/*.yaml`
- `core/contracts/baseline/*.yaml`
- `core/contracts/pipeline_spec.yaml`

This legacy shim is retained for compatibility only and will be removed after 2 minor releases.
Deprecation policy: `docs/architecture/deprecation-policy.md`.

Path resolution is handled by `scripts/lib/paths.sh` and prefers canonical paths when present.

Optional overrides:
- `ANIMUS_CONTRACTS_DIR`
- `ANIMUS_DEPLOY_DIR`
- `ANIMUS_ENTERPRISE_SCRIPTS_DIR`
