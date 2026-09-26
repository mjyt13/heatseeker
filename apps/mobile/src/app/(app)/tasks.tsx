import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useDeferredValue, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FlatList, RefreshControl } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  isClosed,
  myStatus,
  useSetMyTaskStatus,
  useTaskBoard,
  useTasks,
  type Task,
} from '@heatseeker/core';
import {
  Button,
  Chip,
  EmptyState,
  ErrorText,
  H3,
  Input,
  LoadingScreen,
  Paragraph,
  ScrollView,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { NotificationsBell } from '@/components/notifications-bell';
import { SubjectPicker } from '@/components/materials';
import { TaskRow } from '@/components/tasks';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Вкладка «Задачи»: сводка, фильтры, список и свой прогресс одним нажатием. */
export default function TasksScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, permissions, subjectById, subjects } = useGroupContext();
  const [mine, setMine] = useState(false);
  const [open, setOpen] = useState(true);
  const [overdue, setOverdue] = useState(false);
  const [subjectId, setSubjectId] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const deferredQuery = useDeferredValue(query);

  const tasks = useTasks(groupId, {
    mine,
    open,
    overdue,
    subject_id: subjectId ?? undefined,
    q: deferredQuery,
  });
  const board = useTaskBoard(groupId);
  const setMyStatus = useSetMyTaskStatus(groupId ?? '');

  if (!groupId) return <Redirect href="/(app)/groups" />;

  const items = tasks.data ?? [];
  const counts = board.data;
  const toggleMine = (task: Task) =>
    setMyStatus.mutate({ taskId: task.id, status: isClosed(myStatus(task)) ? 'TODO' : 'DONE' });

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack
          paddingHorizontal="$4"
          paddingTop="$2"
          alignItems="center"
          justifyContent="space-between"
        >
          <H3 flex={1}>{t('tasks.title')}</H3>
          {counts ? (
            <Paragraph size="$2" color="$color10">
              {[
                `${t('tasks.board_open')}: ${counts.open}`,
                counts.overdue ? `${t('tasks.board_overdue')}: ${counts.overdue}` : null,
                counts.due_soon ? `${t('tasks.board_soon')}: ${counts.due_soon}` : null,
              ]
                .filter(Boolean)
                .join(' · ')}
            </Paragraph>
          ) : null}
          <NotificationsBell groupId={groupId} />
        </XStack>

        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          flexGrow={0}
          flexShrink={0}
          keyboardShouldPersistTaps="handled"
        >
          <XStack gap="$2" paddingHorizontal="$4" paddingVertical="$2">
            <Chip
              label={t('tasks.filter_mine')}
              badge={counts?.mine}
              selected={mine}
              onPress={() => setMine((v) => !v)}
            />
            <Chip
              label={t('tasks.filter_open')}
              selected={open}
              onPress={() => setOpen((v) => !v)}
            />
            <Chip
              label={t('tasks.filter_overdue')}
              badge={counts?.overdue}
              selected={overdue}
              onPress={() => setOverdue((v) => !v)}
            />
          </XStack>
        </ScrollView>

        <XStack paddingHorizontal="$4" paddingBottom="$2">
          <Input
            flex={1}
            size="$3"
            value={query}
            onChangeText={setQuery}
            placeholder={t('tasks.search_placeholder')}
            returnKeyType="search"
            clearButtonMode="while-editing"
            autoCorrect={false}
          />
        </XStack>
        <XStack paddingHorizontal="$4" paddingBottom="$2">
          <SubjectPicker
            subjects={subjects}
            value={subjectId}
            onChange={setSubjectId}
            emptyLabel={t('common.all')}
          />
        </XStack>

        {tasks.isPending ? (
          <LoadingScreen />
        ) : (
          <FlatList
            data={items}
            keyExtractor={(task) => task.id}
            keyboardShouldPersistTaps="handled"
            keyboardDismissMode="on-drag"
            contentContainerStyle={{ paddingHorizontal: 8, paddingBottom: 96, flexGrow: 1 }}
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
                <EmptyState
                  title={t('tasks.empty')}
                  hint={
                    mine || overdue || subjectId || deferredQuery
                      ? t('tasks.empty_filtered')
                      : t('tasks.empty_hint')
                  }
                />
              )
            }
          />
        )}

        {permissions.can('task.create') ? (
          <YStack position="absolute" right="$4" bottom="$4">
            <Button
              theme="accent"
              size="$5"
              borderRadius="$10"
              icon={<Ionicons name="add" size={20} />}
              onPress={() => router.push('/(app)/task/new')}
            >
              {t('tasks.add')}
            </Button>
          </YStack>
        ) : null}
      </YStack>
    </SafeAreaView>
  );
}
