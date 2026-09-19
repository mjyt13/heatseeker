import { QueryClient } from '@tanstack/react-query';
import { describe, expect, test } from 'vitest';

import { ApiError, type ApiClient } from '@heatseeker/api-client';

import { keys } from '../src/queries/keys';
import { pollGroupChanges } from '../src/queries/sync';

type Page = {
  events: { entity_type: string; entity_id?: string }[];
  next_seq: number;
  latest: number;
  has_more: boolean;
};

/** Fake GET /groups/{id}/sync: pages keyed by `since`, or an error status. */
function fakeApi(pages: Record<number, Page | number>, latest: number) {
  const calls: number[] = [];
  const api = {
    GET: async (_path: string, init: { params: { query: { since: number; limit: number } } }) => {
      const { since, limit } = init.params.query;
      calls.push(since);
      if (since === 0 && limit === 1) {
        return {
          data: { events: [], next_seq: 0, latest, has_more: true },
          response: new Response(),
        };
      }
      const page = pages[since];
      if (typeof page === 'number') {
        return { error: { status: page }, response: new Response(null, { status: page }) };
      }
      return {
        data: page ?? { events: [], next_seq: since, latest, has_more: false },
        response: new Response(),
      };
    },
  } as unknown as Pick<ApiClient, 'GET'>;
  return { api, calls };
}

function seededClient() {
  const qc = new QueryClient();
  qc.setQueryData(keys.materials('g', {}), { pages: [] });
  qc.setQueryData(keys.material('m1'), { id: 'm1' });
  qc.setQueryData(keys.material('m2'), { id: 'm2' });
  qc.setQueryData(keys.sync('g'), 5);
  qc.setQueryData(keys.group('other'), {});
  return qc;
}

const stale = (qc: QueryClient, key: readonly unknown[]) => qc.getQueryState(key)?.isInvalidated;

describe('pollGroupChanges', () => {
  test('first run only learns the end of the log', async () => {
    const { api, calls } = fakeApi({}, 42);
    const qc = seededClient();
    expect(await pollGroupChanges(api, qc, 'g', undefined)).toBe(42);
    expect(calls).toEqual([0]);
    expect(stale(qc, keys.materials('g', {}))).toBe(false);
  });

  test('no events: nothing is invalidated', async () => {
    const { api } = fakeApi({}, 5);
    const qc = seededClient();
    expect(await pollGroupChanges(api, qc, 'g', 5)).toBe(5);
    expect(stale(qc, keys.materials('g', {}))).toBe(false);
  });

  test('new events invalidate the group and touched materials, following pages', async () => {
    const { api, calls } = fakeApi(
      {
        5: {
          events: [{ entity_type: 'material', entity_id: 'm1' }],
          next_seq: 6,
          latest: 7,
          has_more: true,
        },
        6: {
          events: [{ entity_type: 'drive_connection', entity_id: 'c' }],
          next_seq: 7,
          latest: 7,
          has_more: false,
        },
      },
      7,
    );
    const qc = seededClient();
    expect(await pollGroupChanges(api, qc, 'g', 5)).toBe(7);
    expect(calls).toEqual([5, 6]);
    expect(stale(qc, keys.materials('g', {}))).toBe(true);
    expect(stale(qc, keys.material('m1'))).toBe(true);
    expect(stale(qc, keys.material('m2'))).toBe(false);
    expect(stale(qc, keys.group('other'))).toBe(false);
    expect(stale(qc, keys.sync('g'))).toBe(false);
  });

  test('pruned history (410) refreshes the group and restarts from the end', async () => {
    const { api } = fakeApi({ 5: 410 }, 90);
    const qc = seededClient();
    expect(await pollGroupChanges(api, qc, 'g', 5)).toBe(90);
    expect(stale(qc, keys.materials('g', {}))).toBe(true);
  });

  test('other errors propagate', async () => {
    const { api } = fakeApi({ 5: 500 }, 5);
    await expect(pollGroupChanges(api, seededClient(), 'g', 5)).rejects.toBeInstanceOf(ApiError);
  });
});
