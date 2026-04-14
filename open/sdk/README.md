# Legacy SDK Stub

`open/sdk` is a compatibility stub retained for the SDK path migration.

Canonical SDK location:
- `sdk` (git submodule)

This legacy shim is retained for compatibility only and will be removed after 2 minor releases.
Deprecation policy: `docs/architecture/deprecation-policy.md`.

Resolver behavior:
- `scripts/lib/paths.sh` prefers `./sdk` when initialized with SDK content.
- Temporary fallback to `./open/sdk` exists for migration window only.
