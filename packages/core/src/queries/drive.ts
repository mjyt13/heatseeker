import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  unwrap,
  type DriveConnection,
  type DriveItem,
  type DriveStatus,
} from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

/** Подключение Google Диска; пока идёт синхронизация — опрашивается. */
export function useDriveStatus(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.drive(groupId ?? ''),
    enabled: !!groupId,
    refetchInterval: (q) => {
      const status = q.state.data?.connection?.status;
      return status === 'SYNCING' || status === 'PENDING' ? 5000 : false;
    },
    queryFn: async (): Promise<DriveStatus> =>
      unwrap(
        await api.GET('/groups/{groupId}/drive/status', {
          params: { path: { groupId: groupId! } },
        }),
      ),
  });
}

/** Подключить папку по ссылке. */
export function useConnectDrive(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (folder: string): Promise<DriveConnection> =>
      unwrap(
        await api.PUT('/groups/{groupId}/drive/connection', {
          params: { path: { groupId } },
          body: { folder },
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.drive(groupId) }),
  });
}

/** Отключить Диск (материалы остаются). */
export function useDisconnectDrive(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      unwrap(
        await api.DELETE('/groups/{groupId}/drive/connection', { params: { path: { groupId } } }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.drive(groupId) }),
  });
}

/** Запустить синхронизацию сейчас. */
export function useSyncDrive(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (full: boolean = false) =>
      unwrap(
        await api.POST('/groups/{groupId}/drive/sync', {
          params: { path: { groupId } },
          body: { full },
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.drive(groupId) }),
  });
}

/** Файлы индекса (диагностика для управляющих Диском). */
export function useDriveItems(groupId: string | null | undefined, state?: DriveItem['state']) {
  const api = useApi();
  return useQuery({
    queryKey: [...keys.drive(groupId ?? ''), 'items', state ?? 'all'],
    enabled: !!groupId,
    queryFn: async (): Promise<DriveItem[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/drive/items', {
          params: { path: { groupId: groupId! }, query: { state, limit: 200 } },
        }),
      ).items ?? [],
  });
}
