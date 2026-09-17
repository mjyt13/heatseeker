import '@/lib/i18n';

import { PersistQueryClientProvider } from '@tanstack/react-query-persist-client';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { useColorScheme } from 'react-native';

import { ApiProvider, useSession } from '@heatseeker/core';
import { TamaguiProvider, tamaguiConfig } from '@heatseeker/ui';

import { api } from '@/lib/api';
import { QUERY_CACHE_BUSTER, queryClient, queryPersister } from '@/lib/query';
import { groupStore, tokenStore } from '@/lib/storage';

void SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const colorScheme = useColorScheme();
  const status = useSession((s) => s.status);
  const bootstrap = useSession((s) => s.bootstrap);

  useEffect(() => {
    void bootstrap(tokenStore, groupStore).finally(() => SplashScreen.hideAsync());
  }, [bootstrap]);

  return (
    <TamaguiProvider config={tamaguiConfig} defaultTheme={colorScheme === 'dark' ? 'dark' : 'light'}>
      <PersistQueryClientProvider client={queryClient} persistOptions={{ persister: queryPersister, buster: QUERY_CACHE_BUSTER }}>
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
