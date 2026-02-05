# Индекс контрактов (Control Plane / Data Plane)

**Версия документа:** 1.0

## 1. Контракты Control Plane (CP)
### 1.1 OpenAPI (источник истины)
- `open/api/openapi/gateway.yaml` — шлюз (auth, proxy в сервисы).
- Gateway OpenAPI включает внешнюю поверхность Model Registry (M8).
- `open/api/openapi/dataset-registry.yaml` — Dataset и DatasetVersion.
- `open/api/openapi/quality.yaml` — quality‑правила и оценки.
- `open/api/openapi/experiments.yaml` — эксперименты, Run/Execution, PolicySnapshot, EnvironmentDefinition/Lock, политика, артефакты, evidence.
- `open/api/openapi/dataplane_internal.yaml` — внутренний протокол CP↔DP (исполнение, heartbeat, terminal, статус).
- `open/api/openapi/lineage.yaml` — lineage‑события.
- `open/api/openapi/audit.yaml` — аудит и экспорт.
- Контроль контрактов: `make openapi-lint` (пинованный валидатор kin-openapi, режим `-mod=vendor`) проверяет спецификации CP и DP и исполним в офлайн‑среде.

### 1.2 Набор CP‑ресурсов (минимум)
- Project, Dataset, DatasetVersion, Artifact
- Experiment, Run, Model, ModelVersion, ModelExport, PipelineSpec, PipelinePlan, PipelineRun, PipelineNode
- CodeRef, EnvironmentDefinition, EnvironmentLock
- PolicySnapshot, AuditEvent, EvidenceBundle

### 1.3 Семантика версии API
- Версионирование по SemVer с явным `/v1` при необходимости.
- Изменения контрактов фиксируются до изменения реализации (contract‑first).

### 1.4 Планировщик, очереди, квоты, ретраи и отмена (M4)
- Очереди и состояние планирования персистентны: `run_queues`, `run_queue_entries`.
- Переходы планировщика: `CREATED → QUEUED → DISPATCHABLE → DISPATCHED`.
- Приоритеты числовые; детерминированный порядок: `priority DESC`, затем `enqueued_at ASC`, затем `run_id ASC`.
- Квоты проектов — конкурентные лимиты (concurrency). Блокировки отражаются причинами `QUOTA_EXCEEDED` и `QUEUE_BLOCKED`.
- Retry‑политика хранится в `RunSpec` (`retryPolicy`), ретраи создают новые Run с ссылкой на оригинал (`run_retries`), backoff детерминирован.
- Отмена Run: CP API `POST /projects/{project_id}/runs/{run_id}:cancel`, DP API `POST /internal/dp/runs/{run_id}:cancel`.
- Терминальные состояния неизменяемы; повторная отмена идемпотентна.

### 1.5 Безопасность и governance (M5)
- Аутентификация: OIDC (primary) и SAML (опция) в шлюзе; сессии с TTL, лимитами параллельных сессий и принудительным logout.
- RBAC: project‑scoped роли, биндинги ролей проекта (`/projects/{project_id}/role-bindings`) и deny‑by‑default.
- Secrets: значения секретов выдаются только в DP на время исполнения; CP получает только метаданные доступа (`SecretAccessed`).
- Audit: append‑only, экспорт в webhook/syslog через outbox; события `audit.export.attempted`, `audit.export.delivered`.
- Retention/legal hold: политики хранятся в `retention_policies`, доступ к истёкшим ресурсам блокируется (HTTP 410), удаление при legal hold запрещено и аудируется.

### 1.6 Пайплайны (DAG) и детерминизм (M6)
- PipelineSpec v1 хранится иммутабельно и валидируется до материализации.
- PipelineBindings фиксируют dataset‑привязки, CodeRef, EnvLock, параметры и PolicySnapshot.
- PipelinePlan формируется детерминированно: топологическая сортировка с тай‑брейком `node_name ASC`, стабильный `plan_hash` из канонической сериализации.
- Оркестрация не использует скрытого состояния: прогресс вычисляется из `pipeline_runs`, `pipeline_plans`, `pipeline_nodes`, `runs`, `run_retries`, `run_queue_entries`.
- Очередь и квоты M4 применяются к узлам DAG; лимит `max_parallelism` ограничивает число активных node‑runs.

