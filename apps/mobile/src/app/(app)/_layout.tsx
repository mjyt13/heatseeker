import Ionicons from '@expo/vector-icons/Ionicons';
import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';
import type { ColorValue } from 'react-native';

import { useTheme } from '@heatseeker/ui';

type IconName = keyof typeof Ionicons.glyphMap;

const icon =
  (name: IconName) =>
  ({ color, size }: { color: ColorValue; size: number }) => (
    <Ionicons name={name} color={color as string} size={size} />
  );

/** Нижние вкладки: Лента / Расписание / Задачи / Обсуждения / Ещё. */
export default function AppLayout() {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: theme.color10?.val,
        tabBarStyle: { backgroundColor: theme.background?.val, borderTopColor: theme.borderColor?.val },
      }}
    >
      <Tabs.Screen name="index" options={{ title: t('tabs.feed'), tabBarIcon: icon('home-outline') }} />
      <Tabs.Screen name="schedule" options={{ title: t('tabs.schedule'), tabBarIcon: icon('calendar-outline') }} />
      <Tabs.Screen name="tasks" options={{ title: t('tabs.tasks'), tabBarIcon: icon('checkbox-outline') }} />
      <Tabs.Screen name="threads" options={{ title: t('tabs.threads'), tabBarIcon: icon('chatbubbles-outline') }} />
      <Tabs.Screen name="more" options={{ title: t('tabs.more'), tabBarIcon: icon('ellipsis-horizontal') }} />
      <Tabs.Screen name="groups" options={{ href: null }} />
    </Tabs>
  );
}
