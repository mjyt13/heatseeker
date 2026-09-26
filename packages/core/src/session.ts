import { create } from 'zustand';

import type { Session, TokenStore, Tokens, User } from '@heatseeker/api-client';

export type SessionStatus = 'loading' | 'anonymous' | 'authenticated';

export interface SessionState {
  status: SessionStatus;
  user: User | null;
  /** Группа, в которой пользователь сейчас работает (сохраняется между запусками). */
  currentGroupId: string | null;
  /** Это устройство на сервере: по нему регистрируется push-токен. */
  deviceId: string | null;
  /** Восстановить сессию из хранилища токенов при старте приложения. */
  bootstrap: (tokens: TokenStore, groupStore: GroupIdStore, deviceStore?: IdStore) => Promise<void>;
  /**
   * Принять сессию после регистрации/входа. newAccount: группа, запомненная
   * от прошлого аккаунта на этом устройстве, не переносится — у нового
   * аккаунта есть только группа из кода приглашения.
   */
  signIn: (session: Session, opts?: { newAccount?: boolean }) => Promise<void>;
  /** Обновить профиль в состоянии (после PATCH /me). */
  setUser: (user: User) => void;
  setCurrentGroup: (groupId: string | null) => Promise<void>;
  signOut: () => Promise<void>;
}

/** Хранилище одного идентификатора (AsyncStorage / localStorage). */
export interface IdStore {
  get(): Promise<string | null> | string | null;
  set(id: string | null): Promise<void> | void;
}

/** Хранилище выбранной группы. */
export type GroupIdStore = IdStore;

interface Stores {
  tokens: TokenStore | null;
  group: GroupIdStore | null;
  device: IdStore | null;
}

/**
 * Состояние сессии. Токены живут в TokenStore приложения (secure store),
 * здесь — только пользователь, статус и выбранная группа.
 */
export const useSession = create<SessionState>((set, get) => {
  const stores: Stores = { tokens: null, group: null, device: null };

  return {
    status: 'loading',
    user: null,
    currentGroupId: null,
    deviceId: null,

    async bootstrap(tokens, group, device) {
      stores.tokens = tokens;
      stores.group = group;
      stores.device = device ?? null;
      const [t, groupId, deviceId] = await Promise.all([
        tokens.get(),
        group.get(),
        device?.get() ?? null,
      ]);
      set({
        currentGroupId: groupId ?? null,
        deviceId: deviceId ?? null,
        status: t ? 'authenticated' : 'anonymous',
      });
    },

    async signIn(session, opts) {
      const next: Tokens = {
        accessToken: session.access_token,
        refreshToken: session.refresh_token,
      };
      await stores.tokens?.set(next);
      const remembered = opts?.newAccount ? null : get().currentGroupId;
      const joinedGroupId = session.joined?.group.id ?? remembered;
      if (joinedGroupId !== get().currentGroupId) await stores.group?.set(joinedGroupId ?? null);
      await stores.device?.set(session.device_id);
      set({
        status: 'authenticated',
        user: session.user,
        currentGroupId: joinedGroupId ?? null,
        deviceId: session.device_id,
      });
    },

    setUser(user) {
      set({ user });
    },

    async setCurrentGroup(groupId) {
      await stores.group?.set(groupId);
      set({ currentGroupId: groupId });
    },

    async signOut() {
      await stores.tokens?.set(null);
      set({ status: 'anonymous', user: null });
    },
  };
});
