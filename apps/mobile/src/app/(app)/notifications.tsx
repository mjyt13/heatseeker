import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter, type Href } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FlatList, RefreshControl } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  useMarkNotificationsRead,
  useNotifications,
  type Notification,
} from '@heatseeker/core';
import {
  Button,
  Chip,
  EmptyState,
  ErrorText,
  H3,
  LoadingScreen,
  Paragraph,
  SizableText,
  Spinner,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';
import { notificationIcon, notificationRoute } from '@/lib/notifications';

/** Экран «Уведомления»: что произошло, пока меня не было. */
export default function NotificationsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId } = useGroupContext();
  const [unreadOnly, setUnreadOnly] = useState(false);
  const list = useNotifications(groupId, unreadOnly);
  const markRead = useMarkNotificationsRead(groupId ?? '');

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (list.isPending) return <LoadingScreen />;

  const items = list.data?.pages.flatMap((p) => p.items ?? []) ?? [];
  const unread = list.data?.pages[0]?.unread ?? 0;

  const open = (n: Notification) => {
    if (!n.read_at) markRead.mutate({ ids: [n.id] });
    const route = notificationRoute(n.data);
    if (route) router.push(route as Href);
  };

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack alignItems="center" gap="$2" paddingHorizontal="$2" paddingTop="$2">
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="chevron-back" size={20} />}
            onPress={() => router.back()}
          />
          <H3 flex={1}>{t('notifications.title')}</H3>
          {list.isFetching ? <Spinner size="small" /> : null}
          <Button
            size="$3"
            chromeless
            aria-label={t('notifications.settings')}
            icon={<Ionicons name="options-outline" size={20} />}
            onPress={() => router.push('/(app)/notification-settings')}
          />
        </XStack>

        <XStack gap="$2" paddingHorizontal="$4" paddingVertical="$2" alignItems="center">
          <Chip
            label={t('notifications.unread_only')}
            selected={unreadOnly}
            onPress={() => setUnreadOnly((v) => !v)}
          />
          {unread > 0 ? (
            <Button
              size="$2"
              chromeless
              disabled={markRead.isPending}
              onPress={() => markRead.mutate({ all: true })}
            >
              {t('notifications.mark_all')}
            </Button>
          ) : null}
        </XStack>

        <ErrorText>{list.isError ? describeError(t, list.error) : null}</ErrorText>

        <FlatList
          data={items}
          keyExtractor={(n) => n.id}
          contentContainerStyle={{ paddingBottom: 32 }}
          refreshControl={
            <RefreshControl refreshing={list.isRefetching} onRefresh={() => void list.refetch()} />
          }
          ListEmptyComponent={
            <EmptyState
              title={unreadOnly ? t('notifications.empty_unread') : t('notifications.empty')}
              hint={unreadOnly ? undefined : t('notifications.empty_hint')}
            />
          }
          ListFooterComponent={
            list.hasNextPage ? (
              <Button
                chromeless
                disabled={list.isFetchingNextPage}
                onPress={() => void list.fetchNextPage()}
              >
                {t('notifications.load_more')}
              </Button>
            ) : null
          }
          renderItem={({ item }) => <NotificationRow notification={item} onPress={() => open(item)} />}
        />
      </YStack>
    </SafeAreaView>
  );
}

/** Строка списка: иконка по типу, точка у непрочитанного. */
function NotificationRow({
  notification: n,
  onPress,
}: {
  notification: Notification;
  onPress: () => void;
}) {
  const unread = !n.read_at;
  return (
    <XStack
      gap="$3"
      paddingVertical="$2.5"
      paddingHorizontal="$4"
      alignItems="flex-start"
      backgroundColor={unread ? '$color2' : '$background'}
      pressStyle={{ backgroundColor: '$backgroundPress' }}
      role="button"
      onPress={onPress}
    >
      <Ionicons name={notificationIcon(n.type)} size={22} />
      <YStack flex={1} gap="$0.5">
        <SizableText size="$4" fontWeight={unread ? '700' : '400'} numberOfLines={2}>
          {n.title}
        </SizableText>
        {n.body ? (
          <Paragraph size="$2" color="$color10" numberOfLines={3}>
            {n.body}
          </Paragraph>
        ) : null}
        <SizableText size="$1" color="$color10">
          {new Date(n.created_at).toLocaleString()}
        </SizableText>
      </YStack>
      {unread ? (
        <YStack width={8} height={8} borderRadius={4} backgroundColor="$blue9" marginTop="$2" />
      ) : null}
    </XStack>
  );
}
