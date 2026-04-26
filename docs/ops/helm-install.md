# Helm‑установка Animus Datalab (Control Plane + Data Plane)

Документ описывает базовую установку компонентов Animus через Helm‑чарты, что снижает риск расхождения параметров между CP и DP.

## 1. Предпосылки

- Kubernetes 1.25+.
- Helm 3.7+ с поддержкой `values.schema.json`.
- Доступ к реестру образов (online) или предварительно загруженные образы (air‑gapped).
- Подготовленный общий секрет `auth.internalAuthSecret` для CP и DP.
- Выбранный namespace (рекомендуется `animus-system`).

## 2. Подготовка значений

Создайте отдельные файлы значений:
- `values-datapilot.yaml` для `animus-datapilot`.
- `values-dataplane.yaml` для `animus-dataplane`.

**Пример для Control Plane (datapilot, внешние Postgres и S3/MinIO):**
```yaml
image:
  repository: ghcr.io/animus-labs
  tag: "0.1.0"

auth:
  mode: oidc
  internalAuthSecret: "<shared-secret>"

database:
  url: "postgres://animus:animus@postgres.example:5432/animus?sslmode=disable"

postgres:
  enabled: false

minio:
  enabled: false
  endpoint: "s3.example.local:9000"
  accessKey: "<access-key>"
  secretKey: "<secret-key>"
  region: us-east-1
  useSSL: false
  buckets:
    datasets: datasets
    artifacts: artifacts

oidc:
  issuerURL: "https://idp.example.local/realms/animus"
  clientID: "animus"
  clientSecret: "<client-secret>"
  redirectURL: "https://gateway.example.local/auth/callback"
```

**Пример для Data Plane (dataplane):**
```yaml
image:
  repository: ghcr.io/animus-labs
  tag: "0.1.0"

auth:
  internalAuthSecret: "<shared-secret>"

controlPlane:
  baseURL: "http://animus-datapilot-gateway:8080"

secrets:
  provider: vault
  vault:
    addr: "https://vault.example.local"
    role: "animus-dataplane"
    authPath: "auth/kubernetes/login"
    jwtPath: "/var/run/secrets/kubernetes.io/serviceaccount/token"
```

## 3. Установка

```bash
kubectl create namespace animus-system

helm upgrade --install animus-datapilot ./deploy/helm/animus-datapilot \
  --namespace animus-system \
  --values values-datapilot.yaml

helm upgrade --install animus-dataplane ./deploy/helm/animus-dataplane \
  --namespace animus-system \
  --values values-dataplane.yaml
```

## 4. Ожидаемый результат

- Под‑ы в `animus-system` находятся в состоянии `Running`.
- `/readyz` на Gateway возвращает `200`.

```bash
kubectl -n animus-system get pods
kubectl -n animus-system port-forward svc/animus-datapilot-gateway 8080:8080
curl -fsS http://127.0.0.1:8080/readyz
```

## 5. Откат

```bash
helm -n animus-system rollback animus-datapilot
helm -n animus-system rollback animus-dataplane
```

## 6. Диагностика при сбоях

```bash
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

## 7. Ссылки

- `docs/ops/configuration-reference.md` — справочник параметров Helm.
- `docs/ops/airgapped-install.md` — air‑gapped установка.
- `docs/ops/upgrade-rollback.md` — процедура обновления и отката.
