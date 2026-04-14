# Наблюдаемость (метрики, логи, трассировка)

Документ описывает включение и проверку наблюдаемости для CP/DP/Gateway, что снижает риск «слепых» инцидентов.

## 1. Предпосылки

- Prometheus или совместимый скрейпер метрик.
- Центральный сбор логов (Loki/ELK/Datadog) при необходимости.
- OTEL‑коллектор при включении трассировки.

## 2. Метрики

**Пути метрик:**
- `GET /metrics` на каждом сервисе CP и DP.

**Пример конфигурации Prometheus:**
```yaml
scrape_configs:
  - job_name: animus-datapilot
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_namespace]
        regex: animus-system
        action: keep
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_instance]
        regex: animus-datapilot
        action: keep
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        regex: "true"
        action: keep
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        target_label: __metrics_path__
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        regex: (.+):(\d+);(\d+)
        replacement: $1:$3
        target_label: __address__
  - job_name: animus-dataplane
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_namespace]
        regex: animus-system
        action: keep
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_instance]
        regex: animus-dataplane
        action: keep
```

**Ожидаемый результат:**
- Метрики доступны и не содержат секретов.
- Счётчики попыток/ошибок присутствуют для интеграций (webhooks, SIEM, audit).

## 3. Логи

- Формат: JSON (slog).
- Корреляция: `request_id`, `project_id`, `run_id` (где применимо).

**Команды:**
```bash
kubectl -n animus-system logs deploy/animus-datapilot-gateway --tail=200
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

## 4. Трассировка (OTel)

**Включение:**
```yaml
observability:
  otel:
    enabled: true
    endpoint: "http://otel-collector:4317"
    insecure: true
    tracesExporter: otlp
    metricsExporter: otlp
    logsExporter: none
    serviceNamePrefix: animus
```

**Ожидаемый результат:**
- Спаны для входящих HTTP‑запросов и внутренних вызовов.

## 5. Связанные документы

- `docs/ops/observability-slos.md` — базовые SLO и критерии.
- `docs/ops/troubleshooting.md` — диагностика при сбоях.
