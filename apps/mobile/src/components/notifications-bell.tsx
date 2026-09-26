import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { useUnreadNotifications } from '@heatseeker/core';
import { Button, SizableText, XStack, YStack } from '@heatseeker/ui';

/**
 * Колокольчик с числом непрочитанных — в заголовке каждой вкладки, чтобы
 * уведомление было видно на любом экране, а не только на ленте.
 */
export function NotificationsBell({ groupId }: { groupId: string | null | undefined }) {
  const { t } = useTranslation();
  const router = useRouter();
  const unread = useUnreadNotifications(groupId).data ?? 0;
  return (
    <YStack>
      <Button
        size="$3"
        chromeless
        aria-label={t('notifications.title')}
        icon={<Ionicons name={unread > 0 ? 'notifications' : 'notifications-outline'} size={20} />}
        onPress={() => router.push('/(app)/notifications')}
      />
      {unread > 0 ? (
        <XStack
          position="absolute"
          top={-2}
          right={-2}
          minWidth={18}
          height={18}
          paddingHorizontal="$1"
          borderRadius={9}
          backgroundColor="$red9"
          alignItems="center"
          justifyContent="center"
          pointerEvents="none"
        >
          <SizableText size="$1" color="white" fontWeight="700">
            {unread > 99 ? '99+' : unread}
          </SizableText>
        </XStack>
      ) : null}
    </YStack>
  );
}
