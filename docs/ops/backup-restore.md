# Резервное копирование и восстановление

Документ описывает воспроизводимые процедуры backup/restore для Postgres и S3/MinIO, что снижает риск необратимой потери данных.

## 1. Цели (RPO/RTO)

- RPO: 15–60 минут (определяется частотой бэкапов).
- RTO: 1–4 часа (зависит от объёма данных и скорости восстановления).
- Формулы:
  - `RPO = T_incident - T_backup_last_ok`.
  - `RTO = T_restore_done - T_incident`.

## 2. Предпосылки

- Доступ к Postgres и объектному хранилищу.
- Доступ к Gateway для post‑restore проверок.
- Утилиты: `pg_dump`, `pg_restore`, `curl`, `python3`, `mc` или `aws`.
- Секреты передаются только через переменные окружения.

## 3. Переменные окружения

- `BACKUP_DIR` — каталог бэкапа (обязателен).
- `DATABASE_URL` — строка подключения Postgres (обязателен).
- `ANIMUS_MINIO_ENDPOINT` — `host:port` или URL S3‑совместимого хранилища.
- `ANIMUS_MINIO_ACCESS_KEY`, `ANIMUS_MINIO_SECRET_KEY` — ключи доступа.
- `ANIMUS_MINIO_BUCKETS` — список bucket‑ов через пробел.
- Для проверки восстановления:
  - `ANIMUS_GATEWAY_URL`.
  - `ANIMUS_DR_TOKEN`.
  - `ANIMUS_DR_PROJECT_ID`.

## 4. Построение бэкапа

**Команды:**
```bash
BACKUP_DIR=/secure/backups/$(date -u +%Y%m%dT%H%M%SZ) \
DATABASE_URL="postgres://..." \
ANIMUS_MINIO_ENDPOINT="s3.example" \
ANIMUS_MINIO_ACCESS_KEY="..." \
ANIMUS_MINIO_SECRET_KEY="..." \
ANIMUS_MINIO_BUCKETS="datasets artifacts" \
closed/scripts/backup.sh
```

**Ожидаемый результат:**
- `postgres/animus.dump`.
- `minio/<bucket>/`.
- `manifest.env` с контрольными суммами.

## 5. Восстановление

**Команды:**
```bash
BACKUP_DIR=/secure/backups/<timestamp> \
DATABASE_URL="postgres://..." \
ANIMUS_MINIO_ENDPOINT="s3.example" \
ANIMUS_MINIO_ACCESS_KEY="..." \
ANIMUS_MINIO_SECRET_KEY="..." \
ANIMUS_MINIO_BUCKETS="datasets artifacts" \
closed/scripts/restore.sh
```

**Ожидаемый результат:**
- Успешный `pg_restore`.
- Восстановленные объекты в S3/MinIO.

## 6. Проверка восстановления

**Команды:**
```bash
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
closed/scripts/verify-restore.sh
```

**Ожидаемый результат:**
- `healthz/readyz` возвращают `200`.
- Базовый CRUD и upload/download выполняются без ошибок.

## 7. Автоматизированная проверка (dr-validate)

**Команды:**
```bash
ANIMUS_DR_VALIDATE=1 \
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
make dr-validate
```

**Ожидаемый результат:**
- Создан отчёт в `docs/ops/reports/` или во временном каталоге.

## 8. Частичное восстановление (симуляция деградации)

**Сценарии:**
- Восстановить только Postgres.
- Восстановить только S3/MinIO.

**Ожидаемое поведение:**
- Сервисы доступны.
- Запросы к отсутствующим объектам возвращают `404/410` без паники.

## 9. Восстановление из устаревшего бэкапа

**Действия:**
- Восстановить бэкап на 1–2 релиза назад.
- Применить миграции штатной процедурой.
- Проверить совместимость `make openapi-lint` и `verify-restore.sh`.

## 10. Откат и восстановление

**Откат:**
```bash
helm -n animus-system rollback animus-datapilot
helm -n animus-system rollback animus-dataplane
```

**Восстановление:**
- Повторить restore для последнего корректного бэкапа.
- Зафиксировать причину и обновить runbook.

## 11. Диагностика при сбое

```bash
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

## 12. Связанные документы

- `docs/ops/dr-game-day.md`
- `docs/ops/reports/README.md`
