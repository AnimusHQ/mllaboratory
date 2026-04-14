# Диагностика и устранение неполадок

Документ содержит детерминированные процедуры сбора диагностики и типовые сценарии отказов, что снижает риск неполной или неподтверждённой диагностики.

## 1. Базовый сбор диагностики

**Команды:**
```bash
kubectl -n animus-system get pods
kubectl -n animus-system get events --sort-by=.lastTimestamp
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-gateway --tail=200
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

**Ожидаемый результат:**
- Под‑ы в `Running`.
- Отсутствие ошибок и перезапусков по причине конфигурации.

## 2. Блокировка Registry Verification

**Симптом:** создание EnvironmentLock возвращает `422` и код `REGISTRY_VERIFICATION_FAILED`.

**Проверка:**
```bash
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200 | rg "registry"
```

**Действия:**
- Проверить `registry` политику проекта в БД.
- Убедиться, что образ указан с digest (`@sha256:`).

## 3. Блокировка SCM Verification

**Симптом:** Run не переходит в `DISPATCHABLE` и причина `SCM_UNVERIFIED`.

**Проверка:**
```bash
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200 | rg "scm"
```

**Действия:**
- Проверить allowlist `repo_url`.
- Убедиться, что commit SHA валиден.

## 4. SIEM DLQ

**Симптом:** рост `audit_export_dlq_size` и записи в DLQ.

**Проверка:**
```bash
kubectl -n animus-system port-forward svc/animus-datapilot-audit 8085:8085
curl -fsS "http://127.0.0.1:8085/admin/audit/exports/deliveries?status=dlq"
```

**Действия:**
- Проверить доступность SIEM endpoint.
- Выполнить replay DLQ через admin‑endpoint (RBAC required).

## 5. DevEnv proxy недоступен

**Симптом:** `/devenv-sessions/{id}/proxy/*` возвращает `502`.

**Проверка:**
```bash
kubectl -n animus-system get jobs -l app.kubernetes.io/component=devenv
kubectl -n animus-system describe job/<job-name>
```

**Действия:**
- Проверить параметры DevEnv и доступность образов.
- Убедиться, что proxy обращается к корректному `serviceName` и порту.

## 6. Диагностика БД

**Команды:**
```bash
kubectl -n animus-system exec deploy/animus-datapilot-postgres -- psql -U animus -d animus -c "select now();"
```

**Ожидаемый результат:**
- Ответ без ошибок.

## 7. Восстановление

- При критических ошибках — откат Helm (`docs/ops/upgrade-rollback.md`).
- При повреждённой БД — восстановление из бэкапа (`docs/ops/backup-restore.md`).
