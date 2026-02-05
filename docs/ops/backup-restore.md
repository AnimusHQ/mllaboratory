# Резервное копирование и восстановление

## 1. Цели (RPO/RTO)
- RPO: 15–60 минут (определяется частотой бэкапов).
- RTO: 1–4 часа (определяется объёмом данных и скоростью восстановления).
- Метод измерения:
  - RPO = `T_incident - T_backup_last_ok`.
  - RTO = `T_restore_done - T_incident`.

## 2. Предпосылки
- Доступ к Postgres и объектному хранилищу (MinIO/S3).
- Доступ к шлюзу (Gateway) для post‑restore проверок.
- Установлены утилиты:
  - `pg_dump`, `pg_restore`,
  - `curl`,
  - `python3`,
  - `mc` **или** `aws` (для S3‑совместимого хранилища).
- Секреты передаются только через переменные окружения, логирование секретов запрещено.

## 3. Переменные окружения
- `BACKUP_DIR` — каталог бэкапа (обязательно).
- `DATABASE_URL` — строка подключения Postgres (обязательно).
- `ANIMUS_MINIO_ENDPOINT` — `host:port` или URL S3‑совместимого хранилища (опционально).
- `ANIMUS_MINIO_ACCESS_KEY`, `ANIMUS_MINIO_SECRET_KEY` — ключи доступа (обязательны, если задан `ANIMUS_MINIO_ENDPOINT`).
- `ANIMUS_MINIO_BUCKETS` — список bucket‑ов через пробел (опционально, по умолчанию: `datasets artifacts`).
- Для проверки восстановления:
  - `ANIMUS_GATEWAY_URL` — базовый URL gateway (например, `https://gateway.example`).
  - `ANIMUS_DR_TOKEN` — bearer‑токен с правами на проект.
  - `ANIMUS_DR_PROJECT_ID` — идентификатор проекта.

## 4. Построение бэкапа
```bash
BACKUP_DIR=/secure/backups/$(date -u +%Y%m%dT%H%M%SZ) \
DATABASE_URL="postgres://..." \
ANIMUS_MINIO_ENDPOINT="s3.example" \
ANIMUS_MINIO_ACCESS_KEY="..." \
ANIMUS_MINIO_SECRET_KEY="..." \
ANIMUS_MINIO_BUCKETS="datasets artifacts" \
closed/scripts/backup.sh
```
Ожидаемый результат:
- `postgres/animus.dump` (дамп Postgres),
- `minio/<bucket>/` (копии объектов),
- `manifest.env` (метаданные и контрольные суммы).

## 5. Восстановление
```bash
BACKUP_DIR=/secure/backups/<timestamp> \
DATABASE_URL="postgres://..." \
ANIMUS_MINIO_ENDPOINT="s3.example" \
ANIMUS_MINIO_ACCESS_KEY="..." \
ANIMUS_MINIO_SECRET_KEY="..." \
ANIMUS_MINIO_BUCKETS="datasets artifacts" \
closed/scripts/restore.sh
```
Ожидаемый результат:
- успешное выполнение `pg_restore`,
- корректное восстановление bucket‑ов,
- отсутствие ошибок целостности в БД и логах сервисов.

## 6. Проверка восстановления
```bash
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
closed/scripts/verify-restore.sh
```
Проверки включают:
- `healthz/readyz` для gateway, experiments, dataset‑registry, audit,
- базовый CRUD для dataset‑registry,
- загрузку и скачивание dataset‑версии (проверка объектного хранилища).

## 7. Частичное восстановление (симуляция)
Используется для проверки деградации при неполном бэкапе.

- Вариант A: восстановить только Postgres, MinIO оставить пустым.
- Вариант B: восстановить только MinIO, Postgres оставить пустым.
- Ожидаемое поведение: сервисы поднимаются, запросы к отсутствующим объектам возвращают `404/410` без паники и без нарушения целостности.

## 8. Восстановление из устаревшего бэкапа (совместимость)
Используется для проверки миграций и API‑совместимости.

- Восстановить бэкап, отстающий на 1–2 релиза.
- Применить миграции, используя стандартный инструмент инсталляции.
- Проверить API‑совместимость:
  - `make openapi-lint` (схема OpenAPI),
  - базовые проверки из `verify-restore.sh`.

## 9. Откат (rollback)
Если восстановление не соответствует критериям:
1. Остановить CP/DP.
2. Повторить восстановление на предыдущий известный корректный бэкап.
3. Повторить `verify-restore.sh`.
4. Зафиксировать причину и обновить runbook.

## 10. Рекомендации
- Бэкап должен включать БД и объектное хранилище одновременно.
- Проверяйте `manifest.env` для оценки целостности.
- Планируйте регулярные тренировки восстановления (см. `docs/ops/dr-game-day.md`).
