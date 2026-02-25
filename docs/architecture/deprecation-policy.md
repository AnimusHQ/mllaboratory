# Deprecation Policy: Compatibility Shims

## Definition
A compatibility shim is a legacy path or entrypoint retained temporarily to avoid breaking existing workflows while the canonical layout is adopted.

## Current shim directories
- `open/api`
- `closed/deploy`
- `closed/scripts`

## Canonical locations
- Contracts: `core/contracts`
- Deploy assets: `deploy`
- Enterprise scripts: `enterprise/scripts`

## Deprecation window
- Shim directories are retained for **2 minor releases**.
- During this window, canonical paths are the source of truth.

## Removal criteria
- `make compat-check` passes with no shim regressions.
- CI workflows reference only canonical paths.
- Release workflows do not invoke shim script paths.

## Target removal phase
- Phase 5.1
