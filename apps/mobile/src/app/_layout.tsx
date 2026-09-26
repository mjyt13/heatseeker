import '@/lib/i18n';

import Ionicons from '@expo/vector-icons/Ionicons';
import { PersistQueryClientProvider } from '@tanstack/react-query-persist-client';
import * as Font from 'expo-font';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { useColorScheme } from 'react-native';

import { ApiProvider, useOutbox, useSession } from '@heatseeker/core';
import { TamaguiProvider, tamaguiConfig } from '@heatseeker/ui';

import { api } from '@/lib/api';
import { QUERY_CACHE_BUSTER, queryClient, queryPersister } from '@/lib/query';
import { deviceStore, groupStore, outboxStore, tokenStore } from '@/lib/storage';

void SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const colorScheme = useColorScheme();
  const status = useSession((s) => s.status);
  const bootstrap = useSession((s) => s.bootstrap);

  useEffect(() => {
    void bootstrap(tokenStore, groupStore, deviceStore).finally(() => SplashScreen.hideAsync());
    void useOutbox.getState().configure(outboxStore);
    // В dev шрифт иконок приходит по сети от Metro, и на новом устройстве его
    // ещё нет в кеше: грузим сами, чтобы неудача попала в лог понятной строкой,
    // а не необработанной ошибкой промиса. В собранном приложении он внутри.
    Font.loadAsync(Ionicons.font).catch((err: unknown) => {
      console.warn('иконки: шрифт не загрузился с Metro', err);
    });
  }, [bootstrap]);

  // Unsent messages belong to the account that wrote them.
  useEffect(() => {
    if (status === 'anonymous') useOutbox.getState().clear();
  }, [status]);

  return (
    <TamaguiProvider
      config={tamaguiConfig}
      defaultTheme={colorScheme === 'dark' ? 'dark' : 'light'}
    >
      <PersistQueryClientProvider
        client={queryClient}
        persistOptions={{ persister: queryPersister, buster: QUERY_CACHE_BUSTER }}
      >
        <ApiProvider client={api}>
          <StatusBar style="auto" />
          <Stack screenOptions={{ headerShown: false }}>
            <Stack.Protected guard={status === 'authenticated'}>
              <Stack.Screen name="(app)" />
            </Stack.Protected>
            <Stack.Protected guard={status === 'anonymous'}>
              <Stack.Screen name="(auth)" />
            </Stack.Protected>
          </Stack>
        </ApiProvider>
      </PersistQueryClientProvider>
    </TamaguiProvider>
  );
}
