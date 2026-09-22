# Heatseeker — контекст для Claude Code

## Что это

Heatseeker — платформа для учебной группы (~20 человек: магистратура + отдельный поток ДПО).
Единая точка сборки учебных материалов (вместо разрозненных Google Диска и Notion), дедлайны и
задачи с уведомлениями, общее гибкое расписание, обсуждения по предметам, предложения группы,
теги/фильтры, автоматическая раскладка файлов с Google Диска по предметам, системные и кастомные
напоминания. CRM-элементы: доска статусов, лог активности, роли (админ / модератор / староста).
Mobile-first (Android APK), веб — вторым этапом. Вторая волна — ИИ-агенты (отдельный сервис,
self-hosted модель) и markdown-редактор.

## Статус (2026-09-22)

**Этап 1 (материалы с Диска + теги) — реализован, проверен вручную на реальном Диске и телефоне
(2026-09-17); найденные недочёты — `docs/FIXES.md`.** Готово:
материалы и версии, лента с фильтрами/поиском/курсором, загрузка в S3/local (presigned PUT или
подписанная ссылка на API) с проверкой сигнатур, открытие файлов (presigned GET, прокси с Range
для Диска, экспорт Google Docs в PDF), архив/удаление с правилами владения, «Входящие» с
обучением синонимов, подключение папки Диска через сервисный аккаунт, полный скан + лента
изменений (asynq, раз в минуту проверка сроков), классификатор (`internal/app/classify`),
публикация загрузок на Диск (D32), лента активности; экраны mobile: лента, материал, загрузка,
«Входящие», Диск, предметы, активность. Проверено фейковым Диском (`adapters/gdrive/fake.go`)
в интеграционном тесте и MinIO, затем на реальной папке (индексация, раскладка, изменения,
удаление, Google Docs). Публикация на Диск — от подключённого через OAuth аккаунта
старосты (D34, D36) или сервисным аккаунтом в Shared Drive. Markdown в описаниях пока
показывается как текст (редактор — этап 2).

**Этап 2 (обсуждения + уведомления + задачи) — начат с задач и дедлайнов (2026-09-20).** Готово:
`tasks`/`task_assignments`/`task_attachments` (миграция 00008), задачи с предметом, типом
(`TEACHER|GROUP|PERSONAL`), приоритетом, сроком, режимом выдачи (`ALL|SELECTED|SELF`) и личными
задачами (`PRIVATE`); создаёт любой участник (идемпотентно по `client_id`), закрепляет староста
(`task.pin`), общий статус — автор/староста/модератор, личный прогресс — каждый; счётчики доски
(`GET /groups/:id/tasks/board`); сканер дедлайнов раз в минуту объявляет `task.due_soon` /
`task.overdue` в журнал группы (D38, `DEADLINE_REMINDER_OFFSETS`); вкладка «Задачи», карточка
задачи (срок — календарём, без времени — конец дня, D42; файлы задачи — `PUT /tasks/:id/materials`,
прикрепить из материалов или загрузить; загруженный файл — «только в задаче» или в материалы
группы, D43) и форма в mobile; при входе открывается вкладка «Задачи» (первая), «назад» — по истории;
группа выбирается уже на первом экране (открытые группы по названию, D44).
**Обсуждения (2026-09-21):** `threads`/`messages`/`message_hides`/
`thread_reads` (миграция 00009); обсуждение адресуется целью (`SUBJECT|GENERAL|MATERIAL|TASK`) и
создаётся первым сообщением, события `message.*` без текста (D40); ответы, правка, удаление и
восстановление своего, скрытие для себя («Скрытые мной», `include_hidden`), модерация для всех,
непрочитанное по `last_read_seq`; офлайн-outbox в `@heatseeker/core` (AsyncStorage, D41); вкладка «Обсуждения» с
бейджем, экран треда, кнопка «Обсуждение» у материала и задачи. Открытый тред опрашивается раз в
10 с (realtime — W2-0). Не сделано в этапе: уведомления (push, преференции, mute, тихие часы),
кастомные напоминания (`reminders`), объявления; markdown в сообщениях пока показывается текстом.

**Этап 3 (расписание) — начат (2026-09-22), хотя уведомления этапа 2 ещё не сделаны.** Готово:
`schedule_events`/`schedule_exceptions` (миграция 00011); разовые занятия и еженедельные серии
(каждую неделю / через неделю, дни недели, до даты) — хранятся как RRULE-подмножество и
разворачиваются при чтении в часовом поясе серии (D45); отмена и изменение одного занятия
(время, день, аудитория, преподаватель, причина), «изменить с этого дня» (серия делится),
версии правок (409); правит староста/админ с защищённым аккаунтом; события `schedule.*` в журнал;
ICS-подписка по личной ссылке (D46). Mobile: вкладка «Расписание» (неделя, полоска дней, день,
«идёт сейчас» / «следующее»), карточка занятия (`class/[eventId]/[date]`) с обсуждением предмета,
форма (`class/edit`: новое, серия, с даты, одно занятие). Не сделано в этапе: треды занятий
(`LESSON`), запросы на изменение и их утверждение, уведомления об изменениях. Заодно — уменьшенные
копии картинок (`GET /materials/:id/thumbnail`, D47): видны сразу в задаче и на экране материала,
полный размер — во встроенном просмотрщике.

