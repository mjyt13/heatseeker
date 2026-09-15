# Heatseeker — контекст для Claude Code

## Что это

Heatseeker — платформа для учебной группы (~20 человек: магистратура + отдельный поток ДПО).
Единая точка сборки учебных материалов (вместо разрозненных Google Диска и Notion), дедлайны и
задачи с уведомлениями, общее гибкое расписание, обсуждения по предметам, предложения группы,
теги/фильтры, автоматическая раскладка файлов с Google Диска по предметам, системные и кастомные
напоминания. CRM-элементы: доска статусов, лог активности, роли (админ / модератор / староста).
Mobile-first (Android APK), веб — вторым этапом. Вторая волна — ИИ-агенты (отдельный сервис,
self-hosted модель) и markdown-редактор.

## Статус (2026-09-15)

**Планирование завершено, кодирование не начато.** Репозиторий содержит только структуру,
конфиги и документацию. Все принятые решения — в `docs/DECISIONS.md`, нерешённое — в
`docs/OPEN-QUESTIONS.md`. **Не принимать решения по открытым вопросам молча** — спросить или
явно предложить вариант. Следующий шаг — этап 0 из `docs/PLAN.md` (§11 Roadmap).

## Стек (утверждён)

| Слой | Технологии |
|---|---|
| Ядро (`apps/api`) | Go 1.23+, `net/http` + chi + huma v2 (OpenAPI 3.1 code-first), PostgreSQL + pgx + sqlc + goose, Redis + asynq (очереди, cron) + Redis Pub/Sub (realtime), minio-go (S3), Google Drive API (сервисный аккаунт), `coder/websocket`, `log/slog` |
| Mobile (`apps/mobile`) | Expo (React Native) + Expo Router, TypeScript, TanStack Query (+persist), Zustand, react-hook-form + zod, expo-sqlite (офлайн-outbox сообщений), expo-notifications, i18next |
| Web (`apps/web`, этап W2-A) | Vite + React, тот же UI и хуки |
| Общий UI (`packages/ui`) | Tamagui (универсальные компоненты mobile + web) |
| ИИ-сервис (`apps/ai`, этап W2-B) | Python (FastAPI), отдельный процесс, общается с ядром только по REST; провайдер `openai-compatible` (локальная модель), опционально Anthropic |
| Хранилище | MinIO (S3 API) — dev и prod; провайдер `local` для минимального сервера |
| Инфраструктура | docker/podman compose + Caddy; Go — один статический бинарник с подкомандами `api | worker | migrate | gen` |

## Структура репозитория

```
apps/api         Go-ядро: HTTP API + worker (asynq) в одном бинарнике
apps/mobile      Expo-приложение
apps/web         Vite + React (этап 2)
apps/ai          ИИ-сервис (этап 2)
packages/ui      Tamagui-компоненты, общие для mobile и web
packages/shared  enum'ы, матрица прав, константы — ГЕНЕРИРУЮТСЯ из Go (`heatseeker gen`), коммитятся
packages/api-client  openapi-typescript типы + openapi-fetch клиент — генерируются из apps/api/openapi
packages/core    хуки TanStack Query, sync-клиент (офлайн/realtime), permissions helpers
packages/i18n    словари ru (основной) / en (каркас)
packages/config  eslint / prettier / tsconfig base
infra/compose    docker-compose.dev.yml, docker-compose.prod.yml, Caddyfile
infra/scripts    bootstrap бакета, бэкапы
docs/            PLAN.md, DATA-MODEL.md, DECISIONS.md, OPEN-QUESTIONS.md, adr/
```

## Архитектурные принципы

1. **Группа — единица изоляции.** Все доменные сущности несут `group_id`; пользователь может
   состоять в нескольких группах (магистратура + ДПО) с набором ролей в каждой.
2. **Модульный монолит на Go**: `internal/domain` (сущности, интерфейсы) → `internal/app`
   (use cases) → `internal/adapters` (postgres, redis, media, gdrive, push, extract) →
   `internal/transport/http`. Зависимости направлены внутрь (DIP).
3. **Единый журнал событий группы `group_events` (seq)** питает realtime (WS), офлайн-синк
   (`GET /groups/:id/sync?since=`) и ленту активности. Ничего не дублировать.
4. **Права только через матрицу** в `internal/authz` (генерируется в `packages/shared`).
   Роли на членстве — массив; права = объединение. Клиент использует ту же матрицу для скрытия
   кнопок, сервер — для проверки.
