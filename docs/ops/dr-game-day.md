# DR Game‑Day (шаблон)

## 1. Цель
Проверить, что восстановление платформы выполняется в пределах целевых RPO/RTO и соответствует критериям целостности.

## 2. Предпосылки
- Свежий бэкап Postgres и MinIO/S3.
- Зафиксированные версии chart‑ов и digests образов.
- Доступ к cluster‑контролю (kubectl/helm).
- Токен и проект для post‑restore проверок.

## 3. Сценарий (последовательность)
1. Зафиксировать время инцидента `T_incident`.
2. Остановить CP/DP:
```bash
kubectl -n <ns> scale deploy/<cp-deploy> --replicas=0
kubectl -n <ns> scale deploy/<dp-deploy> --replicas=0
```
3. Восстановить Postgres и MinIO/S3:
```bash
BACKUP_DIR=/secure/backups/<timestamp> \
DATABASE_URL="postgres://..." \
ANIMUS_MINIO_ENDPOINT="s3.example" \
ANIMUS_MINIO_ACCESS_KEY="..." \
ANIMUS_MINIO_SECRET_KEY="..." \
ANIMUS_MINIO_BUCKETS="datasets artifacts" \
closed/scripts/restore.sh
```
4. Запустить CP/DP:
```bash
kubectl -n <ns> scale deploy/<cp-deploy> --replicas=1
kubectl -n <ns> scale deploy/<dp-deploy> --replicas=1
```
5. Проверить готовность (`/readyz`) и выполнить smoke‑проверку:
```bash
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
closed/scripts/verify-restore.sh
```
Альтернатива для CI‑сценариев при наличии инфраструктуры:
```bash
ANIMUS_DR_VALIDATE=1 \
ANIMUS_GATEWAY_URL="https://gateway.example" \
ANIMUS_DR_TOKEN="..." \
ANIMUS_DR_PROJECT_ID="proj-1" \
make dr-validate
```
6. (Опционально) Запустить E2E‑набор при наличии инфраструктуры:
```bash
make e2e
```

## 4. Контрольные точки
- База данных восстановлена и доступна для чтения/записи.
- Наборы данных и артефакты доступны (upload → download).
- Работоспособность модели: create → approve → export (при наличии тестовых данных).
- `/healthz` и `/readyz` возвращают `200`.

## 5. Ожидаемые результаты
- Время восстановления `T_restore_done` зафиксировано.
- RPO/RTO вычислены по формулам из `docs/ops/backup-restore.md`.
- Наблюдаемость без утечки секретов (логи и отчёт без секретов).

## 6. Откат (rollback)
Если проверка не проходит:
1. Остановить CP/DP.
2. Восстановить последний корректный бэкап.
3. Повторить `verify-restore.sh`.
4. Зафиксировать причину отклонения и обновить runbook.

## 7. Итог
- Зафиксировать фактические RPO/RTO.
- Сформировать отчёт `docs/ops/dr-gameday-report.md` или приложить ссылку на отчёт в `docs/ops/reports/`.
