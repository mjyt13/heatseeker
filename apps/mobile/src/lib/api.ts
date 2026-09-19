import Constants from 'expo-constants';
import { Platform } from 'react-native';

import { createApiClient } from '@heatseeker/api-client';
import { useSession } from '@heatseeker/core';

import { tokenStore } from './storage';

/** Порт API в dev, когда адрес выводится из хоста Metro. */
const DEV_API_PORT = 8000;

/**
 * Базовый URL API. Главный источник — `EXPO_PUBLIC_API_URL` (попадает в бандл).
 * Без него в dev API ищется на той же машине, что и Metro: на телефоне это IP
 * из `exp://192.168.…:4173`, в вебе — хост, по которому открыта страница.
 */
function resolveApiUrl(): string {
  const fromEnv = process.env.EXPO_PUBLIC_API_URL;
  if (fromEnv) return fromEnv;
  let host = 'localhost';
  if (__DEV__) {
    const devHost =
      Platform.OS === 'web'
        ? globalThis.location?.hostname
        : Constants.expoConfig?.hostUri?.split(':')[0];
    if (devHost) host = devHost;
  }
  const hostPart = host.includes(':') ? `[${host}]` : host;
  return `http://${hostPart}:${DEV_API_PORT}/api/v1`;
}

export const API_URL = resolveApiUrl();

/** Адрес сервера без пути — для сообщений об ошибках сети. */
export const API_ORIGIN = API_URL.replace(/^(https?:\/\/[^/]+).*$/, '$1');

/** Единственный экземпляр клиента API на приложение. */
export const api = createApiClient({
  baseUrl: API_URL,
  tokens: tokenStore,
  onSignedOut: () => {
    void useSession.getState().signOut();
  },
});