**Этап 0 (фундамент) — каркас собран.** Готово: Go-ядро (`apps/api`) с auth L1/L2, группами,
мультиролями, инвайтами, предметами/тегами, журналом событий и `sync`; миграции; OpenAPI;
`packages/shared` (генерируется из Go), `packages/api-client`, `packages/i18n`,
`packages/config`, `packages/core` (сессия, хуки TanStack Query), `packages/ui` (Tamagui);
`apps/mobile` (Expo SDK 57 + Expo Router: имя → группа → главный с чипами, «Ещё» с защитой
аккаунта); compose для dev; CI.
Запуск проверен в Expo Go на Android и в вебе (2026-09-17). Не сделано в этапе 0:
EAS-конфигурация, оптимизирующий плагин Tamagui. Все принятые решения — в `docs/DECISIONS.md`,
нерешённое — в `docs/OPEN-QUESTIONS.md`. **Не принимать решения по открытым вопросам молча** —
спросить или явно предложить вариант.

## Стек (утверждён)

| Слой | Технологии |
|---|---|
| Ядро (`apps/api`) | Go 1.23+, `net/http` + chi + huma v2 (OpenAPI 3.1 code-first), PostgreSQL + pgx + sqlc + goose, Redis + asynq (очереди, cron) + Redis Pub/Sub (realtime), minio-go (S3), Google Drive API (сервисный аккаунт), `coder/websocket`, `log/slog` |
| Mobile (`apps/mobile`) | Expo (React Native) + Expo Router, TypeScript, TanStack Query (+persist), Zustand, react-hook-form + zod, офлайн-outbox сообщений (очередь в `@heatseeker/core` на AsyncStorage, D41; expo-sqlite — когда понадобится локальная БД сообщений), expo-notifications, i18next |
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

## Команды (поддерживать актуальными)

Go-тулчейн стоит в user-space; в каждом shell перед Go-командами:
`source /run/media/deck/EE4S8/go-toolchain/env.sh` (даёт go, sqlc, task, golangci-lint).
Docker Hub недоступен — compose использует зеркала (`mirror.gcr.io`, `quay.io`).

```bash
# инфраструктура для разработки (Postgres :5433, Redis :6379, MinIO :9100/:9101, asynqmon :8082,
# Gotenberg :3030 — PDF-превью офисных файлов, OFFICE_PREVIEW_URL)
task dev:up      # = podman compose -f infra/compose/docker-compose.dev.yml up -d
task dev:down    # остановить (данные сохраняются); task dev:reset — стереть данные
task dev:ip      # IP компьютера в Wi-Fi → apps/api/.env и apps/mobile/.env.local (телефон, после смены сети)

# ядро (из apps/api; конфиг — apps/api/.env, шаблон — .env.example в корне)
task api:migrate            # goose up;  task api:migrate -- status|down|redo
task api:run:api            # HTTP API, документация: http://localhost:8000/api/v1/docs
task api:run:worker         # asynq-воркеры + планировщик
task api:gen                # sqlc → OpenAPI (apps/api/openapi/) → packages/shared/src/generated/
task api:test               # юнит-тесты
task api:test:integration   # сквозные тесты на живой БД + MinIO (compose должен быть поднят; Диск — фейковый)
task api:lint               # go vet + golangci-lint
task api:check              # всё, что гоняет CI

# JS-часть (корень репо)
pnpm install
task gen                                          # api:gen + регенерация @heatseeker/api-client
pnpm lint | pnpm typecheck | pnpm test           # через turbo по всем workspaces

# мобильное приложение
pnpm -F mobile start                              # Metro + QR для Expo Go (EXPO_PUBLIC_API_URL — адрес API в сети)
pnpm -F mobile export:check                       # собрать Android-бандл без устройства (проверка компиляции)
pnpm -F mobile exec expo install <pkg>            # добавлять Expo-модули только так (версии под SDK)
```

**README — инструкция для человека:** при любом изменении инфраструктуры и запуска (новая
команда `task`, контейнер, порт, переменная окружения, шаг настройки, скрипт в `infra/scripts`)
в том же изменении обновить README: раздел «Каждый день», если команда нужна при обычном
запуске, иначе «Запуск» / «Телефон» / таблицу проблем. Список команд выше — тоже.

Не запускать несколько Bash-вызовов с разными `cd` параллельно — shell общий, cwd гоняется;
для команд workspace'ов использовать `pnpm -F <pkg> ...` из корня.

Сгенерированные артефакты (`apps/api/openapi/openapi.json`, `packages/shared/src/generated/`,
`packages/api-client/src/schema.d.ts`, `internal/adapters/postgres/sqlcgen/`) **коммитятся**;
CI проверяет, что они актуальны. После изменения SQL, хендлеров или матрицы прав — `task gen`.

## Ссылки

- `docs/PLAN.md` — полный план разработки (архитектура, модули, режимы медиа, Google Drive,
  обсуждения/синхронизация, уведомления, env, ИИ-сервис, roadmap).
- `docs/DATA-MODEL.md` — таблицы, поля, индексы.
- `docs/DECISIONS.md` — лог принятых решений (D1–D48).
- `docs/OPEN-QUESTIONS.md` — что ещё не решено и к какому этапу нужно.
- `docs/GOOGLE-DRIVE.md` — настройка сервисного аккаунта и подключение папки.
- `docs/FIXES.md` — недочёты из ручной проверки, очередь исправлений.
- `docs/adr/` — архитектурные решения с обоснованием.
