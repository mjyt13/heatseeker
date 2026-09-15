# apps/web — веб-клиент (этап W2-A)

Vite + React, без SSR (всё за логином). Переиспользует `packages/ui` (Tamagui через
`@tamagui/vite-plugin`), `packages/core`, `packages/api-client`, `packages/i18n`. Web push через
service worker + VAPID; realtime — тот же WebSocket-протокол, что и у mobile.

Статус: **создаётся после MVP** (см. `docs/PLAN.md`, §11).
