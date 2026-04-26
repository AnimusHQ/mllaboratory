# CLI и практическая работа

Документ даёт минимальный, проверяемый маршрут работы через `curl` и описывает поведение SDK в контейнере исполнения, что снижает риск ошибок интеграции.

## Предусловия
- Доступен Gateway.
- Есть активная сессия или токен аутентификации.
- Создание сущностей выполняется с `Idempotency-Key`, что снижает риск дублей при сбоях.

## Минимальный маршрут (Dataset → Run → Evidence)

```bash
export GATEWAY_URL=http://localhost:8080
export AUTH_HEADER='-H Authorization: Bearer <token>'
```

### 1) Создать Dataset
```bash
curl -sS -X POST "${GATEWAY_URL}/api/dataset-registry/datasets" \
  ${AUTH_HEADER} \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: ds_create_0001' \
  -d '{"name":"fraud-dataset","description":"Fraud training set","metadata":{"owner":"ml-team"}}'
```

### 2) Создать DatasetVersion и загрузить данные
```bash
curl -sS -X POST "${GATEWAY_URL}/api/dataset-registry/datasets/<dataset_id>/versions/upload" \
  ${AUTH_HEADER} \
  -H 'Idempotency-Key: dv_upload_0001' \
  -F 'file=@open/demo/data/demo.csv' \
  -F 'metadata={"source":"demo"}'
```

### 3) Создать Run / PipelineRun
Создание Run требует явного контекста (`DatasetVersion`, `CodeRef`, `EnvironmentLock`, `PolicySnapshot`), что обеспечивает воспроизводимость и исключает скрытые зависимости. Конкретные поля и эндпоинты фиксируются в OpenAPI.

### 4) Получить Evidence
Evidence‑артефакт извлекается через Gateway и подтверждает provenance, аудит и параметры исполнения, что снижает риск утраты доказательности.

## Использование SDK в контейнере исполнения
SDK использует run‑scoped переменные окружения, что ограничивает доступ рамками конкретного Run и снижает риск повторного использования токена.

```python
import os
from animus_sdk import RunTelemetryLogger, DatasetRegistryClient, ExperimentsClient

logger = RunTelemetryLogger.from_env(timeout_seconds=2.0)
logger.log_status(status="starting")
logger.log_metric(step=1, name="loss", value=0.9)
logger.close(flush=True, timeout_seconds=5.0)

datasets = DatasetRegistryClient.from_env()
datasets.download_dataset_version(dataset_version_id=os.environ["DATASET_VERSION_ID"], dest_path="/tmp/data.zip")

exp = ExperimentsClient.from_env()
exp.upload_run_artifact(kind="model", file_path="/tmp/model.bin")
```

## Где смотреть точные контракты
- `open/api/openapi/gateway.yaml`
- `open/api/openapi/experiments.yaml`

## Связанные документы
- `docs/open/05-api.md`
- `docs/open/07-evidence-format.md`
- `docs/ops/start-here.md`
