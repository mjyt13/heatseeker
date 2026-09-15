# Heatseeker — план разработки

Редакция от 2026-09-15. Принятые решения — [`DECISIONS.md`](DECISIONS.md), нерешённое —
[`OPEN-QUESTIONS.md`](OPEN-QUESTIONS.md), модель данных — [`DATA-MODEL.md`](DATA-MODEL.md),
ADR — [`adr/`](adr/).

## Содержание

1. [Принципы](#1-принципы)
2. [Структура репозитория](#2-структура-репозитория-монорепо)
3. [Фронтенд](#3-фронтенд)
4. [Бэкенд (Go)](#4-бэкенд-go)
5. [Медиа: режимы и хранилище](#5-медиа-режимы-и-хранилище)
6. [Google Drive](#6-google-drive)
7. [Обсуждения, скрытие, синхронизация, realtime](#7-обсуждения-скрытие-синхронизация-realtime)
8. [Уведомления](#8-уведомления)
9. [Конфигурация через env](#9-конфигурация-через-env)
10. [ИИ-сервис и markdown](#10-ии-сервис-и-markdown)
11. [Roadmap](#11-roadmap)

---

## 1. Принципы

- **Mobile-first, но API платформо-независим** — веб и ИИ-сервис — просто ещё клиенты.
- **Группа — единица изоляции**; пользователь состоит в нескольких группах (магистратура + ДПО)
  с набором ролей в каждой.
- **Лёгкий вход**: имя → сразу внутри, всё видно. Усиленная аутентификация — только для
  чувствительных действий.
- **Ядро инфраструктуры**: Go-бинарник + Postgres + Redis (очереди, pub/sub, кеш — не грузим
  БД фоновой работой); MinIO, ClamAV, LibreOffice — опциональны и включаются env'ом.
- **Один журнал событий группы** — источник для realtime, офлайн-синка и ленты активности.
- **Всё, что меняется без кода — в env.**
- **SOLID / DRY / KISS / GoF** — применяем в точках расширения (§4.3), не ради галочки.

---

## 2. Структура репозитория: монорепо

Решение — [ADR-0001](adr/0001-monorepo.md). pnpm workspaces + Turborepo для JS-части, Go-модуль
в `apps/api`, Python в `apps/ai`.

```
heatseeker/
├── apps/
│   ├── api/          # Go: HTTP API + worker в одном бинарнике
│   ├── mobile/       # Expo
│   ├── web/          # Vite + React (W2-A)
│   └── ai/           # ИИ-сервис (W2-B)
├── packages/
│   ├── ui/           # Tamagui-компоненты (mobile + web)
│   ├── shared/       # enum'ы, матрица прав, константы — генерируются из Go
│   ├── api-client/   # openapi-typescript + openapi-fetch
│   ├── core/         # хуки TanStack Query, sync-клиент, permissions helpers
│   ├── i18n/         # словари ru/en
│   └── config/       # eslint / prettier / tsconfig base
├── infra/
│   ├── compose/      # docker-compose.dev/prod, Caddyfile
│   └── scripts/      # bootstrap бакета, бэкапы
└── docs/
```

Почему монорепо: общие типы и клиент между API, mobile и web; OpenAPI → клиент регенерируется
при изменении API и ломает сборку, а не прод; атомарные изменения «эндпоинт + экран» одним
коммитом; один CI и одна конфигурация линтера; для команды из 1–3 человек разделение ничего не
даёт. Минус — pnpm + Expo требует `node-linker=hoisted`.

---

## 3. Фронтенд

### 3.1 Mobile: Expo (React Native)

Expo SDK (последний стабильный) + Expo Router (deep links из пушей) + TypeScript; TanStack
Query (+ persist) для серверных данных; Zustand для локального состояния; react-hook-form +
zod; `openapi-fetch` из `packages/api-client`; `expo-secure-store` (токены);
`expo-notifications`; `expo-document-picker` / `expo-file-system`; `expo-sqlite` — локальная
БД сообщений и outbox (офлайн); `i18next` + `expo-localization`.

Почему Expo, а не Flutter/нативно: единый TypeScript с вебом (типы, клиент, хуки, UI), EAS
Update для обновлений без стора, знакомая React-экосистема. Flutter дал бы веб только на
canvas-рендере (плохо для контентного приложения) и потерю общих типов.

### 3.2 Единый UI-слой mobile + web: `packages/ui` на Tamagui

Решение — [ADR-0005](adr/0005-tamagui-universal-ui.md). Один компонент для Expo и Vite
(`@tamagui/vite-plugin`); на вебе компилируется в CSS, на native — в плоские View; без
runtime-стилей. Страховка: на этапе 0 настройка ограничена 2 днями; при провале — откат на
NativeWind + `react-native-reusables` (mobile) и shadcn/ui (web) с общими Tailwind-токенами.

### 3.3 Первый вход и навигация

- Регистрация: экран «Как тебя зовут?» (+ код/ссылка группы, если пришёл по ней) → сразу в
  группу. Email/пароль/Google — позже, в настройках («Защитить аккаунт»), и только если нужно
  (§4.5).
- Главный экран: сверху — горизонтальная лента чипов-тегов: предметы, «Мои», «Сохранённое»
  (закладки), «Непрочитанное»; выбранный чип фильтрует ленту ниже (материалы, задачи,
  обсуждения). Набор чипов по умолчанию настраивается пользователем и сохраняется.
- Вкладки: Лента / Расписание / Задачи / Обсуждения / Ещё. Вкладка «Обсуждения» — список
  **предметов** с непрочитанным; внутри предмета — его тред (позже — треды занятий).
- Ничего не спрятано от студента: все материалы, расписание, задачи, обсуждения группы видны с
  первого входа; роли влияют только на действия.

### 3.4 Офлайн и синхронизация («как в Telegram»)

- **Чтение:** кеш TanStack Query persist — лента, материалы, расписание, задачи открываются
  офлайн.
- **Сообщения:** локальная SQLite: `messages`, `threads`, `outbox`. Отправка офлайн → запись в
  `outbox` с клиентским `client_id` (UUIDv7); при появлении сети — `POST /threads/:id/messages`
  с `client_id` (сервер идемпотентен по `(thread_id, client_id)`); ответ присваивает серверный
  `seq`. Загрузка файлов офлайн не поддерживается.
- **Дельта-синк:** `GET /groups/:id/sync?since=<seq>` возвращает события журнала группы после
  курсора. Клиент хранит `last_seq` на группу; после MVP тот же поток приходит по WebSocket
  (§7.3), `sync` остаётся для восстановления после разрыва.

### 3.5 Веб (W2-A): Vite + React

`apps/web`: Vite + React + `packages/ui` + `packages/core` + `packages/api-client`. Без SSR
(всё за логином). Web push через service worker + VAPID. Realtime — тот же WebSocket-протокол.

---

## 4. Бэкенд (Go)

Решение — [ADR-0002](adr/0002-go-backend.md), [ADR-0006](adr/0006-redis-jobs-and-pubsub.md).

### 4.1 Стек

| Слой | Выбор | Почему |
|---|---|---|
| Язык / рантайм | Go 1.23+, один статический бинарник `heatseeker` с подкомандами `api`, `worker`, `migrate`, `gen` | скорость, ~30–50 MB RAM, деплой на любой сервер (docker или systemd) |
| HTTP / OpenAPI | `net/http` + chi + huma v2 (code-first: Go-структуры → валидация + OpenAPI 3.1 + `/docs`) | минимум бойлерплейта; спека уходит в `packages/api-client` |
| БД | PostgreSQL 16+ + pgx/v5 + sqlc (типобезопасный SQL) + goose (миграции) | KISS и производительность; tsvector (`russian`) для поиска; pgvector — для ИИ-сервиса |
| Очереди / cron | Redis 7 + asynq (приоритеты, ретраи, отложенные задачи, планировщик, `asynqmon`) | фоновая работа не нагружает Postgres |
| Pub/sub для realtime | Redis Pub/Sub (fan-out между процессами) + in-process hub для dev | тот же Redis |
| Кеш горячих данных | Redis (presigned URL, quick-tags, счётчики) — по мере необходимости | |
| Auth | `golang-jwt/jwt/v5` (access 15 мин), opaque refresh-токены в БД (ротация), argon2id, `google.golang.org/api/idtoken` | |
| RBAC | собственный пакет `internal/authz` (матрица «роль → действия» + условия владения) | KISS |
| Медиа | интерфейс `MediaStore`: `s3` (minio-go/v7), `local`, `drive` (ссылка/прокси) | §5 |
| Google Drive | `google.golang.org/api/drive/v3` + сервисный аккаунт | §6 |
| Push | Expo Push HTTP API (тонкий клиент), `webpush-go` (VAPID), опционально Telegram | §8 |
| WebSocket | `coder/websocket` (+ SSE-фолбэк) | §7.3 |
| Экстракция текста | Strategy-набор: PDF → `pdftotext` (poppler CLI); docx/pptx/odt → zip+XML; xlsx → excelize; rtf → стриппер; txt/md/csv напрямую. Превью офисных → PDF через LibreOffice headless (опционально) | Go-экосистема офисных форматов слабая — CLI надёжнее |
| Антивирус | `clamd` по TCP (опционально), карантин до вердикта | §5.4 |
| Конфиг | `caarlos0/env` + валидация при старте; `.env` через godotenv только в dev | |
| Логи / ошибки / метрики | `log/slog` (JSON); Sentry (опц.); Prometheus `/metrics` (опц.) | |
| Тесты | `testing` + testcontainers-go (Postgres, Redis, MinIO); `httptest`; табличные тесты классификатора/экстракторов | |
| Линт | golangci-lint, gofmt, go vet в CI | |

Отклонено: gin/fiber (без преимуществ перед chi+huma); GORM/ent (ent — запасной вариант, если
sqlc окажется многословным); очередь на Postgres (грузит БД); oapi-codegen spec-first (можно
перейти позже, спека уже будет).

### 4.2 Раскладка кода

```
apps/api/
├── cmd/heatseeker/main.go          # подкоманды: api | worker | migrate | gen
├── internal/
│   ├── domain/                     # сущности, value objects, доменные ошибки, интерфейсы репозиториев
│   │   └── user/ group/ subject/ tag/ material/ task/ schedule/ thread/ proposal/ reminder/ notification/ drive/ event/
│   ├── app/                        # use cases, транзакции, публикация событий
│   ├── authz/                      # матрица прав, Can(actor, action, resource), генерация permissions для packages/shared
│   ├── transport/http/             # huma-роутеры, DTO, middleware (auth, group scope, rate limit), ws-hub
│   ├── jobs/                       # asynq-воркеры: drive.sync, media.ingest, media.scan, text.extract,
│   │                               #   notify.deliver, deadline.scan, reminder.fire, cache.evict
│   ├── adapters/                   # postgres (sqlc), redis, media/{s3,local,drive}, gdrive,
│   │                               #   push/{expo,webpush,telegram}, av/clamd, extract/{pdf,office,rtf,...}
│   ├── events/                     # журнал group_events: append + fan-out → activity, notifications, ws
│   └── platform/                   # config, logger, db pool, redis client, clock, ids (UUIDv7)
├── db/migrations/                  # goose SQL
├── db/queries/                     # sqlc SQL
├── openapi/                        # сгенерированная спека (коммитится) → packages/api-client
└── sqlc.yaml, Taskfile.yml, Dockerfile (multi-stage, distroless)
```

### 4.3 Где живут паттерны

| Точка расширения | Паттерн | Что даёт |
|---|---|---|
| `MediaStore` (s3 / local / drive) | Strategy + Factory (выбор по env) | смена хранилища без правок кода |
| Экстракторы текста, шаги классификации, каналы уведомлений, push-провайдеры | Strategy / Chain of Responsibility / Adapter | добавить формат/канал = новый файл |
| Журнал событий → activity / notifications / ws | Observer (in-process шина + `group_events`) | модули не знают друг о друге |
| Репозитории (`domain` интерфейсы, `adapters/postgres`) | Repository + Dependency Inversion | use cases тестируются без БД |
| Фоновые задачи | Command (asynq task = сериализованная команда) | ретраи, идемпотентность |
| Уведомления | Facade (`Notifier.Emit(event)`) | одна точка входа |
| Кеш медиа с TTL | Proxy/Decorator над `MediaStore` | прозрачное кеширование Drive → S3 |
| Классификатор файлов | Pipeline с confidence, чистые функции | табличные тесты |

Чего **не** делаем: generic-репозитории, абстрактные фабрики фабрик, слои ради слоёв. Одна
транзакция на use case, явный SQL, минимум интерфейсов вне точек расширения.

### 4.4 API (REST, `/api/v1`, OpenAPI 3.1)

| Модуль | Эндпоинты (эскиз) |
|---|---|
| auth | `POST /auth/register` `{name, invite_code?}` → токены; `POST /auth/login`; `/auth/refresh`, `/auth/logout`; `POST /auth/google` (id_token); `POST /me/credentials` («защитить аккаунт») |
| me | `GET/PATCH /me`, `/me/devices`, `/me/preferences`, `/me/mutes`, `/me/bookmarks`, `/me/reminders`, `/me/notifications` |
| groups | `POST /groups`, `GET /groups/:id`, `GET /groups/:id/members` (админ — расширенная инфа), `PATCH /groups/:id/members/:uid/roles`, `POST /groups/:id/invites`, `POST /groups/join/:code`, `GET /groups/:id/sync?since=` |
| subjects / tags | CRUD; `GET /groups/:id/quick-tags` |
| materials | `GET /groups/:id/materials?subject&tags&kind&q&cursor`, `GET /materials/:id`, `GET /materials/:id/open` → `{mode, drive_web_view_link?, stream_url?, s3_url?}`, `GET /materials/:id/stream` (прокси с Range), `POST /groups/:id/materials/uploads` (presigned / прямой при `local`), `POST …/complete`, `PATCH`, `/archive`, `/restore`, `DELETE`, `GET /groups/:id/materials/inbox`, `POST /materials/:id/classify` |
| tasks | CRUD (создаёт любой), `PATCH /tasks/:id/status`, `PATCH /tasks/:id/me/status`, `POST /tasks/:id/pin`, `GET /groups/:id/board` |
| schedule | `GET /groups/:id/schedule?from&to`, CRUD событий, exceptions, change-requests (этап 3), `schedule.ics` |
| threads | `GET /groups/:id/subjects/:sid/threads` (основная навигация — по предмету), `GET /groups/:id/threads?target=subject:<id>\|lesson:<occ>\|material:<id>\|task:<id>\|proposal:<id>\|general`, `GET /threads/:id/messages?before&after&include_hidden`, `POST /threads/:id/messages` `{client_id, body}`, `PATCH/DELETE /messages/:id` (своё), `POST/DELETE /messages/:id/hide` (для себя), `POST /messages/:id/moderate` (для всех), `POST /threads/:id/read` |
| proposals | CRUD, vote, status |
| announcements | `POST /groups/:id/announcements` |
| drive | `POST /groups/:id/drive/connection`, `GET …/status`, `POST …/sync`, `GET …/items`, `POST /groups/:id/drive/upload` (§6.3) |
| activity | `GET /groups/:id/activity` (проекция `group_events`) |
| ws | `GET /ws?group=` (W2-0) |
| internal | `/internal/ai/*` — только для ИИ-сервиса по сервисному токену |

### 4.5 Аутентификация: три уровня

Решение — [ADR-0004](adr/0004-light-auth-tiers.md).

| Уровень | Как получается | Что можно |
|---|---|---|
| **L1 — лёгкий аккаунт** (по умолчанию) | только имя; сессия привязана к устройству (refresh-токен в secure store, ротация) | читать всё в группе, писать в обсуждениях, создавать задачи/предложения, напоминания, загружать в S3/local, закладки |
| **L2 — защищённый аккаунт** | пароль и/или email и/или Google | + вход с другого устройства / восстановление; **обязателен** для ADMIN / MODERATOR / HEADMAN (middleware `RequireSecured`) |
| **L2g — Google подключён** | Google Sign-In (id_token) → `auth_identities` | + загрузка на Google Диск из приложения (§6.3) |
| L0 — публичное чтение | `groups.public_read=true` + ссылка | только чтение без аккаунта; **выключено по умолчанию**, оставлено переключателем |

Email опционален, уникален, если задан. Вступление: `groups.join_policy = OPEN` (по ссылке,
MVP) `| INVITE | APPROVAL` — фундамент под закрытую схему; `APPROVAL` — не в первой версии (D28),
модель и эндпоинты к нему готовы.

### 4.6 RBAC: мультироли

`memberships.roles` — массив `OWNER, ADMIN, MODERATOR, HEADMAN, STUDENT, GUEST`; права =
объединение. Матрица (источник — Go, генерируется в `packages/shared`):

| Действие | STUDENT | HEADMAN | MODERATOR | ADMIN/OWNER | GUEST |
|---|:-:|:-:|:-:|:-:|:-:|
| Читать всё в группе | ✓ | ✓ | ✓ | ✓ | ✓ |
| Писать в обсуждениях, скрывать сообщения для себя | ✓ | ✓ | ✓ | ✓ | – |
| Загружать материалы; править/архивировать свои; удалять свои до первого скачивания | ✓ | ✓ | ✓ | ✓ | – |
| Архивировать/удалять/объединять любые материалы; разбирать Inbox | – | – | ✓ | ✓ | – |
| Предметы и теги | – | ✓ | ✓ | ✓ | – |
| Создавать задачи (в т.ч. «от преподавателя») | ✓ | ✓ | ✓ | ✓ | – |
| Закреплять задачи, менять общий статус групповой задачи | – | ✓ | ✓ | ✓ | – |
| Свой статус по задаче, напоминания, закладки | ✓ | ✓ | ✓ | ✓ | ✓ (кроме статуса) |
| Предлагать изменения расписания | ✓ | ✓ | ✓ | ✓ | – |
| Редактировать расписание, утверждать запросы | – | ✓ | – | ✓ | – |
| Скрывать сообщения для всех, модерировать предложения | – | – | ✓ | ✓ | – |
| Объявления (broadcast) | – | ✓ | – | ✓ | – |
| Расширенная инфа о пользователях (кто что писал, устройства, активность) | – | – | ✓ (только авторство) | ✓ | – |
| Участники, роли, инвайты, политика вступления | – | инвайты | – | ✓ | – |
| Подключать Drive, запускать синк, настройки медиа | – | ✓ | – | ✓ | – |
| Настройки/архив группы | – | – | – | ✓ (архив — OWNER) | – |

Авторство сообщений видно всем участникам; админ дополнительно видит профиль/устройства/историю.

---

## 5. Медиа: режимы и хранилище

Решение — [ADR-0003](adr/0003-media-two-modes.md).

### 5.1 Режимы (`groups.media_mode`, дефолт из `MEDIA_MODE_DEFAULT`)

| Режим | Где лежит файл | Как смотрит студент | Ресурсы |
|---|---|---|---|
| **LINK** («просмотр с Диска») | только на Google Диске; у нас — метаданные, теги, классификация | 1) `web_view_link` — открыть в Google Drive (нужен доступ к папке); 2) `GET /materials/:id/stream` — прокси через сервисный аккаунт с `Range` (`MEDIA_PROXY_ENABLED`) | ~0 диска |
| **CACHE** (по умолчанию при наличии MinIO) | Диск — источник; при первом открытии (или заранее для файлов ≤ `MEDIA_CACHE_PREFETCH_MAX_MB`) копия кладётся в S3 с `cache_expires_at`; TTL продлевается при обращении; вытеснение по TTL и `MEDIA_CACHE_MAX_GB` (LRU) | presigned GET из S3; при промахе — как LINK | диск под кеш |
| **IMPORT** | постоянная копия в S3, Диск — только источник обновлений | presigned GET | диск под всё |

Переключается без правок кода (Proxy/Decorator над `MediaStore`). TTL и лимиты настраиваемые —
с расчётом, что кеш позже станет постоянным хранилищем. Загрузка из приложения всегда идёт в
наше хранилище (S3 или `local`), опционально — ещё и на Диск (§6.3).

### 5.2 Хранилище: MinIO

minio-go/v7, path-style, presigned PUT/GET, multipart для больших файлов, один бакет на
окружение с префиксами:

```
groups/{group_id}/materials/{material_id}/v{n}/{sha256-8}-{safe-name}   # IMPORT и загрузки
groups/{group_id}/cache/{version_id}/{safe-name}                          # CACHE
groups/{group_id}/previews/{version_id}/{page}.webp
groups/{group_id}/text/{version_id}.txt
users/{user_id}/avatar.webp
tmp/uploads/{upload_id}/{name}                                            # TMP_UPLOAD_TTL_HOURS
```

`local`-провайдер (файловая система сервера) — для dev без MinIO и для «дали сервер без
ничего»; тот же интерфейс, отдача через API с `Range`.

### 5.3 Конвейер приёма файла (worker, asynq)

`upload.complete` → mime по magic bytes + allowlist + размер + zip-bomb guard → sha256 → дедуп
(предупреждение модератору) → скан (§5.4) → превью (изображения; PDF — `pdftoppm`; офисные —
через LibreOffice → PDF, если включено) → извлечение текста (pdf, docx, pptx, xlsx, rtf, odt,
txt, md, csv) → индекс поиска → `group_events: material.added` → уведомления (батчем).

### 5.4 Безопасность файлов

- Обязательно без внешних сервисов: allowlist расширений/mime, sniff по magic bytes, лимит
  размера, запрет исполняемых и вложенных архивов глубже 1 уровня, безопасные имена,
  `Content-Disposition: attachment` для всего, кроме изображений/PDF, presigned URL с коротким TTL.
- Антивирус: ClamAV (`clamd`) как опциональный контейнер (`AV_ENABLED`, ~1.2 GB RAM); до
  вердикта версия в `scan_status=PENDING` и недоступна; `INFECTED` → карантин + уведомление
  модератору; без ресурсов — `SKIPPED` с пометкой в UI.
- Файлы в режиме LINK не сканируются (лежат на Google) — UI показывает «источник: Google Диск».
- Секреты в БД (refresh-токены Google и т.п.) — AES-GCM с `APP_ENCRYPTION_KEY`.

---

## 6. Google Drive

### 6.1 Доступ: сервисный аккаунт

Владелец общей папки выдаёт e-mail'у сервисного аккаунта доступ **Редактор** (решено, D29):
чтение для индексации и режимов LINK/CACHE/IMPORT плюс возможность публиковать файлы от имени
сервиса (§6.3). Все остальные пользователи работают с Диском через приложение только на чтение.
Никаких OAuth-экранов для студентов, верификации приложения Google не нужно, токены не протухают.

### 6.2 Индексация и актуализация (worker)

1. Первичный скан `files.list` рекурсивно от `root_folder_id` (id, name, mimeType,
   md5Checksum, size, modifiedTime, parents, description, webViewLink) → `drive_items`.
2. Инкрементально `changes.list` по `changes_page_token` каждые `GDRIVE_SYNC_INTERVAL_SEC`
   (10 мин), полный рескан раз в сутки. Ручной запуск `POST …/sync`.
3. Новый/изменённый файл → `materials` + `material_versions(storage=DRIVE)` (для LINK этого
   достаточно) → классификация (§6.4) → `material.added` → уведомления батчем. В режимах
   CACHE/IMPORT — задача `media.ingest` (скачивание / экспорт Google Docs → PDF).
4. Удалено/перемещено на Диске → `needs_review`, автоматически не удаляем
   (`GDRIVE_DELETE_POLICY=flag|archive`).
5. Ретраи, квоты (12 000 запросов/мин на проект), лог в `group_events`.

Одногруппники продолжают выкладывать на Диск нативно — приложение актуализирует индекс и даёт
смотреть с любого места.

### 6.3 Загрузка на Диск из приложения (`FEATURE_DRIVE_UPLOAD`)

Путь по умолчанию — загрузка в наше хранилище (§5). Дополнительно:

| Способ | Требования | Ограничения |
|---|---|---|
| **A. Через сервисный аккаунт** (рекомендуется): файл → наш API → SA создаёт файл в папке с `description="Загрузил: <имя>"` | SA — Редактор папки (уже есть, D29) | файлы принадлежат SA → его квота 15 GB (в Shared Drive — квота организации) |
| **B. От имени студента** (Google Sign-In + Drive scope) | Google подключён | `drive.file` не пишет в чужую папку без Picker; полный `drive` — restricted scope: в статусе Testing refresh-токен живёт 7 дней, для прода нужна верификация Google |

Google Sign-In нужен только тем, кто выберет способ B. Когда включать —
[`OPEN-QUESTIONS.md`](OPEN-QUESTIONS.md) №1.

### 6.4 Классификация по предметам

Конвейер с уверенностью (чистые функции, табличные тесты на реальных именах файлов вида
`Tema3_Lektsia2_….pdf`, `Praktika_tema_1.pdf` — с непоследовательной нумерацией):

| Шаг | Сигнал | Вклад |
|---|---|---|
| Путь на Диске | папка ≈ предмет/алиас (нормализация, fuzzy по токенам + Левенштейн) | 0.9 |
| Имя файла | токены против `name/short_name/aliases`, фамилия преподавателя; регэкспы типа (`лекц\|лк` → LECTURE, `лаб\|практ\|пз` → ASSIGNMENT, `доклад\|презент` → REPORT, `расч\|рпз\|курсов` → CALC); «тема N», «семестр N» | 0.5–0.8 |
| Метаданные | `description`, владелец файла ↔ предмет, mimeType → kind | +0.1–0.3 |
| Содержимое | первые ~2 000 символов текста против алиасов/ключевых слов | +0.2 |
| ИИ-сервис (W2-B) | structured output `{subject_id, kind, tags, confidence}` от локальной модели | уточняет |

`confidence < GDRIVE_CLASSIFY_MIN_CONFIDENCE` (0.6) → Inbox «Неразобранное» с предложением;
подтверждение одним тапом; правки пополняют `subjects.aliases`.

---

## 7. Обсуждения, скрытие, синхронизация, realtime

### 7.1 Обсуждения (threads)

- **Основная единица навигации — предмет**: у каждого предмета свой тред (`SUBJECT`, создаётся
  вместе с предметом). Треды по **занятию** (`LESSON`: событие расписания + дата) появляются с
  этапа 3 и показываются внутри экрана предмета как под-треды. Та же модель — для
  материала/задачи/предложения (заменяет «комментарии») и общего треда группы (`GENERAL`).
- Пишет любой участник (кроме GUEST); markdown; ответы (`reply_to`); правка/удаление своего.
- **Скрытие для себя**: `POST /messages/:id/hide` → сообщение свёрнуто в «скрыто (1)»;
  переключатель «показать скрытые» (`include_hidden=1`) показывает их **в общем контексте с
  видимыми**, помеченными. Отдельный экран «Скрытые мной» — список для быстрого возврата.
- **Скрытие для всех** (`hidden_for_all_by`) — модератор/админ; автор видит пометку; админ
  видит всё и авторство.
- Непрочитанное: `thread_reads.last_read_seq`; бейджи на чипах сверху.

### 7.2 Офлайн-синхронизация

Серверная сторона к §3.4: идемпотентность по `client_id`, `seq` из `group_events`, эндпоинт
`sync?since=`, ретеншен журнала; при `since` старше ретеншена — `410` и полная перезагрузка кеша.

### 7.3 Realtime (W2-0, сразу после MVP — обязательно)

`GET /ws?group=<id>` (`coder/websocket`), auth по access-токену в первом сообщении; сервер шлёт
события `group_events` с `seq` (те же payload'ы, что в `sync`); клиент применяет их к локальной
БД/кешу; heartbeat; при разрыве — `sync?since=last_seq`. Fan-out между процессами — Redis
Pub/Sub (`WS_FANOUT=redis|inproc`). SSE-фолбэк для веба за проксями без WS. Сценарий: пришёл
пуш «сообщение от одногруппника» → открыл тред — сообщение уже на экране.

---

## 8. Уведомления

### 8.1 Каналы

In-app (MVP); Expo push → FCM/APNs (MVP; APK — FCM); Web push (с вебом); Telegram-бот (W2-D,
рекомендуется); Email (опционально, только у кого есть email). `PUSH_PROVIDER=expo|none`,
интерфейс `PushProvider` (Adapter).

### 8.2 Механизм

`group_events` → `Notifier` → получатели (участники − автор, минус **mutes**, минус
preferences, минус тихие часы) → `notifications(dedupe_key)` → asynq-задачи доставки по каналам
→ `notification_deliveries`.

- Типы: `MATERIAL_ADDED`, `MATERIAL_BATCH` (окно `NOTIFY_MATERIAL_BATCH_WINDOW_SEC`),
  `MESSAGE_NEW`, `MESSAGE_REPLY`, `TASK_CREATED`, `TASK_PINNED`, `TASK_DUE_SOON`,
  `TASK_STATUS_CHANGED`, `SCHEDULE_CHANGED`, `PROPOSAL_*`, `ANNOUNCEMENT`, `REMINDER`,
  `MEMBER_JOINED`, `MODERATION` (модератору: инфицированный файл, Inbox).
- **Mute по теме на время** (`notification_mutes`): «предмет X на 3 дня», «сообщения на 8 часов»,
  «занятия до конца недели», «вся группа до завтра» — из настроек и из контекстного меню
  треда/предмета; срок — обязательное поле (пресеты 1ч/8ч/1д/1нед/до даты).
- **ДПО**: для `groups.kind=DPO` дефолтные преференции — всё выключено, кроме `ANNOUNCEMENT`.
- Дедлайны и напоминания — сканер раз в минуту (asynq scheduler), offsets из преференций
  (`7d,3d,1d,3h`), не шлём, если своя часть DONE.
- Кастомные напоминания (`reminders`) — текст/время/повтор, опциональная привязка; кейс
  старосты: «скинуть материалы преподавателю в четверг» + кнопки «Отложить / Готово».
  Объявления (`announcements`) — broadcast от старосты/админа, `urgent` игнорирует тихие часы.

---

## 9. Конфигурация через env

Одна точка — env; валидация при старте, fail-fast; полный список с описаниями — в
[`.env.example`](../.env.example). Группы: App · Database/Redis/jobs · Auth · Media ·
Google Drive · Notifications · Realtime · AI service link · Feature flags · Mobile
(`EXPO_PUBLIC_*`). ИИ-сервис имеет свой `.env` — ядро о моделях ничего не знает.

---

## 10. ИИ-сервис и markdown

### 10.1 ИИ-сервис (W2-B)

- `apps/ai` — отдельный процесс, общается с ядром только через публичный REST API и
  `/internal/ai/*` по сервисному токену; ядро ставит задания через `AI_SERVICE_URL`, результаты
  приходят колбэком. Падение ИИ-сервиса не влияет на ядро.
- Язык — **Python (FastAPI)** (решено, D30): экосистема локальных моделей (ollama/vLLM/llama.cpp
  через OpenAI-совместимый API, `sentence-transformers`, `faster-whisper` для возможной
  транскрибации записей лекций).
- Провайдер — `AI_PROVIDER=openai-compatible` (локальная лёгкая модель), опционально
  `anthropic` (официальный SDK). Structured output для классификатора; бюджеты на пользователя;
  согласие группы на обработку.
- Агенты: суммаризатор материала; структуризатор «приложить наработки к вопросу»; Q&A по
  предмету (RAG на pgvector в схеме `ai.*` + tsvector-гибрид); классификатор Drive-файлов;
  извлечение дедлайнов из текста задания → черновик задачи.
- Результаты — markdown-артефакты, привязанные к материалу/предмету/задаче.

### 10.2 Markdown — везде

Все текстовые тела (сообщения, задачи, предложения, объявления, описания материалов, артефакты
ИИ) — markdown; рендер `react-native-markdown-display` / `react-markdown` с санитизацией (без
raw HTML); редактор с панелью форматирования и превью — с этапа 2 (сообщения), полноценный —
W2-C.

---

## 11. Roadmap

Разработчик — в основном автор + ИИ-агент; Go — новый язык, этап 0 включает освоение. Оценки
ориентировочные.

| Этап | Содержание | Готово, когда | ~Срок |
|---|---|---|---|
| **0. Фундамент** | закрыть открытые вопросы этапа 0; `apps/api`: каркас Go (chi+huma, sqlc, goose, asynq, config, slog), миграции ядра (users, groups, memberships, invites, subjects, tags, group_events), auth L1/L2 + Google, authz + генерация `packages/shared`, OpenAPI → `packages/api-client`; compose dev (Postgres, Redis, MinIO, asynqmon); CI (go vet/lint/test, tsc/eslint); `apps/mobile`: Expo + Router + Tamagui (`packages/ui`) + i18n; экраны: имя → группа → главный с чипами | зарегистрировался по имени, вошёл в группу по ссылке, видишь пустую ленту с тегами | 3 нед |
| **1. Материалы с Диска + теги** | drive_connections + индексация SA, материалы в режиме **LINK** (открыть в Drive / прокси-стрим), предметы/теги/quick-tags, классификация шаги 1–3 + Inbox, поиск по названиям, загрузка из приложения в S3/local (presigned), архив/удаление модератором, лента активности | всё из общей папки видно по предметам; новые файлы подхватываются ≤10 мин | 3 нед |
| **2. Обсуждения + уведомления + задачи** | threads/messages по предметам (+ общий тред), экран «Обсуждения» = список предметов, личное и модераторское скрытие, «показать скрытые», непрочитанное; офлайн outbox + sync; уведомления: in-app + Expo push, preferences, mutes по теме на время, тихие часы, `MATERIAL_ADDED/BATCH`, `MESSAGE_NEW`; задачи (создаёт любой, закрепление), доска, дедлайн-сканер; напоминания; объявления | пуш о новом файле/сообщении/дедлайне; сообщение, написанное офлайн, доезжает само | 4 нед |
| **3. Расписание** | события + RRULE + исключения, неделя/день, редактирование старостой/админом с логом, треды по занятию как под-треды внутри предмета, уведомления об изменениях, ICS; запросы на изменение/утверждение — второй половиной этапа | группа ведёт расписание в приложении, обсуждает занятие в его треде | 2–3 нед |
| **4. Предложения + ДПО** | proposals/votes (обсуждение — тредом), модерация; группа `kind=DPO`, переключение групп, дефолт «уведомления выкл.», per-group mutes/preferences | поток ДПО живёт изолированно в той же системе | 1–2 нед |
| **5. Медиа-режимы CACHE/IMPORT + обработка** | `media.ingest` (Drive → S3 с TTL/лимитами), presigned GET, экспорт Google Docs → PDF, превью, извлечение текста (pdf/docx/pptx/xlsx/rtf/odt/txt/md/csv), полнотекстовый поиск, классификация шаг 4, ClamAV опц., дедуп по sha256 | лекция открывается из S3 мгновенно, поиск по содержимому работает | 2–3 нед |
| **6. Стабилизация, релиз MVP** | интеграционные тесты ключевых сценариев, Sentry, бэкапы (pg_dump + mc mirror), compose prod + Caddy (или systemd-вариант), EAS Build → APK, онбординг, политика конфиденциальности | группа пользуется ежедневно | 1–2 нед |
| **MVP итого** | | | **~16–20 нед** |
| **W2-0. Realtime** (обязательно, сразу после MVP) | WebSocket-хаб, Redis fan-out, SSE-фолбэк, клиентское применение событий | сообщение одногруппника появляется мгновенно | 1–2 нед |
| **W2-A. Веб** | `apps/web` Vite + React + `packages/ui`, web push, доска/расписание на большом экране | | 3–4 нед |
| **W2-B. ИИ-сервис** | `apps/ai`, openai-compatible локальная модель, классификатор → суммаризатор → RAG Q&A → дедлайны | | 4–6 нед |
| **W2-C. Markdown-редактор** | полноценный редактор/превью mobile+web | | 1 нед |
| **W2-D. По запросу** | Telegram-канал, per-user Drive OAuth, импорт расписания вуза (xlsx/ics), транскрибация лекций в ИИ-сервисе | | — |
