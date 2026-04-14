# DR Game‑Day (процедура)

Документ описывает регламент тренировочного восстановления и фиксации RPO/RTO, что снижает риск формального (непроверенного) DR.

## 1. Цель

- Проверить восстановление в пределах целевых RPO/RTO.
- Подтвердить целостность данных и доступность сервисов.

## 2. Предпосылки

- Актуальный бэкап Postgres и S3/MinIO.
- Зафиксированные версии чартов и digest‑пинning образов.
- Доступ к кластеру (kubectl/helm).
- Токен и проект для post‑restore проверок.

## 3. Сценарий

**Шаг 1. Фиксация инцидента**
- Зафиксировать `T_incident`.

**Шаг 2. Остановка CP/DP**
```bash
kubectl -n animus-system scale deploy/animus-datapilot-experiments --replicas=0
kubectl -n animus-system scale deploy/animus-dataplane --replicas=0
```

**Шаг 3. Восстановление**
```bash
BACKUP_DIR=/secure/backups/<timestamp> \
DATABASE_URL="postgres://..." \
ANIMUS_MINIO_ENDPOINT="s3.example" \
ANIMUS_MINIO_ACCESS_KEY="..." \
ANIMUS_MINIO_SECRET_KEY="..." \
ANIMUS_MINIO_BUCKETS="datasets artifacts" \
closed/scripts/restore.sh
```

**Шаг 4. Запуск CP/DP**
```bash
kubectl -n animus-system scale deploy/animus-datapilot-experiments --replicas=1
kubectl -n animus-system scale deploy/animus-dataplane --replicas=1
```

**Шаг 5. Проверка готовности**
```bash
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
closed/scripts/verify-restore.sh
```

**Шаг 6. Автоматизированная проверка (опционально)**
```bash
ANIMUS_DR_VALIDATE=1 \
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
make dr-validate
```

## 4. Ожидаемые результаты

- `T_restore_done` зафиксировано.
- `RPO` и `RTO` рассчитаны по формулам из `docs/ops/backup-restore.md`.
- `healthz/readyz` возвращают `200`.
- Базовый CRUD и upload/download выполняются без ошибок.

## 5. Откат

```bash
helm -n animus-system rollback animus-datapilot
helm -n animus-system rollback animus-dataplane
```

## 6. Диагностика при сбое

```bash
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

## 7. Отчётность

- Использовать `docs/ops/dr-gameday-report.md` как шаблон.
- Формат отчётов описан в `docs/ops/reports/README.md`.