5. **Три уровня аутентификации**: L1 — только имя (по умолчанию); L2 — пароль/email/Google
   («защитить аккаунт», обязателен для админ/модератор/староста); L2g — Google подключён
   (только для загрузки на Диск). L0 (публичное чтение) — выключенный переключатель.
6. **Медиа только через `MediaStore`** (`s3 | local | drive`) и три режима группы
   `LINK | CACHE | IMPORT`. Никакой прямой работы с S3/Drive из use cases.
7. **Фоновая работа — в Redis/asynq**, не в Postgres. Postgres — только данные.
8. **Конфиг только через env**, валидация при старте, fail-fast. Провайдеры (хранилище, push,
   ИИ-модель) меняются без правок кода.
9. **SOLID / DRY / KISS / GoF — в точках расширения** (Strategy для экстракторов/классификатора/
   провайдеров, Observer для событий, Repository, Adapter для push, Command для задач,
   Decorator для кеша медиа). Без generic-репозиториев и слоёв ради слоёв.

## Конвенции

- **Язык:** код, идентификаторы, комментарии в коде — English. Коммиты, UI-строки,
  документация — Russian (английские термины допустимы).
- **Коммиты:** Conventional Commits с русским описанием:
  `feat(api): добавить загрузку материалов`, `fix(mobile): починить офлайн-outbox`.
  Типы: `feat | fix | refactor | docs | chore | test | ci | build`. Скоупы: `api | mobile | web |
  ai | ui | shared | core | infra | docs`. **Без строк авторства ИИ-агента** (`Co-Authored-By`,
  `Claude-Session` и т.п.) — автор коммитов один; исключение сделано только для первого коммита.
- **Ветки:** `main` (стабильная), `feat/<кратко>`, `fix/<кратко>`.
- **Go:** `gofmt`, `go vet`, `golangci-lint`; раскладка `cmd/ internal/`; ошибки —
  обёртка с контекстом (`fmt.Errorf("...: %w", err)`), доменные ошибки — типизированы;
  контекст первым аргументом; без глобального состояния; табличные тесты.
- **SQL:** только через sqlc-запросы в `db/queries/`; миграции — файлы goose в
  `db/migrations/`, никакого ручного DDL в проде.
- **API:** REST `/api/v1`, OpenAPI — источник правды; каждый эндпоинт — типизированные
  вход/выход huma; курсорная пагинация; идемпотентность по `client_id` там, где клиент может
  повторить запрос.
- **TypeScript:** strict; ESLint + Prettier из `packages/config`; серверные данные — только
  через TanStack Query + `packages/api-client`; формы — react-hook-form + zod.
- **Markdown** разрешён во всех текстовых полях; рендер с санитизацией, без raw HTML.
- **i18n:** все UI-строки через словари `packages/i18n`, даже если пока только ru.
- **Секреты** никогда не коммитятся; `.env.example` — единственный список переменных.
- **Ничего постороннего:** в репозитории не ссылаться на другие проекты автора; репозиторий
  самодостаточен.

## Команды (появятся на этапе 0; поддерживать актуальными)

```bash
# инфраструктура для разработки
podman compose -f infra/compose/docker-compose.dev.yml up -d   # Postgres, Redis, MinIO, asynqmon

# ядро
cd apps/api && go run ./cmd/heatseeker migrate up
cd apps/api && go run ./cmd/heatseeker api        # HTTP + /docs
cd apps/api && go run ./cmd/heatseeker worker     # asynq-воркеры + планировщик
cd apps/api && go run ./cmd/heatseeker gen        # OpenAPI → openapi/, permissions → packages/shared
cd apps/api && go test ./...

# JS-часть
pnpm install
pnpm -F @heatseeker/api-client generate           # openapi-typescript из apps/api/openapi
pnpm -F mobile start                              # Expo dev server
pnpm dev | pnpm lint | pnpm typecheck | pnpm test # через turbo
```

## Ссылки

- `docs/PLAN.md` — полный план разработки (архитектура, модули, режимы медиа, Google Drive,
  обсуждения/синхронизация, уведомления, env, ИИ-сервис, roadmap).
- `docs/DATA-MODEL.md` — таблицы, поля, индексы.
- `docs/DECISIONS.md` — лог принятых решений (D1–D27).
- `docs/OPEN-QUESTIONS.md` — что ещё не решено и к какому этапу нужно.
- `docs/adr/` — архитектурные решения с обоснованием.
