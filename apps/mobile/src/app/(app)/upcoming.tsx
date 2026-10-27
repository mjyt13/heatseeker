import { Redirect, useRouter } from 'expo-router';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { RefreshControl, SectionList } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  groupUpcoming,
  isClosed,
  myStatus,
  upcomingDueBefore,
  useSetMyTaskStatus,
  useTasks,
  type Task,
} from '@heatseeker/core';
import {
  Button,
  EmptyState,
  ErrorText,
  H3,
  LoadingScreen,
  SizableText,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { NotificationsBell } from '@/components/notifications-bell';
import { TaskRow } from '@/components/tasks';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/**
 * Первый экран: мои незакрытые задачи со сроком на две недели вперёд,
 * по разделам «Просрочено / Сегодня / Завтра / На этой неделе / Позже».
 */
export default function UpcomingScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, subjectById } = useGroupContext();
  // Rounded to the end of a day, so the query key stays put while the screen is open.
  const dueBefore = upcomingDueBefore();
  const tasks = useTasks(groupId, { mine: true, open: true, due_before: dueBefore });
  const setMyStatus = useSetMyTaskStatus(groupId ?? '');
  const sections = useMemo(() => groupUpcoming(tasks.data ?? []), [tasks.data]);

  if (!groupId) return <Redirect href="/(app)/groups" />;

  const toggleMine = (task: Task) =>
    setMyStatus.mutate({ taskId: task.id, status: isClosed(myStatus(task)) ? 'TODO' : 'DONE' });
  const allTasks = (
    <XStack padding="$4" justifyContent="center">
      <Button chromeless onPress={() => router.push('/(app)/tasks')}>
        {t('upcoming.all_tasks')}
      </Button>
    </XStack>
  );

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack
          paddingHorizontal="$4"
          paddingTop="$2"
          paddingBottom="$2"
          alignItems="center"
          justifyContent="space-between"
        >
          <H3 flex={1}>{t('upcoming.title')}</H3>
          <NotificationsBell groupId={groupId} />
        </XStack>

        {tasks.isPending ? (
          <LoadingScreen />
        ) : (
          <SectionList
            sections={sections.map((s) => ({ ...s, key: s.bucket }))}
            keyExtractor={(task) => task.id}
            stickySectionHeadersEnabled={false}
            contentContainerStyle={{ paddingHorizontal: 8, paddingBottom: 24, flexGrow: 1 }}
            renderSectionHeader={({ section }) => (
              <SizableText
                size="$3"
                fontWeight="600"
                paddingHorizontal="$2"
                paddingTop="$3"
                paddingBottom="$1"
                color={section.bucket === 'overdue' ? '$red10' : '$color10'}
              >
                {t(`upcoming.${section.bucket}`)}
              </SizableText>
            )}
            renderItem={({ item }) => (
              <TaskRow
                task={item}
                subject={item.subject_id ? subjectById.get(item.subject_id) : undefined}
                onPress={() =>
                  router.push({ pathname: '/(app)/task/[id]', params: { id: item.id } })
                }
                onToggleMine={() => toggleMine(item)}
              />
            )}
            refreshControl={
              <RefreshControl
                refreshing={tasks.isRefetching}
                onRefresh={() => void tasks.refetch()}
              />
            }
            ListEmptyComponent={
              tasks.isError ? (
                <YStack padding="$4" gap="$2">
                  <ErrorText>{describeError(t, tasks.error)}</ErrorText>
                  <Button onPress={() => void tasks.refetch()}>{t('common.retry')}</Button>
                </YStack>
              ) : (
                <EmptyState title={t('upcoming.empty')} hint={t('upcoming.empty_hint')} />
              )
            }
            ListFooterComponent={tasks.isError ? null : allTasks}
          />
        )}
      </YStack>
    </SafeAreaView>
  );
}
