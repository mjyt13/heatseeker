import { createContext, useContext, type ReactNode } from 'react';

import type { ApiClient } from '@heatseeker/api-client';

const ApiContext = createContext<ApiClient | null>(null);

/** Даёт всем хукам доступ к одному экземпляру клиента API. */
export function ApiProvider({ client, children }: { client: ApiClient; children: ReactNode }) {
  return <ApiContext.Provider value={client}>{children}</ApiContext.Provider>;
}

/** Клиент API из контекста; бросает, если провайдер не смонтирован. */
export function useApi(): ApiClient {
  const client = useContext(ApiContext);
  if (!client) throw new Error('useApi must be used inside <ApiProvider>');
  return client;
}
