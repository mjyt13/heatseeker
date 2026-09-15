# packages/core — общая клиентская логика

Платформо-независимая логика для mobile и web: хуки TanStack Query по модулям API (материалы,
задачи, расписание, треды…), sync-клиент (`sync?since=` + WebSocket после MVP, применение
событий `group_events` к кешу), офлайн-outbox сообщений (интерфейс хранилища; реализации —
expo-sqlite / IndexedDB), permissions helpers поверх `packages/shared`, форматирование дат и
RRULE, deep-link parsing.

Статус: создаётся на этапе 0 (минимум — auth + группы), растёт по этапам.
