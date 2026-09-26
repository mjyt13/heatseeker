import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query';

import { unwrap, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

type Schemas = components['schemas'];

export type Notification = Schemas['NotificationDTO'];
export type NotificationType = Notification['type'];
export type NotificationPref = Schemas['NotificationPrefDTO'];
export type NotificationSettings = Omit<Schemas['NotificationSettingsDTO'], '$schema'>;
export type NotificationMute = Schemas['MuteDTO'];
export type MuteScope = NotificationMute['scope_type'];

/** Как часто спрашиваем счётчик непрочитанных, пока приложение открыто. */
const UNREAD_POLL_MS = 60_000;

/** Лента уведомлений группы, новые сверху; листается курсором. */
export function useNotifications(groupId: string | null | undefined, unreadOnly = false) {
  const api = useApi();
  return useInfiniteQuery({
    queryKey: keys.notifications(groupId ?? '', unreadOnly),
    enabled: !!groupId,
    initialPageParam: '',
    queryFn: async ({ pageParam }) =>
      unwrap(
        await api.GET('/me/notifications', {
          params: {
            query: {
              group_id: groupId!,
              unread_only: unreadOnly,
              ...(pageParam ? { cursor: pageParam } : {}),
            },
          },
        }),
      ),
    getNextPageParam: (last) => last.next_cursor || undefined,
  });
}

/** Счётчик для бейджа: опрашивается, пока экран открыт. */
export function useUnreadNotifications(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.notificationsUnread(groupId ?? ''),
    enabled: !!groupId,
    refetchInterval: UNREAD_POLL_MS,
    queryFn: async (): Promise<number> =>
      unwrap(await api.GET('/me/notifications/unread', { params: { query: { group_id: groupId! } } }))
        .unread,
  });
}

function invalidateNotifications(qc: QueryClient, groupId: string) {
  return Promise.all([
    qc.invalidateQueries({ queryKey: keys.notificationsAll(groupId) }),
    qc.invalidateQueries({ queryKey: keys.notificationsUnread(groupId) }),
  ]);
}

/** Пометить прочитанными: список id или всё сразу. */
export function useMarkNotificationsRead(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { ids?: string[]; all?: boolean }) =>
      unwrap(
        await api.POST('/me/notifications/read', {
          body: { group_id: groupId, ids: v.ids, all: v.all },
        }),
      ),
    onSuccess: () => invalidateNotifications(qc, groupId),
  });
}

/** Что присылать в этой группе: все типы со значением по умолчанию. */
export function useNotificationPrefs(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.notificationPrefs(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<NotificationPref[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/notification-prefs', {
          params: { path: { groupId: groupId! } },
        }),
      ).items ?? [],
  });
}

/** Включить или выключить типы уведомлений. */
export function useSetNotificationPrefs(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (items: { type: NotificationType; enabled: boolean }[]) =>
      unwrap(
        await api.PUT('/groups/{groupId}/notification-prefs', {
          params: { path: { groupId } },
          body: { items },
        }),
      ).items ?? [],
    onSuccess: (items) => {
      qc.setQueryData(keys.notificationPrefs(groupId), items);
    },
  });
}

/** Push и тихие часы — общие для всех групп. */
export function useNotificationSettings() {
  const api = useApi();
  return useQuery({
    queryKey: keys.notificationSettings(),
    queryFn: async (): Promise<NotificationSettings> =>
      unwrap(await api.GET('/me/notification-settings', {})),
  });
}

/** Сохранить push и тихие часы. */
export function useSaveNotificationSettings() {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: NotificationSettings) =>
      unwrap(await api.PUT('/me/notification-settings', { body: v })),
    onSuccess: (saved) => {
      qc.setQueryData(keys.notificationSettings(), saved);
    },
  });
}

/**
 * Зарегистрировать push-токен этого устройства. Токен выдаёт Expo; сервер
 * хранит его у устройства и шлёт на него уведомления.
 */
export function useRegisterPushToken() {
  const api = useApi();
  return useMutation({
    mutationFn: async (v: { deviceId: string; token: string }) =>
      unwrap(
        await api.PUT('/me/devices/{deviceId}/push', {
          params: { path: { deviceId: v.deviceId } },
          body: { provider: 'EXPO', token: v.token },
        }),
      ),
  });
}

/** Что сейчас приглушено в группе. */
export function useMutes(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.mutes(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<NotificationMute[]> =>
      unwrap(await api.GET('/me/notification-mutes', { params: { query: { group_id: groupId! } } }))
        .items ?? [],
  });
}

/** Приглушить предмет, обсуждение, тип или всю группу до момента времени. */
export function useMute(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { scopeType: MuteScope; scopeId?: string; until: string }) =>
      unwrap(
        await api.POST('/groups/{groupId}/notification-mutes', {
          params: { path: { groupId } },
          body: { scope_type: v.scopeType, scope_id: v.scopeId, until: v.until },
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.mutes(groupId) }),
  });
}

/** Вернуть звук. */
export function useUnmute(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { scopeType: MuteScope; scopeId?: string }) =>
      unwrap(
        await api.DELETE('/groups/{groupId}/notification-mutes', {
          params: {
            path: { groupId },
            query: { scope_type: v.scopeType, scope_id: v.scopeId },
          },
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.mutes(groupId) }),
  });
}
