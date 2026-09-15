import { expect, test } from 'vitest';

import { ApiError, createApiClient, unwrap, type Tokens } from '../src';

function memoryStore(initial: Tokens | null) {
  let value = initial;
  return {
    get: () => value,
    set: (t: Tokens | null) => {
      value = t;
    },
    current: () => value,
  };
}

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

test('refreshes once on 401 and retries with the new token', async () => {
  const calls: { url: string; auth: string | null }[] = [];
  const store = memoryStore({ accessToken: 'old', refreshToken: 'r1' });
  const fakeFetch: typeof fetch = async (input, init) => {
    const req = input instanceof Request ? input : new Request(input, init);
    const auth = req.headers.get('Authorization');
    calls.push({ url: new URL(req.url).pathname, auth });
    if (req.url.endsWith('/auth/refresh')) {
      return json(200, { access_token: 'new', refresh_token: 'r2', user: {} });
    }
    if (auth === 'Bearer old') return json(401, { title: 'Unauthorized', status: 401 });
    return json(200, { id: 'u1', name: 'x' });
  };
  const client = createApiClient({ baseUrl: 'http://api/api/v1', tokens: store, fetch: fakeFetch });
  const { data, response } = await client.GET('/me');
  expect(response.status).toBe(200);
  expect((data as { id: string }).id).toBe('u1');
  expect(calls.map((c) => c.url)).toEqual(['/api/v1/me', '/api/v1/auth/refresh', '/api/v1/me']);
  expect(store.current()?.accessToken).toBe('new');
});

test('signs out when refresh fails', async () => {
  let signedOut = false;
  const store = memoryStore({ accessToken: 'old', refreshToken: 'r1' });
  const fakeFetch: typeof fetch = async () => json(401, { status: 401 });
  const client = createApiClient({
    baseUrl: 'http://api/api/v1',
    tokens: store,
    fetch: fakeFetch,
    onSignedOut: () => {
      signedOut = true;
    },
  });
  const res = await client.GET('/me');
  expect(res.response.status).toBe(401);
  expect(signedOut).toBe(true);
  expect(store.current()).toBeNull();
  expect(() => unwrap(res)).toThrow(ApiError);
});
