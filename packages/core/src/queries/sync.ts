import { useQuery } from '@tanstack/react-query';

import { unwrap, type GroupEvent } from '@heatseeker/api-client';

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
        await api.GET('/groups/{groupId}/activity', { params: { path: { groupId: groupId! }, query: { limit } } }),
      ).items ?? [],
  });
}

/** Человеко-читаемый ключ i18n для события журнала. */
export function eventLabelKey(kind: string): string {
  return `events.${kind.replace('.', '_')}`;
}
