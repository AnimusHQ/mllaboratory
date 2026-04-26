# DevEnv: эксплуатационные требования и контроль

**Версия документа:** 1.0

## Назначение
DevEnv (управляемая среда разработки) предоставляет интерактивный доступ к рабочему окружению, согласованному с политиками проекта. Контрольная плоскость (CP) выпускает краткоживущие сессии доступа, плоскость данных (DP) создаёт и обслуживает вычислительный workload, а политики и аудит фиксируются при каждом изменении состояния.

## Архитектурная схема
- **CP**: создание DevEnv, выпуск сессий доступа, аудит, периодическая сверка TTL.
- **DP**: создание/удаление workload (Kubernetes Job), проверка готовности окружения.
- **Политики**: фиксируются в `PolicySnapshot` при создании DevEnv и неизменны для жизненного цикла среды.

## Потоки
1) **Создание DevEnv**
- Вход: `templateRef`, `repoUrl`, `refType` (`branch|tag|commit`), `refValue`, опционально `commitPin`, `ttlSeconds`, `Idempotency-Key`.
- CP валидирует `repoUrl` (схемы `https|ssh`, запрет userinfo, allowlist), фиксирует `PolicySnapshot`.
- CP сохраняет DevEnv (repo/ref фиксируются и неизменяемы), вызывает DP (provision).
- DP создаёт Job с initContainer для `git clone/checkout` в рабочий volume и контейнером `code-server`.
- CP записывает аудит: `devenv.created`, `devenv.provisioned` или `devenv.provision_failed`, а также `devenv.repo.cloned` (метаданные репозитория).

2) **Доступ (IDE‑сессия)**
- Вход: `ttlSeconds` для сессии.
- CP выпускает `DevEnvAccessSession` и записывает `devenv.session.opened`.
- CP проверяет готовность DP, возвращает `proxyPath` для IDE.
- Прокси‑доступ: `GET /devenv-sessions/{session_id}/proxy/{path...}` (HTTP + WS), доступ только через CP.
- Аудит доступа агрегируется по интервалу: `devenv.session.accessed`.

3) **Остановка**
- CP инициирует удаление workload в DP, фиксирует `devenv.deleted`.

4) **TTL‑сверка**
- Фоновый reconciler в CP помечает истёкшие DevEnv как `expired` и инициирует удаление в DP.
- События: `devenv.expired`, `devenv.deleted`.

## Конфигурация
### Control Plane
- `ANIMUS_DEVENV_TTL` — TTL DevEnv по умолчанию (например, `2h`).
- `ANIMUS_DEVENV_ACCESS_TTL` — TTL IDE‑сессии (например, `15m`).
- `ANIMUS_DEVENV_ACCESS_AUDIT_INTERVAL` — интервал агрегации `devenv.session.accessed` (например, `1m`).
- `ANIMUS_DEVENV_RECONCILE_INTERVAL` — период TTL‑сверки (например, `30s`).
- `ANIMUS_DEVENV_REPO_ALLOWLIST` — allowlist репозиториев (формат: `host` или `host/org`, CSV).
- `ANIMUS_DEVENV_SERVICE_DOMAIN` — DNS‑суффикс k8s‑сервиса (например, `svc.cluster.local`).
- `ANIMUS_DEVENV_CODE_SERVER_PORT` — порт IDE‑сервиса (должен совпадать с DP).

### Data Plane
- `ANIMUS_DEVENV_K8S_NAMESPACE` — namespace для DevEnv workloads.
- `ANIMUS_DEVENV_K8S_SERVICE_ACCOUNT` — service account для workloads (минимальные привилегии).
- `ANIMUS_DEVENV_JOB_TTL_AFTER_FINISHED` — TTL для завершённых Job (секунды).
- `ANIMUS_DEVENV_WORKSPACE_PATH` — путь рабочей директории (по умолчанию `/workspace`).
- `ANIMUS_DEVENV_GIT_IMAGE` — образ initContainer для `git clone` (по умолчанию `alpine/git:2.43.0`).
- `ANIMUS_DEVENV_CODE_SERVER_CMD` — команда запуска IDE (по умолчанию `code-server --bind-addr 0.0.0.0:8080 --auth none /workspace`).
- `ANIMUS_DEVENV_CODE_SERVER_PORT` — порт IDE‑контейнера.

## Политики и безопасность
- DevEnv создаётся с фиксированным `PolicySnapshot`; изменения политик не применяются ретроактивно.
- Сеть: применяется deny‑by‑default egress, разрешения — через `NetworkClassRef`.
- Секреты: доступ только через DP, CP не получает значения секретов.
- IDE не имеет прямого внешнего ingress; доступ только через CP‑прокси.

## Аудит и наблюдаемость
- Все операции DevEnv фиксируются в append‑only аудит‑логе.
- Ключевые события: `devenv.created`, `devenv.provisioned`, `devenv.provision_failed`, `devenv.repo.cloned`, `devenv.session.opened`, `devenv.session.accessed`, `devenv.expired`, `devenv.deleted`.

## Ограничения
- Все create‑операции требуют `Idempotency-Key` и должны быть повторяемыми без дублирования сущностей.
- DevEnv создаётся только из зарегистрированного `EnvironmentDefinition`.
- `repoUrl` допускает только `https|ssh` без userinfo; проверка allowlist выполняется до provisioning.
