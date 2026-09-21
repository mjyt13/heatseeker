import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
  type QueryClient,
} from '@tanstack/react-query';

import { unwrap, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

type Schemas = components['schemas'];

export type Discussion = Schemas['DiscussionDTO'];
export type Discussions = Schemas['DiscussionsDTO'];
export type Message = Schemas['MessageDTO'];
export type MessagePage = Schemas['MessagePageDTO'];
export type ThreadTargetType = Discussion['target_type'];

/** О чём обсуждение: предмет, вся группа (id группы), материал или задача. */
export interface DiscussionTarget {
  type: ThreadTargetType;
  id: string;
}

/** Как часто обновлять открытое обсуждение, пока нет realtime (этап W2-0). */
export const THREAD_REFRESH_MS = 10_000;

const PAGE_SIZE = 50;

/** Экран «Обсуждения»: общий тред, предметы и обсуждения материалов/задач. */
export function useDiscussions(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.discussions(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<Discussions> =>
      unwrap(
        await api.GET('/groups/{groupId}/discussions', { params: { path: { groupId: groupId! } } }),
      ),
  });
}

/**
 * Сообщения обсуждения. Первая страница — самые новые, следующие — всё более
 * старые; `messages` собраны от старых к новым.
 */
export function useThread(
  groupId: string | null | undefined,
  target: DiscussionTarget | null,
  options: { includeHidden?: boolean; live?: boolean } = {},
) {
  const api = useApi();
  const includeHidden = !!options.includeHidden;
  const query = useInfiniteQuery({
    queryKey: keys.thread(groupId ?? '', target?.type ?? '', target?.id ?? '', includeHidden),
    enabled: !!groupId && !!target,
    initialPageParam: 0,
    refetchInterval: options.live ? THREAD_REFRESH_MS : false,
    // Toggling "show hidden" swaps the query: keep the old list on screen meanwhile.
    placeholderData: keepPreviousData,
    queryFn: async ({ pageParam }): Promise<MessagePage> =>
      unwrap(
        await api.GET('/groups/{groupId}/discussions/{targetType}/{targetId}/messages', {
          params: {
            path: { groupId: groupId!, targetType: target!.type, targetId: target!.id },
            query: {
              limit: PAGE_SIZE,
              include_hidden: includeHidden || undefined,
              before_seq: pageParam || undefined,
            },
          },
        }),
      ),
    getNextPageParam: (last) => (last.has_more ? last.items?.[0]?.seq : undefined),
  });
  return { ...query, messages: flatten(query.data) };
}

/** Страницы идут от новых к старым; внутри страницы — от старых к новым. */
export function flatten(data: InfiniteData<MessagePage> | undefined): Message[] {
  if (!data) return [];
  const out: Message[] = [];
  for (let i = data.pages.length - 1; i >= 0; i--) out.push(...(data.pages[i]?.items ?? []));
  return out;
}

/** Отметить обсуждение прочитанным до seq (0 — до последнего сообщения). */
export function useMarkThreadRead(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ target, seq }: { target: DiscussionTarget; seq?: number }) => {
      unwrap(
        await api.POST('/groups/{groupId}/discussions/{targetType}/{targetId}/read', {
          params: { path: { groupId, targetType: target.type, targetId: target.id } },
          body: { seq: seq ?? 0 },
        }),
      );
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.discussions(groupId) }),
  });
}

/** Изменить своё сообщение. */
export function useEditMessage(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      messageId,
      body,
    }: {
      messageId: string;
      body: string;
    }): Promise<Message> =>
      unwrap(
        await api.PATCH('/messages/{messageId}', {
          params: { path: { messageId } },
          body: { body },
        }),
      ),
    onSuccess: () => invalidateDiscussions(qc, groupId),
  });
}

/** Удалить своё сообщение (в треде остаётся пометка). */
export function useDeleteMessage(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (messageId: string): Promise<void> => {
      unwrap(await api.DELETE('/messages/{messageId}', { params: { path: { messageId } } }));
    },
    onSuccess: () => invalidateDiscussions(qc, groupId),
  });
}

/** Восстановить своё удалённое сообщение. */
export function useRestoreMessage(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (messageId: string): Promise<Message> =>
      unwrap(await api.POST('/messages/{messageId}/restore', { params: { path: { messageId } } })),
    onSuccess: () => invalidateDiscussions(qc, groupId),
  });
}

/** Скрыть сообщение для себя или вернуть его. */
export function useHideMessage(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      messageId,
      hidden,
    }: {
      messageId: string;
      hidden: boolean;
    }): Promise<Message> => {
      const params = { path: { messageId } };
      return unwrap(
        hidden
          ? await api.POST('/messages/{messageId}/hide', { params })
          : await api.DELETE('/messages/{messageId}/hide', { params }),
      );
    },
    onSuccess: () => invalidateDiscussions(qc, groupId),
  });
}

/** Скрыть сообщение для всех или вернуть (модератор, админ). */
export function useModerateMessage(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      messageId,
      hidden,
    }: {
      messageId: string;
      hidden: boolean;
    }): Promise<Message> => {
      const params = { path: { messageId } };
      return unwrap(
        hidden
          ? await api.POST('/messages/{messageId}/moderate', { params })
          : await api.DELETE('/messages/{messageId}/moderate', { params }),
      );
    },
    onSuccess: () => invalidateDiscussions(qc, groupId),
  });
}

/** «Скрытые мной» — чтобы быстро вернуть сообщение. */
export function useHiddenMessages(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.hiddenMessages(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<Message[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/messages/hidden', {
          params: { path: { groupId: groupId! } },
        }),
      ).items ?? [],
  });
}

/** Перечитать список обсуждений, открытые треды и «Скрытые мной». */
export async function invalidateDiscussions(qc: QueryClient, groupId: string) {
  await Promise.all([
    qc.invalidateQueries({ queryKey: keys.discussions(groupId) }),
    qc.invalidateQueries({ queryKey: ['group', groupId, 'thread'] }),
    qc.invalidateQueries({ queryKey: keys.hiddenMessages(groupId) }),
  ]);
}
