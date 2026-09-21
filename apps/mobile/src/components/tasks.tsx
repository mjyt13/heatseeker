import Ionicons from '@expo/vector-icons/Ionicons';
import { useTranslation } from 'react-i18next';

import type { Subject } from '@heatseeker/api-client';
import {
  dueState,
  formatDue,
  isClosed,
  myStatus,
  TASK_STATUS_ORDER,
  type Task,
  type TaskAssignMode,
  type TaskKind,
  type TaskPriority,
  type TaskStatus,
} from '@heatseeker/core';
import { TASK_ASSIGN_MODES, TASK_KINDS, TASK_PRIORITYS } from '@heatseeker/shared';
import { Chip, ListRow, SizableText, XStack, useTheme } from '@heatseeker/ui';

import { subjectLabel } from '@/lib/group';

/** Подпись срока: «Просрочено», «Сегодня», «через 3 дня» или дата. */
export function useDueLabel() {
  const { t } = useTranslation();
  return (task: Task): string | null => {
    const state = dueState(task);
    switch (state.kind) {
      case 'none':
        return null;
      case 'overdue':
        return t('tasks.overdue');
      case 'today':
        return t('tasks.due_today');
      case 'tomorrow':
        return t('tasks.due_tomorrow');
      case 'days':
        return t('tasks.due_in_days', { count: state.days });
      case 'date':
        return t('tasks.due_label', { date: formatDue(task.due_at!) });
    }
  };
}

/** Строка списка задач: свой прогресс слева, предмет и срок — подписью. */
export function TaskRow({
  task,
  subject,
  onPress,
  onToggleMine,
}: {
  task: Task;
  subject?: Subject;
  onPress?: () => void;
  onToggleMine?: () => void;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const dueLabel = useDueLabel();
  const mine = myStatus(task);
  const done = isClosed(mine);
  const overdue = dueState(task).kind === 'overdue';
  const parts = [
    subjectLabel(subject) ?? t('tasks.no_subject'),
    dueLabel(task),
    task.kind === 'TEACHER' ? t('task_kinds.TEACHER') : null,
    task.assigned_count > 0
      ? t('tasks.done_count', { done: task.done_count, total: task.assigned_count })
      : null,
  ].filter(Boolean) as string[];

  return (
    <ListRow
      title={task.title}
      subtitle={parts.join(' · ')}
      onPress={onPress}
      leading={
        <Ionicons
          name={done ? 'checkmark-circle' : 'ellipse-outline'}
          size={24}
          color={done ? theme.green10?.val : overdue ? theme.red10?.val : theme.color10?.val}
          onPress={onToggleMine}
        />
      }
      trailing={
        <XStack gap="$2" alignItems="center">
          {task.priority === 'HIGH' && !done ? (
            <Ionicons name="flag" size={16} color={theme.red10?.val} />
          ) : null}
          {task.pinned_at ? <Ionicons name="pin" size={16} color={theme.color10?.val} /> : null}
          {task.visibility === 'PRIVATE' ? (
            <Ionicons name="lock-closed-outline" size={16} color={theme.color10?.val} />
          ) : null}
        </XStack>
      }
    />
  );
}

/** Статусы задачи: для доски и для своего прогресса. */
export function StatusPicker({
  value,
  onChange,
  statuses = TASK_STATUS_ORDER,
}: {
  value: TaskStatus;
  onChange: (status: TaskStatus) => void;
  statuses?: readonly TaskStatus[];
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$2" flexWrap="wrap">
      {statuses.map((s) => (
        <Chip
          key={s}
          label={t(`task_status.${s}`)}
          selected={value === s}
          onPress={() => onChange(s)}
        />
      ))}
    </XStack>
  );
}

/** Тип задачи: от преподавателя, групповая, личная. */
export function TaskKindPicker({
  value,
  onChange,
}: {
  value: TaskKind;
  onChange: (kind: TaskKind) => void;
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$2" flexWrap="wrap">
      {TASK_KINDS.map((k) => (
        <Chip
          key={k}
          label={t(`task_kinds.${k}`)}
          selected={value === k}
          onPress={() => onChange(k)}
        />
      ))}
    </XStack>
  );
}

/** Приоритет. */
export function PriorityPicker({
  value,
  onChange,
}: {
  value: TaskPriority;
  onChange: (priority: TaskPriority) => void;
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$2" flexWrap="wrap">
      {TASK_PRIORITYS.map((p) => (
        <Chip
          key={p}
          label={t(`task_priority.${p}`)}
          selected={value === p}
          onPress={() => onChange(p)}
        />
      ))}
    </XStack>
  );
}

/** Кому задача: всей группе, выбранным, только мне. */
export function AssignPicker({
  value,
  onChange,
  modes = TASK_ASSIGN_MODES,
}: {
  value: TaskAssignMode;
  onChange: (mode: TaskAssignMode) => void;
  modes?: readonly TaskAssignMode[];
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$2" flexWrap="wrap">
      {modes.map((m) => (
        <Chip
          key={m}
          label={t(`task_assign.${m}`)}
          selected={value === m}
          onPress={() => onChange(m)}
        />
      ))}
    </XStack>
  );
}

/** Строка «сделали N из M» в карточке задачи. */
export function ProgressLine({ task }: { task: Task }) {
  const { t } = useTranslation();
  if (task.assigned_count === 0) return null;
  return (
    <SizableText size="$3" color="$color10">
      {t('tasks.done_count', { done: task.done_count, total: task.assigned_count })}
    </SizableText>
  );
}
