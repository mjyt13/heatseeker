import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Subject } from '@heatseeker/api-client';
import {
  useArchiveSubject,
  useCreateSubject,
  useSubjects,
  useUpdateSubject,
} from '@heatseeker/core';
import {
  Button,
  ErrorText,
  Field,
  H4,
  ListRow,
  Screen,
  ScreenTitle,
  Separator,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { AliasEditor } from '@/components/alias-editor';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Предметы группы: создание, синонимы для авторазбора файлов, архив. */
export default function SubjectsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const subjects = useSubjects(ctx.groupId, true);
  const [editing, setEditing] = useState<Subject | 'new' | null>(null);
  const canManage = ctx.permissions.can('subject.manage');

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
          <ScreenTitle title={t('subjects.manage')} />
        </XStack>
        {(subjects.data ?? []).map((s) => (
          <ListRow
            key={s.id}
            title={s.short_name ? `${s.name} (${s.short_name})` : s.name}
            subtitle={[
              s.teacher,
              s.archived_at ? t('subjects.archived') : null,
              (s.aliases ?? []).join(', ') || null,
            ]
              .filter(Boolean)
              .join(' · ')}
            onPress={canManage ? () => setEditing(s) : undefined}
          />
        ))}
        <ErrorText>{subjects.isError ? describeError(t, subjects.error) : null}</ErrorText>
        {canManage ? (
          editing ? (
            <SubjectForm
              key={editing === 'new' ? 'new' : editing.id}
              subject={editing === 'new' ? null : editing}
              onDone={() => setEditing(null)}
            />
          ) : (
            <Button
              theme="accent"
              icon={<Ionicons name="add" size={18} />}
              onPress={() => setEditing('new')}
            >
              {t('subjects.add')}
            </Button>
          )
        ) : null}
      </Screen>
    </SafeAreaView>
  );
}

function SubjectForm({ subject, onDone }: { subject: Subject | null; onDone: () => void }) {
  const { t } = useTranslation();
  const ctx = useGroupContext();
  const groupId = ctx.groupId ?? '';
  const create = useCreateSubject(groupId);
  const update = useUpdateSubject(groupId);
  const archive = useArchiveSubject(groupId);
  const [name, setName] = useState(subject?.name ?? '');
  const [shortName, setShortName] = useState(subject?.short_name ?? '');
  const [teacher, setTeacher] = useState(subject?.teacher ?? '');
  const [aliases, setAliases] = useState<string[]>(subject?.aliases ?? []);

  const body = {
    name: name.trim(),
    short_name: shortName.trim() || undefined,
    teacher: teacher.trim() || undefined,
    aliases,
    color: subject?.color ?? undefined,
    sort_order: subject?.sort_order,
    semester: subject?.semester ?? undefined,
    teacher_contact: subject?.teacher_contact ?? undefined,
  };
  const pending = create.isPending || update.isPending || archive.isPending;
  const error = create.error ?? update.error ?? archive.error;

  return (
    <YStack gap="$3">
      <Separator />
      <H4>{subject ? subject.name : t('subjects.add')}</H4>
      <Field
        id="subject-name"
        label={t('subjects.name')}
        value={name}
        onChangeText={setName}
        maxLength={120}
      />
      <Field
        id="subject-short"
        label={t('subjects.short_name')}
        value={shortName}
        onChangeText={setShortName}
        maxLength={30}
      />
      <Field
        id="subject-teacher"
        label={t('subjects.teacher')}
        value={teacher}
        onChangeText={setTeacher}
        maxLength={120}
      />
      <AliasEditor value={aliases} onChange={setAliases} />
      <ErrorText>{error ? describeError(t, error) : null}</ErrorText>
      <XStack gap="$2" flexWrap="wrap">
        <Button flex={1} onPress={onDone}>
          {t('common.cancel')}
        </Button>
        {subject ? (
          <Button
            flex={1}
            disabled={pending}
            onPress={() =>
              archive.mutate(
                { subjectId: subject.id, restore: !!subject.archived_at },
                { onSuccess: onDone },
              )
            }
          >
            {subject.archived_at ? t('common.restore') : t('common.archive')}
          </Button>
        ) : null}
        <Button
          flex={1}
          theme="accent"
          disabled={!body.name || pending}
          onPress={() =>
            subject
              ? update.mutate({ subjectId: subject.id, ...body }, { onSuccess: onDone })
              : create.mutate(body, { onSuccess: onDone })
          }
        >
          {t('common.save')}
        </Button>
      </XStack>
    </YStack>
  );
}
