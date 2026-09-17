# apps/mobile — Expo-приложение

Mobile-first клиент (Android APK; iOS — при появлении Apple Developer). Expo + Expo Router +
TypeScript; TanStack Query (+persist), Zustand, react-hook-form + zod; UI — `packages/ui`
(Tamagui); API — `packages/api-client`; офлайн-outbox сообщений — expo-sqlite; i18n —
`packages/i18n`.

Запуск (Metro на :4173, адрес API — `EXPO_PUBLIC_API_URL` в `apps/mobile/.env.local`) и тест на
телефоне — в корневом [`README.md`](../../README.md#запуск-локальная-разработка).
