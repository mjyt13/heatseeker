# Heatseeker

Платформа для учебной группы: единое хранилище материалов (с автоматической раскладкой файлов с
Google Диска по предметам), дедлайны и задачи, общее расписание, обсуждения по предметам,
предложения группы, уведомления и напоминания. Mobile-first (Android), веб — вторым этапом.

## Статус

Этап 0 (фундамент) и этап 1 (материалы с Google Диска, загрузки, теги) реализованы
(2026-09-16). См.:

- [`docs/PLAN.md`](docs/PLAN.md) — план разработки и архитектура
- [`docs/DATA-MODEL.md`](docs/DATA-MODEL.md) — модель данных
- [`docs/DECISIONS.md`](docs/DECISIONS.md) — принятые решения
- [`docs/OPEN-QUESTIONS.md`](docs/OPEN-QUESTIONS.md) — открытые вопросы
- [`docs/adr/`](docs/adr/) — архитектурные решения (ADR)
- [`CLAUDE.md`](CLAUDE.md) — контекст и конвенции для работы с ИИ-агентом

## Стек

Go (chi + huma, sqlc, asynq) · PostgreSQL · Redis · MinIO · Expo + Tamagui · Vite + React ·
отдельный ИИ-сервис с локальной моделью.

## Структура

```
apps/      api (Go), mobile (Expo), web (Vite), ai (ИИ-сервис)
packages/  ui, shared, api-client, core, i18n, config
infra/     compose, scripts
docs/      план, модель данных, решения, ADR
```

## Запуск (локальная разработка)

Нужны: Node 24 + pnpm 10, podman (или docker) с compose, Go-тулчейн в user-space
(`/run/media/deck/EE4S8/go-toolchain`: go, sqlc, task, golangci-lint).

### Один раз

```bash
source /run/media/deck/EE4S8/go-toolchain/env.sh   # в каждом новом терминале (или добавить в ~/.bashrc)
pnpm install
cp .env.example apps/api/.env                      # заполнить секреты: openssl rand -base64 48
```

Google Диск (необязательно) — [`docs/GOOGLE-DRIVE.md`](docs/GOOGLE-DRIVE.md): ключ сервисного
аккаунта в `GDRIVE_SERVICE_ACCOUNT_JSON_BASE64` и **включённый Google Drive API** в облачном проекте.

### Каждый день

После перезагрузки контейнеры сами не поднимаются. Все команды — из корня репозитория, в каждом
терминале сначала `source /run/media/deck/EE4S8/go-toolchain/env.sh`.

```bash
# терминал 1 — инфраструктура и миграции
task dev:up              # Postgres :5433, Redis :6379, MinIO :9100 (консоль :9101), asynqmon :8082
task dev:ps              # проверить, что все четыре контейнера Up
task api:migrate         # применить новые миграции (без изменений — ничего не делает)
mkdir -p .logs           # логи для отладки (*.log в .gitignore)

# терминал 2 — HTTP API на :8000
task api:run:api 2>&1 | tee .logs/api.log

# терминал 3 — воркер (синхронизация Диска, хеши, очистка); без него Диск не индексируется
task api:run:worker 2>&1 | tee .logs/worker.log

# терминал 4 — Expo (Metro) на :4173. Через `script`, а не `| tee`: Expo нужен настоящий
# терминал, иначе он не покажет QR и не примет клавиши (w, r, …)
script -q -f -c "pnpm -F mobile start --clear" .logs/mobile.log
```

| Что | Адрес |
|---|---|
| Приложение в браузере | в терминале Expo нажать `w` → <http://localhost:4173> |
| Swagger API | <http://localhost:8000/api/v1/docs> |
| Проверка API | <http://localhost:8000/healthz> |
| Очереди (asynqmon) | <http://localhost:8082> |
| MinIO (консоль) | <http://localhost:9101> (по умолчанию `heatseeker` / `heatseeker-dev-secret`) |

Остановить: `Ctrl+C` в терминалах 2–4, затем `task dev:down` (данные сохраняются;
`task dev:reset` — стереть всё).

### На телефоне (Expo Go)

