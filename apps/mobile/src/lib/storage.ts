import AsyncStorage from '@react-native-async-storage/async-storage';
import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

import type { TokenStore, Tokens } from '@heatseeker/api-client';
import type { GroupIdStore } from '@heatseeker/core';

const TOKENS_KEY = 'heatseeker.tokens';
const GROUP_KEY = 'heatseeker.current_group';

/**
 * Токены — в защищённом хранилище (Keychain/Keystore). На вебе SecureStore
 * недоступен, используем localStorage (веб — этап 2, там появится своё решение).
 */
export const tokenStore: TokenStore = {
  async get(): Promise<Tokens | null> {
    const raw =
      Platform.OS === 'web' ? globalThis.localStorage?.getItem(TOKENS_KEY) : await SecureStore.getItemAsync(TOKENS_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as Tokens;
    } catch {
      return null;
    }
  },
  async set(tokens) {
    if (Platform.OS === 'web') {
      if (tokens) globalThis.localStorage?.setItem(TOKENS_KEY, JSON.stringify(tokens));
      else globalThis.localStorage?.removeItem(TOKENS_KEY);
      return;
    }
    if (tokens) await SecureStore.setItemAsync(TOKENS_KEY, JSON.stringify(tokens));
    else await SecureStore.deleteItemAsync(TOKENS_KEY);
  },
};

/** Выбранная группа — обычное хранилище. */
export const groupStore: GroupIdStore = {
  get: () => AsyncStorage.getItem(GROUP_KEY),
  set: (id) => (id ? AsyncStorage.setItem(GROUP_KEY, id) : AsyncStorage.removeItem(GROUP_KEY)),
};
