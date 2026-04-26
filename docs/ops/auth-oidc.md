# OIDC авторизация: канонический хост и redirect

## Назначение
Документ описывает правила канонического хоста и безопасной обработки `return_to` для OIDC‑входа.

## Канонический хост
Для предотвращения `invalid_state` необходимо использовать **один** хост при инициации логина и при callback.

**Правило:** вход всегда инициируется на Gateway (порт 8080), а browser использует один и тот же хост на протяжении всего потока.

Пример для Windows + WSL:
- Канонический хост: `http://172.27.173.217:8080`
- Нельзя смешивать `localhost` и `172.27.173.217` в одном потоке.

## Возврат после логина
Gateway принимает `return_to`:
- относительный путь (`/console`) или
- абсолютный URL только из allowlist.

Разрешённые origin определяются через:
- `ANIMUS_PUBLIC_BASE_URL`
- `ANIMUS_ALLOWED_RETURN_TO_ORIGINS` (список через запятую)

Для UI на отдельном порту добавьте:
```
ANIMUS_ALLOWED_RETURN_TO_ORIGINS=http://172.27.173.217:3001
```

## Keycloak настройки
Для клиента `animus-gateway`:
- **Valid Redirect URIs**: `http://172.27.173.217:8080/auth/callback`
- **Web Origins**: `http://172.27.173.217:8080`

## Типовая диагностика
### Ошибка `invalid_state`
Причины:
1) разные хосты между `/auth/login` и `/auth/callback`
2) cookie не сохранился (блокировка third‑party cookies)

Проверка:
```
curl -I http://172.27.173.217:8080/auth/login?return_to=/console
```
Убедитесь, что cookie `animus_oidc_state` выставлен на канонический хост.
