import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { GroupEvent } from '@heatseeker/api-client';
import { eventLabelKey, useActivity } from '@heatseeker/core';
import {
  Button,
  EmptyState,
  ErrorText,
  ListRow,
  LoadingScreen,
  Screen,
  ScreenTitle,
  XStack,
} from '@heatseeker/ui';

import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Лента активности группы: кто что когда сделал. */
export default function ActivityScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const activity = useActivity(ctx.groupId, 100);

  const describe = (e: GroupEvent) => {
    const payload = (e.payload ?? {}) as Record<string, unknown>;
    const who = ctx.memberName(e.actor_id);
    const what =
      typeof payload.title === 'string'
        ? payload.title
        : typeof payload.name === 'string'
          ? payload.name
          : typeof payload.count === 'number'
            ? t('activity.files', { count: payload.count })
            : null;
    return [what, who, new Date(e.created_at).toLocaleString()].filter(Boolean).join(' · ');
  };

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <XStack alignItems="center" gap="$2">
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="chevron-back" size={20} />}
            onPress={() => router.back()}
          />
          <ScreenTitle title={t('activity.title')} />
        </XStack>
        {activity.isPending ? (
          <LoadingScreen />
        ) : (activity.data ?? []).length === 0 ? (
          <EmptyState title={t('common.empty')} />
        ) : (
          (activity.data ?? []).map((e) => (
            <ListRow
              key={e.id}
              title={t(eventLabelKey(e.kind), { defaultValue: e.kind })}
              subtitle={describe(e)}
              onPress={
                e.entity_type === 'material' && e.entity_id && e.kind !== 'material.deleted'
                  ? () =>
                      router.push({
                        pathname: '/(app)/material/[id]',
                        params: { id: e.entity_id! },
                      })
                  : undefined
              }
            />
          ))
        )}
        <ErrorText>{activity.isError ? describeError(t, activity.error) : null}</ErrorText>
      </Screen>
    </SafeAreaView>
  );
}
