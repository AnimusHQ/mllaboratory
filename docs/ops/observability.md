# Наблюдаемость (метрики, логи, OTel)

## Метрики Prometheus
Сервисы публикуют `/metrics` в текстовом формате Prometheus. В Helm‑чартах включены аннотации для скрейпа:
- `prometheus.io/scrape: "true"`
- `prometheus.io/path: /metrics`
- `prometheus.io/port: <порт сервиса>`

Отключение скрейпа:
```yaml
observability:
  metrics:
    enabled: false
```

## Логи
Все сервисы используют структурированные JSON‑логи (slog). Рекомендуется централизованный сбор (Loki/ELK/Datadog).

## OpenTelemetry (минимальные настройки)
В чартах доступны базовые переменные окружения (без обязательного деплоя трейс‑бекенда):
- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_INSECURE`
- `OTEL_TRACES_EXPORTER`
- `OTEL_METRICS_EXPORTER`
- `OTEL_LOGS_EXPORTER`
- `OTEL_SERVICE_NAME`

Пример:
```yaml
observability:
  otel:
    enabled: true
    endpoint: "http://otel-collector:4317"
    insecure: true
    tracesExporter: otlp
    metricsExporter: otlp
    logsExporter: none
```

## Замечания
- Метрики ограничены базовыми показателями процесса (goroutines, uptime).
- Полноценная трассировка и бизнес‑метрики добавляются отдельными инкрементами.
