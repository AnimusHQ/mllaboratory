# Финальная приёмка production‑grade (M0–M9)

Документ агрегирует доказательства выполнения по вехам M0–M9. Статусы синхронизированы с `roadmap.json`.

## Общие гейты (детерминированно)
- `make guardrails-check`
- `make openapi-lint`
- `make openapi-compat`
- `./scripts/go_test.sh ./closed/...`
- `make integrations-test`

## M0 — Repo/Tooling Baseline
Status: done
Evidence:
- `Makefile`
- `docs/roadmap/gap-closure-plan.md`
- `docs/ops/supply-chain.md`

## M1 — Core CP/DP Foundations
Status: done
Evidence:
- `closed/internal/domain/`
- `closed/internal/repo/postgres/`
- `closed/internal/platform/auditlog/`
- `open/api/openapi/experiments.yaml`

## M2 — Run Execution & Reproducibility
Status: in_progress
Evidence:
- `closed/internal/execution/plan/`
- `closed/internal/execution/specvalidator/`
- `docs/enterprise/06-reproducibility-and-determinism.md`

## M3 — Artifacts & Object Storage
Status: in_progress
Evidence:
- `closed/experiments/artifacts_api.go`
- `closed/internal/service/artifacts/`
- `docs/enterprise/04-domain-model.md`

## M4 — Scheduling/Queues/Quotas
Status: not_started
Evidence:
- `docs/roadmap/execution-backlog.md`

## M5 — Security & Governance Hardening
Status: in_progress
Evidence:
- `closed/internal/platform/auth/oidc_flow_test.go`
- `closed/internal/platform/auth/session.go`
- `docs/security/rbac-enforcement.md`
- `docs/security/session-management.md`
- `docs/roadmap/p6-p7-acceptance.md`

## M6 — Pipelines (DAG)
Status: not_started
Evidence:
- `docs/roadmap/execution-backlog.md`

## M7 — Developer Environments
Status: done
Evidence:
- `closed/experiments/dev_env_api.go`
- `closed/experiments/dev_env_reconciler.go`
- `closed/dataplane/dev_env_controller.go`
- `docs/ops/devenv.md`

## M8 — Model Registry & Promotion
Status: done
Evidence:
- `closed/experiments/model_registry_api.go`
- `closed/experiments/model_registry_api_test.go`
- `open/api/openapi/experiments.yaml`

## M9 — Operability, Supply Chain, DR, E2E
Status: in_progress
Evidence:
- `docs/ops/backup-restore.md`
- `docs/ops/dr-game-day.md`
- `docs/ops/supply-chain.md`
- `docs/roadmap/production-grade-criteria-matrix.md`

## Lineage MVP (M9‑support)
Status: done
Evidence:
- `closed/lineage/api.go`
- `closed/lineage/api_test.go`
- `open/api/openapi/lineage.yaml`
