import Constants, { ExecutionEnvironment } from 'expo-constants';
import { router, type Href } from 'expo-router';
import { useEffect, useRef } from 'react';
import { Platform } from 'react-native';

import { keys, useRegisterPushToken, useSession } from '@heatseeker/core';
import { useQueryClient } from '@tanstack/react-query';

import { notificationRoute } from '@/lib/notifications';

// Типы берём из модуля, сам модуль подключается только там, где он работает.
import type * as Notifications from 'expo-notifications';

type NotificationsModule = typeof Notifications;

/**
 * Пуш работает только в собранном приложении на телефоне:
 * - в Expo Go на Android его вырезали (SDK 53+), любой вызов бросает;
 * - в вебе нужен свой канал (Web Push, отдельный этап).
 *
 * Поэтому `expo-notifications` подключается лениво: статический импорт в модуле,
 * который тянет за собой экран, ломает весь экран.
 */
const pushSupported =
  (Platform.OS === 'ios' || Platform.OS === 'android') &&
  !(Platform.OS === 'android' && Constants.executionEnvironment === ExecutionEnvironment.StoreClient);

/** Подключить модуль уведомлений; null — здесь пуша нет. */
async function load(): Promise<NotificationsModule | null> {
  if (!pushSupported) return null;
  try {
    return await import('expo-notifications');
  } catch {
    return null;
  }
}

/** id проекта EAS — без него Expo не выдаёт токен. */
function projectId(): string | undefined {
  const extra = Constants.expoConfig?.extra as { eas?: { projectId?: string } } | undefined;
  return extra?.eas?.projectId ?? Constants.easConfig?.projectId;
}

/**
 * Каналы Android: телефон сам решает, что звенит, а что приходит тихо.
 * Имена совпадают с channelId, который ставит сервер (adapters/push).
 */
async function setUpChannels(N: NotificationsModule) {
  if (Platform.OS !== 'android') return;
  const channels: { id: string; name: string; importance: number }[] = [
    { id: 'default', name: 'Общее', importance: N.AndroidImportance.DEFAULT },
    { id: 'messages', name: 'Сообщения', importance: N.AndroidImportance.HIGH },
    { id: 'deadlines', name: 'Сроки задач', importance: N.AndroidImportance.HIGH },
    { id: 'schedule', name: 'Расписание', importance: N.AndroidImportance.DEFAULT },
  ];
  for (const c of channels) {
    await N.setNotificationChannelAsync(c.id, { name: c.name, importance: c.importance });
  }
}

/**
 * Регистрирует устройство в push-сервисе Expo и обновляет счётчик, когда
 * уведомление приходит на открытое приложение.
 *
 * Там, где пуша нет (Expo Go на Android, веб), хук ничего не делает —
 * уведомления в списке приложения работают как обычно.
 */
export function usePushRegistration(groupId: string | null | undefined) {
  const deviceId = useSession((s) => s.deviceId);
  const status = useSession((s) => s.status);
  const register = useRegisterPushToken();
  const qc = useQueryClient();
  const done = useRef<string | null>(null);
  const { mutate } = register;

  useEffect(() => {
    if (status !== 'authenticated' || !deviceId || done.current === deviceId) return;
    done.current = deviceId;
    let alive = true;
    void (async () => {
      const N = await load();
      if (!N || !alive) return;
      try {
        // Пуш, пришедший при открытом приложении, всё равно показывается баннером.
        N.setNotificationHandler({
          handleNotification: async () => ({
            shouldShowBanner: true,
            shouldShowList: true,
            shouldPlaySound: false,
            shouldSetBadge: false,
          }),
        });
        await setUpChannels(N);
        const current = await N.getPermissionsAsync();
        const granted = current.granted || (await N.requestPermissionsAsync()).granted;
        if (!granted || !alive) return;
        const token = await N.getExpoPushTokenAsync({ projectId: projectId() });
        if (alive && token.data) mutate({ deviceId, token: token.data });
      } catch {
        // Нет разрешения или сборка без пуша: остаёмся на уведомлениях в приложении.
        done.current = null;
      }
    })();
    return () => {
      alive = false;
    };
  }, [status, deviceId, mutate]);

  // Пуш пришёл на открытое приложение — обновить список и бейдж; нажатие на
  // пуш открывает то, о чём он.
  useEffect(() => {
    if (!groupId) return;
    let received: { remove: () => void } | null = null;
    let tapped: { remove: () => void } | null = null;
    void (async () => {
      const N = await load();
      if (!N) return;
      try {
        received = N.addNotificationReceivedListener(() => {
          void qc.invalidateQueries({ queryKey: keys.notificationsAll(groupId) });
        });
        tapped = N.addNotificationResponseReceivedListener((response) => {
          const route = notificationRoute(response.notification.request.content.data);
          if (route) router.push(route as Href);
        });
      } catch {
        // Слушать нечего: пуша в этой сборке нет.
      }
    })();
    return () => {
      received?.remove();
      tapped?.remove();
    };
  }, [groupId, qc]);
}
