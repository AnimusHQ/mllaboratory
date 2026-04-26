# Open-Core Split Plan (Incremental)

This repository is the open-core host. Enterprise and SDK are migrated to separate repositories in staged PRs.

## Canonical open-core roots
- `core/contracts`
- `deploy`
- `closed/*` services (staged move plan only; no big-bang package move)
- `closed/ui` (canonical UI)
- `scripts`
- `docs`

## Target external repos
- Enterprise repo mounted at `./enterprise` (submodule target).
- SDK repo mounted at `./sdk` (submodule target).

### Enterprise path during migration
- Canonical enterprise path: `./enterprise` (gitlink submodule).
- Legacy compatibility path `closed/scripts` is stub-only during the deprecation window.
- Resolver source of truth: `scripts/lib/paths.sh` via `animus_enterprise_scripts_dir`.

### SDK path during migration
- Canonical SDK path: `./sdk` (gitlink submodule).
- Legacy fallback path: `./open/sdk` (stub-only compatibility path).
- Resolver source of truth: `scripts/lib/paths.sh` via `animus_sdk_dir`.

If remote URLs are not finalized, keep scaffolding only and do not block open-core CI.

## Planned `.gitmodules` target (when remotes are ready)
```ini
[submodule "enterprise"]
	path = enterprise
	url = TODO_ENTERPRISE_REPO_URL

[submodule "sdk"]
	path = sdk
	url = TODO_SDK_REPO_URL
```

## Compatibility window
- Keep `open/sdk` and `closed/scripts` as stub-only compatibility paths until migration PRs complete.
- Build/CI must stay green without requiring enterprise submodule checkout.

## Switch Steps (Set Real Submodule URLs)
1. Edit `.gitmodules`:
   - `submodule.enterprise.url=TODO_ENTERPRISE_REPO_URL` -> real enterprise repository URL.
   - `submodule.sdk.url=TODO_SDK_REPO_URL` -> real SDK repository URL.
2. Run:
   - `git submodule sync -- enterprise sdk`
   - `git submodule update --init --recursive enterprise sdk`
3. Validate:
   - `make submodule-check`
   - `ANIMUS_SUBMODULE_CHECK_ENFORCE=1 ANIMUS_SUBMODULE_CHECK_REQUIRE_INIT=1 make submodule-check`
4. Submit the URL-switch PR with updated owner/target metadata in `scripts/guardrail_policy.env`.

## Rollback
- Revert the migration PR commit with `git revert <sha>`.
