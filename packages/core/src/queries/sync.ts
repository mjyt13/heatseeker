import { useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { ApiError, unwrap, type ApiClient, type GroupEvent } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

/**
 * Лента активности группы (кто что когда). Полноценный дельта-синк с локальной
 * БД появится вместе с обсуждениями (этап 2); пока — обычный запрос.
 */
export function useActivity(groupId: string | null | undefined, limit = 50) {
  const api = useApi();
  return useQuery({
    queryKey: [...keys.activity(groupId ?? ''), limit],
    enabled: !!groupId,
    queryFn: async (): Promise<GroupEvent[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/activity', {
          params: { path: { groupId: groupId! }, query: { limit } },
        }),
      ).items ?? [],
  });
}

/** Человеко-читаемый ключ i18n для события журнала. */
export function eventLabelKey(kind: string): string {
  return `events.${kind.replace('.', '_')}`;
}

/** Как часто проверять журнал группы, пока приложение открыто. */
export const GROUP_CHANGES_INTERVAL_MS = 60_000;

type SyncApi = Pick<ApiClient, 'GET'>;

/**
 * Один шаг наблюдения за журналом группы: читает события после `cursor` и
 * помечает данные группы устаревшими, если что-то изменилось. Возвращает новый
 * курсор. Без курсора только узнаёт, где кончается журнал.
 */
export async function pollGroupChanges(
  api: SyncApi,
  qc: QueryClient,
  groupId: string,
  cursor: number | undefined,
): Promise<number> {
  const path = { groupId };
  const head = async () =>
    unwrap(
      await api.GET('/groups/{groupId}/sync', { params: { path, query: { since: 0, limit: 1 } } }),
    ).latest;
  if (cursor === undefined) return head();

  let since = cursor;
  let changed: GroupEvent[] = [];
  try {
    for (let page = 0; page < 5; page++) {
      const res = unwrap(
        await api.GET('/groups/{groupId}/sync', { params: { path, query: { since, limit: 200 } } }),
      );
      changed = changed.concat(res.events ?? []);
      since = res.next_seq;
      if (!res.has_more) break;
    }
  } catch (err) {
    if (!(err instanceof ApiError) || err.status !== 410) throw err;
    // History behind the cursor was pruned: refresh everything.
    await qc.invalidateQueries({ queryKey: keys.group(groupId), predicate: notSyncQuery });
    return head();
  }
  if (changed.length > 0) {
    const materialIds = new Set(
      changed.filter((e) => e.entity_type === 'material' && e.entity_id).map((e) => e.entity_id!),
    );
    await Promise.all([
      qc.invalidateQueries({ queryKey: keys.group(groupId), predicate: notSyncQuery }),
      ...[...materialIds].map((id) => qc.invalidateQueries({ queryKey: keys.material(id) })),
    ]);
  }
  return since;
}

// The watcher must not refetch itself from inside its own query.
const notSyncQuery = (q: { queryKey: readonly unknown[] }) => q.queryKey[2] !== 'sync';

/**
 * Следит за журналом группы (`/sync?since=`), пока приложение открыто, чтобы
 * лента видела изменения с Диска, сделанные воркером. Временная замена realtime
 * (этап W2-0). Данные запроса — курсор (последний учтённый seq).
 */
export function useGroupChanges(groupId: string | null | undefined) {
  const api = useApi();
  const qc = useQueryClient();
  return useQuery({
    queryKey: keys.sync(groupId ?? ''),
    enabled: !!groupId,
    refetchInterval: GROUP_CHANGES_INTERVAL_MS,
    staleTime: 0,
    retry: false,
    queryFn: ({ queryKey }) =>
      pollGroupChanges(api, qc, groupId!, qc.getQueryData<number>(queryKey)),
  });
}
