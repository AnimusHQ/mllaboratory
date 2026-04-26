# Supply‑chain и статические проверки

Документ описывает минимальные проверки целостности поставки (SBOM, уязвимости) и статические сканеры, доступные в репозитории.

## 1. Предпосылки

- Локальные инструменты `syft` и `grype` при необходимости.
- Отсутствие сетевого доступа допустимо; установка инструментов по сети делается только при явном флаге.

## 2. Команды

**SBOM:**
```bash
make sbom
```

**Проверка SBOM и digest-согласованности:**
```bash
make sbom-check
```

**Подпись образов и аттестация provenance (cosign):**
```bash
make sign-images
make verify-images
```

**Скан уязвимостей:**
```bash
make vuln-scan
```

**Композитный запуск:**
```bash
make supply-chain
```

**SAST (env‑gated):**
```bash
ANIMUS_SAST_SCAN=1 make sast-scan
```

**Dependency scan (env‑gated):**
```bash
ANIMUS_DEP_SCAN=1 make dep-scan
```

**OpenAPI совместимость:**
```bash
make openapi-compat
```

## 3. Ожидаемый результат

- Команды завершаются кодом `0`.
- Артефакты сохраняются в `artifacts/` (или `.cache/supply-chain/` для legacy-сканов) и не добавляются в репозиторий.

## 4. Откат и восстановление

- Изменения от сканеров не вносятся в репозиторий.
- При необходимости удалите артефакты:
```bash
rm -rf .cache/supply-chain
```

## 5. Диагностика при сбое

- Проверьте наличие бинарей `syft`/`grype` в `$PATH`.
- Для явной установки инструментов включите:
  - `ANIMUS_INSTALL_TOOLS=1` для `syft`/`cosign`/`grype` в скриптах SBOM/подписи/уязвимостей.
  - `ANIMUS_SAST_INSTALL=1` для SAST.
  - `ANIMUS_DEP_SCAN_INSTALL=1` для dependency scan.

## 6. Air‑gapped контур

- Для offline‑развёртывания используйте `closed/scripts/airgap-bundle.sh`.
- Список образов: `scripts/list_images.sh`.
- Проверка целостности bundle: `sha256sum -c SHA256SUMS`.

Связанный документ: `docs/ops/airgapped-install.md`.
