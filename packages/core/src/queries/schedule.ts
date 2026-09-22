import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query';

import { unwrap, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

type Schemas = components['schemas'];

export type Occurrence = Schemas['OccurrenceDTO'];
export type ScheduleEvent = Schemas['ScheduleEventDTO'];
export type ScheduleKind = ScheduleEvent['kind'];
export type ScheduleRepeat = Schemas['RepeatDTO'];
export type Weekday = NonNullable<ScheduleRepeat['weekdays']>[number];
export type ScheduleEventInput = Omit<Schemas['ScheduleEventBody'], '$schema'>;
export type ScheduleUpdateInput = Omit<Schemas['UpdateScheduleEventInputBody'], '$schema'>;
export type OccurrenceChange = Omit<Schemas['SetOccurrenceInputBody'], '$schema'>;
export type ScheduleScope = ScheduleUpdateInput['scope'];

/** Занятия за период [from, to) — моменты времени ISO (обычно неделя по часам телефона). */
export function useSchedule(groupId: string | null | undefined, from: string, to: string) {
  const api = useApi();
  return useQuery({
    queryKey: keys.schedule(groupId ?? '', from, to),
    enabled: !!groupId,
    // Flipping weeks keeps the previous one on screen until the next arrives.
    placeholderData: keepPreviousData,
    queryFn: async (): Promise<Occurrence[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/schedule', {
          params: { path: { groupId: groupId! }, query: { from, to } },
        }),
      ).items ?? [],
  });
}

/** Занятие или серия — для правки. */
export function useScheduleEvent(eventId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.scheduleEvent(eventId ?? ''),
    enabled: !!eventId,
    queryFn: async (): Promise<ScheduleEvent> =>
      unwrap(
        await api.GET('/schedule/events/{eventId}', { params: { path: { eventId: eventId! } } }),
      ),
  });
}

/** Одно занятие по дню плана — с отменой или изменением. */
export function useOccurrence(eventId: string | null | undefined, date: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.occurrence(eventId ?? '', date ?? ''),
    enabled: !!eventId && !!date,
    queryFn: async (): Promise<Occurrence> =>
      unwrap(
        await api.GET('/schedule/events/{eventId}/occurrences/{date}', {
          params: { path: { eventId: eventId!, date: date! } },
        }),
      ),
  });
}

function invalidateSchedule(qc: QueryClient, groupId: string, ...eventIds: string[]) {
  return Promise.all([
    qc.invalidateQueries({ queryKey: keys.scheduleAll(groupId) }),
    ...eventIds.map((id) => qc.invalidateQueries({ queryKey: keys.scheduleEvent(id) })),
  ]);
}

/** Добавить занятие или серию (client_id делает повтор безопасным). */
export function useCreateScheduleEvent(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: ScheduleEventInput): Promise<ScheduleEvent> =>
      unwrap(
        await api.POST('/groups/{groupId}/schedule/events', {
          params: { path: { groupId } },
          body,
        }),
      ),
    onSuccess: (event) => invalidateSchedule(qc, groupId, event.id),
  });
}

/**
 * Изменить серию целиком (ALL) или с даты (FOLLOWING — ответ новая серия,
 * старая заканчивается накануне).
 */
export function useUpdateScheduleEvent(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      eventId,
      ...body
    }: ScheduleUpdateInput & { eventId: string }): Promise<ScheduleEvent> =>
      unwrap(
        await api.PATCH('/schedule/events/{eventId}', { params: { path: { eventId } }, body }),
      ),
    onSuccess: (event, vars) => invalidateSchedule(qc, groupId, vars.eventId, event.id),
  });
}

/** Удалить серию целиком или занятия с даты. */
export function useDeleteScheduleEvent(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      eventId,
      version,
      scope,
      from,
    }: {
      eventId: string;
      version: number;
      scope: ScheduleScope;
      from?: string;
    }) => {
      unwrap(
        await api.DELETE('/schedule/events/{eventId}', {
          params: { path: { eventId }, query: { version, scope, ...(from ? { from } : {}) } },
        }),
      );
    },
    onSuccess: (_res, vars) => invalidateSchedule(qc, groupId, vars.eventId),
  });
}

/** Отменить или изменить одно занятие. */
export function useSetOccurrence(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      eventId,
      date,
      ...body
    }: OccurrenceChange & { eventId: string; date: string }): Promise<Occurrence> =>
      unwrap(
        await api.PUT('/schedule/events/{eventId}/occurrences/{date}', {
          params: { path: { eventId, date } },
          body,
        }),
      ),
    onSuccess: (_o, vars) => invalidateSchedule(qc, groupId, vars.eventId),
  });
}

/** Вернуть занятие как по плану. */
export function useResetOccurrence(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ eventId, date }: { eventId: string; date: string }): Promise<Occurrence> =>
      unwrap(
        await api.DELETE('/schedule/events/{eventId}/occurrences/{date}', {
          params: { path: { eventId, date } },
        }),
      ),
    onSuccess: (_o, vars) => invalidateSchedule(qc, groupId, vars.eventId),
  });
}

/** Личная ссылка на расписание для календаря телефона (ICS); запрашивается, когда enabled. */
export function useCalendarLink(groupId: string | null | undefined, enabled = true) {
  const api = useApi();
  return useQuery({
    queryKey: keys.calendarLink(groupId ?? ''),
    enabled: !!groupId && enabled,
    // The link works for a year: no need to ask again while the app is open.
    staleTime: 60 * 60_000,
    queryFn: async (): Promise<string> =>
      unwrap(
        await api.GET('/groups/{groupId}/schedule/calendar-link', {
          params: { path: { groupId: groupId! } },
        }),
      ).url,
  });
}
