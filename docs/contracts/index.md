# Индекс контрактов (Control Plane / Data Plane)

**Версия документа:** 1.0

## 1. Контракты Control Plane (CP)
### 1.1 OpenAPI (источник истины)
- `open/api/openapi/gateway.yaml` — шлюз (auth, proxy в сервисы).
- `open/api/openapi/dataset-registry.yaml` — Dataset и DatasetVersion.
- `open/api/openapi/quality.yaml` — quality‑правила и оценки.
- `open/api/openapi/experiments.yaml` — эксперименты, Run/Execution, PolicySnapshot, EnvironmentDefinition/Lock, политика, артефакты, evidence.
- `open/api/openapi/lineage.yaml` — lineage‑события.
- `open/api/openapi/audit.yaml` — аудит и экспорт.

### 1.2 Набор CP‑ресурсов (минимум)
- Project, Dataset, DatasetVersion, Artifact
- Experiment, Run, PipelineRun
- CodeRef, EnvironmentDefinition, EnvironmentLock
- PolicySnapshot, AuditEvent, EvidenceBundle

### 1.3 Семантика версии API
- Версионирование по SemVer с явным `/v1` при необходимости.
- Изменения контрактов фиксируются до изменения реализации (contract‑first).

## 2. Контракт CP↔DP (Data Plane протокол)
### 2.1 Текущий статус
- Транспорт: решение фиксируется в M3‑Design (T0104). Требование: HTTP/gRPC, идемпотентность сообщений, повторяемость.

### 2.2 Обязательные сообщения протокола (roadmap.json)
- `RunExecutionRequest`
- `RunHeartbeat`
- `RunTerminalState`
- `ArtifactCommitted`
- `LogCursorUpdate` (опционально)
- `DevEnvSessionHeartbeat` (если DevEnv включён)

### 2.3 Идемпотентность
- Все сообщения DP→CP должны быть безопасно повторяемыми.
- CP обязан принимать дубликаты без нарушения консистентности.

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
