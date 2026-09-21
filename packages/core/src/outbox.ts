import { useQueryClient, type InfiniteData, type QueryClient } from '@tanstack/react-query';
import { useCallback, useEffect } from 'react';
import { create } from 'zustand';

import { ApiError, unwrap, type ApiClient } from '@heatseeker/api-client';

import { useApi } from './api-provider';
import type {
  DiscussionTarget,
  Message,
  MessagePage,
  ThreadTargetType,
} from './queries/discussions';
import { keys } from './queries/keys';

/**
 * Офлайн-outbox сообщений (D41): написанное без сети ждёт в очереди и уходит
 * само, когда сеть вернётся. Повтор безопасен — сервер узнаёт сообщение по
 * client_id и не создаёт второе.
 */
export interface OutboxItem {
  client_id: string;
  group_id: string;
  target_type: ThreadTargetType;
  target_id: string;
  body: string;
  reply_to?: { id: string; author_name: string; body: string };
  created_at: string;
  /** failed — сервер отказал (не сеть); ждёт решения автора: повторить или удалить. */
  status: 'pending' | 'failed';
  error?: string;
}

/** Где очередь переживает перезапуск приложения (AsyncStorage / localStorage). */
export interface OutboxStorage {
  load(): Promise<OutboxItem[]> | OutboxItem[];
  save(items: OutboxItem[]): Promise<void> | void;
}

interface OutboxState {
  items: OutboxItem[];
  ready: boolean;
  configure: (storage: OutboxStorage) => Promise<void>;
  enqueue: (item: OutboxItem) => void;
  update: (clientId: string, patch: Partial<OutboxItem>) => void;
  remove: (clientId: string) => void;
  /** Забыть очередь (выход из аккаунта: чужие сообщения не должны уйти от нового). */
  clear: () => void;
}

let storage: OutboxStorage | null = null;
// clear() arrived before the saved queue was loaded: drop what loads.
let clearedBeforeLoad = false;

function persist(items: OutboxItem[]) {
  void Promise.resolve(storage?.save(items)).catch(() => undefined);
}

export const useOutbox = create<OutboxState>((set, get) => ({
  items: [],
  ready: false,

  async configure(s) {
    storage = s;
    let saved: OutboxItem[];
    try {
      saved = (await s.load()) ?? [];
    } catch {
      saved = [];
    }
    if (clearedBeforeLoad) saved = [];
    clearedBeforeLoad = false;
    // Keep anything enqueued before the storage finished loading.
    const known = new Set(saved.map((i) => i.client_id));
    const items = [...saved, ...get().items.filter((i) => !known.has(i.client_id))];
    set({ items, ready: true });
    persist(items);
  },

  enqueue(item) {
    const items = [...get().items.filter((i) => i.client_id !== item.client_id), item];
    set({ items });
    persist(items);
  },

  update(clientId, patch) {
    const items = get().items.map((i) => (i.client_id === clientId ? { ...i, ...patch } : i));
    set({ items });
    persist(items);
  },

  remove(clientId) {
    const items = get().items.filter((i) => i.client_id !== clientId);
    set({ items });
    persist(items);
  },

  clear() {
    if (!get().ready) clearedBeforeLoad = true;
    set({ items: [] });
    persist([]);
  },
}));

/** Как часто повторять отправку, пока в очереди что-то есть. */
export const OUTBOX_RETRY_MS = 15_000;

/**
 * Временная ли ошибка: сеть, таймаут, перегрузка, сбой сервера. Тогда
 * сообщение остаётся в очереди; иначе сервер его отверг и нужен автор.
 */
export function isTransientError(err: unknown): boolean {
  if (!(err instanceof ApiError)) return true;
  return err.status === 408 || err.status === 429 || err.status >= 500;
}

type SendApi = Pick<ApiClient, 'POST'>;

let flushing: Promise<void> | null = null;

/** Отправить очередь по порядку; параллельный вызов ждёт текущий проход. */
export function flushOutbox(api: SendApi, qc: QueryClient): Promise<void> {
  flushing ??= runFlush(api, qc).finally(() => {
    flushing = null;
  });
  return flushing;
}

