# HA и масштабирование

Документ описывает рекомендации по высокой доступности и масштабированию Control Plane (CP) и Data Plane (DP), что снижает риск деградации при росте нагрузки.

## 1. Предпосылки

- Внешний Postgres в HA‑режиме или управляемый сервис.
- Надёжное S3‑совместимое хранилище с репликацией.
- Ingress/LoadBalancer для Gateway при необходимости внешнего доступа.

## 2. Control Plane (datapilot)

**Свойства:**
- Stateless‑логика, масштабируется горизонтально.
- Состояние хранится в Postgres и S3/MinIO.

**Масштабирование:**
```bash
kubectl -n animus-system scale deploy/animus-datapilot-experiments --replicas=3
kubectl -n animus-system scale deploy/animus-datapilot-gateway --replicas=3
```

**Ожидаемый результат:**
- Роуты Gateway обслуживаются всеми репликами.
- Состояние согласовано через БД и audit‑outbox.

**Примечание:**
- При увеличении реплик важно обеспечить достаточную пропускную способность Postgres.

## 3. Data Plane (dataplane)

**Свойства:**
- Масштабируется горизонтально.
- Отвечает за исполнение и доступ к секретам.

**Масштабирование:**
```bash
kubectl -n animus-system scale deploy/animus-dataplane --replicas=2
```

**Ожидаемый результат:**
- Планировщик CP равномерно распределяет dispatch‑запросы.
- DP обеспечивает устойчивую доставку heartbeat/terminal сообщений.

## 4. Postgres и S3/MinIO

- Postgres должен обеспечивать транзакционные гарантии для планировщика, audit и provenance.
- Рекомендуется включить синхронную репликацию и резервное копирование.
- S3/MinIO должны быть настроены на репликацию и неизменяемость при необходимости compliance.

## 5. Multi‑cluster (ограничения)

- Текущая реализация предполагает единый кластер для CP и DP.
- Разделение на несколько кластеров допускается только при гарантированном сетевом доступе DP → Gateway и согласованной политике секретов.

## 6. Типовые отказоустойчивые сценарии

- **Потеря одной реплики CP:** трафик продолжает обслуживаться через Gateway.
- **Потеря реплики DP:** выполнение переносится на живые DP‑реплики; терминальные состояния не нарушаются.
- **Потеря БД:** операции управления невозможны до восстановления; требуется DR‑процедура.

## 7. Диагностика

```bash
kubectl -n animus-system get pods
kubectl -n animus-system top pods
kubectl -n animus-system describe deploy/animus-datapilot-experiments
kubectl -n animus-system describe deploy/animus-dataplane
```
