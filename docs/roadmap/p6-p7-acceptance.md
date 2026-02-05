# Приёмка P6/P7: Security & Ops Hardening

Документ фиксирует выполнение критериев P6/P7 и содержит ссылки на проверяемые артефакты. Все пункты имеют статус **satisfied**.

## P6 — Security Checklist

### P6‑1 RBAC deny‑by‑default и полный охват авторизации
Evidence:
- `closed/internal/platform/auth/middleware_rbac_test.go`
- `closed/internal/platform/rbac/authorizer.go`
- `docs/security/rbac-enforcement.md`
Status: satisfied

### P6‑2 Жизненный цикл сессии: TTL, принудительный logout, лимит сессий
Evidence:
- `closed/internal/platform/auth/session.go`
- `closed/internal/platform/auth/session_test.go`
- `docs/security/session-management.md`
Status: satisfied

### P6‑3 Secrets: DP‑only доступ, отсутствие утечек, redaction
Evidence:
- `closed/internal/platform/secrets/vault_k8s.go`
- `closed/internal/platform/secrets/vault_k8s_test.go`
- `closed/internal/platform/redaction/redaction_test.go`
- `docs/security/network-and-secrets.md`
- `docs/ops/secrets-backends.md`
Status: satisfied

### P6‑4 Audit: append‑only, полнота событий, экспортируемость
Evidence:
- `closed/internal/platform/auditlog/auditlog.go`
- `closed/internal/platform/auditlog/auditlog_test.go`
- `closed/internal/auditexport/worker.go`
- `closed/internal/auditexport/worker_idempotency_test.go`
- `docs/ops/siem-export.md`
Status: satisfied

### P6‑5 Network egress hardening (deny‑by‑default)
Evidence:
- `closed/dataplane/egress_policy.go`
- `closed/dataplane/egress_policy_test.go`
- `docs/security/network-and-secrets.md`
Status: satisfied

## P7 — Ops Checklist

### P7‑1 HA‑предпосылки и retry‑безопасные операции
Evidence:
- `closed/internal/service/runs/idempotency_test.go`
- `closed/experiments/dp_reconciler_test.go`
- `closed/internal/execution/executor/dryrun/dryrun_test.go`
- `docs/ops/failure-modes.md`
Status: satisfied

### P7‑2 Наблюдаемость и SLO‑контуры
Evidence:
- `closed/internal/platform/httpserver/http_metrics.go`
- `closed/internal/platform/httpserver/http_metrics_test.go`
- `docs/ops/observability-slos.md`
Status: satisfied

### P7‑3 Backup/Restore + DR runbooks + dr‑validate harness
Evidence:
- `docs/ops/backup-restore.md`
- `docs/ops/dr-game-day.md`
- `docs/ops/dr-gameday-report.md`
- `docs/ops/reports/README.md`
- `closed/scripts/backup.sh`
- `closed/scripts/restore.sh`
- `closed/scripts/verify-restore.sh`
- `closed/scripts/dr-validate.sh`
- `Makefile`
Status: satisfied

### P7‑4 Failure injection и сходимость под отказами
Evidence:
- `docs/ops/failure-modes.md`
- `closed/internal/integrations/webhooks/worker_idempotency_test.go`
- `closed/internal/auditexport/worker_idempotency_test.go`
- `closed/internal/execution/executor/dryrun/dryrun_test.go`
- `closed/experiments/registry_verification_test.go`
Status: satisfied

### P7‑5 Air‑gapped и upgrade‑совместимость
Evidence:
- `docs/ops/airgapped-install.md`
- `docs/ops/helm-upgrade-rollback.md`
- `docs/ops/backup-restore.md`
Status: satisfied

## Финальные гейты (2026‑02‑05)

Команды выполнены в контролируемой среде без внешних зависимостей:
- `make guardrails-check`
- `make openapi-lint`
- `./scripts/go_test.sh ./closed/...`
- `make integrations-test`
- `make dr-validate` (без `ANIMUS_DR_VALIDATE` — no‑op)
