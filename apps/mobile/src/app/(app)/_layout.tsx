import Ionicons from '@expo/vector-icons/Ionicons';
import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';
import type { ColorValue } from 'react-native';

import {
  useCurrentGroupGuard,
  useDiscussions,
  useGroupChanges,
  useOutboxFlusher,
  useSession,
} from '@heatseeker/core';
import { useTheme } from '@heatseeker/ui';

type IconName = keyof typeof Ionicons.glyphMap;

const icon =
  (name: IconName) =>
  ({ color, size }: { color: ColorValue; size: number }) => (
    <Ionicons name={name} color={color as string} size={size} />
  );

/** Нижние вкладки: Задачи / Лента / Расписание / Обсуждения / Ещё. */
export default function AppLayout() {
  const { t } = useTranslation();
  const theme = useTheme();
  const groupId = useSession((st) => st.currentGroupId);
  // Another account on this phone: forget a group it is not a member of.
  useCurrentGroupGuard();
  // Picks up changes made elsewhere (other members, the Drive worker).
  useGroupChanges(groupId);
  // Sends messages written offline once the network is back.
  useOutboxFlusher();
  const unread = useDiscussions(groupId).data?.unread ?? 0;
  return (
    <Tabs
      // "Back" returns where you came from (a discussion → the discussions list),
      // not to the first tab.
      backBehavior="history"
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: theme.color10?.val,
        tabBarStyle: {
          backgroundColor: theme.background?.val,
          borderTopColor: theme.borderColor?.val,
        },
      }}
    >
      <Tabs.Screen
        name="tasks"
        options={{ title: t('tabs.tasks'), tabBarIcon: icon('checkbox-outline') }}
      />
      <Tabs.Screen
        name="feed"
        options={{ title: t('tabs.feed'), tabBarIcon: icon('home-outline') }}
      />
      <Tabs.Screen
        name="schedule"
        options={{ title: t('tabs.schedule'), tabBarIcon: icon('calendar-outline') }}
      />
      <Tabs.Screen
        name="threads"
        options={{
          title: t('tabs.threads'),
          tabBarIcon: icon('chatbubbles-outline'),
          tabBarBadge: unread > 0 ? (unread > 99 ? '99+' : unread) : undefined,
        }}
      />
      <Tabs.Screen
        name="more"
        options={{ title: t('tabs.more'), tabBarIcon: icon('ellipsis-horizontal') }}
      />
      <Tabs.Screen name="index" options={{ href: null }} />
      <Tabs.Screen name="groups" options={{ href: null }} />
      <Tabs.Screen name="material/[id]" options={{ href: null }} />
      <Tabs.Screen name="upload" options={{ href: null }} />
      <Tabs.Screen name="inbox" options={{ href: null }} />
      <Tabs.Screen name="drive" options={{ href: null }} />
      <Tabs.Screen name="activity" options={{ href: null }} />
      <Tabs.Screen name="subjects" options={{ href: null }} />
      <Tabs.Screen name="task/[id]" options={{ href: null }} />
      <Tabs.Screen name="task/new" options={{ href: null }} />
      <Tabs.Screen name="discussion/[type]/[id]" options={{ href: null }} />
      <Tabs.Screen name="discussion/hidden" options={{ href: null }} />
    </Tabs>
  );
}
