import { create } from 'zustand';

import type { Session, TokenStore, Tokens, User } from '@heatseeker/api-client';

export type SessionStatus = 'loading' | 'anonymous' | 'authenticated';

export interface SessionState {
  status: SessionStatus;
  user: User | null;
  /** Группа, в которой пользователь сейчас работает (сохраняется между запусками). */
  currentGroupId: string | null;
  /** Восстановить сессию из хранилища токенов при старте приложения. */
  bootstrap: (tokens: TokenStore, groupStore: GroupIdStore) => Promise<void>;
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

/** Хранилище выбранной группы (AsyncStorage / localStorage). */
export interface GroupIdStore {
  get(): Promise<string | null> | string | null;
  set(id: string | null): Promise<void> | void;
}

interface Stores {
  tokens: TokenStore | null;
  group: GroupIdStore | null;
}

/**
 * Состояние сессии. Токены живут в TokenStore приложения (secure store),
 * здесь — только пользователь, статус и выбранная группа.
 */
export const useSession = create<SessionState>((set, get) => {
  const stores: Stores = { tokens: null, group: null };

  return {
    status: 'loading',
    user: null,
    currentGroupId: null,

    async bootstrap(tokens, group) {
      stores.tokens = tokens;
      stores.group = group;
      const [t, groupId] = await Promise.all([tokens.get(), group.get()]);
      set({ currentGroupId: groupId ?? null, status: t ? 'authenticated' : 'anonymous' });
    },

    async signIn(session, opts) {
      const next: Tokens = { accessToken: session.access_token, refreshToken: session.refresh_token };
      await stores.tokens?.set(next);
      const remembered = opts?.newAccount ? null : get().currentGroupId;
      const joinedGroupId = session.joined?.group.id ?? remembered;
      if (joinedGroupId !== get().currentGroupId) await stores.group?.set(joinedGroupId ?? null);
      set({ status: 'authenticated', user: session.user, currentGroupId: joinedGroupId ?? null });
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
