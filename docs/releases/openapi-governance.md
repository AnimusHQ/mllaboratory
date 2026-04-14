# OpenAPI governance

## Policy

- OpenAPI contracts live in `core/contracts/openapi/*.yaml`.
- Compatibility baseline lives in `core/contracts/baseline/*.yaml`.
- Scripts resolve contract paths via `scripts/lib/paths.sh` (`ANIMUS_CONTRACTS_DIR` override).
- CI enforces both lint and baseline compatibility via `make openapi-check`.
- Drift between current spec and baseline fails CI.
- Compatibility bypass flags are not allowed in CI.

## Checks

- `make openapi-lint`: schema and style checks for all published specs.
- `make openapi-compat`: compares `core/contracts/openapi` with `core/contracts/baseline`.
- `make openapi-check`: runs lint + compat together.

## Baseline update procedure

Use this only for intentional API changes.

1. Update spec files in `core/contracts/openapi/*.yaml`.
2. Run local verification:
   - `make openapi-lint`
   - `make openapi-compat` (expected to fail before baseline update)
3. Update baseline explicitly:
   - `OPENAPI_BASELINE_UPDATE=1 make openapi-baseline-update`
4. Re-run:
   - `make openapi-check`
5. Open a dedicated baseline PR containing only:
   - changed files under `core/contracts/baseline/*.yaml`
   - optional contract release notes.

## Versioning expectations

- Backward-compatible additions (new optional fields/endpoints) are allowed with baseline update.
- Breaking changes (removing/renaming fields, tightening required fields, changing response semantics) require:
  - explicit versioning plan,
  - migration notes,
  - client impact assessment,
  - release communication before merge.
