# Исполнительный бэклог

Документ формирует исполнимый бэклог по `roadmap.json` (v3). Для каждой задачи T* указаны пути реализации, контракты, миграции, аудит, безопасность, наблюдаемость, тесты, критерии приёмки и шаги верификации.

## M0: Базовые основы и архитектурный базис

**Статус:** завершено

### E00: Базовый репозиторий, инструменты, CI

#### T0001 — Определить структуру репозитория и границы модулей
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: Определить структуру репозитория и границы модулей
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

#### T0002 — CI‑конвейеры: линт, юнит‑тесты, форматирование, PR‑гейты
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: CI‑конвейеры: линт, юнит‑тесты, форматирование, PR‑гейты
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

#### T0003 — Базовая безопасность сканирования: SAST + скан зависимостей
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: Базовая безопасность сканирования: SAST + скан зависимостей
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

#### T0004 — Линтинг спецификаций/контрактов в CI (OpenAPI/protos + проверка совместимости)
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: Линтинг спецификаций/контрактов в CI (OpenAPI/protos + проверка совместимости)
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

### E01: Архитектурный базис и контракты

#### T0101 — Зафиксировать список сервисов CP/DP, границы доверия и ответственности
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Зафиксировать список сервисов CP/DP, границы доверия и ответственности
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0102 — Выбрать формат контрактов и схему версионирования (OpenAPI/gRPC + SemVer)
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Выбрать формат контрактов и схему версионирования (OpenAPI/gRPC + SemVer)
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0103 — Определить модель ошибок и идемпотентности
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Определить модель ошибок и идемпотентности
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0104 — Определить контракт протокола выполнения CP↔DP (сообщения + ретраи + идемпотентность)
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Определить контракт протокола выполнения CP↔DP (сообщения + ретраи + идемпотентность)
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

### E09: Упаковка и деплой базовой линии

#### T0901 — Helm‑чарты для компонентов CP/DP с поддержкой внешней БД/объектного хранилища
- implementation_paths: `closed/deploy/helm/`, `closed/deploy/`
- openapi: не применимо
- migrations: обновление helm‑миграций
- audit_events: не применимо
- security_controls: secure defaults в values
- observability: не применимо
- tests: helm‑lint/upgrade
- acceptance_criteria: выполнено и подтверждено: Helm‑чарты для компонентов CP/DP с поддержкой внешней БД/объектного хранилища
- verification_steps: Установка/апгрейд/rollback на чистом кластере.

#### T0902 — Схема конфигурации и валидация Helm values
- implementation_paths: `closed/deploy/helm/`, `closed/deploy/`
- openapi: не применимо
- migrations: обновление helm‑миграций
- audit_events: не применимо
- security_controls: secure defaults в values
- observability: не применимо
- tests: helm‑lint/upgrade
- acceptance_criteria: выполнено и подтверждено: Схема конфигурации и валидация Helm values
- verification_steps: Установка/апгрейд/rollback на чистом кластере.

#### T0903 — Тесты апгрейда/роллбэка между версиями (с политикой совместимости)
- implementation_paths: `closed/deploy/helm/`, `closed/deploy/`
- openapi: не применимо
- migrations: обновление helm‑миграций
- audit_events: не применимо
- security_controls: secure defaults в values
- observability: не применимо
- tests: helm‑lint/upgrade
- acceptance_criteria: выполнено и подтверждено: Тесты апгрейда/роллбэка между версиями (с политикой совместимости)
- verification_steps: Установка/апгрейд/rollback на чистом кластере.


## M1: Доменные модели и ядро метаданных

**Статус:** завершено

### E02: Хранилище метаданных и доменная персистентность

#### T0201 — Фреймворк миграций и поддержка внешней БД
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Фреймворк миграций и поддержка внешней БД
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0202 — CRUD Project + семантика архивации
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: CRUD Project + семантика архивации
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0203 — Иммутабельность Dataset и DatasetVersion
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Иммутабельность Dataset и DatasetVersion
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0204 — Ввести сущность CodeRef и ограничения production‑run
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Ввести сущность CodeRef и ограничения production‑run
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0205 — Сущности EnvironmentDefinition + EnvironmentLock
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Сущности EnvironmentDefinition + EnvironmentLock
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0206 — Сущность PolicySnapshot, хранимая с Run (RBAC, retention, network policy, ограничения шаблонов)
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Сущность PolicySnapshot, хранимая с Run (RBAC, retention, network policy, ограничения шаблонов)
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

### E03: Ядро API Control Plane

#### T0301 — Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0302 — CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0303 — Идемпотентные операции Create (Idempotency‑Key с сохранением)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Идемпотентные операции Create (Idempotency‑Key с сохранением)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0304 — Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

### E07: Артефакты и объектное хранилище

#### T0701 — Интеграция S3‑совместимого хранилища артефактов (write‑through из DP)
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Интеграция S3‑совместимого хранилища артефактов (write‑through из DP)
- verification_steps: Upload/download с аудитом.

#### T0702 — Медиация доступа к артефактам через CP (signed URL/proxy) с аудитом и RBAC
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Медиация доступа к артефактам через CP (signed URL/proxy) с аудитом и RBAC
- verification_steps: Upload/download с аудитом.

#### T0703 — Политики retention + безопасное удаление + legal hold
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Политики retention + безопасное удаление + legal hold
- verification_steps: Upload/download с аудитом.

### E11: Аудит и governance (append‑only + экспорт)

#### T1101 — Хранилище AuditEvent + покрытие обязательных hooks эмиссии
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Хранилище AuditEvent + покрытие обязательных hooks эмиссии
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1102 — Экспорт аудита в SIEM (webhook/syslog) с ретраями и идемпотентностью
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Экспорт аудита в SIEM (webhook/syslog) с ретраями и идемпотентностью
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1103 — Governance hooks: retention + legal hold
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Governance hooks: retention + legal hold
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1104 — Тесты полноты аудита (non‑disableable regression suite)
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Тесты полноты аудита (non‑disableable regression suite)
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.


## M2: Контракты исполнения, Run и детерминированное планирование

**Статус:** в работе

### E01: Архитектурный базис и контракты

#### T0101 — Зафиксировать список сервисов CP/DP, границы доверия и ответственности
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Зафиксировать список сервисов CP/DP, границы доверия и ответственности
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0102 — Выбрать формат контрактов и схему версионирования (OpenAPI/gRPC + SemVer)
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Выбрать формат контрактов и схему версионирования (OpenAPI/gRPC + SemVer)
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0103 — Определить модель ошибок и идемпотентности
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Определить модель ошибок и идемпотентности
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0104 — Определить контракт протокола выполнения CP↔DP (сообщения + ретраи + идемпотентность)
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Определить контракт протокола выполнения CP↔DP (сообщения + ретраи + идемпотентность)
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

### E02: Хранилище метаданных и доменная персистентность

#### T0201 — Фреймворк миграций и поддержка внешней БД
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Фреймворк миграций и поддержка внешней БД
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0202 — CRUD Project + семантика архивации
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: CRUD Project + семантика архивации
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0203 — Иммутабельность Dataset и DatasetVersion
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Иммутабельность Dataset и DatasetVersion
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0204 — Ввести сущность CodeRef и ограничения production‑run
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Ввести сущность CodeRef и ограничения production‑run
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0205 — Сущности EnvironmentDefinition + EnvironmentLock
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Сущности EnvironmentDefinition + EnvironmentLock
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0206 — Сущность PolicySnapshot, хранимая с Run (RBAC, retention, network policy, ограничения шаблонов)
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Сущность PolicySnapshot, хранимая с Run (RBAC, retention, network policy, ограничения шаблонов)
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

### E03: Ядро API Control Plane

#### T0301 — Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0302 — CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0303 — Идемпотентные операции Create (Idempotency‑Key с сохранением)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Идемпотентные операции Create (Idempotency‑Key с сохранением)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0304 — Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

### E04-Design: Контракты исполнения и валидация спецификаций

#### T0400 — Фиксация схемы PipelineSpec v1 + правила валидации (циклы, ссылки, дайджесты)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: Фиксация схемы PipelineSpec v1 + правила валидации (циклы, ссылки, дайджесты)
- verification_steps: Проверка валидаторов и стабильности хешей.

