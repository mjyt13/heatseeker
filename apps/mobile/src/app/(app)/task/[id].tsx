import Ionicons from '@expo/vector-icons/Ionicons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  isClosed,
  myStatus,
  useDeleteTask,
  usePinTask,
  useSetMyTaskStatus,
  useSetTaskStatus,
  useTask,
  useUpdateTask,
} from '@heatseeker/core';
import {
  Button,
  confirm,
  EmptyState,
  ErrorText,
  H3,
  LoadingScreen,
  Paragraph,
  Screen,
  Separator,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { DiscussionButton } from '@/components/discussion-button';
import { SectionLabel } from '@/components/materials';
import { TaskFiles } from '@/components/task-files';
import { TaskForm } from '@/components/task-form';
import { ProgressLine, StatusPicker, useDueLabel } from '@/components/tasks';
import { describeError } from '@/lib/errors';
import { subjectLabel, useGroupContext } from '@/lib/group';

/** Карточка задачи: срок, свой прогресс, статус группы, правка и удаление. */
export default function TaskScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  // The route is a hidden tab and stays mounted: a fresh view per task resets
  // the edit form and the rest of the local state.
  return <TaskView key={id} id={id} />;
}

function TaskView({ id }: { id: string }) {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const groupId = ctx.groupId ?? '';
  const task = useTask(id);
  const dueLabel = useDueLabel();
  const update = useUpdateTask(groupId);
  const setStatus = useSetTaskStatus(groupId);
  const setMine = useSetMyTaskStatus(groupId);
  const pin = usePinTask(groupId);
  const remove = useDeleteTask(groupId);
  const [editing, setEditing] = useState(false);

  if (task.isPending) return <LoadingScreen />;
  if (!task.data) {
    return (
      <SafeAreaView style={{ flex: 1 }} edges={['top']}>
        <Screen>
          <EmptyState
            title={t('errors.not_found')}
            hint={task.isError ? describeError(t, task.error) : undefined}
          />
          <Button onPress={() => router.back()}>{t('common.back')}</Button>
        </Screen>
      </SafeAreaView>
    );
  }

  const item = task.data;
  const subject = item.subject_id ? ctx.subjectById.get(item.subject_id) : undefined;
  const author = ctx.memberName(item.created_by);
  const mineStatus = myStatus(item);
  // The author always manages their own task; so do the headman and moderators.
  const canManage = item.created_by === ctx.me?.id || ctx.permissions.can('task.status.group');
  const error = update.error ?? setStatus.error ?? setMine.error ?? pin.error ?? remove.error;

  const confirmDelete = async () => {
    const ok = await confirm({
      title: t('common.delete'),
      message: t('tasks.delete_confirm', { title: item.title }),
      confirmText: t('common.delete'),
      cancelText: t('common.cancel'),
      destructive: true,
    });
    if (ok) remove.mutate(item.id, { onSuccess: () => router.back() });
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
          <H3 flex={1} numberOfLines={2}>
            {item.title}
          </H3>
        </XStack>

        {editing ? (
          <TaskForm
            task={item}
            submitLabel={t('common.save')}
            pending={update.isPending}
            error={update.isError ? describeError(t, update.error) : null}
            onCancel={() => setEditing(false)}
            onSubmit={(body) =>
              update.mutate({ taskId: item.id, ...body }, { onSuccess: () => setEditing(false) })
            }
          />
        ) : (
          <YStack gap="$3">
            <Paragraph color="$color10">
              {[
                subjectLabel(subject) ?? t('tasks.no_subject'),
                t(`task_kinds.${item.kind}`),
                t(`task_assign.${item.assign_mode}`),
                dueLabel(item) ?? t('tasks.due_none'),
                item.pinned_at ? t('tasks.pinned') : null,
                author ? t('tasks.author', { name: author }) : null,
              ]
                .filter(Boolean)
                .join(' · ')}
            </Paragraph>
            {item.description ? <Paragraph>{item.description}</Paragraph> : null}
            <ProgressLine task={item} />

            <Separator />
            <SectionLabel>{t('tasks.my_progress')}</SectionLabel>
            <StatusPicker
              value={mineStatus}
              statuses={['TODO', 'IN_PROGRESS', 'DONE']}
              onChange={(status) => setMine.mutate({ taskId: item.id, status })}
            />
            <Button
              theme={isClosed(mineStatus) ? undefined : 'accent'}
              icon={<Ionicons name="checkmark-done-outline" size={18} />}
              disabled={setMine.isPending}
              onPress={() =>
                setMine.mutate({
                  taskId: item.id,
                  status: isClosed(mineStatus) ? 'TODO' : 'DONE',
                })
              }
            >
              {isClosed(mineStatus) ? t('tasks.mark_todo') : t('tasks.mark_done')}
            </Button>

            {canManage ? (
              <>
                <Separator />
                <SectionLabel>{t('tasks.group_status')}</SectionLabel>
                <StatusPicker
                  value={item.status}
                  onChange={(status) => setStatus.mutate({ taskId: item.id, status })}
                />
              </>
            ) : null}

            <Separator />
            <TaskFiles task={item} canManage={canManage} />

            {item.visibility !== 'PRIVATE' ? (
              <DiscussionButton target={{ type: 'TASK', id: item.id }} title={item.title} />
            ) : null}

            <ErrorText>{error ? describeError(t, error) : null}</ErrorText>

            <XStack gap="$2" flexWrap="wrap">
              {ctx.permissions.can('task.pin') && item.visibility !== 'PRIVATE' ? (
                <Button
                  flex={1}
                  icon={<Ionicons name={item.pinned_at ? 'pin-outline' : 'pin'} size={18} />}
                  disabled={pin.isPending}
                  onPress={() => pin.mutate({ taskId: item.id, pinned: !item.pinned_at })}
                >
                  {item.pinned_at ? t('tasks.unpin') : t('tasks.pin')}
                </Button>
              ) : null}
              {canManage ? (
                <>
                  <Button
                    flex={1}
                    icon={<Ionicons name="create-outline" size={18} />}
                    onPress={() => setEditing(true)}
                  >
                    {t('common.edit')}
                  </Button>
                  <Button
                    flex={1}
                    theme="red"
                    icon={<Ionicons name="trash-outline" size={18} />}
                    disabled={remove.isPending}
                    onPress={confirmDelete}
                  >
                    {t('common.delete')}
                  </Button>
                </>
              ) : null}
            </XStack>
          </YStack>
        )}
      </Screen>
    </SafeAreaView>
  );
}
