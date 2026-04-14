# Интеграционная документация (open)

Документы описывают интеграционный контур Animus Datalab и фиксируют источники истины для API, SDK и evidence‑артефактов, что снижает риск расхождения интеграций с фактическим поведением.

## С чего начать
1. `docs/open/00-overview.md` — обзор и цели.
2. `docs/open/01-architecture.md` — архитектура и связи.
3. `docs/open/02-security-and-compliance.md` — безопасность и комплаенс.

## Основные пути чтения
- **Оценка безопасности и доверия**: `00-overview`, `01-architecture`, `02-security-and-compliance`.
- **Интеграция**: `05-api`, `06-cli-and-usage`, `07-evidence-format`.
- **Эксплуатация**: `03-deployment`, `04-operations`, `08-troubleshooting`.

## Источники истины
- OpenAPI: `open/api/openapi/`.
- SDK: `open/sdk/python/`.
- Демо‑клиенты: `open/cmd/demo/`, `open/demo/`.

## Гарантии
Документация описывает контракты интеграций; реализация следует этим контрактам, что снижает риск несовместимости.