#### T0401 — RunSpec: принудительное соблюдение требований production‑run (DatasetVersionId+commit_sha+EnvLock)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: RunSpec: принудительное соблюдение требований production‑run (DatasetVersionId+commit_sha+EnvLock)
- verification_steps: Проверка валидаторов и стабильности хешей.

#### T0402 — Определить контракт диспетчеризации CP→DP (RunExecutionRequest)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: Определить контракт диспетчеризации CP→DP (RunExecutionRequest)
- verification_steps: Проверка валидаторов и стабильности хешей.

#### T0403 — Определить контракты отчётности DP→CP (heartbeat, terminal state, artifact commit)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: Определить контракты отчётности DP→CP (heartbeat, terminal state, artifact commit)
- verification_steps: Проверка валидаторов и стабильности хешей.

### E05-Prep: Жизненный цикл Run и автомат состояний (CP)

#### T0500 — Формализация автомата состояний Run (queued/running/succeeded/failed/canceled/unknown) + правила производного состояния
- implementation_paths: `closed/internal/execution/state/`, `closed/internal/service/runs/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: таблицы состояния/переходов (если добавляются)
- audit_events: EV005
- security_controls: project‑scoping на статусах
- observability: структурные логи
- tests: юнит (state machine)
- acceptance_criteria: выполнено и подтверждено: Формализация автомата состояний Run (queued/running/succeeded/failed/canceled/unknown) + правила производного состояния
- verification_steps: Проверка переходов и аудит‑событий.

#### T0501 — Укрепление хранения execution‑plan и hashing
- implementation_paths: `closed/internal/execution/state/`, `closed/internal/service/runs/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: таблицы состояния/переходов (если добавляются)
- audit_events: EV005
- security_controls: project‑scoping на статусах
- observability: структурные логи
- tests: юнит (state machine)
- acceptance_criteria: выполнено и подтверждено: Укрепление хранения execution‑plan и hashing
- verification_steps: Проверка переходов и аудит‑событий.

#### T0502 — Внутренний CP→DP endpoint диспетчеризации + идемпотентность
- implementation_paths: `closed/internal/execution/state/`, `closed/internal/service/runs/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: таблицы состояния/переходов (если добавляются)
- audit_events: EV005
- security_controls: project‑scoping на статусах
- observability: структурные логи
- tests: юнит (state machine)
- acceptance_criteria: выполнено и подтверждено: Внутренний CP→DP endpoint диспетчеризации + идемпотентность
- verification_steps: Проверка переходов и аудит‑событий.

### E08: Пайплайны (DAG) и PipelineRun

#### T0801 — Создание PipelineRun: материализация node‑runs + граф зависимостей
- implementation_paths: `closed/internal/execution/plan/`, `closed/internal/service/pipelines/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (PipelineRun)
- migrations: pipeline_runs/node_runs
- audit_events: EV005
- security_controls: RBAC на pipeline operations
- observability: pipeline‑метрики/трейсы
- tests: юнит/интеграционные DAG
- acceptance_criteria: выполнено и подтверждено: Создание PipelineRun: материализация node‑runs + граф зависимостей
- verification_steps: PipelineRun → node‑runs → завершение.

#### T0802 — DAG‑движок: диспетчеризация узлов в DP с retry/backoff
- implementation_paths: `closed/internal/execution/plan/`, `closed/internal/service/pipelines/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (PipelineRun)
- migrations: pipeline_runs/node_runs
- audit_events: EV005
- security_controls: RBAC на pipeline operations
- observability: pipeline‑метрики/трейсы
- tests: юнит/интеграционные DAG
- acceptance_criteria: выполнено и подтверждено: DAG‑движок: диспетчеризация узлов в DP с retry/backoff
- verification_steps: PipelineRun → node‑runs → завершение.

#### T0803 — API запроса графа (pipeline graph view) + пагинация/фильтры
- implementation_paths: `closed/internal/execution/plan/`, `closed/internal/service/pipelines/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (PipelineRun)
- migrations: pipeline_runs/node_runs
- audit_events: EV005
- security_controls: RBAC на pipeline operations
- observability: pipeline‑метрики/трейсы
- tests: юнит/интеграционные DAG
- acceptance_criteria: выполнено и подтверждено: API запроса графа (pipeline graph view) + пагинация/фильтры
- verification_steps: PipelineRun → node‑runs → завершение.


## M3: Data Plane runtime и реальное исполнение (Kubernetes)

**Статус:** в работе (частично выполнено)

### E01: Архитектурный базис и контракты

#### T0101 — Зафиксировать список сервисов CP/DP, границы доверия и ответственности
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Зафиксировать список сервисов CP/DP, границы доверия и ответственности
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0102 — Выбрать формат контрактов и схему версионирования (OpenAPI/gRPC + SemVer)
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Выбрать формат контрактов и схему версионирования (OpenAPI/gRPC + SemVer)
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0103 — Определить модель ошибок и идемпотентности
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Определить модель ошибок и идемпотентности
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

