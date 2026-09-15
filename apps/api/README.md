# apps/api — ядро Heatseeker (Go)

HTTP API (`/api/v1`, OpenAPI 3.1 через huma на chi) и фоновые воркеры (asynq на Redis) в
одном бинарнике `heatseeker` с подкомандами:

```
heatseeker api            HTTP API (+ /api/v1/docs, /healthz)
heatseeker worker         asynq-воркеры и планировщик периодических задач
heatseeker migrate [cmd]  goose: up (по умолчанию) | down | redo | status | version
heatseeker gen [what]     openapi | shared | all — артефакты для packages/*
```

## Раскладка

```
cmd/heatseeker/        точка входа и проводка (wire.go)
db/migrations/         goose SQL (встраиваются в бинарник)
db/queries/            SQL для sqlc
internal/domain/       сущности, ошибки, интерфейсы репозиториев — без зависимостей
internal/app/          use cases: access, auth, groups, subjects, tags, sync
internal/authz/        матрица прав (источник правды для packages/shared)
internal/adapters/     postgres (sqlc + транзакции через context), google (id_token)
internal/events/       журнал group_events + in-process шина
internal/transport/http/  huma-операции, middleware (auth, rate limit), DTO, маппинг ошибок
internal/jobs/         asynq-задачи и расписание
internal/bootstrap/    сборка графа сервисов (используется командами и интеграционным тестом)
internal/gen/          генерация OpenAPI и TypeScript-артефактов
internal/platform/     config (env), logger, db, redisx, ids (UUIDv7), clock, slug
openapi/               сгенерированная спека (коммитится)
```

## Запуск

Требуется Go 1.27+, `sqlc`, `task` (см. корневой `CLAUDE.md` — путь к тулчейну), поднятая
dev-инфраструктура (`task dev:up` из корня) и `apps/api/.env` (шаблон — `.env.example` в корне).

```bash
task migrate          # применить миграции
task run:api          # http://localhost:8080/api/v1/docs
task test             # юнит-тесты
task test:integration # сквозной тест на живой БД
task lint             # go vet + golangci-lint
task gen              # sqlc + OpenAPI + packages/shared
```

## Принципы

- Всё scoped по группе; права — только через `internal/authz` (`actor.Require(action)`).
- Транзакция = один use case; репозитории берут `pgx.Tx` из контекста (`Store.RunInTx`),
  события публикуются внутри транзакции, подписчики уведомляются после коммита.
- Конфиг только через env с валидацией при старте (`internal/platform/config`).
- Ошибки: доменные sentinel-ошибки → HTTP-статусы в `transport/http/errors.go`.
