# Обновление и откат (Control Plane + Data Plane)

Документ описывает воспроизводимую процедуру обновления и отката Helm‑релизов с проверкой совместимости, что снижает риск потери данных и несогласованных версий.

## 1. Предпосылки

- Доступ к кластеру `kubectl` и `helm`.
- Подготовленные значения `values-datapilot.yaml` и `values-dataplane.yaml`.
- Ознакомление с политикой совместимости: `docs/enterprise/10-versioning-and-compatibility.md`.

## 2. Перед обновлением

**Команды:**
```bash
kubectl -n animus-system get pods
helm -n animus-system list
```

**Ожидаемый результат:**
- Все поды в `Running`.
- Релизы `animus-datapilot` и `animus-dataplane` присутствуют.

**Резервная копия:**
- Выполнить процедуру из `docs/ops/backup-restore.md`.

## 3. Обновление

**Команды:**
```bash
helm upgrade animus-datapilot ./deploy/helm/animus-datapilot \
  --namespace animus-system \
  --values values-datapilot.yaml

helm upgrade animus-dataplane ./deploy/helm/animus-dataplane \
  --namespace animus-system \
  --values values-dataplane.yaml
```

**Ожидаемый результат:**
- Релизы успешно применены.
- `/readyz` Gateway возвращает `200`.

**Проверки совместимости (рекомендуемые):**
```bash
make openapi-compat
make openapi-lint
```

## 4. Валидация после обновления

```bash
kubectl -n animus-system get pods
kubectl -n animus-system port-forward svc/animus-datapilot-gateway 8080:8080
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8080/metrics | head -n 5
```

**Критерии успеха:**
- Нет перезапусков подов из‑за ошибок конфигурации.
- Метрики доступны и не содержат секретов.

## 5. Откат

**Команды:**
```bash
helm -n animus-system rollback animus-datapilot
helm -n animus-system rollback animus-dataplane
```

**Ожидаемый результат:**
- Релизы возвращены к предыдущей ревизии.
- `/readyz` возвращает `200`.

## 6. Диагностика при сбое

```bash
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

**Типовые причины отказа:**
- Некорректные значения Helm (см. `docs/ops/configuration-reference.md`).
- Несовместимые изменения контрактов (см. `make openapi-compat`).
- Ошибки миграций (см. логи сервиса `experiments`).

## 7. Восстановление

- При ошибках миграции выполните откат Helm и восстановление БД по `docs/ops/backup-restore.md`.
- При ошибках в конфигурации внесите корректировки и повторите обновление.
