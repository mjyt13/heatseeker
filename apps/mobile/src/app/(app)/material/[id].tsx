import Ionicons from '@expo/vector-icons/Ionicons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { MaterialDetails } from '@heatseeker/api-client';
import {
  materialRights,
  useMaterial,
  useMaterialTransition,
  useOpenMaterial,
  useUpdateMaterial,
  type MaterialKind,
  type MaterialTransition,
} from '@heatseeker/core';
import {
  Button,
  EmptyState,
  ErrorText,
  Field,
  H3,
  ListRow,
  LoadingScreen,
  Paragraph,
  Screen,
  Separator,
  TextArea,
  XStack,
  YStack,
} from '@heatseeker/ui';

import {
  FileGlyph,
  KindPicker,
  SectionLabel,
  SubjectPicker,
  useFormatSize,
} from '@/components/materials';
import { describeError } from '@/lib/errors';
import { subjectLabel, useGroupContext } from '@/lib/group';
import { showMaterial } from '@/lib/open';

/** Материал: просмотр, история версий, правка и модерация. */
export default function MaterialScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();
  const ctx = useGroupContext();
  const material = useMaterial(id);
  const open = useOpenMaterial();
  const transition = useMaterialTransition(ctx.groupId ?? '');
  const [editing, setEditing] = useState(false);

  if (material.isPending) return <LoadingScreen />;
  if (!material.data) {
    return (
      <SafeAreaView style={{ flex: 1 }} edges={['top']}>
        <Screen>
          <EmptyState
            title={material.isError ? describeError(t, material.error) : t('errors.not_found')}
          />
          <Button onPress={() => router.back()}>{t('common.back')}</Button>
        </Screen>
      </SafeAreaView>
    );
  }

  const m = material.data;
  const rights = materialRights(m, ctx.me?.id, {
    moderate: ctx.permissions.can('material.moderate'),
    editOwn: ctx.permissions.can('material.edit.own'),
  });
  const subject = m.subject_id ? ctx.subjectById.get(m.subject_id) : undefined;
  const suggested =
    m.needs_review && m.classification.subject_id
      ? ctx.subjectById.get(m.classification.subject_id)
      : undefined;

  const doOpen = (preferDrive: boolean, download = false) =>
    open.mutate(
      { materialId: m.id, download },
      { onSuccess: (links) => void showMaterial(links, preferDrive) },
    );

  const doTransition = (action: MaterialTransition) => {
    const run = () =>
      transition.mutate(
        { materialId: m.id, action },
        { onSuccess: () => (action === 'delete' ? router.back() : undefined) },
      );
    if (action === 'delete') {
      Alert.alert(t('common.delete'), t('common.confirm_delete'), [
        { text: t('common.cancel'), style: 'cancel' },
        { text: t('common.delete'), style: 'destructive', onPress: run },
      ]);
    } else {
      run();
    }
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
          <FileGlyph mime={m.file.mime} size={28} />
          <H3 flex={1} numberOfLines={3}>
            {m.title}
          </H3>
        </XStack>

        <MaterialFacts
          material={m}
          subjectName={subjectLabel(subject)}
          uploader={ctx.memberName(m.uploader_id)}
        />

        {m.needs_review ? (
          <YStack gap="$1" padding="$3" borderRadius="$4" backgroundColor="$yellow3">
            <Paragraph fontWeight="600">{t('materials.needs_review')}</Paragraph>
            {m.review_reason ? (
              <Paragraph>{t(`materials.review_reason.${m.review_reason}`)}</Paragraph>
            ) : null}
            {suggested ? (
              <Paragraph>
                {t('materials.suggested', { subject: subjectLabel(suggested) })}
              </Paragraph>
            ) : null}
            {ctx.permissions.can('material.moderate') ? (
              <Button size="$3" onPress={() => router.push('/(app)/inbox')}>
                {t('inbox.title')}
              </Button>
            ) : null}
          </YStack>
        ) : null}

        <YStack gap="$2">
          <Button
            theme="accent"
            icon={<Ionicons name="eye-outline" size={18} />}
            disabled={open.isPending}
            onPress={() => doOpen(false)}
          >
            {t('materials.open_in_app')}
          </Button>
          {m.file.drive_web_view_link ? (
            <Button
              icon={<Ionicons name="logo-google" size={18} />}
              disabled={open.isPending}
              onPress={() => doOpen(true)}
            >
              {t('materials.open_in_drive')}
            </Button>
          ) : null}
          {m.file.storage !== 'DRIVE' ? (
            <Button
              icon={<Ionicons name="download-outline" size={18} />}
              disabled={open.isPending}
              onPress={() => doOpen(false, true)}
            >
              {t('common.download')}
            </Button>
          ) : null}
          <ErrorText>{open.isError ? describeError(t, open.error) : null}</ErrorText>
        </YStack>

        {m.description ? (
          <YStack gap="$1">
            <SectionLabel>{t('materials.description')}</SectionLabel>
            {/* Markdown rendering arrives with the editor (stage 2); plain text is safe meanwhile. */}
            <Paragraph selectable>{m.description}</Paragraph>
          </YStack>
        ) : null}

        {m.drive_path && m.drive_path.length > 0 ? (
          <YStack gap="$1">
            <SectionLabel>{t('materials.drive_path')}</SectionLabel>
            <Paragraph>{m.drive_path.join(' / ')}</Paragraph>
          </YStack>
        ) : null}

        {(m.versions ?? []).length > 1 ? (
          <YStack gap="$1">
            <SectionLabel>{t('materials.versions')}</SectionLabel>
            {(m.versions ?? []).map((v) => (
              <ListRow
                key={v.id}
                title={t('materials.version_n', { n: v.version_no })}
                subtitle={`${v.original_name} · ${new Date(v.created_at).toLocaleString()}`}
                onPress={() =>
                  open.mutate(
                    { materialId: m.id, versionId: v.id },
                    { onSuccess: (links) => void showMaterial(links) },
                  )
                }
              />
            ))}
          </YStack>
        ) : null}

        {rights.edit ? (
          <>
            <Separator />
            {editing ? (
              <EditMaterial material={m} onDone={() => setEditing(false)} />
            ) : (
              <Button
                icon={<Ionicons name="create-outline" size={18} />}
                onPress={() => setEditing(true)}
              >
                {t('common.edit')}
              </Button>
            )}
          </>
        ) : null}

        <XStack gap="$2" flexWrap="wrap">
          {rights.archive ? (
            <Button
              flex={1}
              disabled={transition.isPending}
              onPress={() => doTransition('archive')}
            >
              {t('common.archive')}
            </Button>
          ) : null}
          {m.status !== 'ACTIVE' && (rights.edit || ctx.permissions.can('material.moderate')) ? (
            <Button
              flex={1}
              disabled={transition.isPending}
              onPress={() => doTransition('restore')}
            >
              {t('common.restore')}
            </Button>
          ) : null}
          {rights.delete && m.status !== 'DELETED' ? (
            <Button
              flex={1}
              theme="red"
              disabled={transition.isPending}
              onPress={() => doTransition('delete')}
            >
              {t('common.delete')}
            </Button>
          ) : null}
        </XStack>
        {rights.edit && !rights.delete ? (
          <Paragraph color="$color10">{t('materials.cannot_delete_opened')}</Paragraph>
        ) : null}
        <ErrorText>{transition.isError ? describeError(t, transition.error) : null}</ErrorText>
      </Screen>
    </SafeAreaView>
  );
}

