# Deprecation Policy: Compatibility Shims

## Definition
A compatibility shim is a legacy path or entrypoint retained temporarily to avoid breaking existing workflows while the canonical layout is adopted.

## Current shim directories
- `open/api`
- `closed/deploy`
- `closed/scripts`
- `open/sdk`

## Canonical locations
- Contracts: `core/contracts`
- Deploy assets: `deploy`
- Enterprise scripts: `enterprise/scripts`
- SDK: `sdk` (git submodule)

## Deprecation window
- Shim directories are retained for **2 minor releases**.
- During this window, canonical paths are the source of truth.

## Enforcement rollout
- PR CI remains visibility-first for legacy usage until the agreed milestone (no fail mode on PR jobs yet).
- Nightly enforcement uses: `ANIMUS_LEGACY_SCAN_FAIL=1 make legacy-scan`.
- Release-candidate validation must include fail mode before cut.
- PR fail-mode enforcement is controlled by `scripts/guardrail_policy.env` (`ANIMUS_PR_LEGACY_ENFORCE`).

## Allowlist discipline (`scripts/legacy_scan.allow`)
- Update allowlist entries only in reviewed PRs with explicit rationale.
- Do not add broad patterns; use narrow, file-specific patterns.
- Every allowlist entry must be preceded by metadata comment with `owner=` and `expiry=YYYY-MM-DD`.

## Removal criteria
- `make compat-check` passes with no shim regressions.
- CI workflows reference only canonical paths.
- Release workflows do not invoke shim script paths.
- Enterprise submodule flip prerequisite: workflows use recursive submodule checkout and `enterprise/scripts` validation remains green.

## Target removal phase
- Phase 6
