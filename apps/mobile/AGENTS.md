# apps/mobile — заметки для ИИ-агента

Expo SDK 57 (React Native 0.86, React 19.2, Expo Router 57). API Expo меняется от версии к
версии — сверяться с документацией именно этой версии: https://docs.expo.dev/versions/v57.0.0/

- Маршруты — `src/app/` (Expo Router, file-based). Группы: `(auth)` — регистрация по имени и
  вход; `(app)` — вкладки. Доступ ограничен `Stack.Protected` по статусу сессии в `_layout.tsx`.
- UI — только компоненты из `@heatseeker/ui` (Tamagui); стили RN напрямую — лишь там, где
  Tamagui не применим (`SafeAreaView`, `contentContainerStyle`).
- Данные — хуки из `@heatseeker/core` (TanStack Query); прямые вызовы `api.*` в экранах не делать.
- Строки — через `useTranslation()` и словари `@heatseeker/i18n`; новые ключи добавлять в `ru`
  и `en` одновременно.
- Токены — `src/lib/storage.ts` (SecureStore); базовый URL API — `EXPO_PUBLIC_API_URL`.
- Проверка без устройства: `pnpm typecheck`, `pnpm lint`, `pnpm export:check` (бандл Metro).
- Babel-плагин Tamagui (оптимизирующий компилятор) пока не подключён — включить, когда
  сборка стабилизируется (см. docs/adr/0005).
