import { beforeEach, expect, test } from 'vitest';

import type { Session, Tokens } from '@heatseeker/api-client';

import { useSession, type GroupIdStore } from '../src/session';

function tokenStore() {
  let value: Tokens | null = null;
  return {
    get: () => value,
    set: (t: Tokens | null) => {
      value = t;
    },
    current: () => value,
  };
}

function groupStore(
  initial: string | null = null,
): GroupIdStore & { current: () => string | null } {
  let value = initial;
  return {
    get: () => value,
    set: (id) => {
      value = id;
    },
    current: () => value,
  };
}

const session = (joined?: string): Session =>
  ({
    access_token: 'a',
    access_expires_at: '2026-01-01T00:00:00Z',
    refresh_token: 'r',
    device_id: 'd',
    user: {
      id: 'u',
      name: 'X',
      locale: 'ru',
      timezone: 'UTC',
      global_role: 'USER',
      secured: false,
      has_password: false,
      settings: {},
      created_at: '',
    },
    ...(joined ? { joined: { group: { id: joined }, membership: {} } } : {}),
  }) as unknown as Session;

beforeEach(() => {
  useSession.setState({ status: 'loading', user: null, currentGroupId: null });
});

test('bootstrap without tokens → anonymous, keeps remembered group', async () => {
  const groups = groupStore('g-remembered');
  await useSession.getState().bootstrap(tokenStore(), groups);
  expect(useSession.getState().status).toBe('anonymous');
  expect(useSession.getState().currentGroupId).toBe('g-remembered');
});

test('signIn stores tokens and switches to the joined group', async () => {
  const tokens = tokenStore();
  const groups = groupStore(null);
  await useSession.getState().bootstrap(tokens, groups);
  await useSession.getState().signIn(session('g-joined'));
  expect(tokens.current()).toEqual({ accessToken: 'a', refreshToken: 'r' });
  expect(groups.current()).toBe('g-joined');
  expect(useSession.getState().status).toBe('authenticated');
  expect(useSession.getState().user?.name).toBe('X');
});

test('signOut clears tokens but not the remembered group', async () => {
  const tokens = tokenStore();
  const groups = groupStore('g1');
  await useSession.getState().bootstrap(tokens, groups);
  await useSession.getState().signIn(session());
  await useSession.getState().signOut();
  expect(tokens.current()).toBeNull();
  expect(useSession.getState().status).toBe('anonymous');
  expect(useSession.getState().currentGroupId).toBe('g1');
});

test('isActiveMemberOf: only an active membership of that group counts', async () => {
  const { isActiveMemberOf } = await import('../src/queries/groups');
  const gm = (id: string, status: 'ACTIVE' | 'PENDING' | 'BANNED') =>
    ({ group: { id }, membership: { status } }) as Parameters<typeof isActiveMemberOf>[0][number];
  const mine = [gm('g1', 'ACTIVE'), gm('g2', 'BANNED')];
  expect(isActiveMemberOf(mine, 'g1')).toBe(true);
  expect(isActiveMemberOf(mine, 'g2')).toBe(false);
  expect(isActiveMemberOf(mine, 'other-account-group')).toBe(false);
});

test('a new account does not inherit the group remembered on this device', async () => {
  const tokens = tokenStore();
  const groups = groupStore('someone-elses-group');
  await useSession.getState().bootstrap(tokens, groups);
  await useSession.getState().signIn({ ...session(), joined: undefined }, { newAccount: true });
  expect(useSession.getState().currentGroupId).toBeNull();
});
