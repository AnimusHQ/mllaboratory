# Webhook-доставка (P5)

**Версия документа:** 1.0

## Обзор
Исходящие вебхуки формируются в Control Plane на основе доменных событий и доставляются через персистентную очередь. Доставка идемпотентна по `event_id` и `subscription_id`, поддерживает ретраи и аудит.

## Контракты событий
Минимальный полезный payload включает:
- `event_id` (детерминированный), `event_type`, `emitted_at`, `project_id`.
- `subject` (одно из: `run_id`, `model_version_id`, `dataset_version_id`).
- `api_links` для получения полных деталей через API.

## Идемпотентность
- Идентификатор доставки: `(subscription_id, event_id)`.
- Заголовок `Idempotency-Key` имеет значение `event_id:subscription_id`.
- Повторные попытки используют тот же ключ, сохраняя детерминизм.

## Подпись запросов
- Заголовок `X-Animus-Signature: sha256=<hex>`.
- Подпись вычисляется над сырой JSON‑строкой payload.
- Значение секрета извлекается по `secret_ref` из secrets‑backend; CP не хранит секреты.

## Retry и backoff
- 5xx и сетевые ошибки → ретрай.
- 429 → ретрай.
- 4xx (кроме 429) → терминальная ошибка.
- Экспоненциальный backoff с ограничением по `ANIMUS_WEBHOOK_RETRY_MAX`.

## Replay
- API: `POST /projects/{project_id}/webhooks/deliveries/{delivery_id}:replay`.
- Идемпотентность обеспечивается `replay_token` (или `Idempotency-Key` заголовком).

## Метрики
Метрики публикуются на `/metrics`:
- `animus_webhook_delivery_attempts_total`
- `animus_webhook_delivery_success_total`
- `animus_webhook_delivery_failure_total`
- `animus_webhook_delivery_latency_seconds_sum`
- `animus_webhook_delivery_latency_seconds_count`
- `animus_webhook_dlq_size` (в текущей версии DLQ не используется, значение 0)

## Переменные окружения
- `ANIMUS_WEBHOOKS_ENABLED` — включить доставку (default: `true`).
- `ANIMUS_WEBHOOK_BATCH_SIZE` — размер батча выдачи (default: `50`).
- `ANIMUS_WEBHOOK_POLL_INTERVAL` — интервал опроса (default: `5s`).
- `ANIMUS_WEBHOOK_RETRY_BASE` — базовая задержка retry (default: `5s`).
- `ANIMUS_WEBHOOK_RETRY_MAX` — максимальная задержка retry (default: `5m`).
- `ANIMUS_WEBHOOK_INFLIGHT_TIMEOUT` — таймаут «в полёте» (default: `2m`).
- `ANIMUS_WEBHOOK_MAX_ATTEMPTS` — максимум попыток (default: `10`).
- `ANIMUS_WEBHOOK_WORKER_CONCURRENCY` — параллелизм воркера (default: `4`).
- `ANIMUS_WEBHOOK_HTTP_TIMEOUT` — таймаут запроса (default: `10s`).
- `ANIMUS_WEBHOOK_SIGNING_SECRET_KEY` — ключ в секретах (default: `WEBHOOK_SIGNING_SECRET`).

## Типовые отказы
- 4xx → терминальный отказ, запись в delivery attempts.
- 5xx/timeout → ретрай до исчерпания лимита попыток.
- Отсутствующий `secret_ref` → доставка без подписи.
- Отсутствующий ключ подписи → терминальный отказ.
