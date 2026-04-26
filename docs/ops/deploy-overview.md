# Обзор развёртывания Animus Datalab

**Назначение:** документ фиксирует архитектуру развёртывания Control Plane (CP), Data Plane (DP), Gateway и зависимостей, что снижает риск несогласованной установки. Краткий воспроизводимый quickstart приведён ниже; полные процедуры см. в профильных документах.

## 1. Архитектура развертывания и границы доверия

**Состав компонентов:**

| Компонент | Назначение | Основные зависимости | Примечания по безопасности |
|---|---|---|---|
| Gateway | Внешняя API‑поверхность, аутентификация, RBAC | CP сервисы, IdP (OIDC/SAML) | Единственная публичная точка входа |
| CP (datapilot) | Реестры, планирование, аудит, политика | Postgres, S3/MinIO, SIEM | Не исполняет пользовательский код |
| DP (dataplane) | Исполнение, доступ к секретам | CP Gateway, K8s, Secrets backend | Секреты выдаются только DP |
| Postgres | Транзакционная метадата | Хранилище PVC | Требует резервного копирования |
| S3/MinIO | Датасеты/артефакты | Сеть/объектное хранилище | Доступ ограничен ролями |
| IdP | OIDC/SAML | Внешний | Группы → роли проектов |
| SIEM | Экспорт аудита | Webhook/Syslog | Без секретов в payload |

**Границы доверия:**
- **Gateway** — внешний периметр и единственный публичный вход. Весь трафик к внутренним сервисам проходит через него.
- **CP** — доверенная плоскость управления, не исполняет пользовательский код.
- **DP** — плоскость исполнения, единственная получающая значения секретов.
- **Postgres/S3** — критические зависимости, требующие резервирования и строгих IAM/ACL.

## 2. Сетевые порты и сервисы (базовый профиль)

| Сервис | Порт | Назначение |
|---|---|---|
| Gateway | 8080 | Внешний API (Ingress/LoadBalancer) |
| Dataset Registry | 8081 | Внутренний сервис CP |
| Quality | 8082 | Внутренний сервис CP |
| Experiments | 8083 | Внутренний сервис CP |
| Lineage | 8084 | Внутренний сервис CP |
| Audit | 8085 | Внутренний сервис CP |
| DP | 8086 | Внутренний сервис DP |

## 3. Ресурсные ориентиры (минимальные для стенда)

**Control Plane (datapilot):**
- CPU: 2–4 vCPU
- RAM: 4–8 GiB

**Data Plane (dataplane):**
- CPU: 2–4 vCPU
- RAM: 4–8 GiB

**Postgres:**
- CPU: 2 vCPU
- RAM: 4–8 GiB
- Диск: от 20 GiB, IOPS с учётом журналирования

**S3/MinIO:**
- Диск: от 50 GiB, с запасом под артефакты и датасеты

## 4. Deployment Quickstart (внутренний стенд)

**Предпосылки:**
- Kubernetes 1.25+ с доступом `kubectl` и `helm`.
- Доступ к образам и чартам (онлайн либо предварительно загруженные в air‑gapped контуре).
- Класс хранения StorageClass по умолчанию.

**Команды:**
```bash
# пространство имён
kubectl create namespace animus-system

# установка Control Plane (datapilot)
helm upgrade --install animus-datapilot ./deploy/helm/animus-datapilot \
  --namespace animus-system \
  --values ./deploy/helm/animus-datapilot/values.yaml

# установка Data Plane (dataplane)
helm upgrade --install animus-dataplane ./deploy/helm/animus-dataplane \
  --namespace animus-system \
  --values ./deploy/helm/animus-dataplane/values.yaml
```

**Ожидаемый результат:**
- Все поды в `animus-system` перешли в `Running`.
- `Gateway /readyz` возвращает `200`.

**Проверка готовности:**
```bash
kubectl -n animus-system get pods
kubectl -n animus-system port-forward svc/animus-datapilot-gateway 8080:8080
curl -fsS http://127.0.0.1:8080/readyz
```

**Откат/восстановление:**
```bash
helm -n animus-system rollback animus-datapilot
helm -n animus-system rollback animus-dataplane
```

**Диагностика при сбое:**
```bash
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

## 5. Связанные документы

- `docs/ops/helm-install.md` — детальная установка.
- `docs/ops/configuration-reference.md` — справочник параметров Helm.
- `docs/ops/airgapped-install.md` — air‑gapped установка.
- `docs/ops/upgrade-rollback.md` — обновление и откат.
- `docs/ops/ha-and-scaling.md` — HA и масштабирование.
- `docs/ops/observability.md` — наблюдаемость.
- `docs/ops/backup-restore.md` и `docs/ops/dr-game-day.md` — резервное копирование и DR.
- `docs/ops/security-hardening.md` — безопасность и hardening.
- `docs/ops/troubleshooting.md` — диагностика и типовые сбои.
