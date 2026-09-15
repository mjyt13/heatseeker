import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { eventLabelKey, useActivity, useCurrentGroup, useQuickTags, useSession } from '@heatseeker/core';
import { Button, Chip, EmptyState, H3, ListRow, LoadingScreen, ScrollView, XStack, YStack } from '@heatseeker/ui';

/** Главный экран: чипы быстрых тегов сверху, лента ниже. */
export default function FeedScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const currentGroupId = useSession((s) => s.currentGroupId);
  const group = useCurrentGroup();
  const quickTags = useQuickTags(currentGroupId);
  const activity = useActivity(currentGroupId, 30);
  const [selected, setSelected] = useState<string | null>(null);

  if (!currentGroupId) return <Redirect href="/(app)/groups" />;
  if (group.isPending) return <LoadingScreen />;

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack alignItems="center" justifyContent="space-between" paddingHorizontal="$4" paddingVertical="$2">
          <H3 numberOfLines={1} flex={1}>
            {group.data?.group.name ?? '…'}
          </H3>
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="swap-horizontal-outline" size={18} />}
            onPress={() => router.push('/(app)/groups')}
          >
            {t('groups.switch')}
          </Button>
        </XStack>

        <ScrollView horizontal showsHorizontalScrollIndicator={false} flexGrow={0}>
          <XStack gap="$2" paddingHorizontal="$4" paddingVertical="$2">
            {(quickTags.data ?? []).map((tag) => (
              <Chip
                key={tag.key}
                label={tag.kind === 'SYSTEM' ? t(tag.label) : tag.label}
                color={tag.color}
                selected={selected === tag.key}
                onPress={() => setSelected(selected === tag.key ? null : tag.key)}
              />
            ))}
          </XStack>
        </ScrollView>

        <ScrollView flex={1} contentContainerStyle={{ paddingHorizontal: 16, paddingBottom: 24 }}>
          {activity.data && activity.data.length > 0 ? (
            activity.data.map((e) => (
              <ListRow
                key={e.id}
                title={t(eventLabelKey(e.kind), { defaultValue: e.kind })}
                subtitle={new Date(e.created_at).toLocaleString()}
              />
            ))
          ) : (
            <EmptyState title={t('common.empty')} hint={t('feed.empty_hint')} />
          )}
        </ScrollView>
      </YStack>
    </SafeAreaView>
  );
}
