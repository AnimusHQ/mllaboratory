# Наблюдаемость и SLO

## 1. Метрики покрытия

Минимальный набор метрик для мониторинга:

- `animus_http_requests_total{service,method,status_class}` — скорость запросов и ошибки по классам ответа.
- `animus_http_request_duration_seconds_*{service,method}` — латентность HTTP.
- `animus_webhook_delivery_*` — попытки, успехи, ошибки и латентность доставки webhooks.
- `animus_audit_export_attempts_total{sink_type,outcome}` и `animus_audit_export_dlq_size` — экспорт аудита.
- Метрики очередей/ретраев соответствующих воркеров (webhooks, audit export).

## 2. Корреляция запросов

Сквозной идентификатор запроса — `X-Request-Id`.

- Gateway генерирует/пробрасывает `X-Request-Id` и подписывает внутренние заголовки.
- Control Plane сохраняет `request_id` в аудите и включает его в события.
- Data Plane получает `correlationId` при dispatch и включает его в события DP.
- Экспорт аудита сохраняет `request_id` и доступен в SIEM‑потоке.

## 3. Базовые SLO (рекомендации)

- Доступность Control Plane: 99.9% за месяц.
- Диспетчеризация запуска (Create+Dispatch): p95 ≤ 5s при стабильном хранилище и DP.
- Доставка audit‑событий в SIEM: p95 ≤ 30s при доступном sink.

## 4. Дашборды как документация

Рекомендуемые графики:

- HTTP: `rate(animus_http_requests_total{status_class=~\"5..\"}[5m])` и p95 по `animus_http_request_duration_seconds`.
- Webhooks: `rate(animus_webhook_delivery_failure_total[5m])`, p95 латентности.
- Audit export: `rate(animus_audit_export_attempts_total{outcome=\"retry\"}[5m])`, `animus_audit_export_dlq_size`.

Каждый график должен иметь алерт на превышение SLO или рост очередей/reties.