Телефон и компьютер — в одной Wi-Fi-сети. `<IP>` — адрес компьютера (`ip -4 addr`, например
`192.168.0.104`); Expo показывает его в `exp://<IP>:4173`.

1. `apps/mobile/.env.local` (не коммитится; Expo читает его сам):
   ```
   EXPO_PUBLIC_API_URL=http://<IP>:8000/api/v1
   ```
2. Чтобы с телефона открывались и загружались файлы — открыть MinIO в сеть и отдавать ссылки с IP:
   ```
   # infra/compose/.env (не коммитится), затем task dev:up
   MINIO_BIND=0.0.0.0

   # apps/api/.env
   APP_BASE_URL=http://<IP>:8000
   S3_PUBLIC_ENDPOINT=http://<IP>:9100
   CORS_ORIGINS=http://localhost:5173,http://localhost:4173,http://<IP>:4173
   ```
   `APP_BASE_URL` нужен и для файлов с Диска: они открываются через API. `CORS_ORIGINS` — только
   если открывать веб по `http://<IP>:4173`. Без MinIO: `STORAGE_DRIVER=local` +
   `STORAGE_LOCAL_ROOT=./.media` (файлы отдаёт сам API).
3. Перезапустить API и воркер, Expo — с `--clear` (значения `EXPO_PUBLIC_*` вшиваются в бандл).
   Без `.env.local` веб, открытый по `http://<IP>:4173`, тоже ходит на `localhost:8000`, а телефон
   до сервера не достучится вовсе.
4. Открыть в Expo Go `exp://<IP>:4173` (или QR из терминала).
5. Проверка сети: на телефоне в браузере открыть `http://<IP>:8000/healthz` — должен ответить `{"status":"ok",…}`.
   Нет ответа — другая сеть, «изоляция клиентов» на роутере или файрвол на компьютере.

IP меняется при переподключении к Wi-Fi — тогда поправить `apps/mobile/.env.local` и `apps/api/.env`.

### Логи

- API и воркер — `.logs/api.log`, `.logs/worker.log` (каждый запрос: метод, путь, статус, время; `/healthz` не пишется).
- Приложение — `console.log` и ошибки JS с телефона и браузера выводит Metro, то есть
  `.logs/mobile.log`. Ошибки в браузере — ещё и DevTools (F12 → Console / Network).
- Android целиком — `adb logcat` по USB (обычно не нужно).

### Если что-то не так

| Симптом | Что проверить |
|---|---|
| `task: Failed to parse …` / `task: command not found` | не выполнен `source …/go-toolchain/env.sh` или ошибка в Taskfile |
| API падает при старте с ошибкой БД/Redis | `task dev:ps` — контейнеры не подняты; `task dev:up` |
| `relation … does not exist` | `task api:migrate` |
| Expo не показывает QR | вывод Expo перенаправлен (`\| tee`) — запускать через `script`, как выше |
| CORS в браузере | страница открыта не с `localhost` → добавить адрес в `CORS_ORIGINS`; при остановленном API браузер тоже пишет «CORS» — проверить `/healthz` |
| Файл на телефоне открывается как `localhost` | `APP_BASE_URL` / `S3_PUBLIC_ENDPOINT` не на IP, MinIO без `MINIO_BIND=0.0.0.0` |
| Телефон не заходит | `EXPO_PUBLIC_API_URL` в `apps/mobile/.env.local`, Expo с `--clear`, `healthz` с телефона |
| «Папка не открыта для сервисного аккаунта» | папка не расшарена на `client_email` **или** не включён Google Drive API (см. [`docs/GOOGLE-DRIVE.md`](docs/GOOGLE-DRIVE.md)) |
| Диск подключён, файлов нет | не запущен воркер; очереди — в asynqmon |
| Порт занят | `ss -ltnp \| grep <порт>`; API — `APP_PORT`, Expo — `--port` в `apps/mobile/package.json` |

Известные недочёты, найденные при ручной проверке, — [`docs/FIXES.md`](docs/FIXES.md).
Все команды (тесты, линтеры, генерация) — `CLAUDE.md`, раздел «Команды».
