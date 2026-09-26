# Heatseeker

Платформа для учебной группы: единое хранилище материалов (с автоматической раскладкой файлов с
Google Диска по предметам), дедлайны и задачи, общее расписание, обсуждения по предметам,
предложения группы, уведомления и напоминания. Mobile-first (Android), веб — вторым этапом.

## Статус

Этап 0 (фундамент) и этап 1 (материалы с Google Диска, загрузки, теги) реализованы и проверены
вручную (2026-09-17). Этап 2 закрыт: задачи и дедлайны (2026-09-20), обсуждения (2026-09-21),
уведомления (2026-09-23), напоминания и объявления (2026-09-24). Этап 3 (расписание) начат
2026-09-22. См.:

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
Публикация загрузок в «Мой диск» — через аккаунт Google старосты (OAuth-клиент, там же). Пока
OAuth-приложение в статусе **Testing**, Google выдаёт доступ на **7 дней**: потом публикация падает с
«Google отозвал доступ», и аккаунт нужно подключить заново (Ещё → Google Диск). Чтобы не
переподключать — Google Cloud → **Google Auth Platform → Audience → Publish app** (проверка Google не
нужна, до 100 пользователей; при подключении будет «приложение не проверено» → «Дополнительно →
Перейти») и один раз переподключить аккаунт. Читает Диск всегда сервисный аккаунт, аккаунт старосты —
только для загрузки.

### Каждый день

После перезагрузки контейнеры сами не поднимаются. Все команды — из корня репозитория, в каждом
терминале сначала `source /run/media/deck/EE4S8/go-toolchain/env.sh`.

```bash
# терминал 1 — инфраструктура и миграции
task dev:up              # Postgres :5433, Redis :6379, MinIO :9100 (консоль :9101), asynqmon :8082, Gotenberg :3030
task dev:ps              # проверить, что все контейнеры Up
task dev:ip              # адрес компьютера в Wi-Fi → конфиги API, воркера и Expo (см. «Телефон» ниже);
                         # запускать до API/воркера/Expo — они читают адрес только при старте
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
| Превью офисных файлов (Gotenberg) | <http://localhost:3030/health> |
| MinIO (консоль) | <http://localhost:9101> (по умолчанию `heatseeker` / `heatseeker-dev-secret`) |

Остановить: `Ctrl+C` в терминалах 2–4, затем `task dev:down` (данные сохраняются;
`task dev:reset` — стереть всё).

### На телефоне (Expo Go)

Телефон и компьютер — в одной Wi-Fi-сети.

1. Один раз — открыть MinIO в сеть, чтобы с телефона открывались и загружались файлы:
   `MINIO_BIND=0.0.0.0` в `infra/compose/.env` (не коммитится), затем `task dev:up`. Без MinIO:
   `STORAGE_DRIVER=local` + `STORAGE_LOCAL_ROOT=./.media` в `apps/api/.env` (файлы отдаёт сам API).
2. **`task dev:ip`** — после каждой смены сети. Находит адрес компьютера в Wi-Fi/LAN (VPN-туннели,
   контейнеры и мосты пропускает; выбрать вручную — `task dev:ip -- 192.168.x.y`) и прописывает его:
   - `apps/api/.env` — `APP_BASE_URL` (ссылки на файлы с Диска идут через API) и
     `S3_PUBLIC_ENDPOINT` (ссылки на MinIO), IP в `CORS_ORIGINS`;
   - `apps/mobile/.env.local` — `REACT_NATIVE_PACKAGER_HOSTNAME` (адрес, который Expo показывает в
     `exp://…`; без него при включённом VPN Expo может выбрать адрес туннеля) и хост
     `EXPO_PUBLIC_API_URL`, если он задан.
3. Перезапустить API и воркер, Expo — с `--clear`.
4. Открыть в Expo Go `exp://<IP>:4173` (или QR из терминала).
5. Проверка сети: на телефоне в браузере открыть `http://<IP>:8000/healthz` — должен ответить `{"status":"ok",…}`.
   Нет ответа — другая сеть, «изоляция клиентов» на роутере или файрвол на компьютере.