### 1.7 Регистр моделей и продвижение (M8)
- Model/ModelVersion иммутабельны, статусы версий: `draft → validated → approved → deprecated`.
- Provenance версии фиксирует `run_id`, `artifact_ids`, `dataset_version_ids`, `code_ref`, `env_lock_id`, `policy_snapshot_sha256`.
- Аппрув/депрекейт требуют админ‑ролей; валидация доступна editor.
- Экспорт версии разрешён только для `approved`; операции идемпотентны по `Idempotency-Key` и аудируются.
- Таблицы: `model_versions`, `model_version_transitions`, `model_exports` (idempotency по `(project_id, idempotency_key)`).
- API:
  - модели: `GET/POST /projects/{project_id}/models`, `GET /projects/{project_id}/models/{model_id}`;
  - версии: `GET/POST /projects/{project_id}/models/{model_id}/versions`, `GET /projects/{project_id}/model-versions/{model_version_id}`;
  - provenance: `GET /projects/{project_id}/model-versions/{model_version_id}/provenance`;
  - переходы: `POST /projects/{project_id}/model-versions/{model_version_id}:validate|approve|deprecate`;
  - экспорт: `POST /projects/{project_id}/model-versions/{model_version_id}:export`.

### 1.8 Целостность образов и подписи реестра (P5)
- Enforcement: создание EnvironmentLock (ADR‑0012), CP не исполняет пользовательский код.
- Политики: `allow_unsigned` | `verify_only` | `deny_unsigned`.
- Результаты проверки сохраняются в `image_verifications` (project‑scoped) с уникальностью `(project_id, image_digest_ref, policy_mode, provider)`.
- Проектные переопределения политики хранятся в `registry_policies` (без API управления в v1.1).
- API (read‑only): `GET /projects/{project_id}/registry/verifications`.
- Аудит: `image.verification.requested`, `image.verified`, `image.verification_failed`, `environment.lock.creation_blocked`.

### 1.9 Секреты и Vault‑подобный backend (P5)
- Значения секретов выдаются только в DP на время исполнения; CP хранит лишь `secret_access_class_ref`.
- DP отправляет CP событие `SecretAccessed` с метаданными доступа (без значений).
- Провайдеры: `noop`, `static`, `vault_k8s` (Kubernetes auth, краткоживущие токены).
- Запрос секретов выполняется по `secret_access_class_ref`; TTL обязателен, ошибки приводят к отказу исполнения.
- Redaction: значения секретов не попадают в логи, аудит и ответы API.

### 1.10 Исходящие вебхуки (P5)
- Подписки: `POST/GET /projects/{project_id}/webhooks/subscriptions`, обновление `PATCH/PUT /projects/{project_id}/webhooks/subscriptions/{subscription_id}`.
- Доставки: `GET /projects/{project_id}/webhooks/deliveries`, попытки `GET /projects/{project_id}/webhooks/deliveries/{delivery_id}/attempts`, replay `POST /projects/{project_id}/webhooks/deliveries/{delivery_id}:replay`.
- События: `RunFinished`, `ModelApproved`, `DatasetVersionCreated`; `event_id` детерминирован по `event_type + project_id + subject_id`.
- Идемпотентность: уникальность `(subscription_id, event_id)`; заголовок `Idempotency-Key = event_id:subscription_id`.
- Подпись: `X-Animus-Signature: sha256=<hex>` (HMAC от payload); секрет извлекается по `secret_ref`.
- Очередь персистентна: `webhook_deliveries` + `webhook_delivery_attempts`, ретраи детерминированы.
- Аудит: `webhook.subscription.created|updated|enabled|disabled`, `webhook.delivery.enqueued|attempted|delivered|failed`, `webhook.delivery.replay_requested`.

