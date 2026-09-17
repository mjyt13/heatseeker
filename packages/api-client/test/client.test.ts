import { afterEach, expect, test, vi } from 'vitest';

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

test('retries a POST with its body after refresh', async () => {
  const bodies: string[] = [];
  const store = memoryStore({ accessToken: 'old', refreshToken: 'r1' });
  const fakeFetch: typeof fetch = async (input, init) => {
    const req = input instanceof Request ? input : new Request(input, init);
    if (req.url.endsWith('/auth/refresh')) {
      return json(200, { access_token: 'new', refresh_token: 'r2', user: {} });
    }
    bodies.push(await req.text());
    expect(req.headers.get('x-heatseeker-retried')).toBeNull();
    if (req.headers.get('Authorization') === 'Bearer old') return json(401, { status: 401 });
    return json(201, { upload_id: 'u' });
  };
  const client = createApiClient({ baseUrl: 'http://api/api/v1', tokens: store, fetch: fakeFetch });
  const res = await client.POST('/groups/{groupId}/materials/uploads', {
    params: { path: { groupId: 'g' } },
    body: { file_name: 'a.pdf', size_bytes: 1, mime: 'application/pdf' } as never,
  });
  expect(res.response.status).toBe(201);
  expect(bodies).toHaveLength(2);
  expect(bodies[1]).toBe(bodies[0]);
  expect(JSON.parse(bodies[1]!)).toMatchObject({ file_name: 'a.pdf' });
});

test('accepts responses that are not instances of the global Response (expo/fetch)', async () => {
  // A minimal stand-in for a platform response class.
  class ForeignResponse {
    constructor(private inner: Response) {}
    get status() {
      return this.inner.status;
    }
    get statusText() {
      return this.inner.statusText;
    }
    get ok() {
      return this.inner.ok;
    }
    get headers() {
      return this.inner.headers;
    }
    get body() {
      return this.inner.body;
    }
    arrayBuffer() {
      return this.inner.arrayBuffer();
    }
    json() {
      return this.inner.json();
    }
    text() {
      return this.inner.text();
    }
    clone() {
      return new ForeignResponse(this.inner.clone());
    }
  }
  const store = memoryStore({ accessToken: 'old', refreshToken: 'r1' });
  const fakeFetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = input instanceof Request ? input : new Request(input, init);
    if (req.url.endsWith('/auth/refresh')) {
      return new ForeignResponse(json(200, { access_token: 'new', refresh_token: 'r2', user: {} }));
    }
    if (req.url.endsWith('/auth/register'))
      return new ForeignResponse(json(201, { access_token: 'a' }));
    if (req.headers.get('Authorization') === 'Bearer old')
      return new ForeignResponse(json(401, {}));
    return new ForeignResponse(json(200, { id: 'u1' }));
  }) as unknown as typeof fetch;
  const client = createApiClient({ baseUrl: 'http://api/api/v1', tokens: store, fetch: fakeFetch });

  const reg = await client.POST('/auth/register', { body: { name: 'x' } as never });
  expect(reg.response.status).toBe(201);
  expect(reg.error).toBeUndefined();

  const me = await client.GET('/me');
  expect(me.response.status).toBe(200);
  expect((me.data as { id: string }).id).toBe('u1');
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('keeps UTF-8 intact when wrapping a retried response (React Native polyfill)', async () => {
  // whatwg-fetch, React Native's Response, decodes ArrayBuffer bodies as Latin-1.
  const NativeResponse = Response;
  class PolyfillResponse extends NativeResponse {
    constructor(body?: BodyInit | null, init?: ResponseInit) {
      const latin1 =
        body instanceof ArrayBuffer
          ? Array.from(new Uint8Array(body), (b) => String.fromCharCode(b)).join('')
          : body;
      super(latin1, init);
    }
  }
  const foreign = (inner: Response) =>
    ({
      status: inner.status,
      statusText: inner.statusText,
      ok: inner.ok,
      headers: inner.headers,
      body: inner.body,
      arrayBuffer: () => inner.arrayBuffer(),
      text: () => inner.text(),
      json: () => inner.json(),
    }) as unknown as Response;
  const store = memoryStore({ accessToken: 'old', refreshToken: 'r1' });
  const fakeFetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = input instanceof Request ? input : new Request(input, init);
    if (req.url.endsWith('/auth/refresh')) {
      return foreign(json(200, { access_token: 'new', refresh_token: 'r2', user: {} }));
    }
    if (req.headers.get('Authorization') === 'Bearer old') return foreign(json(401, {}));
    return foreign(json(200, { id: 'u1', name: 'Психология · фрейд' }));
  }) as unknown as typeof fetch;
  const client = createApiClient({ baseUrl: 'http://api/api/v1', tokens: store, fetch: fakeFetch });
  vi.stubGlobal('Response', PolyfillResponse);

  const me = await client.GET('/me');
  expect(me.response.status).toBe(200);
  expect((me.data as { name: string }).name).toBe('Психология · фрейд');
});
