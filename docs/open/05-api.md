# API‑контракты

Документ фиксирует общие правила API. Полные определения и схемы находятся в `open/api/openapi/` и являются источником истины.

## Базовые правила
- Все внешние запросы проходят через Gateway, что предотвращает прямой доступ к внутренним сервисам и снижает риск обхода RBAC.
- API описаны в OpenAPI и считаются стабильными для интеграции, что снижает риск расхождения документации и фактического поведения.

## Аутентификация
- Для пользовательских запросов используется сессия или `Authorization: Bearer <token>`, что обеспечивает контроль доступа по RBAC и снижает риск несанкционированных действий.
- Для исполнения в Data Plane используются run‑scoped токены, что ограничивает доступ контекстом Run и снижает риск повторного использования.

## Идемпотентность
- Операции создания принимают `Idempotency-Key`; повтор с тем же ключом возвращает тот же результат, что снижает риск дублей при сетевых сбоях.
- Полный список эндпоинтов с идемпотентностью фиксируется в OpenAPI.

## Ошибки
Единый формат ошибок обеспечивает однозначную диагностику и снижает риск неверной интерпретации:

```json
{
  "error": "validation_failed",
  "request_id": "req_01J1X9K7B3ZJ4A1XH6Y1C9QZ8Q"
}
```

## Разделы API (логическая группировка)
- **Datasets**: регистрация Dataset и DatasetVersion, загрузка и скачивание.
- **Runs / Pipelines**: создание Run/PipelineRun, статусы, артефакты, метрики.
- **Environments**: определения и EnvironmentLock.
- **DevEnvs**: создание среды разработки и IDE‑сессии.
- **Model Registry**: модели, версии, статусы, экспорт.
- **Lineage**: подграфы происхождения для Run и моделей.
- **Audit / SIEM**: аудит, экспорт, DLQ и replay.

Полные контракты: `open/api/openapi/gateway.yaml` и `open/api/openapi/experiments.yaml`.

## Пример запроса (создание Dataset)
```bash
curl -sS -X POST http://localhost:8080/api/dataset-registry/datasets \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: ds_create_0001' \
  -d '{"name":"fraud-dataset","description":"Fraud training set","metadata":{"owner":"ml-team"}}'
```

## Связанные документы
- `docs/open/06-cli-and-usage.md`
- `docs/open/07-evidence-format.md`
- `docs/open/08-troubleshooting.md`