Адрес API приложение берёт само: на телефоне — IP из `exp://<IP>:4173`, в вебе — хост страницы,
порт `8000`; текущий адрес виден внизу первого экрана («API (dev): …»). `EXPO_PUBLIC_API_URL` в
`apps/mobile/.env.local` нужен только для особых случаев (другой порт, туннель). CORS в dev пропускает
`localhost` и адреса локальной сети на любом порту.

### Уведомления

Уведомления в приложении (экран «Уведомления», колокольчик на главной и раздел в «Ещё») работают
всегда: их пишет воркер, разбирая журнал событий группы, поэтому **воркер должен быть запущен**
(`task api:run:worker`). Он же раз в минуту проверяет личные напоминания («Ещё» → «Напоминания»)
и рассылает объявления старосты («Ещё» → «Объявления»). Что присылать — на экране настроек уведомлений, там же тихие часы и
список приглушённого; приглушить предмет или обсуждение можно из самого обсуждения.

Push на телефон (`PUSH_PROVIDER=expo`) требует отдельной сборки: **в Expo Go на Android пуш не
приходит** (SDK 53+), токен не выдаётся и приложение молча остаётся на уведомлениях в списке.
Для пуша нужны EAS-сборка (dev build или APK) и `projectId` проекта Expo в `app.json`;
`EXPO_ACCESS_TOKEN` — только если в проекте Expo включена «enhanced security».

### Проверить уведомления

**Без сборки (работает сразу).** Нужны запущенные API и **воркер** — уведомления пишет он.
Проверять удобно с двух аккаунтов (телефон и веб):

1. Кто-то пишет в обсуждение предмета или общий чат → у остальных через ~12 с появляется
   уведомление (колокольчик с числом в заголовке любой вкладки, список — экран «Уведомления»).
2. Староста публикует объявление («Ещё» → «Объявления») → приходит всем, кроме автора.
3. Личное напоминание («Ещё» → «Напоминания») на ближайшую минуту → приходит только владельцу.
4. Задача со сроком → `task.due_soon` по расписанию из `DEADLINE_REMINDER_OFFSETS`.

Что смотреть, если не пришло: `.logs/worker.log` — строки `notification fan-out scheduled` и
`notifications written group=… count=N`; очереди — asynqmon на http://localhost:8082. Счётчик
непрочитанных в приложении обновляется по опросу (раз в 60 с), на экране «Уведомления» — сразу
при открытии; realtime (WS) — этап W2-0.

**Push на телефон требует своей сборки.** В Expo Go на Android пуша нет с SDK 53, токен не
выдаётся (в логах — «Use a development build instead of Expo Go»), поэтому на закрытом приложении
ничего не приходит. Порядок один раз:

1. Аккаунт Expo (бесплатного плана хватает, сборки идут в очереди) и вход: из `apps/mobile` —
   `pnpm exec eas login`, затем `pnpm exec eas init` (создаёт проект и прописывает
   `extra.eas.projectId` в `app.json` — без него токен не выдаётся).
2. Проект Firebase для доставки на Android (можно взять существующий проект Google Cloud — тот
   же, что у сервисного аккаунта Диска). Нужны **два разных файла**:
   - `google-services.json` — конфиг приложения. Появляется только после регистрации
     Android-приложения: Firebase Console → Project settings → General → Your apps → Add app →
     Android, имя пакета **`app.heatseeker.mobile`** (ровно как в `app.json`), SHA-1 для пуша не
     нужен → скачать файл и положить в `apps/mobile/`. В репозиторий он не попадает
     (`.gitignore`), а EAS Build берёт только то, что видит git, — поэтому файл отдаётся сборке
     переменной окружения (делается один раз, из `apps/mobile`):
     `pnpm exec eas env:set --name GOOGLE_SERVICES_JSON --type file --value ./google-services.json --visibility secret --environment development --environment preview --environment production`.
     В `app.json` уже стоит `"googleServicesFile": "$GOOGLE_SERVICES_JSON"`; проверить —
     `pnpm exec eas env:list --environment development`;
   - ключ сервисного аккаунта — JSON с приватным ключом от аккаунта
     `firebase-adminsdk-…@<проект>.iam.gserviceaccount.com` (Firebase Console → Project settings →
     Service accounts → **Generate new private key**; тот же файл получается в Google Cloud
     Console → IAM → Service accounts для этого аккаунта). Это секрет, в репозиторий не кладём.