function MaterialFacts({
  material: m,
  subjectName,
  uploader,
}: {
  material: MaterialDetails;
  subjectName?: string;
  uploader?: string;
}) {
  const { t } = useTranslation();
  const formatSize = useFormatSize();
  const facts = [
    subjectName ?? t('materials.no_subject'),
    t(`kinds.${m.kind}`),
    m.source === 'GDRIVE'
      ? t('materials.source_drive')
      : uploader
        ? t('materials.source_upload', { name: uploader })
        : null,
    m.file.size_bytes > 0 ? formatSize(m.file.size_bytes) : null,
    m.download_count > 0 ? t('materials.opened_times', { count: m.download_count }) : null,
    m.status === 'ARCHIVED' ? t('materials.archived_badge') : null,
    m.status === 'DELETED' ? t('materials.deleted_badge') : null,
  ].filter(Boolean);
  return (
    <YStack gap="$1">
      <Paragraph color="$color10">{facts.join(' · ')}</Paragraph>
      {m.file.drive_upload_status ? (
        <Paragraph color={m.file.drive_upload_status === 'FAILED' ? '$red10' : '$color10'}>
          {t(`materials.drive_upload.${m.file.drive_upload_status}`)}
          {m.file.drive_upload_error ? ` — ${m.file.drive_upload_error}` : ''}
        </Paragraph>
      ) : null}
    </YStack>
  );
}

function EditMaterial({ material: m, onDone }: { material: MaterialDetails; onDone: () => void }) {
  const { t } = useTranslation();
  const ctx = useGroupContext();
  const update = useUpdateMaterial(ctx.groupId ?? '');
  const [title, setTitle] = useState(m.title);
  const [description, setDescription] = useState(m.description);
  const [subjectId, setSubjectId] = useState<string | null>(m.subject_id ?? null);
  const [kind, setKind] = useState<MaterialKind>(m.kind);

  const save = () =>
    update.mutate(
      {
        materialId: m.id,
        title: title.trim(),
        description,
        kind,
        ...(subjectId ? { subject_id: subjectId } : { clear_subject: true }),
      },
      { onSuccess: onDone },
    );

  return (
    <YStack gap="$3">
      <Field
        id="material-title"
        label={t('materials.name')}
        value={title}
        onChangeText={setTitle}
        maxLength={200}
      />
      <YStack gap="$1.5">
        <SectionLabel>{t('materials.description')}</SectionLabel>
        <TextArea
          value={description}
          onChangeText={setDescription}
          minHeight={100}
          maxLength={10000}
        />
      </YStack>
      <SectionLabel>{t('materials.subject')}</SectionLabel>
      <SubjectPicker
        subjects={ctx.subjects}
        value={subjectId}
        onChange={setSubjectId}
        emptyLabel={t('materials.no_subject')}
      />
      <SectionLabel>{t('materials.kind')}</SectionLabel>
      <KindPicker value={kind} onChange={(k) => k && setKind(k)} />
      <ErrorText>{update.isError ? describeError(t, update.error) : null}</ErrorText>
      <XStack gap="$2">
        <Button flex={1} onPress={onDone}>
          {t('common.cancel')}
        </Button>
        <Button flex={1} theme="accent" disabled={!title.trim() || update.isPending} onPress={save}>
          {t('common.save')}
        </Button>
      </XStack>
    </YStack>
  );
}