async function runFlush(api: SendApi, qc: QueryClient) {
  const { items, ready } = useOutbox.getState();
  if (!ready) return;
  for (const item of items.filter((i) => i.status === 'pending')) {
    try {
      const message = unwrap(
        await api.POST('/groups/{groupId}/discussions/{targetType}/{targetId}/messages', {
          params: {
            path: {
              groupId: item.group_id,
              targetType: item.target_type,
              targetId: item.target_id,
            },
          },
          body: { client_id: item.client_id, body: item.body, reply_to_id: item.reply_to?.id },
        }),
      );
      appendToThread(qc, item, message);
      useOutbox.getState().remove(item.client_id);
    } catch (err) {
      // Offline: the rest of the queue would fail the same way.
      if (isTransientError(err)) return;
      useOutbox.getState().update(item.client_id, {
        status: 'failed',
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }
  await qc.invalidateQueries({ queryKey: ['group'], predicate: isDiscussionsQuery });
}

const isDiscussionsQuery = (q: { queryKey: readonly unknown[] }) => q.queryKey[2] === 'discussions';

/** Показать отправленное сразу, не дожидаясь перезапроса треда. */
function appendToThread(qc: QueryClient, item: OutboxItem, message: Message) {
  for (const includeHidden of [false, true]) {
    qc.setQueryData<InfiniteData<MessagePage>>(
      keys.thread(item.group_id, item.target_type, item.target_id, includeHidden),
      (data) => {
        const [newest, ...rest] = data?.pages ?? [];
        if (!data || !newest) return data;
        const items = newest.items ?? [];
        if (items.some((m) => m.id === message.id)) return data;
        const page: MessagePage = {
          ...newest,
          items: [...items, message],
          last_seq: Math.max(newest.last_seq, message.seq),
        };
        return { ...data, pages: [page, ...rest] };
      },
    );
  }
}

/** Сообщения обсуждения, ещё не принятые сервером (не показанные в треде). */
export function pendingFor(
  items: OutboxItem[],
  groupId: string,
  target: DiscussionTarget,
  delivered: readonly Message[],
): OutboxItem[] {
  const seen = new Set(delivered.map((m) => m.client_id));
  return items.filter(
    (i) =>
      i.group_id === groupId &&
      i.target_type === target.type &&
      i.target_id === target.id &&
      !seen.has(i.client_id),
  );
}

/**
 * Отправка из экрана обсуждения: в очередь и сразу попытка отправить.
 * client_id создаёт приложение (expo-crypto / crypto.randomUUID).
 */
export function useSendMessage(groupId: string, target: DiscussionTarget) {
  const api = useApi();
  const qc = useQueryClient();
  return useCallback(
    (clientId: string, body: string, replyTo?: OutboxItem['reply_to']) => {
      useOutbox.getState().enqueue({
        client_id: clientId,
        group_id: groupId,
        target_type: target.type,
        target_id: target.id,
        body,
        reply_to: replyTo,
        created_at: new Date().toISOString(),
        status: 'pending',
      });
      void flushOutbox(api, qc);
    },
    [api, qc, groupId, target.type, target.id],
  );
}

/** Повторить отправку сообщения, которое сервер отверг. */
export function useRetryMessage() {
  const api = useApi();
  const qc = useQueryClient();
  return useCallback(
    (clientId: string) => {
      useOutbox.getState().update(clientId, { status: 'pending', error: undefined });
      void flushOutbox(api, qc);
    },
    [api, qc],
  );
}

/**
 * Держит очередь в движении: при запуске, по таймеру, пока она не пуста, и
 * при возвращении приложения на экран (`active` меняется снаружи).
 */
export function useOutboxFlusher(active = true) {
  const api = useApi();
  const qc = useQueryClient();
  const ready = useOutbox((s) => s.ready);
  const pending = useOutbox((s) => s.items.some((i) => i.status === 'pending'));
  useEffect(() => {
    if (!ready || !active || !pending) return;
    void flushOutbox(api, qc);
    const timer = setInterval(() => void flushOutbox(api, qc), OUTBOX_RETRY_MS);
    return () => clearInterval(timer);
  }, [api, qc, ready, active, pending]);
}
