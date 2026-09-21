import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useMembers, type Task, type TaskInput } from '@heatseeker/core';
import {
  Button,
  Chip,
  ErrorText,
  Field,
  Paragraph,
  Separator,
  TextArea,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { DuePicker } from '@/components/due-picker';
import { SectionLabel, SubjectPicker } from '@/components/materials';
import { AssignPicker, PriorityPicker, TaskKindPicker } from '@/components/tasks';
import { useGroupContext } from '@/lib/group';

export interface TaskFormProps {
  /** Задача для правки; null — новая. */
  task: Task | null;
  submitLabel: string;
  pending?: boolean;
  error?: string | null;
  onSubmit: (body: TaskInput) => void;
  onCancel: () => void;
}

/** Форма задачи: название, описание, предмет, тип, приоритет, срок, кому. */
export function TaskForm({ task, submitLabel, pending, error, onSubmit, onCancel }: TaskFormProps) {
  const { t } = useTranslation();
  const ctx = useGroupContext();
  const members = useMembers(ctx.groupId);
  const [title, setTitle] = useState(task?.title ?? '');
  const [description, setDescription] = useState(task?.description ?? '');
  const [subjectId, setSubjectId] = useState<string | null>(task?.subject_id ?? null);
  const [kind, setKind] = useState<TaskInput['kind']>(task?.kind ?? 'GROUP');
  const [priority, setPriority] = useState<TaskInput['priority']>(task?.priority ?? 'NORMAL');
  const [assignMode, setAssignMode] = useState<TaskInput['assign_mode']>(
    task?.assign_mode ?? 'ALL',
  );
  const [assignees, setAssignees] = useState<string[]>(task?.assignee_ids ?? []);
  const [dueIso, setDueIso] = useState<string | null>(task?.due_at ?? null);

  const canSubmit = !!title.trim() && !pending;

  const submit = () =>
    onSubmit({
      title: title.trim(),
      description: description.trim() || undefined,
      subject_id: subjectId ?? undefined,
      kind,
      priority,
      assign_mode: assignMode,
      visibility: assignMode === 'SELF' ? 'PRIVATE' : 'GROUP',
      due_at: dueIso ?? undefined,
      assignee_ids: assignMode === 'SELECTED' ? assignees : undefined,
    });

  return (
    <YStack gap="$3">
      <Field
        id="task-title"
        label={t('tasks.form_title')}
        value={title}
        onChangeText={setTitle}
        maxLength={200}
      />
      <YStack gap="$1.5">
        <SectionLabel>{t('tasks.form_description')}</SectionLabel>
        <TextArea
          value={description}
          onChangeText={setDescription}
          numberOfLines={4}
          maxLength={5000}
        />
      </YStack>
      <YStack gap="$1.5">
        <SectionLabel>{t('tasks.form_due')}</SectionLabel>
        <DuePicker value={dueIso} onChange={setDueIso} />
      </YStack>
      <YStack gap="$1.5">
        <SectionLabel>{t('tasks.form_subject')}</SectionLabel>
        <SubjectPicker
          subjects={ctx.subjects}
          value={subjectId}
          onChange={setSubjectId}
          emptyLabel={t('tasks.no_subject')}
        />
      </YStack>
      <YStack gap="$1.5">
        <SectionLabel>{t('tasks.form_kind')}</SectionLabel>
        <TaskKindPicker value={kind ?? 'GROUP'} onChange={setKind} />
      </YStack>
      <YStack gap="$1.5">
        <SectionLabel>{t('tasks.form_priority')}</SectionLabel>
        <PriorityPicker value={priority ?? 'NORMAL'} onChange={setPriority} />
      </YStack>
      <YStack gap="$1.5">
        <SectionLabel>{t('tasks.form_assign')}</SectionLabel>
        <AssignPicker value={assignMode ?? 'ALL'} onChange={setAssignMode} />
        {assignMode === 'SELECTED' ? (
          <XStack gap="$2" flexWrap="wrap" paddingTop="$1">
            {(members.data?.items ?? []).map((m) => (
              <Chip
                key={m.user.id}
                label={m.user.name}
                selected={assignees.includes(m.user.id)}
                onPress={() =>
                  setAssignees((prev) =>
                    prev.includes(m.user.id)
                      ? prev.filter((id) => id !== m.user.id)
                      : [...prev, m.user.id],
                  )
                }
              />
            ))}
          </XStack>
        ) : null}
        {assignMode === 'SELF' ? (
          <Paragraph size="$2" color="$color10">
            {t('tasks.form_private')}
          </Paragraph>
        ) : null}
      </YStack>
      <ErrorText>{error}</ErrorText>
      <Separator />
      <XStack gap="$2">
        <Button flex={1} onPress={onCancel}>
          {t('common.cancel')}
        </Button>
        <Button flex={1} theme="accent" disabled={!canSubmit} onPress={submit}>
          {submitLabel}
        </Button>
      </XStack>
    </YStack>
  );
}
