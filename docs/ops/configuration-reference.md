# Справочник конфигурации Helm

Документ содержит нормализованный перечень параметров Helm‑чартов Animus Datalab, что снижает риск несогласованной конфигурации. Источник истины — `values.schema.json` каждого чарта.

## 1. Общие правила

- Все секреты задаются через значения, Secret‑объекты или внешние секрет‑менеджеры.
- Значения должны быть детерминированы и воспроизводимы (без скрытого состояния).
- Для production рекомендуется использовать digest‑pinning образов.

## 2. Control Plane (animus-datapilot)

### 2.1 Образы

| Ключ | Назначение | Примечание |
|---|---|---|
| `image.repository` | Репозиторий образов CP | Например `ghcr.io/animus-labs` |
| `image.tag` | Тег | Для релиза фиксировать версию |
| `image.digest` | Дигест основного образа | Рекомендуется для air‑gapped |
| `image.digests` | Дигесты по сервисам | Для точной пин‑фиксации |
| `image.pullPolicy` | Политика pull | `IfNotPresent` по умолчанию |

### 2.2 Аутентификация и сессии

| Ключ | Назначение | Примечание |
|---|---|---|
| `auth.mode` | Режим auth | `dev` или `oidc` |
| `auth.sessionCookieSecure` | Secure‑cookie | Для HTTPS `true` |
| `auth.internalAuthSecret` | Секрет внутренней подписи | Должен совпадать с DP |

### 2.3 OIDC

| Ключ | Назначение | Примечание |
|---|---|---|
| `oidc.issuerURL` | URL Issuer | Требуется при `auth.mode=oidc` |
| `oidc.clientID` | Client ID | Выдаётся IdP |
| `oidc.clientSecret` | Client Secret | Секретный параметр |
| `oidc.redirectURL` | Redirect URL | Должен совпадать с IdP |
| `oidc.scopes` | Scopes | По умолчанию `openid profile email` |
| `oidc.rolesClaim` | Claim ролей | Обычно `roles` |
| `oidc.emailClaim` | Claim email | Обычно `email` |
| `oidc.sessionCookieName` | Имя cookie | По умолчанию `animus_session` |
| `oidc.sessionMaxAgeSeconds` | TTL сессии | Контроль времени жизни |
| `oidc.sessionCookieSameSite` | SameSite | `Lax` по умолчанию |

### 2.4 База данных

| Ключ | Назначение | Примечание |
|---|---|---|
| `database.url` | URL внешнего Postgres | При задании `postgres.enabled=false` |
| `postgres.enabled` | Встроенный Postgres | `true` для dev‑стендов |
| `postgres.image` | Образ Postgres | По умолчанию `postgres:14-alpine` |
| `postgres.user` | Пользователь | |
| `postgres.password` | Пароль | Секретный параметр |
| `postgres.db` | База | |
| `postgres.persistence.enabled` | PVC | Для стендов `true` |
| `postgres.persistence.size` | Размер PVC | |

### 2.5 Объектное хранилище

| Ключ | Назначение | Примечание |
|---|---|---|
| `minio.enabled` | Встроенный MinIO | `false` для внешнего S3 |
| `minio.endpoint` | Endpoint | Для внешнего S3/MinIO |
| `minio.accessKey` | Access Key | Секретный параметр |
| `minio.secretKey` | Secret Key | Секретный параметр |
| `minio.region` | Регион | |
| `minio.useSSL` | TLS | `true` для HTTPS |
| `minio.buckets.datasets` | Бакет датасетов | |
| `minio.buckets.artifacts` | Бакет артефактов | |

### 2.6 Сервисы и UI

| Ключ | Назначение | Примечание |
|---|---|---|
| `services.gateway.port` | Порт Gateway | Обычно `8080` |
| `services.*.serviceType` | Тип Service | `ClusterIP` или `LoadBalancer` |
| `ui.enabled` | Встроенный UI | При наличии фронтенда |
| `ui.env.gatewayURL` | URL Gateway | Для корректного проксирования |

### 2.7 Обсервабилити

| Ключ | Назначение | Примечание |
|---|---|---|
| `observability.metrics.enabled` | Экспорт метрик | Рекомендуется `true` |
| `observability.metrics.path` | Путь метрик | Обычно `/metrics` |
| `observability.otel.*` | OTEL | Включается при наличии коллектора |

### 2.8 Прочее

| Ключ | Назначение | Примечание |
|---|---|---|
| `migrations.enabled` | Миграции | Обычно `true` |
| `ingress.*` | Ingress | Включается при необходимости |
| `training.*` | Исполнение training | `disabled` по умолчанию |
| `evaluation.*` | Evaluation контур | Включается при необходимости |
| `tests.image.*` | Образ тестов чарта | Используется `helm test` |

## 3. Data Plane (animus-dataplane)

### 3.1 Образы и сервис

| Ключ | Назначение | Примечание |
|---|---|---|
| `image.repository` | Репозиторий DP | |
| `image.tag` | Тег | |
| `image.digest` | Дигест | Рекомендуется в prod |
| `image.pullPolicy` | Политика pull | |
| `service.port` | Порт DP | Обычно `8086` |
| `service.serviceType` | Тип Service | `ClusterIP` |

### 3.2 Аутентификация и связь с CP

| Ключ | Назначение | Примечание |
|---|---|---|
| `auth.internalAuthSecret` | Внутренний секрет | Должен совпадать с CP |
| `auth.allowDirectRoles` | Прямые роли | Только для dev |
| `controlPlane.baseURL` | URL Gateway | Внутренний адрес |

### 3.3 K8s и исполнение

| Ключ | Назначение | Примечание |
|---|---|---|
| `k8s.namespace` | Namespace для job | Обычно тот же namespace |
| `k8s.jobTTLSeconds` | TTL job | Параметр GC |
| `k8s.jobServiceAccount` | SA | Минимальные привилегии |
| `k8s.rbac.create` | Создавать RBAC | `true` для новых кластеров |
| `runtime.heartbeatInterval` | Heartbeat | Для контроля исполнения |
| `runtime.statusPollInterval` | Poll interval | Для реконсиляции |

### 3.4 Secrets

| Ключ | Назначение | Примечание |
|---|---|---|
| `secrets.provider` | Провайдер | `noop`, `vault`, `static` |
| `secrets.leaseTTLSeconds` | TTL аренды | Для Vault |
| `secrets.staticJSON` | Статические секреты | Только для dev |
| `secrets.vault.*` | Параметры Vault | Обязательны при `vault` |

### 3.5 Обсервабилити

| Ключ | Назначение | Примечание |
|---|---|---|
| `observability.metrics.enabled` | Экспорт метрик | Рекомендуется `true` |
| `observability.metrics.path` | Путь метрик | Обычно `/metrics` |
| `observability.otel.*` | OTEL | Включается при наличии коллектора |

## 4. Ссылки

- `deploy/helm/animus-datapilot/values.schema.json`
- `deploy/helm/animus-dataplane/values.schema.json`
