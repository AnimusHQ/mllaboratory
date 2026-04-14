# Архитектура

Документ фиксирует структуру системы, интерфейсы и причинно‑следственные связи, что снижает риск расхождения между описанием и фактическим поведением.

## Высокоуровневая схема

```mermaid
flowchart LR
  User[Пользователь / CI] --> Console[Консоль]
  User --> Gateway[Gateway]
  Console --> Gateway
  Gateway --> DatasetRegistry[Dataset Registry]
  Gateway --> Experiments[Experiments]
  Gateway --> Quality[Quality]
  Gateway --> Lineage[Lineage]
  Gateway --> Audit[Audit]
  DatasetRegistry --> Postgres[(Postgres)]
  Quality --> Postgres
  Experiments --> Postgres
  Lineage --> Postgres
  Audit --> Postgres
  DatasetRegistry --> ObjectStore[(S3/MinIO)]
  Experiments --> ObjectStore
  Quality --> ObjectStore
  Gateway --> DataPlane[Data Plane]
  DataPlane --> Gateway
```

## Компоненты и назначение

| Компонент | Действие | Практический эффект | Риск, который снижается |
| --- | --- | --- | --- |
| Gateway | принимает внешний трафик, выполняет аутентификацию и RBAC, проксирует запросы | единая точка входа и контроль доступа | обход политик и «теневые» интерфейсы |
| Control Plane (CP) | хранит метаданные, политики, аудит и планирование | воспроизводимость опирается на явный контекст | невоспроизводимые результаты из‑за скрытых зависимостей |
| Data Plane (DP) | исполняет пользовательский код и получает значения секретов | изоляция исполнения и контроль среды | запуск кода в доверенной плоскости |
| Postgres | хранит метаданные, аудит, lineage, очереди | сохранение доказательной базы | потеря данных и несоответствие аудиту |
| S3/MinIO | хранит датасеты и артефакты | воспроизводимость артефактов | утрата результатов и проверяемости |

## Ключевые системные связи
- **Контекст Run фиксируется как набор ссылок** (`DatasetVersion`, `CodeRef`, `EnvironmentLock`, `PolicySnapshot`), что обеспечивает проверяемость результата и исключает скрытое состояние.
- **Gateway изолирует внешний доступ**, что снижает риск обхода RBAC и прямого доступа к внутренним сервисам.
- **DP — единственная плоскость с доступом к секретам**, что снижает риск утечки в контрольной плоскости.
- **AuditEvent является append‑only**, что сохраняет доказательность и снижает риск подмены действий.

## Поток данных (обобщённо)

```mermaid
sequenceDiagram
  participant User
  participant Gateway
  participant DatasetRegistry
  participant Experiments
  participant Quality
  participant Lineage
  participant Audit
  participant DataPlane
  participant Postgres
  participant ObjectStore

  User->>Gateway: Создать Dataset/Version
  Gateway->>DatasetRegistry: Запись метаданных
  DatasetRegistry->>Postgres: Сохранение версии
  DatasetRegistry->>ObjectStore: Загрузка данных
  DatasetRegistry->>Audit: AuditEvent

  User->>Gateway: Создать Run / PipelineRun
  Gateway->>Experiments: Валидация входов
  Experiments->>Postgres: Сохранение Run
  Experiments->>Lineage: Связи входов/выходов
  Experiments->>Audit: AuditEvent

  Gateway->>DataPlane: Запрос исполнения
  DataPlane->>Gateway: Статусы и артефакты
  Gateway->>Experiments: Сохранение результатов

  Experiments->>Audit: AuditEvent
```

## Где искать детали
- Контракты API: `open/api/openapi/`
- Эксплуатация: `docs/ops/`
- Безопасность: `docs/open/02-security-and-compliance.md`
- Evidence‑формат: `docs/open/07-evidence-format.md`