3. Загрузить ключ в EAS: `pnpm exec eas credentials` → Android → выбрать профиль →
   Google Service Account → **FCM V1** → загрузить JSON (или на expo.dev: Project settings →
   Credentials → FCM V1 service account key).
4. Собрать и поставить на телефон: `task mobile:build:dev` (APK с dev-клиентом, подключается к
   Metro как Expo Go — удобно для разработки) или `task mobile:build:apk` (обычный APK, работает
   без Metro; перед сборкой добавить в профиль `preview` в `apps/mobile/eas.json` адрес
   сервера: `"env": { "EXPO_PUBLIC_API_URL": "http://<адрес>:8000" }`, иначе приложение не найдёт
   API — Metro, от которого он берётся в dev, там нет).
5. В приложении разрешить уведомления; токен зарегистрируется сам, проверить —
   `GET /me/devices` (`push_provider: EXPO`) или в БД: `select push_token from devices`.
6. Отправить что-нибудь (объявление, сообщение, напоминание) — придёт баннером. Ночью push
   придерживают тихие часы: срочные объявления пускает переключатель «Срочные объявления ночью».

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
| CORS в браузере | адрес не локальный → добавить в `CORS_ORIGINS`; при остановленном API браузер тоже пишет «CORS» — проверить `/healthz` |
| Файл на телефоне открывается как `localhost` | `APP_BASE_URL` / `S3_PUBLIC_ENDPOINT` не на IP, MinIO без `MINIO_BIND=0.0.0.0` |
| Телефон не заходит («Сервер недоступен: …») | сменилась сеть → `task dev:ip`, перезапуск API и Expo; адрес внизу первого экрана, `healthz` с телефона по этому адресу |
| «Папка не открыта для сервисного аккаунта» | папка не расшарена на `client_email` **или** не включён Google Drive API (см. [`docs/GOOGLE-DRIVE.md`](docs/GOOGLE-DRIVE.md)) |
| Диск подключён, файлов нет | не запущен воркер; очереди — в asynqmon |
| Публикация на Диск: «Google отозвал доступ» через неделю | OAuth-приложение в статусе Testing (токен на 7 дней) → Publish app и переподключить аккаунт (см. «Один раз») |
| Уведомления не появляются | не запущен воркер (`task api:run:worker`); задача `notify:fanout` — в asynqmon |
| В логе «Use a development build instead of Expo Go» | пуша в Expo Go нет — нужна EAS-сборка (см. «Проверить уведомления») |
| Пуш не приходит в своей сборке | нет `extra.eas.projectId` (`eas init`), не загружен ключ FCM V1 (`eas credentials`), не выдано разрешение на телефоне или идут тихие часы |
| Пуш не приходит на телефон | Expo Go на Android его не получает — нужна отдельная сборка; проверить `PUSH_PROVIDER=expo`, разрешение на уведомления и тихие часы в настройках |
| Порт занят | `ss -ltnp \| grep <порт>`; API — `APP_PORT`, Expo — `--port` в `apps/mobile/package.json` |

Известные недочёты, найденные при ручной проверке, — [`docs/FIXES.md`](docs/FIXES.md).
Все команды (тесты, линтеры, генерация) — `CLAUDE.md`, раздел «Команды».
