import createClient, { type Middleware } from 'openapi-fetch';

import type { components, paths } from './schema';

export type { components, paths } from './schema';

/** Удобные алиасы схем ответов. */
export type Schemas = components['schemas'];
export type Session = Schemas['SessionDTO'];
export type User = Schemas['UserDTO'];
export type Group = Schemas['GroupDTO'];
export type Membership = Schemas['MembershipDTO'];
export type GroupWithMembership = Schemas['GroupWithMembershipDTO'];
export type Subject = Schemas['SubjectDTO'];
export type Tag = Schemas['TagDTO'];
export type QuickTag = Schemas['QuickTagDTO'];
export type GroupEvent = Schemas['EventDTO'];
export type Material = Schemas['MaterialDTO'];
export type ClassifiedMaterial = Schemas['ClassifiedDTO'];
export type MaterialDetails = Schemas['MaterialDetailsDTO'];
export type MaterialVersion = Schemas['MaterialVersionDTO'];
export type MaterialOpen = Schemas['OpenDTO'];
export type UploadTicket = Schemas['UploadTicketDTO'];
export type DriveStatus = Schemas['DriveStatusDTO'];
export type DriveConnection = Schemas['DriveConnectionDTO'];
export type DriveItem = Schemas['DriveItemDTO'];
export type ServerMeta = Schemas['MetaOutputBody'];
export type ApiErrorBody = Schemas['ErrorModel'];

/** Пара токенов, которую хранит клиент. */
export interface Tokens {
  accessToken: string;
  refreshToken: string;
}

/** Хранилище токенов, реализуется приложением (secure store на mobile, память/IndexedDB на web). */
export interface TokenStore {
  get(): Promise<Tokens | null> | Tokens | null;
  set(tokens: Tokens | null): Promise<void> | void;
}

export interface ClientOptions {
  baseUrl: string;
  tokens: TokenStore;
  /** Вызывается, когда refresh не удался: приложение должно показать экран входа. */
  onSignedOut?: () => void;
  fetch?: typeof fetch;
}

/** Ошибка API в виде исключения (для мутаций TanStack Query). */
export class ApiError extends Error {
  readonly status: number;
  readonly body: ApiErrorBody | undefined;
  constructor(status: number, body: ApiErrorBody | undefined) {
    super(body?.detail ?? body?.title ?? `HTTP ${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

/** Access-токен обновляется заранее, если до истечения осталось меньше минуты. */
const REFRESH_AHEAD_MS = 60_000;

/** Срок действия JWT (`exp`, мс) или null, если токен не JWT. Подпись не проверяется. */
export function tokenExpiresAt(accessToken: string): number | null {
  const payload = accessToken.split('.')[1];
  if (!payload) return null;
  try {
    const b64 = payload
      .replace(/-/g, '+')
      .replace(/_/g, '/')
      .padEnd(Math.ceil(payload.length / 4) * 4, '=');
    const claims = JSON.parse(atob(b64)) as { exp?: unknown };
    return typeof claims.exp === 'number' ? claims.exp * 1000 : null;
  } catch {
    return null;
  }
}

/**
 * Создаёт типизированный клиент. Подставляет bearer-токен и обновляет пару
 * токенов через /auth/refresh заранее (до истечения меньше минуты) или при 401
 * с повтором запроса; refresh выполняется единожды на все параллельные запросы.
 */
export function createApiClient(options: ClientOptions) {
  const client = createClient<paths>({ baseUrl: options.baseUrl, fetch: options.fetch });
  const doFetch = (input: Request | string, init?: RequestInit) =>
    (options.fetch ?? fetch)(input, init);
  let refreshing: Promise<Tokens | null> | null = null;
  // Untouched copies of requests with a body: the original body is consumed by
  // the first attempt, so a retry after 401 is built from the copy.
  const replayable = new WeakMap<Request, Request>();

  async function refresh(): Promise<Tokens | null> {
    if (!refreshing) {
      refreshing = (async () => {
        const current = await options.tokens.get();
        if (!current) return null;
        const res = await doFetch(`${options.baseUrl}/auth/refresh`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: current.refreshToken }),
        });
        if (!res.ok) {
          await options.tokens.set(null);
          options.onSignedOut?.();
          return null;
        }
        const session = (await res.json()) as Session;
        const next = { accessToken: session.access_token, refreshToken: session.refresh_token };
        await options.tokens.set(next);
        return next;
      })().finally(() => {
        refreshing = null;
      });
    }
    return refreshing;
  }

  const auth: Middleware = {
    async onRequest({ request, schemaPath }) {
      if (schemaPath.startsWith('/auth/') && !schemaPath.startsWith('/auth/google'))
        return undefined;
      let tokens = await options.tokens.get();
      // A long upload or a request right at expiry would otherwise hit 401.
      const expiresAt = tokens ? tokenExpiresAt(tokens.accessToken) : null;
      if (expiresAt !== null && expiresAt - Date.now() < REFRESH_AHEAD_MS) tokens = await refresh();
      if (tokens) request.headers.set('Authorization', `Bearer ${tokens.accessToken}`);
      if (request.body !== null) replayable.set(request, request.clone());
      return request;
    },
    // Return undefined unless the response is replaced: openapi-fetch checks
    // `instanceof Response`, and React Native's fetch (expo/fetch) returns its
    // own response class, so even handing back the same object fails there.
    async onResponse({ request, response, schemaPath }) {
      if (response.status !== 401 || schemaPath.startsWith('/auth/')) return undefined;
      const next = await refresh();
      if (!next) return undefined;
      // The retry goes straight to fetch, bypassing this middleware, so it
      // cannot loop.
      const retry = new Request(replayable.get(request) ?? request, {
        headers: new Headers(request.headers),
      });
      retry.headers.set('Authorization', `Bearer ${next.accessToken}`);
      return asResponse(await doFetch(retry));
    },
  };
  client.use(auth);
  return client;
}

/**
 * Wraps a platform-specific response into the global Response class. The body
 * is passed as already decoded text: React Native's Response polyfill
 * (whatwg-fetch) turns an ArrayBuffer body into a string byte by byte, which
 * garbles any non-ASCII (UTF-8) JSON. API responses are JSON, so text is enough.
 */
async function asResponse(res: Response): Promise<Response> {
  // Typed as Response, but at runtime it may be another class.
  if ((res as unknown) instanceof Response) return res;
  const empty = res.status === 204 || res.status === 205 || res.status === 304;
  return new Response(empty ? null : await res.text(), {
    status: res.status,
    statusText: res.statusText,
    headers: res.headers,
  });
}

export type ApiClient = ReturnType<typeof createApiClient>;

/** Разворачивает результат openapi-fetch в данные или бросает ApiError. */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.error !== undefined || !result.response.ok) {
    throw new ApiError(result.response.status, result.error as ApiErrorBody | undefined);
  }
  return result.data as T;
}
