# Экспорт аудита в SIEM (P5)

**Версия документа:** 1.0

## Обзор
Экспорт аудита реализован как персистентная очередь доставки поверх append‑only `audit_events`. События выгружаются в внешние SIEM‑системы через коннекторы `syslog` (TCP/UDP) или `webhook`. Доставка идемпотентна по `(sink_id, event_id)` и использует детерминированные ретраи.

## Очередь, попытки и DLQ
- Очередь: `audit_export_deliveries` (статус, next_attempt_at, attempt_count).
- История попыток: `audit_export_attempts` (append‑only).
- DLQ: статус `dlq` с `dlq_reason`.
- Replay: `POST /admin/audit/exports/dlq/{delivery_id}:replay` (идемпотентно по `Idempotency-Key` или `replay_token`).

## Идемпотентность и ретраи
- Идентификатор доставки: `(sink_id, event_id)`.
- Webhook: заголовки `Idempotency-Key` и `X-Audit-Event-Id`.
- Ретраи детерминированы; max attempts задаётся через `AUDIT_EXPORT_MAX_ATTEMPTS`.
- Коды 5xx/429/timeout → retry; 4xx (кроме 429) → DLQ.

## Подпись webhook
- Заголовок `X-Animus-Signature: sha256=<hex>`.
- Секрет извлекается по `webhook_secret_ref` из secrets‑backend; значение секретов не логируется и не возвращается через API.

## Метрики
Метрики доступны на `/metrics` в сервисе audit:
- `animus_audit_export_attempts_total{sink_type,outcome}`
- `animus_audit_export_latency_seconds_sum{sink_type}`
- `animus_audit_export_latency_seconds_count{sink_type}`
- `animus_audit_export_dlq_size`

## Переменные окружения
- `AUDIT_EXPORT_DESTINATION` — `webhook`, `syslog`, `syslog_tcp`, `syslog_udp` (default: `none`).
- `AUDIT_EXPORT_FORMAT` — `ndjson`.
- `AUDIT_EXPORT_WEBHOOK_URL` — URL для webhook.
- `AUDIT_EXPORT_WEBHOOK_HEADERS_JSON` — дополнительные заголовки (JSON‑объект, без секретов).
- `AUDIT_EXPORT_WEBHOOK_SECRET_REF` — ссылка на секрет (class_ref).
- `AUDIT_EXPORT_SIGNING_SECRET_KEY` — имя ключа в секрете (default: `AUDIT_EXPORT_SIGNING_SECRET`).
- `AUDIT_EXPORT_SYSLOG_ADDR` — `host:port`.
- `AUDIT_EXPORT_SYSLOG_PROTOCOL` — `udp` или `tcp`.
- `AUDIT_EXPORT_SYSLOG_TAG` — syslog tag.
- `AUDIT_EXPORT_BATCH_SIZE` — размер батча выборки (default: `50`).
- `AUDIT_EXPORT_POLL_INTERVAL` — интервал опроса (default: `5s`).
- `AUDIT_EXPORT_RETRY_BASE` — базовый backoff (default: `5s`).
- `AUDIT_EXPORT_RETRY_MAX` — максимальный backoff (default: `5m`).
- `AUDIT_EXPORT_INFLIGHT_TIMEOUT` — таймаут «в полёте» (default: `2m`).
- `AUDIT_EXPORT_HTTP_TIMEOUT` — таймаут HTTP‑запроса (default: `10s`).
- `AUDIT_EXPORT_MAX_ATTEMPTS` — максимум попыток (default: `10`).
- `AUDIT_EXPORT_WORKER_CONCURRENCY` — параллелизм воркера (default: `4`).

## Типовые отказы
- Неверный адрес/протокол syslog → DLQ.
- Ошибка сетевого подключения → retry.
- 4xx (кроме 429) → DLQ.
- Ошибки secret‑backend → retry; отсутствующий ключ подписи → DLQ.
