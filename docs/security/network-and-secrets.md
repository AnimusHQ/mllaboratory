# Сеть и секреты: принципы защиты

## 1. Политика egress в Data Plane

Data Plane работает в режиме deny‑by‑default для исходящего трафика. Требования:

- Явное указание `networkClassRef` в `EnvLock`.
- Привязка `networkClassRef` к кластерной сетевой политике с allowlist по DNS/IP/CIDR.

Контроль включается через переменную:

- `ANIMUS_DP_EGRESS_MODE=deny|allow` (по умолчанию `deny`).

При режиме `deny` запуск отклоняется, если `networkClassRef` пустой.

Пример шаблона Kubernetes NetworkPolicy (минимальный, для иллюстрации):

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: animus-egress-allowlist
spec:
  podSelector:
    matchLabels:
      animus.network_class_ref: egress-default
  policyTypes:
  - Egress
  egress:
  - to:
    - ipBlock:
        cidr: 10.0.0.0/16
    ports:
    - protocol: TCP
      port: 443
```

## 2. Секреты и отсутствие bypass

Данные секретов извлекаются только в Data Plane во время исполнения. Control Plane хранит и передаёт только ссылки (`secret_access_class_ref`) и не получает значения.

Дополнительные меры:

- В `PolicySnapshot` фиксируется режим `secrets` и `classRef` для детерминизма.
- В событиях аудита и логах отсутствуют значения секретов.
- В отладочных и аварийных строках применяется редактирование.

## 3. Регрессии по редактированию

Редактирование обязано удалять значения токенов и ключей из строковых сообщений (например, `panic:` и `debug:`) и из JSON/metadata payload.
