# Полный контур тестирования (full-stack)

**Версия документа:** 1.0

## 1. Назначение

Полный контур тестирования предназначен для детерминированной проверки всей платформы Animus Datalab в условиях, приближенных к production‑эксплуатации. Контур охватывает unit/интеграционные проверки, контрактные проверки OpenAPI, системный прогон в k8s (kind), e2e‑сценарии и устойчивость к сбоям. Результаты собираются в каталог артефактов.

## 2. Быстрый запуск

```bash
make full-stack
```

Контур включает:
- `make guardrails-check`
- `make openapi-lint`
- `make test` (unit + integration при `ANIMUS_INTEGRATION=1`)
- `make integrations-test`
- `make ui-build`, `make ui-test`
- `make system-up` + `make system-test` (e2e) + `make system-down`
- `make dr-validate` (только при `ANIMUS_DR_VALIDATE=1`)

## 3. Предварительные требования

- Go (соответствующий версии проекта)
- Docker + Docker Compose (интеграционный контур)
- kind, kubectl, helm (системный контур)
- Node.js/NPM для UI (см. `closed/frontend_console/README.md`)

## 4. Переменные окружения

### 4.1 Интеграционный контур (без k8s)
- `ANIMUS_INTEGRATION=1` — включает integration‑тесты.
- `ANIMUS_TEST_DATABASE_URL` — PostgreSQL для integration‑тестов.
- `ANIMUS_TEST_MINIO_ENDPOINT`, `ANIMUS_TEST_MINIO_ACCESS_KEY`, `ANIMUS_TEST_MINIO_SECRET_KEY` — MinIO для integration‑тестов.

### 4.2 Системный контур (k8s/kind)
- `ANIMUS_SYSTEM_ENABLE=1` — включает `system-up`.
- `ANIMUS_KIND_CLUSTER_NAME` — имя kind‑кластера (по умолчанию `animus-fullstack`).
- `ANIMUS_SYSTEM_NAMESPACE` — namespace (по умолчанию `animus-system`).
- `ANIMUS_SYSTEM_IMAGE`, `ANIMUS_SYSTEM_CP_IMAGE`, `ANIMUS_SYSTEM_DP_IMAGE` — образы CP/DP.
- `ANIMUS_SYSTEM_LOAD_IMAGES=1` — загрузка локальных образов в kind.

### 4.3 E2E и устойчивость
- `ANIMUS_E2E_GATEWAY_URL` — URL gateway, если система уже поднята.
- `ANIMUS_E2E_DATABASE_URL` — БД для e2e‑действий (например, проверки политик).
- `ANIMUS_E2E_FAILURES=1` — включает проверку устойчивости (идемпотентность, DLQ, replay).

### 4.4 DR‑валидация
- `ANIMUS_DR_VALIDATE=1` — включает `make dr-validate` (опционально).

### 4.5 Артефакты
- `ANIMUS_ARTIFACTS_DIR` — путь к каталогу артефактов. Если не задан, используется `artifacts/<timestamp>`.

## 5. Артефакты и структура

Каталог артефактов формируется автоматически:
- `go-test-unit.json`, `go-test-integration.json`, `go-test-integrations.json` — JSON‑репорты Go‑тестов.
- `go-test-e2e.json` — JSON‑репорт e2e.
- `e2e_ids.json` — идентификаторы сущностей из e2e‑прогона.
- `gateway_metrics.txt` — снимок `/metrics`.
- `audit_events.json`, `audit_events.ndjson` — выборка аудита.
- `reproducibility_bundle.json`, `reproducibility_bundle.sha256` — экспорт входов Run и хэш.
- `k8s/` — события, список ресурсов и логи CP/DP/siem‑mock (при наличии кластера).
- `ui/` — артефакты UI‑тестов (если создаются).

## 6. Частичные запуски

- `make test` — unit + integration (интеграции при `ANIMUS_INTEGRATION=1`).
- `make integrations-test` — интеграционные проверки только в `closed/...`.
- `make system-up` / `make system-down` — поднятие и остановка kind‑кластера.
- `make system-test` / `make e2e-full` — e2e‑прогон.
- `make artifacts-collect` — сбор артефактов.

## 7. Типовые ошибки и диагностика

- `system-up: ANIMUS_SYSTEM_ENABLE not set` — установите `ANIMUS_SYSTEM_ENABLE=1`.
- `missing required tool: kind|kubectl|helm` — установите отсутствующие инструменты.
- `ANIMUS_E2E_GATEWAY_URL not set` — либо запустите `system-up`, либо укажите URL существующего gateway.
- `docker compose is required for integration harness` — установите Docker Compose.

## 8. Связь с критериями production‑grade

Полный контур тестирования используется как доказательная база для критериев AC‑01…AC‑10. Ссылки и соответствие приведены в `docs/roadmap/production-grade-criteria-matrix.md`.
