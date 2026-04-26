# План закрытия разрывов: M7, M8, OIDC SSO, Lineage MVP, CI Gates

**Цель:** довести платформу до состояния production‑grade, закрыв оставшиеся функциональные пробелы без изменения семантики M2–M9/P5, кроме исправлений дефектов.

## 1. Канонические источники истины
- `roadmap.json` (roadmap_version=3).
- `docs/contracts/index.md` — свод контрактов и инвариантов.
- OpenAPI спецификации:
  - `open/api/openapi/gateway.yaml`
  - `open/api/openapi/experiments.yaml`
  - `open/api/openapi/dataset-registry.yaml`
  - `open/api/openapi/dataplane_internal.yaml`
  - `open/api/openapi/lineage.yaml`
  - `open/api/openapi/audit.yaml`

## 2. Разрывы и целевые deliverables

### 2.1 M7 Dev Environments
**Разрыв:** отсутствуют сущность DevEnv, персистентность, DP‑контроллер и CP‑доступ; в OpenAPI нет endpoints.

**Цели:**
- Доменные сущности DevEnvironment (project‑scoped), policy snapshot, ttl.
- Персистентность и миграции.
- DP‑контроллер (TTL‑reconcile, egress deny‑by‑default, SA least privilege).
- CP API + proxy‑доступ с audit + RBAC.
- Обновление OpenAPI (`experiments.yaml`, опционально `gateway.yaml`).

**Тесты:** idempotent create, TTL expiry, proxy access with audit/RBAC.

**Документы:** `docs/ops/devenv.md` + матрица критериев.

### 2.2 M8 Model Registry
**Разрыв:** модельный реестр частично отражён в контрактах, но отсутствует реализация хранения/статусной машины/экспорта.

**Цели:**
- Схемы БД: `models`, `model_versions`, `model_transitions`, `model_exports` (idempotent).
- Статусная машина (`draft → validated → approved → deprecated`) в сервисном слое.
- Provenance: Run + Artifact(s) + DatasetVersion + EnvLock + CodeRef.
- RBAC + audit для всех state‑change и export.
- OpenAPI: реализовать в `experiments.yaml` + gateway‑surface.

**Тесты:** допустимые/недопустимые переходы, RBAC запреты, idempotent export, audit coverage.

**Документы:** обновление `docs/contracts/index.md`, `docs/roadmap/production-grade-criteria-matrix.md`.

### 2.3 OIDC SSO
**Разрыв:** OpenAPI содержит `/auth/login`/`/auth/callback`, требуется полноценная реализация потока.

**Цели:**
- Auth Code + PKCE, конфигурируемые endpoints/redirects.
- Маппинг групп в роли (project‑scoped) с аудитом изменений.
- Сессии: TTL, forced logout, concurrency limit.

**Тесты:** mock OIDC provider через RoundTripper, deterministic TTL expiry, group mapping.

**Документы:** `docs/security/session-management.md`, `docs/enterprise/08-rbac-matrix.md` (если требуется).

### 2.4 Lineage MVP
**Разрыв:** OpenAPI `lineage.yaml` определяет events + subgraphs; необходима материализация и API.

**Цели:**
- Материализация линий (edges) и/или событий, предпочтительно materialized edges.
- Реализация endpoint’ов из `open/api/openapi/lineage.yaml`.
- RBAC‑контроль на lineage‑query.

**Тесты:** детерминированный subgraph для Run/Dataset/Model‑chain, RBAC отрицательные кейсы.

**Документы:** `docs/contracts/index.md`, матрица критериев.

### 2.5 CI Gates
**Разрыв:** нужны env‑gated SAST/dep scan и OpenAPI breaking‑change check.

**Цели:**
- Make targets `sast-scan` и `dep-scan` (env‑gated, offline‑fallback).
- OpenAPI breaking‑change gate с baseline snapshot.
- Документация: `docs/ops/supply-chain.md`, release‑процесс.

**Тесты:** не требуются; верификация — запуск gated‑targets в CI.

## 3. Общие требования к реализации
- Идемпотентность create‑эндпоинтов по `Idempotency-Key`.
- Любые state‑changes — audit append‑only.
- CP не исполняет пользовательский код.
- Никаких внешних сетевых слушателей в тестах.
- Детерминизм и reproducibility.

## 4. Зависимости и порядок выполнения
1. M7 DevEnv (база для доступа/политик).
2. M8 Model Registry (provenance и export).
3. OIDC SSO (auth completeness).
4. Lineage MVP (materialization + queries).
5. CI gates (контроль качества релиза).

## 5. Критерии завершения
- Все OpenAPI‑контракты реализованы и проходят `make openapi-lint`.
- Все новые операции — audited и idempotent.
- Тесты детерминированы: `./scripts/go_test.sh ./closed/...` и `make integrations-test`.
- Документы обновлены в Russian scientific‑technical стиле.
