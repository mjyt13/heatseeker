import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { unwrap, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

type Schemas = components['schemas'];

export type Announcement = Schemas['AnnouncementDTO'];
export type AnnouncementInput = Omit<Schemas['AnnouncementBody'], '$schema'>;
export type Reminder = Schemas['ReminderDTO'];
export type ReminderInput = Omit<Schemas['ReminderBody'], '$schema'>;
export type ReminderRepeat = Reminder['repeat'];

/** Объявления группы: закреплённые сверху. */
export function useAnnouncements(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.announcements(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<Announcement[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/announcements', {
          params: { path: { groupId: groupId! } },
        }),
      ).items ?? [],
  });
}

const invalidateAnnouncements = (qc: QueryClient, groupId: string) =>
  qc.invalidateQueries({ queryKey: keys.announcements(groupId) });

/** Объявить группе (староста или админ с защищённым аккаунтом). */
export function useCreateAnnouncement(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: AnnouncementInput) =>
      unwrap(
        await api.POST('/groups/{groupId}/announcements', {
          params: { path: { groupId } },
          body: v,
        }),
      ),
    onSuccess: () => invalidateAnnouncements(qc, groupId),
  });
}

/** Изменить объявление: группе об этом не сообщат повторно. */
export function useUpdateAnnouncement(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { announcementId: string } & AnnouncementInput) => {
      const { announcementId, ...body } = v;
      return unwrap(
        await api.PATCH('/announcements/{announcementId}', {
          params: { path: { announcementId } },
          body,
        }),
      );
    },
    onSuccess: () => invalidateAnnouncements(qc, groupId),
  });
}

/** Снять объявление. */
export function useDeleteAnnouncement(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (announcementId: string) =>
      unwrap(
        await api.DELETE('/announcements/{announcementId}', {
          params: { path: { announcementId } },
        }),
      ),
    onSuccess: () => invalidateAnnouncements(qc, groupId),
  });
}

/** Мои напоминания в этой группе. */
export function useReminders(groupId: string | null | undefined, openOnly = false) {
  const api = useApi();
  return useQuery({
    queryKey: keys.reminders(groupId ?? '', openOnly),
    enabled: !!groupId,
    queryFn: async (): Promise<Reminder[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/reminders', {
          params: { path: { groupId: groupId! }, query: { open_only: openOnly } },
        }),
      ).items ?? [],
  });
}

const invalidateReminders = (qc: QueryClient, groupId: string) =>
  qc.invalidateQueries({ queryKey: keys.remindersAll(groupId) });

/** Поставить напоминание. */
export function useCreateReminder(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: ReminderInput) =>
      unwrap(
        await api.POST('/groups/{groupId}/reminders', { params: { path: { groupId } }, body: v }),
      ),
    onSuccess: () => invalidateReminders(qc, groupId),
  });
}

/** Изменить напоминание (поля заменяются целиком). */
export function useUpdateReminder(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { reminderId: string } & ReminderInput) => {
      const { reminderId, ...body } = v;
      return unwrap(
        await api.PATCH('/reminders/{reminderId}', { params: { path: { reminderId } }, body }),
      );
    },
    onSuccess: () => invalidateReminders(qc, groupId),
  });
}

/** Отложить напоминание на заданное число минут. */
export function useSnoozeReminder(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { reminderId: string; minutes: number }) =>
      unwrap(
        await api.POST('/reminders/{reminderId}/snooze', {
          params: { path: { reminderId: v.reminderId } },
          body: { minutes: v.minutes },
        }),
      ),
    onSuccess: () => invalidateReminders(qc, groupId),
  });
}

/** «Готово» — и обратно в работу. */
export function useDoneReminder(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { reminderId: string; done: boolean }) =>
      unwrap(
        await api.POST('/reminders/{reminderId}/done', {
          params: { path: { reminderId: v.reminderId } },
          body: { done: v.done },
        }),
      ),
    onSuccess: () => invalidateReminders(qc, groupId),
  });
}

/** Удалить напоминание. */
export function useDeleteReminder(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (reminderId: string) =>
      unwrap(await api.DELETE('/reminders/{reminderId}', { params: { path: { reminderId } } })),
    onSuccess: () => invalidateReminders(qc, groupId),
  });
}