### 1.11 Экспорт аудита в SIEM (P5)
- Источник: append‑only `audit_events`, доставка событий в внешние SIEM‑системы через персистентную очередь.
- Коннекторы: syslog (`tcp`/`udp`) и webhook; payload NDJSON, без секретов.
- Идемпотентность: уникальность `(sink_id, event_id)`; `Idempotency-Key = sink_id:event_id`.
- Очередь: `audit_export_deliveries` + `audit_export_attempts`; статусы `pending|retry|delivered|dlq`.
- DLQ: статус `dlq` с `dlq_reason`, replay через `POST /admin/audit/exports/dlq/{delivery_id}:replay` (идемпотентность по `Idempotency-Key`/`replay_token`).
- Админ‑API (read‑only): `GET /admin/audit/exports/sinks`, `GET /admin/audit/exports/deliveries`, `GET /admin/audit/exports/deliveries/{delivery_id}/attempts`.
- Подпись webhook: `X-Animus-Signature: sha256=<hex>`, секрет извлекается по `webhook_secret_ref` (CP не хранит значения).
- Аудит: `audit.export.attempted`, `audit.export.delivered`, `audit.export.dlq`, `audit.export.replay_requested`.
- Метрики: `animus_audit_export_attempts_total`, `animus_audit_export_latency_seconds_*`, `animus_audit_export_dlq_size`.

## 2. Контракт CP↔DP (Data Plane протокол)
### 2.1 Текущий статус
- Транспорт: **HTTP + OpenAPI**, контракт зафиксирован в `open/api/openapi/dataplane_internal.yaml` (ADR‑0007).
- Аутентификация: внутренняя подпись заголовков (X‑Animus‑Auth‑*) до внедрения mTLS/OIDC (M5).

### 2.2 Обязательные сообщения протокола (roadmap.json)
- CP→DP: `RunExecutionRequest` (запуск Run), `RunExecutionStatus` (reconciliation).
- DP→CP: `RunHeartbeat`, `RunTerminalState`, `ArtifactCommitted` (M3 — заглушка контракта).
- DP→CP: `SecretAccessed` (метаданные доступа к секретам, без значений).
- `LogCursorUpdate` (опционально)
- `DevEnvSessionHeartbeat` (если DevEnv включён)
 - Реконсиляция: CP использует `RunExecutionStatus` для разрешения орфанных состояний; итог фиксируется аудитом `run.reconciled`.

### 2.3 Идемпотентность
- Все сообщения DP→CP должны быть безопасно повторяемыми по `eventId`.
- CP→DP запросы исполнения идемпотентны по `dispatchId`.
- CP принимает дубликаты без нарушения консистентности; терминальные состояния неизменяемы.

## 3. Контракты событий (Event Contracts)
### 3.1 Список EV
- EV001 RunCreated
- EV002 RunValidated
- EV003 ExecutionPlanned
- EV004 RunDispatched
- EV005 RunStateChanged
- EV006 ArtifactDownloaded
- EV007 RoleBindingChanged
- EV008 AuditExportDelivered
- EV009 EnvironmentDefined
- EV010 EnvironmentUpdated
- EV011 EnvironmentArchived
- EV012 EnvironmentLocked
- EV013 PolicySnapshotMaterialized
- EV014 RunReconciled
- EV015 PipelineRunCreated
- EV016 PipelinePlanned
- EV017 PipelineNodeMaterialized
- EV018 PipelineNodeDispatched
- EV019 PipelineNodeCompleted
- EV020 PipelineCompleted
- EV021 PipelineCanceled
- EV022 ModelCreated
- EV023 ModelVersionCreated
- EV024 ModelValidated
- EV025 ModelApproved
- EV026 ModelDeprecated
- EV027 ModelExportRequested
- EV028 ModelExportAllowed
- EV029 ModelExportDenied
- EV030 ModelExportCompleted
- EV031 ModelExportFailed

### 3.2 Версионирование событий
- События имеют стабильные идентификаторы EV###.
- Добавление полей допускается при сохранении обратной совместимости.

## 4. Схемы данных (Schemas)
### 4.1 Реестр схем (S###)
- S001 Project
- S002 Dataset/DatasetVersion
- S003 Artifact
- S004 Run/ExecutionPlan/StepExecution
- S005 AuditEvent
- S006 CodeRef
- S007 EnvironmentDefinition/EnvironmentLock
- S008 PolicySnapshot
- S009 Model/ModelVersion

### 4.2 Примечания по версионированию
- Схемы хранятся в БД как иммутабельные сущности.
- Изменения, влияющие на воспроизводимость, сопровождаются миграциями и обновлением контрактов.

## 5. Соответствие инвариантам
- CP не исполняет пользовательский код.
- Все входы Run фиксируются и экспортируемы.
- Аудит append‑only и экспортируемый.
- Секреты не попадают в логи, аудит и ответы API; доступ фиксируется метаданными.
