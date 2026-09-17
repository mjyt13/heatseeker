import { createApiClient } from '@heatseeker/api-client';
import { useSession } from '@heatseeker/core';

import { tokenStore } from './storage';

/** Базовый URL API из env (EXPO_PUBLIC_* попадает в бандл). */
export const API_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8000/api/v1';

/** Единственный экземпляр клиента API на приложение. */
export const api = createApiClient({
  baseUrl: API_URL,
  tokens: tokenStore,
  onSignedOut: () => {
    void useSession.getState().signOut();
  },
});
