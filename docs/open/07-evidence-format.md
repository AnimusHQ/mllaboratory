# Evidence‑формат и проверка

Документ описывает структуру evidence‑артефактов и минимальную процедуру проверки целостности, что снижает риск использования недостоверных результатов.

## Что входит в evidence
Evidence‑пакет формируется для Run и включает:
- **ledger** с входами (`DatasetVersion`, `CodeRef`, `EnvironmentLock`, `PolicySnapshot`), что сохраняет воспроизводимость и снижает риск спорных интерпретаций;
- **lineage** для связей входов и выходов, что снижает риск утраты происхождения результатов;
- **audit‑срез**, что сохраняет доказательность действий;
- **manifest** с хэшами, что позволяет проверять неизменность артефактов.

Точные поля и названия объектов зафиксированы в OpenAPI.

## Минимальная проверка целостности
1. Получить запись evidence через Gateway.
2. Скачать пакет и проверить SHA256.
3. Сопоставить хэш с записью evidence.

```bash
curl -sS "http://localhost:8080/api/experiments/experiment-runs/${RUN_ID}/evidence-bundles/${BUNDLE_ID}"
curl -sS -o evidence.zip "http://localhost:8080/api/experiments/experiment-runs/${RUN_ID}/evidence-bundles/${BUNDLE_ID}/download"
sha256sum evidence.zip
```

## Где смотреть точные схемы
- `open/api/openapi/experiments.yaml` (ExecutionLedger, EvidenceBundle)

## Связанные документы
- `docs/open/05-api.md`
- `docs/open/06-cli-and-usage.md`
