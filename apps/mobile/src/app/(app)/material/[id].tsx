import Ionicons from '@expo/vector-icons/Ionicons';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { MaterialDetails } from '@heatseeker/api-client';
import {
  materialRights,
  useMaterial,
  useMaterialTransition,
  useOpenMaterial,
  useRequestPreview,
  useShareMaterial,
  useUpdateMaterial,
  type MaterialKind,
  type MaterialTransition,
} from '@heatseeker/core';
import {
  Button,
  confirm,
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
import { MediaPlayer, playableKind, type PlayableKind } from '@/components/media-player';
import { DiscussionButton } from '@/components/discussion-button';
import { describeError } from '@/lib/errors';
import { subjectLabel, useGroupContext } from '@/lib/group';
import { showMaterial } from '@/lib/open';

type PreviewStatus = MaterialDetails['file']['preview_status'];
type PreviewProblem = 'timeout' | 'too_large' | 'failed';
/** How long "Смотреть" waits for a PDF preview, and how often it checks. */
const PREVIEW_WAIT_MS = 120_000;
const PREVIEW_POLL_MS = 2_500;

/** Материал: просмотр, история версий, правка и модерация. */
export default function MaterialScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  // The route is a hidden tab and stays mounted: a fresh view per material
  // resets the player, the edit form and the rest of the local state.
  return <MaterialView key={id} id={id} />;
}

function MaterialView({ id }: { id: string }) {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const material = useMaterial(id);
  const open = useOpenMaterial();
  const transition = useMaterialTransition(ctx.groupId ?? '');
  const share = useShareMaterial(ctx.groupId ?? '');
  const [editing, setEditing] = useState(false);
  // Audio/video play inline; undefined version = the current one.
  const [playing, setPlaying] = useState<{ versionId?: string; kind: PlayableKind } | null>(null);
  // Leaving the screen stops playback (the tab itself is not unmounted).
  useFocusEffect(useCallback(() => () => setPlaying(null), []));
  const requestPreview = useRequestPreview(id);
  // Version whose PDF preview is being prepared, and why the last one failed.
  const [preparing, setPreparing] = useState<string | null>(null);
  const [previewProblem, setPreviewProblem] = useState<PreviewProblem | null>(null);
  const alive = useRef(true);
  useEffect(
    () => () => {
      alive.current = false;
    },
    [],
  );

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

  const playable = playableKind(m.file.mime);

  /**
   * Office files are shown through a PDF preview made by the server: request
   * it if needed, wait while it is converted, then open it. Without a preview
   * (too large, failed, turned off) the file opens as before — downloaded.
   */
  const view = async (versionId: string, status: PreviewStatus | undefined) => {
    setPreviewProblem(null);
    if (status !== 'READY' && status !== 'NONE' && status !== 'PENDING' && status !== 'FAILED') {
      open.mutate(
        { materialId: m.id, versionId },
        { onSuccess: (links) => void showMaterial(links) },
      );
      return;
    }
    setPreparing(versionId);
    try {
      if (status === 'NONE' || status === 'FAILED') await requestPreview.mutateAsync(versionId);
      const deadline = Date.now() + PREVIEW_WAIT_MS;
      let current = status === 'READY' ? status : 'PENDING';
      while (current === 'PENDING' && Date.now() < deadline && alive.current) {
        await new Promise((resolve) => setTimeout(resolve, PREVIEW_POLL_MS));
        const fresh = await material.refetch();
        current = fresh.data?.versions?.find((v) => v.id === versionId)?.preview_status ?? '';
      }
      if (!alive.current) return;
      if (current !== 'READY') {
        setPreviewProblem(
          current === 'PENDING' ? 'timeout' : current === 'SKIPPED' ? 'too_large' : 'failed',
        );
        return;
      }
      const links = await open.mutateAsync({ materialId: m.id, versionId });
      await showMaterial(links);
    } catch {
      // Request errors are shown by the mutations below the buttons.
    } finally {
      if (alive.current) setPreparing(null);
    }
  };

  const doOpen = (preferDrive: boolean, download = false) =>
    !preferDrive && !download && playable
      ? setPlaying(playing && !playing.versionId ? null : { kind: playable })
      : !preferDrive && !download
        ? void view(m.file.id, m.file.preview_status)
        : open.mutate(
            { materialId: m.id, download },
            { onSuccess: (links) => void showMaterial(links, preferDrive) },
          );

  const doTransition = (action: MaterialTransition) => {
    const run = () =>
      transition.mutate(
        { materialId: m.id, action },
        { onSuccess: () => (action === 'delete' ? router.back() : undefined) },
      );
    if (action !== 'delete') {
      run();
      return;
    }
    void confirm({
      title: t('common.delete'),
      message: t('common.confirm_delete'),
      confirmText: t('common.delete'),
      cancelText: t('common.cancel'),
      destructive: true,
    }).then((ok) => ok && run());
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

        {m.task_id ? (
          <YStack gap="$2" padding="$3" borderRadius="$4" backgroundColor="$blue3">
            <Paragraph fontWeight="600">{t('materials.task_only')}</Paragraph>
            <Paragraph>{t('materials.task_only_hint')}</Paragraph>
            <XStack gap="$2" flexWrap="wrap">
              <Button
                size="$3"
                icon={<Ionicons name="checkbox-outline" size={16} />}
                onPress={() =>
                  router.push({ pathname: '/(app)/task/[id]', params: { id: m.task_id! } })
                }
              >
                {t('materials.open_task')}
              </Button>
              {m.can_edit ? (
                <Button
                  size="$3"
                  icon={<Ionicons name="people-outline" size={16} />}
                  disabled={share.isPending}
                  onPress={() => share.mutate(m.id)}
                >
                  {t('materials.share')}
                </Button>
              ) : null}
            </XStack>
            <ErrorText>{share.isError ? describeError(t, share.error) : null}</ErrorText>
          </YStack>
        ) : null}

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
            icon={<Ionicons name={playable ? 'play' : 'eye-outline'} size={18} />}
            disabled={open.isPending || preparing !== null}
            onPress={() => doOpen(false)}
          >
            {preparing === m.file.id
              ? t('materials.preview_preparing')
              : playable
                ? t('player.listen', { context: playable })
                : t('materials.open_in_app')}
          </Button>
          {previewProblem ? (
            <Paragraph color="$color10">
              {t(`materials.preview_problem.${previewProblem}`)}
            </Paragraph>
          ) : null}
          <ErrorText>
            {requestPreview.isError ? describeError(t, requestPreview.error) : null}
          </ErrorText>
          {playing ? (
            <YStack gap="$1">
              {playing.versionId ? (
                <Paragraph size="$2" color="$color10">
                  {t('materials.version_n', {
                    n: m.versions?.find((v) => v.id === playing.versionId)?.version_no,
                  })}
                </Paragraph>
              ) : null}
              <MediaPlayer
                key={playing.versionId ?? 'current'}
                materialId={m.id}
                versionId={playing.versionId}
                kind={playing.kind}
              />
            </YStack>
          ) : null}
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
            <Paragraph userSelect="text">{m.description}</Paragraph>
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
                subtitle={
                  preparing === v.id
                    ? t('materials.preview_preparing')
                    : `${v.original_name} · ${new Date(v.created_at).toLocaleString()}`
                }
                onPress={() =>
                  playableKind(v.mime)
                    ? setPlaying({ versionId: v.id, kind: playableKind(v.mime)! })
                    : void view(v.id, v.preview_status)
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

        {m.status !== 'DELETED' && !m.task_id ? (
          <DiscussionButton target={{ type: 'MATERIAL', id: m.id }} title={m.title} />
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
