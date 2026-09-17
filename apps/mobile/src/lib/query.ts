import AsyncStorage from '@react-native-async-storage/async-storage';
import { createAsyncStoragePersister } from '@tanstack/query-async-storage-persister';
import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';

/**
 * Кеш запросов переживает перезапуск (офлайн-чтение — must-have):
 * данные считаются свежими минуту, хранятся сутки.
 */
export const queryClient = new QueryClient({
  // In dev, surface every failed request in the Metro log (screens only show a short message).
  queryCache: new QueryCache({
    onError: (err, query) => logDevError('query', query.queryKey, err),
  }),
  mutationCache: new MutationCache({
    onError: (err, _vars, _ctx, mutation) =>
      logDevError('mutation', mutation.options.mutationKey, err),
  }),
  defaultOptions: {
    queries: {
      staleTime: 60_000,
      gcTime: 24 * 60 * 60_000,
      retry: 1,
    },
  },
});

/**
 * Версия сохранённого кеша: смена значения сбрасывает его при запуске.
 * v2 — после исправления кодировки ответов (кеш мог сохранить искажённую кириллицу).
 */
export const QUERY_CACHE_BUSTER = 'v2';

export const queryPersister = createAsyncStoragePersister({
  storage: AsyncStorage,
  key: 'heatseeker.query-cache',
  throttleTime: 1000,
});

function logDevError(kind: string, key: unknown, err: unknown) {
  if (!__DEV__) return;
  const detail =
    err instanceof Error ? `${err.name}: ${err.message}\n${err.stack ?? ''}` : String(err);
  const body = (err as { body?: unknown }).body;
  console.warn(`[${kind}] ${JSON.stringify(key ?? null)} failed: ${detail}`, body ?? '');
}