#### T0104 — Определить контракт протокола выполнения CP↔DP (сообщения + ретраи + идемпотентность)
- implementation_paths: `docs/enterprise/03-*.md`, `docs/contracts/`, `open/api/openapi/`, `api/pipeline_spec.yaml`
- openapi: open/api/openapi/*
- migrations: не применимо
- audit_events: не применимо
- security_controls: контрактные проверки ошибок/идемпотентности
- observability: не применимо
- tests: ревью + schema‑lint
- acceptance_criteria: выполнено и подтверждено: Определить контракт протокола выполнения CP↔DP (сообщения + ретраи + идемпотентность)
- verification_steps: Ревью документов и схем; запуск schema‑lint (после внедрения).

### E04-Design: Контракты исполнения и валидация спецификаций

#### T0400 — Фиксация схемы PipelineSpec v1 + правила валидации (циклы, ссылки, дайджесты)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: Фиксация схемы PipelineSpec v1 + правила валидации (циклы, ссылки, дайджесты)
- verification_steps: Проверка валидаторов и стабильности хешей.

#### T0401 — RunSpec: принудительное соблюдение требований production‑run (DatasetVersionId+commit_sha+EnvLock)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: RunSpec: принудительное соблюдение требований production‑run (DatasetVersionId+commit_sha+EnvLock)
- verification_steps: Проверка валидаторов и стабильности хешей.

#### T0402 — Определить контракт диспетчеризации CP→DP (RunExecutionRequest)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: Определить контракт диспетчеризации CP→DP (RunExecutionRequest)
- verification_steps: Проверка валидаторов и стабильности хешей.

#### T0403 — Определить контракты отчётности DP→CP (heartbeat, terminal state, artifact commit)
- implementation_paths: `docs/contracts/`, `docs/enterprise/05-execution-model.md`, `closed/internal/execution/`
- openapi: open/api/openapi/experiments.yaml (RunSpec)
- migrations: не применимо
- audit_events: EV001–EV003 (определение)
- security_controls: контрактная идемпотентность DP→CP
- observability: не применимо
- tests: юнит (валидаторы)
- acceptance_criteria: выполнено и подтверждено: Определить контракты отчётности DP→CP (heartbeat, terminal state, artifact commit)
- verification_steps: Проверка валидаторов и стабильности хешей.

### E04: Data Plane executor (Kubernetes)

#### T0410 — Каркас сервиса DP executor + базовый K8s контроллер/клиент
- implementation_paths: `closed/dataplane/`, `closed/internal/platform/k8s/`, `closed/internal/dataplane/`
- openapi: open/api/openapi/dataplane_internal.yaml
- migrations: `closed/migrations/000019_dp_runtime.*`
- audit_events: EV004, EV005
- security_controls: изоляция, network‑policy, least‑privilege
- observability: run‑scoped метрики/логи/трейсы
- tests: юнит (DP runtime)
- acceptance_criteria: выполнено и подтверждено: каркас сервиса DP executor с healthz/readyz и базовым K8s клиентом
- verification_steps: запуск DP сервиса и проверка `/healthz`, `/readyz`.

#### T0411 — Запуск рабочих нагрузок Run (Job/Pod) с лимитами ресурсов и изоляцией
- implementation_paths: `closed/dataplane/`, `closed/internal/platform/k8s/`, `closed/internal/dataplane/`
- openapi: open/api/openapi/dataplane_internal.yaml
- migrations: `closed/migrations/000019_dp_runtime.*`
- audit_events: EV004, EV005
- security_controls: изоляция, network‑policy, least‑privilege
- observability: run‑scoped метрики/логи/трейсы
- tests: юнит (DP job spec builder)
- acceptance_criteria: выполнено и подтверждено: Job/Pod формируется по EnvLock с digest‑pinned образами и лимитами ресурсов
- verification_steps: юнит‑тесты buildJobSpec и проверка label/env‑инъекций.

#### T0412 — Отчётность DP→CP: heartbeats и терминальные состояния
- implementation_paths: `closed/dataplane/`, `closed/experiments/dp_internal_api.go`, `closed/internal/repo/postgres/dp_events.go`
- openapi: open/api/openapi/dataplane_internal.yaml
- migrations: `closed/migrations/000019_dp_runtime.*`
- audit_events: EV004, EV005
- security_controls: изоляция, network‑policy, least‑privilege
- observability: run‑scoped метрики/логи/трейсы
- tests: юнит (идемпотентность событий DP→CP)
- acceptance_criteria: выполнено и подтверждено: DP отправляет heartbeats и терминальные события; CP принимает дубликаты
- verification_steps: повторная отправка heartbeat/terminal по одному event_id не изменяет терминальный статус.

#### T0413 — Запись артефактов с проверкой checksum + событие commit
- implementation_paths: `closed/internal/runtimeexec/`, `closed/internal/platform/k8s/`, новый DP сервис
- openapi: внутренний DP‑протокол (документ в `docs/contracts/`)
- migrations: таблицы диспетчеризации/статусов (при необходимости)
- audit_events: EV004, EV005, EV006
- security_controls: изоляция, network‑policy, least‑privilege
- observability: run‑scoped метрики/логи/трейсы
- tests: интеграционные/e2e DP
- acceptance_criteria: не выполнено: запись артефактов и commit‑событие отложены до M4/M5 интеграции артефактного хранилища
- verification_steps: требуется реализация артефактного канала и контроль checksum.

#### T0414 — Реконсиляция orphaned/unknown runs
- implementation_paths: `closed/experiments/dp_reconciler.go`, `closed/internal/repo/postgres/dp_events.go`
- openapi: open/api/openapi/dataplane_internal.yaml
- migrations: `closed/migrations/000019_dp_runtime.*`
- audit_events: EV005, EV014
- security_controls: изоляция, network‑policy, least‑privilege
- observability: run‑scoped метрики/логи/трейсы
- tests: юнит (правила реконсиляции)
- acceptance_criteria: выполнено и подтверждено: периодическая реконсиляция Run по данным DP с неизменяемыми терминальными состояниями
- verification_steps: симуляция устаревшего heartbeat и проверка `run.reconciled`.

#### T0415 — Телеметрия Run‑scoped: логи и трассы по run_id
- implementation_paths: `closed/internal/runtimeexec/`, `closed/internal/platform/k8s/`, новый DP сервис
- openapi: внутренний DP‑протокол (документ в `docs/contracts/`)
- migrations: таблицы диспетчеризации/статусов (при необходимости)
- audit_events: EV004, EV005, EV006
- security_controls: изоляция, network‑policy, least‑privilege
- observability: run‑scoped метрики/логи/трейсы
- tests: интеграционные/e2e DP
- acceptance_criteria: не выполнено: трассировка и метрики Run‑scoped запланированы в M5/M9
- verification_steps: требуется интеграция OTel/Prometheus и корреляция по run_id.

### E07: Артефакты и объектное хранилище

#### T0701 — Интеграция S3‑совместимого хранилища артефактов (write‑through из DP)
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Интеграция S3‑совместимого хранилища артефактов (write‑through из DP)
- verification_steps: Upload/download с аудитом.

#### T0702 — Медиация доступа к артефактам через CP (signed URL/proxy) с аудитом и RBAC
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Медиация доступа к артефактам через CP (signed URL/proxy) с аудитом и RBAC
- verification_steps: Upload/download с аудитом.

#### T0703 — Политики retention + безопасное удаление + legal hold
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Политики retention + безопасное удаление + legal hold
- verification_steps: Upload/download с аудитом.


## M4: Планирование, очереди, квоты, отмена

**Статус:** выполнено

### E03: Ядро API Control Plane

#### T0301 — Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0302 — CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0303 — Идемпотентные операции Create (Idempotency‑Key с сохранением)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Идемпотентные операции Create (Idempotency‑Key с сохранением)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0304 — Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

### E05-Prep: Жизненный цикл Run и автомат состояний (CP)

#### T0500 — Формализация автомата состояний Run (queued/running/succeeded/failed/canceled/unknown) + правила производного состояния
- implementation_paths: `closed/internal/execution/state/`, `closed/internal/service/runs/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: таблицы состояния/переходов (если добавляются)
- audit_events: EV005
- security_controls: project‑scoping на статусах
- observability: структурные логи
- tests: юнит (state machine)
- acceptance_criteria: выполнено и подтверждено: Формализация автомата состояний Run (queued/running/succeeded/failed/canceled/unknown) + правила производного состояния
- verification_steps: Проверка переходов и аудит‑событий.

#### T0501 — Укрепление хранения execution‑plan и hashing
- implementation_paths: `closed/internal/execution/state/`, `closed/internal/service/runs/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: таблицы состояния/переходов (если добавляются)
- audit_events: EV005
- security_controls: project‑scoping на статусах
- observability: структурные логи
- tests: юнит (state machine)
- acceptance_criteria: выполнено и подтверждено: Укрепление хранения execution‑plan и hashing
- verification_steps: Проверка переходов и аудит‑событий.

#### T0502 — Внутренний CP→DP endpoint диспетчеризации + идемпотентность
- implementation_paths: `closed/internal/execution/state/`, `closed/internal/service/runs/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: таблицы состояния/переходов (если добавляются)
- audit_events: EV005
- security_controls: project‑scoping на статусах
- observability: структурные логи
- tests: юнит (state machine)
- acceptance_criteria: выполнено и подтверждено: Внутренний CP→DP endpoint диспетчеризации + идемпотентность
- verification_steps: Проверка переходов и аудит‑событий.

### E05: Планирование, очереди, квоты, ретраи, отмена

#### T0510 — Сервис планировщика: очереди + приоритеты + квоты по проекту
- implementation_paths: `closed/internal/service/scheduling/`, `closed/internal/repo/postgres/run_queue.go`, `closed/internal/repo/postgres/project_quotas.go`, `closed/experiments/run_scheduler.go`
- openapi: open/api/openapi/experiments.yaml (queue/priority fields)
- migrations: `closed/migrations/000020_scheduler_queues_quotas.*.sql`
- audit_events: run.queued, run.dequeued, run.dispatch_attempted, run.dispatch_blocked, run.dispatched
- security_controls: квоты по проектам, project‑scoping
- observability: логи scheduler (batch/decisions)
- tests: `closed/internal/service/scheduling/selector_test.go`
- acceptance_criteria: выполнено и подтверждено: Сервис планировщика: очереди + приоритеты + квоты по проекту
- verification_steps: Создать Run, проверить очередь, квоту, детерминированный порядок и отказ по QUOTA_EXCEEDED.

#### T0511 — Политики retry/backoff для Run (платформенные vs пользовательские ошибки)
- implementation_paths: `closed/internal/domain/scheduling.go`, `closed/internal/service/scheduling/backoff.go`, `closed/internal/repo/postgres/run_retries.go`, `closed/experiments/run_retry.go`
- openapi: open/api/openapi/experiments.yaml (retryPolicy в RunSpec/создании)
- migrations: `closed/migrations/000020_scheduler_queues_quotas.*.sql`
- audit_events: run.retry_scheduled, run.queued
- security_controls: пер‑проекты, ретраи только по политике RunSpec
- observability: audit trail ретраев
- tests: `closed/internal/service/scheduling/backoff_test.go`
- acceptance_criteria: выполнено и подтверждено: Политики retry/backoff для Run (платформенные vs пользовательские ошибки)
- verification_steps: Сымитировать failed Run, проверить создание retry‑run и детерминированный backoff.

#### T0512 — Семантика отмены end‑to‑end (CP→DP) + правила retention артефактов
- implementation_paths: `closed/experiments/run_cancel_api.go`, `closed/dataplane/api.go`, `closed/internal/platform/k8s/client.go`
- openapi: open/api/openapi/experiments.yaml, open/api/openapi/dataplane_internal.yaml
- migrations: не требуется
- audit_events: run.canceled, run.cancellation_propagated
- security_controls: терминальные состояния неизменяемы
- observability: audit trail отмены
- tests: `closed/internal/domain/execution_state_test.go`
- acceptance_criteria: выполнено и подтверждено: Семантика отмены end‑to‑end (CP→DP)
- verification_steps: Отменить Run в очереди и в выполнении; проверить dispatch status и audit.

#### T0513 — Backpressure и rate‑limits (API + scheduler) по проектам
- implementation_paths: планируется
- openapi: планируется
- migrations: планируется
- audit_events: планируется
- security_controls: планируется
- observability: планируется
- tests: планируется
- acceptance_criteria: не выполнено: rate‑limits/backpressure отложены
- verification_steps: не применимо


## M5: Усиление безопасности и управления

**Статус:** в работе

### E02: Хранилище метаданных и доменная персистентность

#### T0201 — Фреймворк миграций и поддержка внешней БД
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Фреймворк миграций и поддержка внешней БД
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0202 — CRUD Project + семантика архивации
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: CRUD Project + семантика архивации
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0203 — Иммутабельность Dataset и DatasetVersion
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Иммутабельность Dataset и DatasetVersion
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0204 — Ввести сущность CodeRef и ограничения production‑run
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Ввести сущность CodeRef и ограничения production‑run
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0205 — Сущности EnvironmentDefinition + EnvironmentLock
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Сущности EnvironmentDefinition + EnvironmentLock
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

#### T0206 — Сущность PolicySnapshot, хранимая с Run (RBAC, retention, network policy, ограничения шаблонов)
- implementation_paths: `closed/internal/domain/`, `closed/internal/repo/postgres/`, `closed/migrations/`
- openapi: open/api/openapi/dataset-registry.yaml, open/api/openapi/experiments.yaml
- migrations: `closed/migrations/*` (новые сущности/индексы)
- audit_events: EV001, EV002 (Run lifecycle), локальные audit‑события для Create
- security_controls: проверки project‑scoping, неизменяемость
- observability: не применимо
- tests: юнит (repo/domain), миграции (migrate‑up)
- acceptance_criteria: выполнено и подтверждено: Сущность PolicySnapshot, хранимая с Run (RBAC, retention, network policy, ограничения шаблонов)
- verification_steps: Применить миграции; создать сущности через API; проверить неизменяемость.

### E03: Ядро API Control Plane

#### T0301 — Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0302 — CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0303 — Идемпотентные операции Create (Idempotency‑Key с сохранением)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Идемпотентные операции Create (Idempotency‑Key с сохранением)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0304 — Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

### E06: Аутентификация и авторизация (SSO + RBAC)

#### T0601 — OIDC‑аутентификация с TTL сессии и принудительным logout
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: OIDC‑аутентификация с TTL сессии и принудительным logout
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

#### T0602 — Опция SAML поверх той же auth‑абстракции (при необходимости)
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: Опция SAML поверх той же auth‑абстракции (при необходимости)
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

#### T0603 — RBAC‑роли и биндинги ролей проекта (группы IdP → роли)
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: RBAC‑роли и биндинги ролей проекта (группы IdP → роли)
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

#### T0604 — Object‑level авторизация (Dataset/Run/Model) для read/download/export
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: Object‑level авторизация (Dataset/Run/Model) для read/download/export
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

### E07: Артефакты и объектное хранилище

#### T0701 — Интеграция S3‑совместимого хранилища артефактов (write‑through из DP)
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Интеграция S3‑совместимого хранилища артефактов (write‑through из DP)
- verification_steps: Upload/download с аудитом.

#### T0702 — Медиация доступа к артефактам через CP (signed URL/proxy) с аудитом и RBAC
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Медиация доступа к артефактам через CP (signed URL/proxy) с аудитом и RBAC
- verification_steps: Upload/download с аудитом.

#### T0703 — Политики retention + безопасное удаление + legal hold
- implementation_paths: `closed/internal/storage/objectstore/`, `closed/internal/service/artifacts/`, `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml
- migrations: artifact‑таблицы/retention
- audit_events: EV006
- security_controls: RBAC на download/export
- observability: метрики выдачи URL
- tests: юнит/интеграционные artifacts
- acceptance_criteria: выполнено и подтверждено: Политики retention + безопасное удаление + legal hold
- verification_steps: Upload/download с аудитом.

### E10: Управление секретами (Vault‑like)

#### T1001 — Интеграция secrets‑manager для эпhemeral‑учёток (только во время исполнения)
- implementation_paths: `closed/internal/platform/secrets/` (новое), DP executor
- openapi: experiments.yaml (execution), DP protocol
- migrations: таблицы секретов/TTL (если требуется)
- audit_events: audit.secret.access (локальный action)
- security_controls: TTL, redaction, deny‑by‑default
- observability: метрики выдачи секретов
- tests: security suite (redaction)
- acceptance_criteria: выполнено и подтверждено: Интеграция secrets‑manager для эпhemeral‑учёток (только во время исполнения)
- verification_steps: Секреты выдаются только в DP, логов/ответов без значений.

#### T1002 — Пайплайн редактирования секретов в логах/артефактах
- implementation_paths: `closed/internal/platform/secrets/` (новое), DP executor
- openapi: experiments.yaml (execution), DP protocol
- migrations: таблицы секретов/TTL (если требуется)
- audit_events: audit.secret.access (локальный action)
- security_controls: TTL, redaction, deny‑by‑default
- observability: метрики выдачи секретов
- tests: security suite (redaction)
- acceptance_criteria: выполнено и подтверждено: Пайплайн редактирования секретов в логах/артефактах
- verification_steps: Секреты выдаются только в DP, логов/ответов без значений.

### E11: Аудит и governance (append‑only + экспорт)

#### T1101 — Хранилище AuditEvent + покрытие обязательных hooks эмиссии
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Хранилище AuditEvent + покрытие обязательных hooks эмиссии
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1102 — Экспорт аудита в SIEM (webhook/syslog) с ретраями и идемпотентностью
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Экспорт аудита в SIEM (webhook/syslog) с ретраями и идемпотентностью
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1103 — Governance hooks: retention + legal hold
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Governance hooks: retention + legal hold
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1104 — Тесты полноты аудита (non‑disableable regression suite)
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Тесты полноты аудита (non‑disableable regression suite)
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

### E18: Network policy, egress control, zero‑trust

#### T1801 — Default‑deny egress для нагрузок DP; allowlist‑модель (по проекту/шаблону)
- implementation_paths: `closed/internal/platform/k8s/`, `closed/internal/runtimeexec/`
- openapi: DP protocol (policy snapshots)
- migrations: network policy tables (если требуются)
- audit_events: policy/network change events
- security_controls: default‑deny egress
- observability: network policy metrics
- tests: security regression (egress)
- acceptance_criteria: выполнено и подтверждено: Default‑deny egress для нагрузок DP; allowlist‑модель (по проекту/шаблону)
- verification_steps: Запрет egress без allowlist.

#### T1802 — Стратегия изоляции namespace (по проекту) + усиление K8s RBAC
- implementation_paths: `closed/internal/platform/k8s/`, `closed/internal/runtimeexec/`
- openapi: DP protocol (policy snapshots)
- migrations: network policy tables (если требуются)
- audit_events: policy/network change events
- security_controls: default‑deny egress
- observability: network policy metrics
- tests: security regression (egress)
- acceptance_criteria: выполнено и подтверждено: Стратегия изоляции namespace (по проекту) + усиление K8s RBAC
- verification_steps: Запрет egress без allowlist.


## M6: Пайплайны (DAG) и оркестрация

**Статус:** выполнено

### E03: Ядро API Control Plane

#### T0301 — Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0302 — CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0303 — Идемпотентные операции Create (Idempotency‑Key с сохранением)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Идемпотентные операции Create (Idempotency‑Key с сохранением)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0304 — Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

### E05: Планирование, очереди, квоты, ретраи, отмена

#### T0510 — Сервис планировщика: очереди + приоритеты + квоты по проекту
- implementation_paths: `closed/internal/scheduler/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (очереди/квоты)
- migrations: таблицы очередей/квот
- audit_events: EV005 (+ локальные audit‑события очередей)
- security_controls: квоты по проектам, RBAC
- observability: метрики очередей
- tests: юнит/интеграционные scheduler
- acceptance_criteria: выполнено и подтверждено: Сервис планировщика: очереди + приоритеты + квоты по проекту
- verification_steps: Очереди/квоты/отмена в интеграционном сценарии.

#### T0511 — Политики retry/backoff для Run (платформенные vs пользовательские ошибки)
- implementation_paths: `closed/internal/scheduler/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (очереди/квоты)
- migrations: таблицы очередей/квот
- audit_events: EV005 (+ локальные audit‑события очередей)
- security_controls: квоты по проектам, RBAC
- observability: метрики очередей
- tests: юнит/интеграционные scheduler
- acceptance_criteria: выполнено и подтверждено: Политики retry/backoff для Run (платформенные vs пользовательские ошибки)
- verification_steps: Очереди/квоты/отмена в интеграционном сценарии.

#### T0512 — Семантика отмены end‑to‑end (CP→DP) + правила retention артефактов
- implementation_paths: `closed/internal/scheduler/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (очереди/квоты)
- migrations: таблицы очередей/квот
- audit_events: EV005 (+ локальные audit‑события очередей)
- security_controls: квоты по проектам, RBAC
- observability: метрики очередей
- tests: юнит/интеграционные scheduler
- acceptance_criteria: выполнено и подтверждено: Семантика отмены end‑to‑end (CP→DP) + правила retention артефактов
- verification_steps: Очереди/квоты/отмена в интеграционном сценарии.

#### T0513 — Backpressure и rate‑limits (API + scheduler) по проектам
- implementation_paths: `closed/internal/scheduler/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (очереди/квоты)
- migrations: таблицы очередей/квот
- audit_events: EV005 (+ локальные audit‑события очередей)
- security_controls: квоты по проектам, RBAC
- observability: метрики очередей
- tests: юнит/интеграционные scheduler
- acceptance_criteria: выполнено и подтверждено: Backpressure и rate‑limits (API + scheduler) по проектам
- verification_steps: Очереди/квоты/отмена в интеграционном сценарии.

### E08: Пайплайны (DAG) и PipelineRun

#### T0801 — Создание PipelineRun: материализация node‑runs + граф зависимостей
- implementation_paths: `closed/internal/execution/plan/`, `closed/internal/service/pipelines/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (PipelineRun)
- migrations: pipeline_runs/node_runs
- audit_events: EV005
- security_controls: RBAC на pipeline operations
- observability: pipeline‑метрики/трейсы
- tests: юнит/интеграционные DAG
- acceptance_criteria: выполнено и подтверждено: PipelineSpec сохранён, PipelineRun создаёт plan_hash и node‑runs.
- verification_steps: `GOCACHE=/tmp/go-cache go test ./closed/...`; создать PipelineSpec → PipelineRun → убедиться в наличии plan_hash и node‑runs.

#### T0802 — DAG‑движок: диспетчеризация узлов в DP с retry/backoff
- implementation_paths: `closed/internal/execution/plan/`, `closed/internal/service/pipelines/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (PipelineRun)
- migrations: pipeline_runs/node_runs
- audit_events: EV005
- security_controls: RBAC на pipeline operations
- observability: pipeline‑метрики/трейсы
- tests: юнит/интеграционные DAG
- acceptance_criteria: выполнено и подтверждено: DAG‑движок диспетчеризует узлы, учитывает retry/backoff и отмену.
- verification_steps: запустить PipelineRun с DAG; вызвать terminal `failed` для узла → проверить retry‑run и backoff; `POST /projects/{project_id}/pipeline-runs/{pipeline_run_id}:cancel` → отмена узлов.

#### T0803 — API запроса графа (pipeline graph view) + пагинация/фильтры
- implementation_paths: `closed/internal/execution/plan/`, `closed/internal/service/pipelines/` (новое), `closed/experiments/`
- openapi: open/api/openapi/experiments.yaml (PipelineRun)
- migrations: pipeline_runs/node_runs
- audit_events: EV005
- security_controls: RBAC на pipeline operations
- observability: pipeline‑метрики/трейсы
- tests: юнит/интеграционные DAG
- acceptance_criteria: выполнено и подтверждено: Graph API возвращает узлы/рёбра со статусами и пагинацией.
- verification_steps: `GET /projects/{project_id}/pipeline-runs/{pipeline_run_id}/graph?limit=2&offset=0` → проверить nodes/edges и nextOffset.


## M7: Среды разработки (Notebooks/IDE/Terminals)

**Статус:** не начато

### E12: Управляемые среды разработки

#### T1201 — Контроллер DevEnv (create/stop/TTL/квоты) в DP
- implementation_paths: `closed/internal/runtimeexec/` + новый DevEnv контроллер
- openapi: DP protocol + CP APIs (новое)
- migrations: dev_env_sessions
- audit_events: session access events
- security_controls: RBAC, TTL, network policies
- observability: session‑метрики
- tests: интеграционные DevEnv
- acceptance_criteria: выполнено и подтверждено: Контроллер DevEnv (create/stop/TTL/квоты) в DP
- verification_steps: Создание/остановка DevEnv, аудит сессий.

#### T1202 — Эндпоинты удалённого доступа (terminal + IDE proxy) с auth и аудитом
- implementation_paths: `closed/internal/runtimeexec/` + новый DevEnv контроллер
- openapi: DP protocol + CP APIs (новое)
- migrations: dev_env_sessions
- audit_events: session access events
- security_controls: RBAC, TTL, network policies
- observability: session‑метрики
- tests: интеграционные DevEnv
- acceptance_criteria: выполнено и подтверждено: Эндпоинты удалённого доступа (terminal + IDE proxy) с auth и аудитом
- verification_steps: Создание/остановка DevEnv, аудит сессий.

#### T1203 — Шаблоны окружений (CPU/GPU) с версионированием и политикой выбора
- implementation_paths: `closed/internal/runtimeexec/` + новый DevEnv контроллер
- openapi: DP protocol + CP APIs (новое)
- migrations: dev_env_sessions
- audit_events: session access events
- security_controls: RBAC, TTL, network policies
- observability: session‑метрики
- tests: интеграционные DevEnv
- acceptance_criteria: выполнено и подтверждено: Шаблоны окружений (CPU/GPU) с версионированием и политикой выбора
- verification_steps: Создание/остановка DevEnv, аудит сессий.


## M8: Регистр моделей и workflow продвижения

**Статус:** выполнено

### E03: Ядро API Control Plane

#### T0301 — Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Каркас API‑шлюза/сервиса с auth middleware, request ID и структурным логированием
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0302 — CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: CRUD эндпоинты для базовых ресурсов (Projects/Datasets/DatasetVersions/Artifacts/Runs/Models/Environments)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0303 — Идемпотентные операции Create (Idempotency‑Key с сохранением)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Идемпотентные операции Create (Idempotency‑Key с сохранением)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

#### T0304 — Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- implementation_paths: `closed/gateway/`, `closed/experiments/`, `closed/dataset-registry/`, `closed/quality/`, `closed/lineage/`, `open/api/openapi/`
- openapi: open/api/openapi/gateway.yaml, experiments.yaml, dataset-registry.yaml, quality.yaml, lineage.yaml, audit.yaml
- migrations: по необходимости (новые API‑сущности)
- audit_events: EV001, EV002, EV003, EV006 (+ локальные audit‑события)
- security_controls: RBAC/Project‑scoping на API
- observability: request_id, структурные логи
- tests: API юнит/интеграционные
- acceptance_criteria: выполнено и подтверждено: Эндпоинт воспроизводимости: экспорт входов Run (DatasetVersion + CodeRef + EnvLock + параметры + policy snapshot)
- verification_steps: Прогон API‑сценариев и аудит‑событий.

### E13: Регистр моделей и продвижение

#### T1301 — Сущности Model/ModelVersion + переходы статусов
- implementation_paths: `closed/internal/domain/model.go`, `closed/internal/repo/postgres/models.go`, новые API
- openapi: новый OpenAPI (models.yaml) или расширение experiments.yaml
- migrations: models/model_versions
- audit_events: model.create/promotion events
- security_controls: RBAC на promote/export
- observability: метрики по промоуту
- tests: юнит/интеграционные model registry
- acceptance_criteria: выполнено и подтверждено: Сущности Model/ModelVersion + переходы статусов
- verification_steps: CRUD модели и промоут с аудитом.

#### T1302 — Workflow валидации/аппрува с enforce по ролям
- implementation_paths: `closed/internal/domain/model.go`, `closed/internal/repo/postgres/models.go`, новые API
- openapi: новый OpenAPI (models.yaml) или расширение experiments.yaml
- migrations: models/model_versions
- audit_events: model.create/promotion events
- security_controls: RBAC на promote/export
- observability: метрики по промоуту
- tests: юнит/интеграционные model registry
- acceptance_criteria: выполнено и подтверждено: Workflow валидации/аппрува с enforce по ролям
- verification_steps: CRUD модели и промоут с аудитом.

#### T1303 — Интерфейс экспорта модели + политические ограничения + аудит/события
- implementation_paths: `closed/internal/domain/model.go`, `closed/internal/repo/postgres/models.go`, новые API
- openapi: новый OpenAPI (models.yaml) или расширение experiments.yaml
- migrations: models/model_versions
- audit_events: model.create/promotion events
- security_controls: RBAC на promote/export
- observability: метрики по промоуту
- tests: юнит/интеграционные model registry
- acceptance_criteria: выполнено и подтверждено: Интерфейс экспорта модели + политические ограничения + аудит/события
- verification_steps: CRUD модели и промоут с аудитом.


## M9: Эксплуатационная готовность, HA/DR, упаковка, supply chain, E2E‑приёмка и релиз

**Статус:** не начато

### E00: Базовый репозиторий, инструменты, CI

#### T0001 — Определить структуру репозитория и границы модулей
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: Определить структуру репозитория и границы модулей
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

#### T0002 — CI‑конвейеры: линт, юнит‑тесты, форматирование, PR‑гейты
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: CI‑конвейеры: линт, юнит‑тесты, форматирование, PR‑гейты
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

#### T0003 — Базовая безопасность сканирования: SAST + скан зависимостей
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: Базовая безопасность сканирования: SAST + скан зависимостей
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

#### T0004 — Линтинг спецификаций/контрактов в CI (OpenAPI/protos + проверка совместимости)
- implementation_paths: `Makefile`, `.golangci.yml`, `.github/workflows/`, `tools/`
- openapi: open/api/openapi/* (линт/совместимость контрактов)
- migrations: не применимо
- audit_events: не применимо
- security_controls: SAST и сканирование зависимостей в CI, policy‑гейты
- observability: не применимо
- tests: CI‑линт/юнит, smoke‑проверки
- acceptance_criteria: выполнено и подтверждено: Линтинг спецификаций/контрактов в CI (OpenAPI/protos + проверка совместимости)
- verification_steps: Запуск `make lint` и `make test`; проверка CI‑гейтов на PR.

### E06: Аутентификация и авторизация (SSO + RBAC)

#### T0601 — OIDC‑аутентификация с TTL сессии и принудительным logout
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: OIDC‑аутентификация с TTL сессии и принудительным logout
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

#### T0602 — Опция SAML поверх той же auth‑абстракции (при необходимости)
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: Опция SAML поверх той же auth‑абстракции (при необходимости)
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

#### T0603 — RBAC‑роли и биндинги ролей проекта (группы IdP → роли)
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: RBAC‑роли и биндинги ролей проекта (группы IdP → роли)
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

#### T0604 — Object‑level авторизация (Dataset/Run/Model) для read/download/export
- implementation_paths: `closed/internal/platform/auth/`, `closed/gateway/`, `closed/internal/platform/rbac/` (новое)
- openapi: open/api/openapi/gateway.yaml
- migrations: таблицы ролей/биндингов
- audit_events: EV007 + authn‑события
- security_controls: OIDC/SAML, RBAC enforcement
- observability: auth‑метрики/логи
- tests: RBAC regression suite
- acceptance_criteria: выполнено и подтверждено: Object‑level авторизация (Dataset/Run/Model) для read/download/export
- verification_steps: SSO‑логин, запрет/разрешение операций по матрице.

### E09: Упаковка и деплой базовой линии

#### T0901 — Helm‑чарты для компонентов CP/DP с поддержкой внешней БД/объектного хранилища
- implementation_paths: `closed/deploy/helm/`, `closed/deploy/`
- openapi: не применимо
- migrations: обновление helm‑миграций
- audit_events: не применимо
- security_controls: secure defaults в values
- observability: не применимо
- tests: helm‑lint/upgrade
- acceptance_criteria: выполнено и подтверждено: Helm‑чарты для компонентов CP/DP с поддержкой внешней БД/объектного хранилища
- verification_steps: Установка/апгрейд/rollback на чистом кластере.

#### T0902 — Схема конфигурации и валидация Helm values
- implementation_paths: `closed/deploy/helm/`, `closed/deploy/`
- openapi: не применимо
- migrations: обновление helm‑миграций
- audit_events: не применимо
- security_controls: secure defaults в values
- observability: не применимо
- tests: helm‑lint/upgrade
- acceptance_criteria: выполнено и подтверждено: Схема конфигурации и валидация Helm values
- verification_steps: Установка/апгрейд/rollback на чистом кластере.

#### T0903 — Тесты апгрейда/роллбэка между версиями (с политикой совместимости)
- implementation_paths: `closed/deploy/helm/`, `closed/deploy/`
- openapi: не применимо
- migrations: обновление helm‑миграций
- audit_events: не применимо
- security_controls: secure defaults в values
- observability: не применимо
- tests: helm‑lint/upgrade
- acceptance_criteria: выполнено и подтверждено: Тесты апгрейда/роллбэка между версиями (с политикой совместимости)
- verification_steps: Установка/апгрейд/rollback на чистом кластере.

### E10: Управление секретами (Vault‑like)

#### T1001 — Интеграция secrets‑manager для эпhemeral‑учёток (только во время исполнения)
- implementation_paths: `closed/internal/platform/secrets/` (новое), DP executor
- openapi: experiments.yaml (execution), DP protocol
- migrations: таблицы секретов/TTL (если требуется)
- audit_events: audit.secret.access (локальный action)
- security_controls: TTL, redaction, deny‑by‑default
- observability: метрики выдачи секретов
- tests: security suite (redaction)
- acceptance_criteria: выполнено и подтверждено: Интеграция secrets‑manager для эпhemeral‑учёток (только во время исполнения)
- verification_steps: Секреты выдаются только в DP, логов/ответов без значений.

#### T1002 — Пайплайн редактирования секретов в логах/артефактах
- implementation_paths: `closed/internal/platform/secrets/` (новое), DP executor
- openapi: experiments.yaml (execution), DP protocol
- migrations: таблицы секретов/TTL (если требуется)
- audit_events: audit.secret.access (локальный action)
- security_controls: TTL, redaction, deny‑by‑default
- observability: метрики выдачи секретов
- tests: security suite (redaction)
- acceptance_criteria: выполнено и подтверждено: Пайплайн редактирования секретов в логах/артефактах
- verification_steps: Секреты выдаются только в DP, логов/ответов без значений.

### E11: Аудит и governance (append‑only + экспорт)

#### T1101 — Хранилище AuditEvent + покрытие обязательных hooks эмиссии
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Хранилище AuditEvent + покрытие обязательных hooks эмиссии
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1102 — Экспорт аудита в SIEM (webhook/syslog) с ретраями и идемпотентностью
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Экспорт аудита в SIEM (webhook/syslog) с ретраями и идемпотентностью
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1103 — Governance hooks: retention + legal hold
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Governance hooks: retention + legal hold
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

#### T1104 — Тесты полноты аудита (non‑disableable regression suite)
- implementation_paths: `closed/audit/`, `closed/internal/platform/auditlog/`, `closed/internal/auditexport/`
- openapi: open/api/openapi/audit.yaml
- migrations: audit_events, экспортные таблицы
- audit_events: EV008 + покрытие EV001–EV007
- security_controls: защита экспорта, project‑filters
- observability: метрики экспорта
- tests: audit completeness suite
- acceptance_criteria: выполнено и подтверждено: Тесты полноты аудита (non‑disableable regression suite)
- verification_steps: Экспорт в NDJSON/вебхук с ретраями.

### E12: Управляемые среды разработки

#### T1201 — Контроллер DevEnv (create/stop/TTL/квоты) в DP
- implementation_paths: `closed/internal/runtimeexec/` + новый DevEnv контроллер
- openapi: DP protocol + CP APIs (новое)
- migrations: dev_env_sessions
- audit_events: session access events
- security_controls: RBAC, TTL, network policies
- observability: session‑метрики
- tests: интеграционные DevEnv
- acceptance_criteria: выполнено и подтверждено: Контроллер DevEnv (create/stop/TTL/квоты) в DP
- verification_steps: Создание/остановка DevEnv, аудит сессий.

#### T1202 — Эндпоинты удалённого доступа (terminal + IDE proxy) с auth и аудитом
- implementation_paths: `closed/internal/runtimeexec/` + новый DevEnv контроллер
- openapi: DP protocol + CP APIs (новое)
- migrations: dev_env_sessions
- audit_events: session access events
- security_controls: RBAC, TTL, network policies
- observability: session‑метрики
- tests: интеграционные DevEnv
- acceptance_criteria: выполнено и подтверждено: Эндпоинты удалённого доступа (terminal + IDE proxy) с auth и аудитом
- verification_steps: Создание/остановка DevEnv, аудит сессий.

#### T1203 — Шаблоны окружений (CPU/GPU) с версионированием и политикой выбора
- implementation_paths: `closed/internal/runtimeexec/` + новый DevEnv контроллер
- openapi: DP protocol + CP APIs (новое)
- migrations: dev_env_sessions
- audit_events: session access events
- security_controls: RBAC, TTL, network policies
- observability: session‑метрики
- tests: интеграционные DevEnv
- acceptance_criteria: выполнено и подтверждено: Шаблоны окружений (CPU/GPU) с версионированием и политикой выбора
- verification_steps: Создание/остановка DevEnv, аудит сессий.

### E13: Регистр моделей и продвижение

#### T1301 — Сущности Model/ModelVersion + переходы статусов
- implementation_paths: `closed/internal/domain/model.go`, `closed/internal/repo/postgres/models.go`, новые API
- openapi: новый OpenAPI (models.yaml) или расширение experiments.yaml
- migrations: models/model_versions
- audit_events: model.create/promotion events
- security_controls: RBAC на promote/export
- observability: метрики по промоуту
- tests: юнит/интеграционные model registry
- acceptance_criteria: выполнено и подтверждено: Сущности Model/ModelVersion + переходы статусов
- verification_steps: CRUD модели и промоут с аудитом.

#### T1302 — Workflow валидации/аппрува с enforce по ролям
- implementation_paths: `closed/internal/domain/model.go`, `closed/internal/repo/postgres/models.go`, новые API
- openapi: новый OpenAPI (models.yaml) или расширение experiments.yaml
- migrations: models/model_versions
- audit_events: model.create/promotion events
- security_controls: RBAC на promote/export
- observability: метрики по промоуту
- tests: юнит/интеграционные model registry
- acceptance_criteria: выполнено и подтверждено: Workflow валидации/аппрува с enforce по ролям
- verification_steps: CRUD модели и промоут с аудитом.

#### T1303 — Интерфейс экспорта модели + политические ограничения + аудит/события
- implementation_paths: `closed/internal/domain/model.go`, `closed/internal/repo/postgres/models.go`, новые API
- openapi: новый OpenAPI (models.yaml) или расширение experiments.yaml
- migrations: models/model_versions
- audit_events: model.create/promotion events
- security_controls: RBAC на promote/export
- observability: метрики по промоуту
- tests: юнит/интеграционные model registry
- acceptance_criteria: выполнено и подтверждено: Интерфейс экспорта модели + политические ограничения + аудит/события
- verification_steps: CRUD модели и промоут с аудитом.

### E14: Наблюдаемость (метрики/логи/трейсы)

#### T1401 — Инструментирование CP: Prometheus‑метрики и OTel‑трейсы
- implementation_paths: `closed/internal/platform/httpserver/metrics.go`, `closed/*/main.go`, `closed/deploy/helm/animus-datapilot`, `docs/ops/observability.md`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: редактирование секретов в логах
- observability: Prometheus `/metrics` + базовые OTel‑переменные окружения
- tests: smoke‑проверка `/metrics`
- acceptance_criteria: выполнено и подтверждено: Инструментирование CP: Prometheus‑метрики и OTel‑трейсы
- verification_steps: Проверка `/metrics` для ключевых сервисов CP и наличия аннотаций скрейпа в Helm.

#### T1402 — Инструментирование DP: метрики/логи/трейсы (run‑scoped)
- implementation_paths: `closed/dataplane/main.go`, `closed/internal/platform/httpserver/metrics.go`, `closed/deploy/helm/animus-dataplane`, `docs/ops/observability.md`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: редактирование секретов в логах
- observability: Prometheus `/metrics` + базовые OTel‑переменные окружения
- tests: smoke‑проверка `/metrics`
- acceptance_criteria: выполнено и подтверждено: Инструментирование DP: метрики/логи/трейсы (run‑scoped)
- verification_steps: Проверка `/metrics` у DP и доступности порта сервиса.

#### T1403 — Стандартизация структурного логирования + correlation IDs
- implementation_paths: `closed/*` (instrumentation), `tools/observability/` (новое)
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: редактирование секретов в логах
- observability: Prometheus + OTel
- tests: observability smoke
- acceptance_criteria: выполнено и подтверждено: Стандартизация структурного логирования + correlation IDs
- verification_steps: Проверка метрик/трейсов для критичных путей.

### E15: Надёжность, HA, Backup & DR

#### T1501 — HA‑рекомендации и реализация для CP (stateless replicas + external DB)
- implementation_paths: `closed/deploy/`, `docs/enterprise/09-operations-and-reliability.md`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: backup/restore policies
- observability: SLO/SLA
- tests: DR drills
- acceptance_criteria: выполнено и подтверждено: HA‑рекомендации и реализация для CP (stateless replicas + external DB)
- verification_steps: Backup/restore на стенде и фиксация RPO/RTO.

#### T1502 — Инструменты backup/restore + DR game‑day
- implementation_paths: `docs/ops/backup-restore.md`, `docs/ops/dr-game-day.md`, `closed/deploy/helm/`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: backup/restore policies
- observability: SLO/SLA
- tests: DR drills
- acceptance_criteria: выполнено и подтверждено: Инструменты backup/restore + DR game‑day
- verification_steps: Backup/restore на стенде и фиксация RPO/RTO.

#### T1503 — Валидация операционных runbooks (game‑days) + отслеживание разрывов
- implementation_paths: `docs/ops/dr-game-day.md`, `docs/ops/backup-restore.md`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: backup/restore policies
- observability: SLO/SLA
- tests: DR drills
- acceptance_criteria: выполнено и подтверждено: Валидация операционных runbooks (game‑days) + отслеживание разрывов
- verification_steps: Backup/restore на стенде и фиксация RPO/RTO.

### E16: Air‑gapped доставка + SBOM + supply chain

#### T1601 — Генерация SBOM для всех образов и публикация с релизами
- implementation_paths: `scripts/sbom.sh`, `scripts/vuln_scan.sh`, `scripts/supply_chain.sh`, `scripts/list_images.sh`, `cmd/helm-images`, `.github/workflows/ci.yml`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: SBOM, подписи, supply‑chain gates
- observability: не применимо
- tests: SBOM/vuln scans
- acceptance_criteria: выполнено и подтверждено: Генерация SBOM для всех образов и публикация с релизами
- verification_steps: Генерация SBOM и проверка подписи в CI.

#### T1602 — Air‑gapped bundle (образы/чарты/зависимости) + процедура верификации
- implementation_paths: `closed/scripts/airgap-bundle.sh`, `scripts/list_images.sh`, `cmd/helm-images`, `docs/ops/airgapped-install.md`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: SBOM, подписи, supply‑chain gates
- observability: не применимо
- tests: SBOM/vuln scans
- acceptance_criteria: выполнено и подтверждено: Air‑gapped bundle (образы/чарты/зависимости) + процедура верификации
- verification_steps: Генерация SBOM и проверка подписи в CI.

#### T1603 — Проверка подписи образов (опционально, но рекомендуется)
- implementation_paths: `tools/ci/`, `closed/deploy/`, `docs/enterprise/10-versioning-and-compatibility.md`
- openapi: не применимо
- migrations: не применимо
- audit_events: не применимо
- security_controls: SBOM, подписи, supply‑chain gates
- observability: не применимо
- tests: SBOM/vuln scans
- acceptance_criteria: выполнено и подтверждено: Проверка подписи образов (опционально, но рекомендуется)
- verification_steps: Генерация SBOM и проверка подписи в CI.

### E17: E2E QA, приёмка, релиз

#### T1701 — Интеграционные тесты: git→run→artifacts→model promotion
- implementation_paths: `closed/e2e/`, `scripts/e2e.sh`, `.github/workflows/ci.yml`
- openapi: все сервисы
- migrations: не применимо
- audit_events: EV001–EV008 coverage
- security_controls: RBAC/secret/audit suites
- observability: не применимо
- tests: e2e интеграционные
- acceptance_criteria: выполнено и подтверждено: Интеграционные тесты: git→run→artifacts→model promotion
- verification_steps: git→run→artifacts→promotion сценарий.

#### T1702 — Тесты безопасности: authz‑регрессия, redaction секретов, полнота аудита
- implementation_paths: `closed/tests/` (новое), `open/demo/`, `tools/`
- openapi: все сервисы
- migrations: не применимо
- audit_events: EV001–EV008 coverage
- security_controls: RBAC/secret/audit suites
- observability: не применимо
- tests: e2e интеграционные
- acceptance_criteria: выполнено и подтверждено: Тесты безопасности: authz‑регрессия, redaction секретов, полнота аудита
- verification_steps: git→run→artifacts→promotion сценарий.

#### T1703 — Release‑чеклист + go/no‑go‑гейты
- implementation_paths: `.github/workflows/ci.yml`, `scripts/e2e.sh`, `scripts/supply_chain.sh`, `scripts/openapi_lint.sh`
- openapi: все сервисы
- migrations: не применимо
- audit_events: EV001–EV008 coverage
- security_controls: RBAC/secret/audit suites
- observability: не применимо
- tests: e2e интеграционные
- acceptance_criteria: выполнено и подтверждено: Release‑чеклист + go/no‑go‑гейты
- verification_steps: git→run→artifacts→promotion сценарий.

### E18: Network policy, egress control, zero‑trust

#### T1801 — Default‑deny egress для нагрузок DP; allowlist‑модель (по проекту/шаблону)
- implementation_paths: `closed/internal/platform/k8s/`, `closed/internal/runtimeexec/`
- openapi: DP protocol (policy snapshots)
- migrations: network policy tables (если требуются)
- audit_events: policy/network change events
- security_controls: default‑deny egress
- observability: network policy metrics
- tests: security regression (egress)
- acceptance_criteria: выполнено и подтверждено: Default‑deny egress для нагрузок DP; allowlist‑модель (по проекту/шаблону)
- verification_steps: Запрет egress без allowlist.

#### T1802 — Стратегия изоляции namespace (по проекту) + усиление K8s RBAC
- implementation_paths: `closed/internal/platform/k8s/`, `closed/internal/runtimeexec/`
- openapi: DP protocol (policy snapshots)
- migrations: network policy tables (если требуются)
- audit_events: policy/network change events
- security_controls: default‑deny egress
- observability: network policy metrics
- tests: security regression (egress)
- acceptance_criteria: выполнено и подтверждено: Стратегия изоляции namespace (по проекту) + усиление K8s RBAC
- verification_steps: Запрет egress без allowlist.


## Верификация по вехам

### M0: проверка
- команды: `make lint`, `make test`, `make build`

### M1: проверка
- команды: `make lint`, `make test`, `make build`

### M2: проверка
- команды: `make lint`, `make test`, `make build`
- API‑потоки: создание Run → план → (dry‑run) → экспорт входов Run
- reproducibility bundle: `GET /projects/{project_id}/runs/{run_id}/reproducibility-bundle` и сверка `spec_hash`
- аудит: проверка событий `run.created`, `run.validated`, `execution.planned`
- CP не исполняет код: `make lint` (depguard) и отсутствие импортов runtimeexec в CP

### M3: проверка
- команды: `scripts/go_test.sh ./closed/...`
- API‑потоки: создание Run → `POST /projects/{project_id}/runs/{run_id}:dispatch` → DP создаёт Job → heartbeat → terminal → проверка статуса Run
- идемпотентность: повторная отправка heartbeat/terminal с тем же `event_id` не изменяет итоговое состояние
- реконсиляция: имитация устаревшего heartbeat → запуск reconciler → аудит `run.reconciled`

### M4: проверка
- команды: `scripts/go_test.sh ./closed/...`
- API‑потоки: создание Run → очередь → план → scheduler → dispatch → DP heartbeat → terminal
- квоты: установить лимит `max_concurrent_runs=0` для проекта → ожидать `QUOTA_EXCEEDED` в решениях очереди
- ретраи: завершить Run с `failed` → проверить создание retry‑run и backoff
- отмена: `POST /projects/{project_id}/runs/{run_id}:cancel` → DP cancel → terminal `canceled`
- аудит: `run.queued`, `run.dequeued`, `run.dispatch_attempted`, `run.dispatch_blocked`, `run.retry_scheduled`, `run.canceled`

### M5: проверка
- команды: `scripts/go_test.sh ./closed/...`
- аутентификация: логин OIDC/SAML (или dev‑mode), проверка TTL сессии и `POST /auth/force-logout`
- RBAC: создать role‑binding → убедиться, что viewer не может write, editor может; аудит `access.denied`
- secrets: DP получает секреты, CP фиксирует `secret.accessed`, значения не появляются в ответах/аудите
- retention/legal hold: истечение срока → `410 Gone`, legal hold → `deletion.blocked_by_legal_hold`
- аудит‑экспорт: включить webhook/syslog, проверить ретраи и `audit.export.*`

### M6: проверка
- команды: `scripts/go_test.sh ./closed/...`, `make openapi-lint`
- кэш: `scripts/go_env.sh`/CI размещают `GOCACHE/GOMODCACHE` в безопасной директории (repo `.cache` или `${TMPDIR}`), без ручных экспортов
- API‑потоки: PipelineSpec → PipelineRun → graph → node‑runs → terminal → pipeline завершён
- ретраи: узел `failed` → retry‑run + backoff → продолжение DAG
- отмена: `POST /projects/{project_id}/pipeline-runs/{pipeline_run_id}:cancel` → отмена узлов и pipeline‑статуса
- аудит: `pipeline.run_created`, `pipeline.planned`, `pipeline.node_materialized`, `pipeline.node_dispatched`, `pipeline.node_completed`, `pipeline.completed`

### M7: проверка
- команды: `make lint`, `make test`, `make build`

### M8: проверка
- команды: `scripts/go_test.sh ./closed/...`, `make openapi-lint`
- офлайн‑линт контрактов: `make openapi-lint` использует `-mod=vendor`, не требует сети
- API‑потоки: Model → ModelVersion → validate → approve → export → deprecate (с аудитом)
- provenance: `GET /projects/{project_id}/model-versions/{model_version_id}/provenance` возвращает RunSpec и привязки

### M9: проверка
- команды: `make lint`, `make test`, `make openapi-lint`, `make supply-chain`, `make e2e`
- E2E: `make e2e` (docker или внешние Postgres/MinIO через `ANIMUS_E2E_*`)
- наблюдаемость: `/metrics` у CP/DP, аннотации скрейпа Prometheus, OTel‑переменные включаются через values
- air‑gapped: `closed/scripts/airgap-bundle.sh --output ./bundle --values values-control-plane.yaml --values values-data-plane.yaml`
- HA/DR: пройти `docs/ops/backup-restore.md` и чек‑лист `docs/ops/dr-game-day.md`, зафиксировать RPO/RTO
- упаковка: `helm upgrade --install ...`, `helm test animus-cp`, `helm test animus-dp`

### V1.1 (P5): проверка
- команды: `make integrations-test`, `make openapi-lint`, `./scripts/go_test.sh ./closed/...`
- SCM: прогон unit‑тестов с mock‑провайдером; проверка allowlist и блокировки диспетча.
- вебхуки: симуляция получателя (HTTP 500/429 → ретраи, HTTP 200 → delivered), проверка идемпотентности по `event_id`.
- observability: наличие метрик `animus_webhook_*` на `/metrics` и аудит `webhook.delivery.*`.
