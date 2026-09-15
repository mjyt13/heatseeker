# packages/api-client — типизированный клиент API

Генерируется из `apps/api/openapi/openapi.json` (`openapi-typescript`) + тонкая обёртка над
`openapi-fetch`: базовый URL из env, подстановка access-токена, автоматический refresh, обработка
ошибок API в типизированные ошибки, `Idempotency-Key`/`client_id` там, где нужно.

Используется `apps/mobile`, `apps/web`, `packages/core`. При изменении API — `pnpm -F
@heatseeker/api-client generate`; несовпадение типов ломает сборку клиентов, а не прод.

Статус: создаётся на этапе 0.
