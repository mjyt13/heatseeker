import { useQuery } from '@tanstack/react-query';

import { unwrap } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

/** Быстрые теги для главного экрана: системные фильтры + предметы. */
export function useQuickTags(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.quickTags(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async () =>
      unwrap(await api.GET('/groups/{groupId}/quick-tags', { params: { path: { groupId: groupId! } } })).items,
  });
}

/** Все теги группы. */
export function useTags(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.tags(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async () =>
      unwrap(await api.GET('/groups/{groupId}/tags', { params: { path: { groupId: groupId! } } })).items,
  });
}
