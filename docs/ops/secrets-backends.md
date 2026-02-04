# Бэкенды секретов (Vault‑like)

**Версия:** 1.0  
**Дата:** 2026-02-04

## 1. Назначение
Подсистема секретов обеспечивает выдачу значений только в Data Plane (DP) во время исполнения. Control Plane (CP) хранит лишь ссылки (`secret_access_class_ref`) и метаданные доступа, что исключает утечку значений через API или аудит.

## 2. Поддерживаемые провайдеры
- `noop` — пустой поставщик, используется для тестов и сред без секретов.
- `static` — статическое отображение `classRef → env`, конфигурируется через `SECRETS_STATIC_JSON`.
- `vault_k8s` — Vault‑подобный backend с аутентификацией через Kubernetes ServiceAccount JWT.

## 3. Vault‑подобный backend (K8s auth)
- DP читает ServiceAccount JWT и выполняет логин в Vault‑подобный backend по `auth/kubernetes/login`.
- Возвращается краткоживущий токен (TTL обязателен).
- Запрос секрета выполняется по пути, равному `secret_access_class_ref` (например, `secret/data/app`).
- Значения секретов используются только для формирования окружения выполнения и не передаются в CP.

## 4. Конфигурация
Общие параметры:
- `SECRETS_PROVIDER` — `noop` | `static` | `vault_k8s`.
- `SECRETS_LEASE_TTL_SECONDS` — TTL лиза по умолчанию (fallback).

Static‑провайдер:
- `SECRETS_STATIC_JSON` — JSON‑словарь `classRef → {KEY: VALUE}`.

Vault‑подобный провайдер:
- `SECRETS_VAULT_ADDR` — базовый URL backend.
- `SECRETS_VAULT_ROLE` — роль Kubernetes auth.
- `SECRETS_VAULT_AUTH_PATH` — путь логина (по умолчанию `auth/kubernetes/login`).
- `SECRETS_VAULT_JWT_PATH` — путь к SA JWT (по умолчанию `/var/run/secrets/kubernetes.io/serviceaccount/token`).
- `SECRETS_VAULT_NAMESPACE` — namespace (опционально, для Enterprise Vault).
- `SECRETS_VAULT_TIMEOUT` — тайм‑аут HTTP (например, `5s`).

## 5. Отказы и реакции
- Тайм‑аут backend → ошибка `secret_fetch_failed`.
- Отозванный токен / 403 → ошибка `secret_fetch_failed`.
- Нулевой/истёкший TTL → ошибка `secret_fetch_failed`.
- Истёкший lease на стороне DP → ошибка `secret_lease_expired`.

## 6. Гарантии редактирования
- Значения секретов не попадают в логи, аудит, OpenAPI и ответы API.
- В аудит передаются только метаданные доступа (`SecretAccessed`).
- Любые сообщения об ошибках исключают значения секретов и токенов.

## 7. Эксплуатация
- Используйте `vault_k8s` с ограниченными ролями и минимальными TTL.
- Настраивайте политики доступа в Vault‑подобном backend на конкретные `classRef`.
- Ротация ServiceAccount JWT выполняется штатно в Kubernetes.
