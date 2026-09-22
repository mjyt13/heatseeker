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
- Токены — `src/lib/storage.ts` (SecureStore); базовый URL API — `EXPO_PUBLIC_API_URL`, без него в dev —
  хост Metro/страницы и порт 8000 (`src/lib/api.ts`).
- Аудио и видео — встроенный плеер `src/components/media-player.tsx` (expo-audio / expo-video).
- Скрытые вкладки (`href: null`) не размонтируются: экрану с параметром (`material/[id]`,
  `task/[id]`) нужен внутренний компонент с `key={id}`, иначе состояние переезжает на другую
  запись.
- Сообщения обсуждений уходят через офлайн-outbox (`useSendMessage` из `@heatseeker/core`, D41):
  очередь хранится в AsyncStorage (`src/lib/storage.ts`), отправляет её `useOutboxFlusher` в
  `(app)/_layout.tsx`. `client_id` — `randomUUID` из expo-crypto.
- Экран, который опрашивает сервер по таймеру, должен останавливаться вне фокуса
  (`useScreenFocused` из `src/lib/focus.ts`): скрытые вкладки остаются смонтированными.
- Срок задачи — календарь `src/components/due-picker.tsx` (логика — `calendar.ts` в
  `@heatseeker/core`); даты форматируются вручную (`formatDue`), Intl в Hermes неполный. Срок без
  времени — конец дня (D42).
- Поля формы — `Field` из `@heatseeker/ui`: id делается уникальным сам (`useId`), потому что
  скрытые вкладки держат одну форму смонтированной дважды. В вебе — `aria-*`, не `accessibility*`.
- Стартовый экран — `src/app/(app)/index.tsx` (перенаправление, сейчас на «Задачи»); лента —
  `feed.tsx`. Вкладки возвращаются «назад» по истории (`backBehavior="history"`), а не на первую.
- Расписание — вкладка `schedule.tsx`, занятие — `class/[eventId]/[date].tsx`, форма —
  `class/edit.tsx` (новое занятие, серия целиком или с даты, одно занятие; переход передаёт `n`,
  чтобы форма открывалась чистой). Часовой пояс серии — `deviceTimeZone()` из `src/lib/timezone.ts`;
  календарь месяца — общий `src/components/calendar-month.tsx`.
- Картинки: `Thumbnail` и `PictureViewer` из `src/components/picture.tsx` (expo-image, кеш по id
  версии: `thumbnail_url` временная, картинка — нет).
- Проверка без устройства: `pnpm typecheck`, `pnpm lint`, `pnpm export:check` (бандл Metro).
- Babel-плагин Tamagui (оптимизирующий компилятор) пока не подключён — включить, когда
  сборка стабилизируется (см. docs/adr/0005).
