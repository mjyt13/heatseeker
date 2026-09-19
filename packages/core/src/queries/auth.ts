import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { unwrap, type Session, type User } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { useSession } from '../session';
import { keys } from './keys';

export interface RegisterInput {
  name: string;
  invite_code?: string;
  /**
   * Случайный UUID, созданный при открытии экрана регистрации: повтор запроса
   * (двойное нажатие, потерянный ответ) вернёт тот же аккаунт.
   */
  client_id?: string;
  locale?: string;
  timezone?: string;
  platform?: 'IOS' | 'ANDROID' | 'WEB';
  device_name?: string;
}

/** Текущий пользователь (только когда сессия аутентифицирована). */
export function useMe() {
  const api = useApi();
  const status = useSession((s) => s.status);
  const setUser = useSession((s) => s.setUser);
  return useQuery({
    queryKey: keys.me(),
    enabled: status === 'authenticated',
    queryFn: async () => {
      const user = unwrap(await api.GET('/me'));
      setUser(user);
      return user;
    },
  });
}

/** Регистрация по имени (+ код группы). После успеха сессия сохраняется. */
export function useRegister() {
  const api = useApi();
  const signIn = useSession((s) => s.signIn);
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: RegisterInput): Promise<Session> =>
      unwrap(await api.POST('/auth/register', { body })),
    onSuccess: async (session) => {
      await signIn(session);
      qc.setQueryData(keys.me(), session.user);
      await qc.invalidateQueries({ queryKey: keys.myGroups() });
    },
  });
}

/** Вход по email и паролю (защищённый аккаунт). */
export function useLogin() {
  const api = useApi();
  const signIn = useSession((s) => s.signIn);
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      email: string;
      password: string;
      platform?: 'IOS' | 'ANDROID' | 'WEB';
    }): Promise<Session> => unwrap(await api.POST('/auth/login', { body })),
    onSuccess: async (session) => {
      await signIn(session);
      qc.setQueryData(keys.me(), session.user);
      await qc.invalidateQueries({ queryKey: keys.myGroups() });
    },
  });
}

/** «Защитить аккаунт»: задать email и/или пароль. */
export function useSetCredentials() {
  const api = useApi();
  const setUser = useSession((s) => s.setUser);
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { email?: string; password?: string }): Promise<User> =>
      unwrap(await api.POST('/me/credentials', { body })),
    onSuccess: (user) => {
      setUser(user);
      qc.setQueryData(keys.me(), user);
    },
  });
}

/** Обновление профиля. */
export function useUpdateProfile() {
  const api = useApi();
  const setUser = useSession((s) => s.setUser);
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      name?: string;
      locale?: string;
      timezone?: string;
      settings?: Record<string, unknown>;
    }): Promise<User> => unwrap(await api.PATCH('/me', { body })),
    onSuccess: (user) => {
      setUser(user);
      qc.setQueryData(keys.me(), user);
    },
  });
}

/** Выход: отзывает refresh-токен на сервере и чистит локальную сессию. */
export function useLogout(getRefreshToken: () => Promise<string | null> | string | null) {
  const api = useApi();
  const signOut = useSession((s) => s.signOut);
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const refresh = await getRefreshToken();
      if (refresh) await api.POST('/auth/logout', { body: { refresh_token: refresh } });
    },
    onSettled: async () => {
      await signOut();
      qc.clear();
    },
  });
}
