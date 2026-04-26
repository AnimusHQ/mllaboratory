# Air‑gapped установка (offline)

Документ описывает подготовку офлайн‑набора и установку без доступа к интернету, что снижает риск зависимости от внешних сетей.

## 1. Предпосылки

- Подготовленные `values-datapilot.yaml` и `values-dataplane.yaml`.
- Доступ к контейнерному registry в изолированном контуре.
- Helm и kubectl доступны в изолированной среде.

## 2. Формирование офлайн‑набора (в online‑контуре)

**Команды:**
```bash
# список образов по значениям
scripts/list_images.sh --values values-datapilot.yaml --values values-dataplane.yaml > images.txt

# сборка bundle
closed/scripts/airgap-bundle.sh \
  --output ./bundle \
  --values values-datapilot.yaml \
  --values values-dataplane.yaml
```

**Ожидаемый результат:**
- В `bundle/` есть `*.tgz` чарты, `images.txt`, `images.tar`, `SHA256SUMS`.

## 3. Проверка целостности

**Команды:**
```bash
cd bundle
sha256sum -c SHA256SUMS
```

**Ожидаемый результат:**
- Все строки отмечены как `OK`.

## 4. Импорт образов в offline‑контуре

**Команды:**
```bash
# локальная загрузка
docker load -i images.tar

# публикация в локальный registry
# пример: registry.local:5000
cat images.txt
```

**Ожидаемый результат:**
- Все образы доступны по digest.

## 5. Установка в offline‑контуре

**Команды:**
```bash
kubectl create namespace animus-system

helm upgrade --install animus-datapilot ./bundle/animus-datapilot-*.tgz \
  --namespace animus-system \
  --values values-datapilot.yaml

helm upgrade --install animus-dataplane ./bundle/animus-dataplane-*.tgz \
  --namespace animus-system \
  --values values-dataplane.yaml
```

**Ожидаемый результат:**
- Под‑ы в `animus-system` в состоянии `Running`.
- `/readyz` Gateway возвращает `200`.

## 6. Откат

```bash
helm -n animus-system rollback animus-datapilot
helm -n animus-system rollback animus-dataplane
```

## 7. Диагностика при сбое

```bash
kubectl -n animus-system describe pods
kubectl -n animus-system logs deploy/animus-datapilot-experiments --tail=200
kubectl -n animus-system logs deploy/animus-dataplane --tail=200
```

## 8. Примечания по supply‑chain

- Рекомендуется фиксировать `image.digest` или `image.digests`.
- Подробные проверки SBOM и уязвимостей: `docs/ops/supply-chain.md`.
