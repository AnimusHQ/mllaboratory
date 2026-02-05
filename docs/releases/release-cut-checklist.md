# Checklist релиз‑ката v1.1 (Hardening)

## 1. Ветвление
1. Создать ветку `release/v1.1` от актуального `main`.
2. Проверить чистоту рабочего дерева и отсутствие неучтённых файлов.

## 2. Обязательные гейты
1. `make guardrails-check`
2. `make openapi-lint`
3. `./scripts/go_test.sh ./closed/...`
4. `make integrations-test`
5. `make dr-validate` (ожидаемый no‑op без `ANIMUS_DR_VALIDATE=1`)

## 3. Supply‑chain гейты
1. `make sbom`
2. `make vuln-scan`
3. `make supply-chain`

## 4. Минимальные smoke‑проверки
1. `/healthz` и `/readyz` для gateway, experiments, dataset‑registry, audit.
2. Dataset CRUD: create → upload version → download.
3. Audit export: создание sink и проверка доставки в тестовый endpoint (если доступно).

## 5. DR‑валидация (инфраструктура доступна)
1. Установить `ANIMUS_DR_VALIDATE=1`.
2. Запустить `make dr-validate`.
3. Сохранить отчёт по пути из `ANIMUS_DR_REPORT_PATH` или `/tmp`.

## 6. Тегирование и публикация
1. Проставить тег `v1.1.0` (или утверждённый семантический тег).
2. Опубликовать контейнерные образы.
3. Опубликовать Helm charts и values schema.
4. Опубликовать OpenAPI спецификации из `open/api/openapi/*.yaml`.

## 7. Артефакты релиза
1. Release notes: `docs/releases/v1.1-hardening.md`.
2. Acceptance: `docs/roadmap/p6-p7-acceptance.md`.
3. Матрица критериев: `docs/roadmap/production-grade-criteria-matrix.md`.
