# Отчёт о гигиене рабочей ветки

**Контекст:** инвентаризация выполнена командными срезами `git status --porcelain=v1`, `git diff --name-only`, `git ls-files --others --exclude-standard`.  
**Цель:** зафиксировать остаточные файлы/диффы, предложить безопасные действия без вмешательства в M9‑коммиты.

## A) Вероятные локальные артефакты
Не обнаружены по текущему инвентарю.

## B) Кандидаты на активы репозитория (рекомендуемое действие — TRACK)
Эти файлы функционально связаны с опубликованными M9‑runbook’ами и должны быть добавлены отдельной janitor‑веткой.

- `closed/scripts/backup.sh` — TRACK
- `closed/scripts/restore.sh` — TRACK
- `closed/scripts/verify-restore.sh` — TRACK

## C) Подозрительные/крупные диффы, требующие отдельного ревью
Рекомендуемое действие — IGNORE (отложить, вынести в отдельный PR после ревью).  
Причина: объём и/или несоответствие текущему M9‑скоупу.

### C1. Изменённые отслеживаемые файлы (diffs)
- `closed/dataplane/api.go` — IGNORE
- `closed/dataplane/main.go` — IGNORE
- `closed/dataplane/run_spec_parser.go` — IGNORE
- `closed/dataplane/runtime.go` — IGNORE
- `closed/dataset-registry/api.go` — IGNORE
- `closed/dataset-registry/main.go` — IGNORE
- `closed/dataset-registry/service.go` — IGNORE
- `closed/experiments/api.go` — IGNORE
- `closed/experiments/artifacts_api.go` — IGNORE
- `closed/experiments/dp_client.go` — IGNORE
- `closed/experiments/dp_dispatch_api.go` — IGNORE
- `closed/experiments/dp_internal_api.go` — IGNORE
- `closed/experiments/dp_reconciler.go` — IGNORE
- `closed/experiments/execution_ledger_api.go` — IGNORE
- `closed/experiments/main.go` — IGNORE
- `closed/experiments/rbac_helpers.go` — IGNORE
- `closed/experiments/run_bindings_api.go` — IGNORE
- `closed/experiments/run_repro_bundle_api.go` — IGNORE
- `closed/experiments/run_spec_api.go` — IGNORE
- `closed/experiments/training_api.go` — IGNORE
- `closed/internal/dataplane/protocol.go` — IGNORE
- `closed/internal/domain/artifact.go` — IGNORE
- `closed/internal/domain/dataset.go` — IGNORE
- `closed/internal/domain/execution_state_test.go` — IGNORE
- `closed/internal/domain/model.go` — IGNORE
- `closed/internal/domain/run.go` — IGNORE
- `closed/internal/domain/run_spec.go` — IGNORE
- `closed/internal/execution/specvalidator/run_validator.go` — IGNORE
- `closed/internal/platform/k8s/client.go` — IGNORE
- `closed/internal/repo/interfaces.go` — IGNORE
- `closed/internal/repo/postgres/artifacts.go` — IGNORE
- `closed/internal/repo/postgres/datasets.go` — IGNORE
- `closed/internal/repo/postgres/models.go` — IGNORE
- `closed/internal/repo/postgres/run_bindings.go` — IGNORE
- `closed/internal/repo/postgres/run_specs.go` — IGNORE
- `closed/internal/repo/postgres/runs.go` — IGNORE
- `closed/internal/service/artifacts/service.go` — IGNORE
- `open/api/openapi/dataplane_internal.yaml` — IGNORE
- `open/api/openapi/dataset-registry.yaml` — IGNORE
- `open/api/openapi/experiments.yaml` — IGNORE

