# apps/api — ядро Heatseeker (Go)

HTTP API (`/api/v1`, OpenAPI 3.1 через huma) и фоновые воркеры (asynq) в одном бинарнике
`heatseeker` с подкомандами `api | worker | migrate | gen`.

Статус: **код не написан** — каркас создаётся на этапе 0 (см. `docs/PLAN.md`, §11).

Планируемая раскладка — `docs/PLAN.md`, §4.2. Стек: Go 1.23+, chi + huma v2, pgx + sqlc + goose,
Redis + asynq, minio-go, Google Drive API (сервисный аккаунт), `coder/websocket`, `log/slog`.
