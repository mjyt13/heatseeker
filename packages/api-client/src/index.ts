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

/**
 * Создаёт типизированный клиент. Подставляет bearer-токен, при 401 один раз
 * обновляет пару токенов через /auth/refresh и повторяет запрос; refresh
 * выполняется единожды на все параллельные запросы.
 */
export function createApiClient(options: ClientOptions) {
  const client = createClient<paths>({ baseUrl: options.baseUrl, fetch: options.fetch });
  let refreshing: Promise<Tokens | null> | null = null;

  async function refresh(): Promise<Tokens | null> {
    if (!refreshing) {
      refreshing = (async () => {
        const current = await options.tokens.get();
        if (!current) return null;
        const res = await (options.fetch ?? fetch)(`${options.baseUrl}/auth/refresh`, {
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
      if (schemaPath.startsWith('/auth/') && !schemaPath.startsWith('/auth/google')) return request;
      const tokens = await options.tokens.get();
      if (tokens) request.headers.set('Authorization', `Bearer ${tokens.accessToken}`);
      return request;
    },
    async onResponse({ request, response, schemaPath }) {
      if (response.status !== 401 || schemaPath.startsWith('/auth/')) return response;
      if (request.headers.get('x-heatseeker-retried') === '1') return response;
      const next = await refresh();
      if (!next) return response;
      const retry = new Request(request, { headers: new Headers(request.headers) });
      retry.headers.set('Authorization', `Bearer ${next.accessToken}`);
      retry.headers.set('x-heatseeker-retried', '1');
      return (options.fetch ?? fetch)(retry);
    },
  };
  client.use(auth);
  return client;
}

export type ApiClient = ReturnType<typeof createApiClient>;

/** Разворачивает результат openapi-fetch в данные или бросает ApiError. */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.error !== undefined || !result.response.ok) {
    throw new ApiError(result.response.status, result.error as ApiErrorBody | undefined);
  }
  return result.data as T;
}
