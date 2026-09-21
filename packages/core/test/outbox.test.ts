import { QueryClient, type InfiniteData } from '@tanstack/react-query';
import { beforeEach, describe, expect, test } from 'vitest';

import { ApiError, type ApiClient } from '@heatseeker/api-client';

import {
  flushOutbox,
  isTransientError,
  pendingFor,
  useOutbox,
  type OutboxItem,
} from '../src/outbox';
import { flatten, type Message, type MessagePage } from '../src/queries/discussions';
import { keys } from '../src/queries/keys';

const GROUP = 'g1';
const SUBJECT = { type: 'SUBJECT' as const, id: 's1' };

function item(clientId: string, patch: Partial<OutboxItem> = {}): OutboxItem {
  return {
    client_id: clientId,
    group_id: GROUP,
    target_type: SUBJECT.type,
    target_id: SUBJECT.id,
    body: `text ${clientId}`,
    created_at: '2026-09-21T10:00:00Z',
    status: 'pending',
    ...patch,
  };
}

function message(id: string, seq: number, clientId = `c-${id}`): Message {
  return {
    id,
    thread_id: 't1',
    client_id: clientId,
    seq,
    author_name: 'Студент',
    body: `body ${id}`,
    mine: true,
    deleted: false,
    hidden_for_all: false,
    hidden_by_me: false,
    created_at: '2026-09-21T10:00:00Z',
  };
}

function page(items: Message[], hasMore = false): MessagePage {
  return {
    items,
    has_more: hasMore,
    last_read_seq: 0,
    last_seq: items.at(-1)?.seq ?? 0,
  };
}

/** Fake POST: answers per client_id with a message, an HTTP status or a network failure. */
function fakeApi(answers: Record<string, Message | number | 'offline'>) {
  const sent: string[] = [];
  const api = {
    POST: async (_path: string, init: { body: { client_id: string } }) => {
      const id = init.body.client_id;
      sent.push(id);
      const answer = answers[id];
      if (answer === 'offline' || answer === undefined)
        throw new TypeError('Network request failed');
      if (typeof answer === 'number') {
        return {
          error: { status: answer, title: 'nope' },
          response: new Response(null, { status: answer }),
        };
      }
      return { data: answer, response: new Response(null, { status: 201 }) };
    },
  } as unknown as Pick<ApiClient, 'POST'>;
  return { api, sent };
}

beforeEach(async () => {
  useOutbox.setState({ items: [], ready: false });
  await useOutbox.getState().configure({ load: () => [], save: () => undefined });
});

describe('isTransientError', () => {
  test('network failures and server trouble keep the message queued', () => {
    expect(isTransientError(new TypeError('Network request failed'))).toBe(true);
    expect(isTransientError(new ApiError(503, undefined))).toBe(true);
    expect(isTransientError(new ApiError(429, undefined))).toBe(true);
  });
  test('a refusal needs the author', () => {
    expect(isTransientError(new ApiError(403, undefined))).toBe(false);
    expect(isTransientError(new ApiError(422, undefined))).toBe(false);
  });
});

describe('flushOutbox', () => {
  test('sends in order, shows the message in the thread and empties the queue', async () => {
    const qc = new QueryClient();
    const key = keys.thread(GROUP, SUBJECT.type, SUBJECT.id);
    qc.setQueryData<InfiniteData<MessagePage>>(key, {
      pages: [page([message('m1', 5)])],
      pageParams: [0],
    });
    useOutbox.getState().enqueue(item('a'));
    useOutbox.getState().enqueue(item('b'));
    const { api, sent } = fakeApi({ a: message('m2', 6, 'a'), b: message('m3', 7, 'b') });

    await flushOutbox(api, qc);

    expect(sent).toEqual(['a', 'b']);
    expect(useOutbox.getState().items).toEqual([]);
    const data = qc.getQueryData<InfiniteData<MessagePage>>(key);
    expect(flatten(data).map((m) => m.id)).toEqual(['m1', 'm2', 'm3']);
    expect(data?.pages[0]?.last_seq).toBe(7);
  });

  test('offline stops the pass and keeps everything for later', async () => {
    useOutbox.getState().enqueue(item('a'));
    useOutbox.getState().enqueue(item('b'));
    const { api, sent } = fakeApi({ a: 'offline', b: message('m3', 7, 'b') });

    await flushOutbox(api, new QueryClient());

    expect(sent).toEqual(['a']);
    expect(useOutbox.getState().items.map((i) => [i.client_id, i.status])).toEqual([
      ['a', 'pending'],
      ['b', 'pending'],
    ]);
  });

  test('a refused message is marked failed and the rest still go', async () => {
    useOutbox.getState().enqueue(item('a'));
    useOutbox.getState().enqueue(item('b'));
    const { api } = fakeApi({ a: 403, b: message('m3', 7, 'b') });

    await flushOutbox(api, new QueryClient());

    const items = useOutbox.getState().items;
    expect(items.map((i) => [i.client_id, i.status])).toEqual([['a', 'failed']]);
    expect(items[0]?.error).toBeTruthy();
  });

  test('a queue saved before a restart is picked up', async () => {
    useOutbox.setState({ items: [], ready: false });
    const saved: OutboxItem[][] = [];
    await useOutbox
      .getState()
      .configure({ load: async () => [item('old')], save: (i) => void saved.push(i) });
    const { api, sent } = fakeApi({ old: message('m9', 9, 'old') });

    await flushOutbox(api, new QueryClient());

    expect(sent).toEqual(['old']);
    expect(saved.at(-1)).toEqual([]);
  });
});

describe('pendingFor', () => {
  test('only this discussion, minus what the server already shows', () => {
    const items = [
      item('a'),
      item('b'),
      item('c', { target_type: 'GENERAL', target_id: GROUP }),
      item('d', { group_id: 'other' }),
    ];
    const delivered = [message('m1', 1, 'b')];
    expect(pendingFor(items, GROUP, SUBJECT, delivered).map((i) => i.client_id)).toEqual(['a']);
  });
});

describe('flatten', () => {
  test('pages go newest to oldest, messages come out oldest first', () => {
    const data: InfiniteData<MessagePage> = {
      pages: [
        page([message('m3', 3), message('m4', 4)], true),
        page([message('m1', 1), message('m2', 2)]),
      ],
      pageParams: [0, 3],
    };
    expect(flatten(data).map((m) => m.id)).toEqual(['m1', 'm2', 'm3', 'm4']);
    expect(flatten(undefined)).toEqual([]);
  });
});

describe('clear', () => {
  test('signing out before the queue loaded drops the saved messages', async () => {
    useOutbox.setState({ items: [], ready: false });
    useOutbox.getState().clear();
    await useOutbox
      .getState()
      .configure({ load: async () => [item('stale')], save: () => undefined });
    expect(useOutbox.getState().items).toEqual([]);
  });
});