### C2. Неотслеживаемые файлы (untracked)
- `closed/dataset-registry/retention_worker.go` — IGNORE
- `closed/deploy/Dockerfile` — IGNORE
- `closed/deploy/docker-compose.yml` — IGNORE
- `closed/e2e/health_test.go` — IGNORE
- `closed/experiments/checksum_pair.go` — IGNORE
- `closed/experiments/model_export_store.go` — IGNORE
- `closed/experiments/model_registry_api.go` — IGNORE
- `closed/experiments/model_registry_service.go` — IGNORE
- `closed/experiments/model_registry_service_test.go` — IGNORE
- `closed/experiments/pipeline_helpers.go` — IGNORE
- `closed/experiments/pipeline_run_api.go` — IGNORE
- `closed/experiments/pipeline_scheduler.go` — IGNORE
- `closed/experiments/pipeline_spec_api.go` — IGNORE
- `closed/experiments/rbac_helpers_test.go` — IGNORE
- `closed/experiments/retention_helpers.go` — IGNORE
- `closed/experiments/retention_worker.go` — IGNORE
- `closed/experiments/run_artifact_store.go` — IGNORE
- `closed/experiments/run_cancel_api.go` — IGNORE
- `closed/experiments/run_dispatcher.go` — IGNORE
- `closed/experiments/run_retry.go` — IGNORE
- `closed/experiments/run_scheduler.go` — IGNORE
- `closed/experiments/run_spec_parse.go` — IGNORE
- `closed/internal/domain/model_lifecycle_test.go` — IGNORE
- `closed/internal/domain/pipeline_plan.go` — IGNORE
- `closed/internal/domain/pipeline_state.go` — IGNORE
- `closed/internal/domain/retention.go` — IGNORE
- `closed/internal/domain/scheduling.go` — IGNORE
- `closed/internal/execution/pipelineplan/plan.go` — IGNORE
- `closed/internal/execution/runspec/runspec.go` — IGNORE
- `closed/internal/platform/retention/config.go` — IGNORE
- `closed/internal/platform/retention/policy.go` — IGNORE
- `closed/internal/repo/postgres/model_export_migration_test.go` — IGNORE
- `closed/internal/repo/postgres/model_registry_migration_test.go` — IGNORE
- `closed/internal/repo/postgres/model_versions.go` — IGNORE
- `closed/internal/repo/postgres/model_versions_test.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_migration_test.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_nodes.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_nodes_test.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_plans.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_plans_test.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_runs.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_runs_test.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_specs.go` — IGNORE
- `closed/internal/repo/postgres/pipeline_specs_test.go` — IGNORE
- `closed/internal/repo/postgres/project_quotas.go` — IGNORE
- `closed/internal/repo/postgres/retention_policies.go` — IGNORE
- `closed/internal/repo/postgres/run_queue.go` — IGNORE
- `closed/internal/repo/postgres/run_retries.go` — IGNORE
- `closed/internal/service/pipelines/pipeline_smoke_test.go` — IGNORE
- `closed/internal/service/pipelines/pipelines_test.go` — IGNORE
- `closed/internal/service/pipelines/plan.go` — IGNORE
- `closed/internal/service/pipelines/state.go` — IGNORE
- `closed/internal/service/scheduling/backoff.go` — IGNORE
- `closed/internal/service/scheduling/backoff_test.go` — IGNORE
- `closed/internal/service/scheduling/selector.go` — IGNORE
- `closed/internal/service/scheduling/selector_test.go` — IGNORE
- `closed/migrations/000020_scheduler_queues_quotas.down.sql` — IGNORE
- `closed/migrations/000020_scheduler_queues_quotas.up.sql` — IGNORE
- `closed/migrations/000023_retention_policies.down.sql` — IGNORE
- `closed/migrations/000023_retention_policies.up.sql` — IGNORE
- `closed/migrations/000024_pipelines.down.sql` — IGNORE
- `closed/migrations/000024_pipelines.up.sql` — IGNORE
- `closed/migrations/000025_model_registry_versions.down.sql` — IGNORE
- `closed/migrations/000025_model_registry_versions.up.sql` — IGNORE
- `closed/migrations/000026_model_exports.down.sql` — IGNORE
- `closed/migrations/000026_model_exports.up.sql` — IGNORE
- `closed/scripts/README.md` — IGNORE
- `closed/scripts/dev.sh` — IGNORE
- `closed/scripts/migrate.sh` — IGNORE
- `closed/scripts/ml_cicd.py` — IGNORE
- `docs/README_ENTERPRISE_CHECKLIST.md` — IGNORE (guardrail: не трогать)
