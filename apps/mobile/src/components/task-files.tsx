import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useDeferredValue, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  useMaterial,
  useMaterials,
  useSetTaskMaterials,
  useShareMaterial,
  type Task,
} from '@heatseeker/core';
import {
  Button,
  ErrorText,
  Input,
  ListRow,
  Paragraph,
  Spinner,
  XStack,
  YStack,
  useTheme,
} from '@heatseeker/ui';

import { FileGlyph, MaterialRow, SectionLabel } from '@/components/materials';
import { PictureViewer, Thumbnail } from '@/components/picture';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

const MAX_ATTACHMENTS = 20;

/**
 * Файлы задачи. Список — из кеша материалов; у картинок сразу видна
 * уменьшенная копия (она остаётся в кеше), полный размер — по нажатию на неё.
 * Остальные файлы открываются на экране материала («Смотреть» / «Скачать»).
 */
export function TaskFiles({ task, canManage }: { task: Task; canManage: boolean }) {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, me } = useGroupContext();
  const setMaterials = useSetTaskMaterials(groupId ?? '');
  const share = useShareMaterial(groupId ?? '');
  const [picking, setPicking] = useState(false);
  const ids = task.material_ids ?? [];

  const save = (next: string[]) => setMaterials.mutate({ taskId: task.id, materialIds: next });
  const toggle = (id: string) =>
    save(ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]);

  return (
    <YStack gap="$2">
      <SectionLabel>{t('tasks.files')}</SectionLabel>
      {ids.length === 0 ? (
        <Paragraph color="$color10">{t('tasks.files_empty')}</Paragraph>
      ) : (
        ids.map((id) => (
          <AttachedFile
            key={id}
            materialId={id}
            onOpen={() => router.push({ pathname: '/(app)/material/[id]', params: { id } })}
            onRemove={canManage ? () => toggle(id) : undefined}
            canShare={(uploaderId) => canManage || (!!me && uploaderId === me.id)}
            onShare={() => share.mutate(id)}
            sharing={share.isPending}
          />
        ))
      )}
      <ErrorText>
        {setMaterials.isError
          ? describeError(t, setMaterials.error)
          : share.isError
            ? describeError(t, share.error)
            : null}
      </ErrorText>
      {canManage ? (
        <XStack gap="$2" flexWrap="wrap">
          <Button
            flex={1}
            icon={<Ionicons name={picking ? 'close' : 'attach-outline'} size={18} />}
            onPress={() => setPicking((v) => !v)}
          >
            {picking ? t('common.done') : t('tasks.files_attach')}
          </Button>
          <Button
            flex={1}
            icon={<Ionicons name="cloud-upload-outline" size={18} />}
            disabled={ids.length >= MAX_ATTACHMENTS}
            onPress={() =>
              router.push({
                pathname: '/(app)/upload',
                params: { task: task.id, ...(task.subject_id ? { subject: task.subject_id } : {}) },
              })
            }
          >
            {t('tasks.files_upload')}
          </Button>
        </XStack>
      ) : null}
      {picking ? (
        <MaterialPicker
          subjectId={task.subject_id}
          selected={ids}
          disabled={setMaterials.isPending}
          full={ids.length >= MAX_ATTACHMENTS}
          onToggle={toggle}
        />
      ) : null}
    </YStack>
  );
}

/** Прикреплённый файл: название и тип из кеша материала. */
function AttachedFile({
  materialId,
  onOpen,
  onRemove,
  canShare,
  onShare,
  sharing,
}: {
  materialId: string;
  onOpen: () => void;
  onRemove?: () => void;
  canShare: (uploaderId: string | undefined) => boolean;
  onShare: () => void;
  sharing: boolean;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const material = useMaterial(materialId);
  const m = material.data;
  const keptInTask = !!m?.task_id;
  const [viewing, setViewing] = useState(false);
  return (
    <YStack gap="$1">
      {m?.file.thumbnail_url ? <Thumbnail file={m.file} onPress={() => setViewing(true)} /> : null}
      {viewing && m ? (
        <PictureViewer materialId={m.id} file={m.file} onClose={() => setViewing(false)} />
      ) : null}
      <ListRow
        leading={m ? <FileGlyph mime={m.file.mime} /> : <Spinner size="small" />}
        title={m?.title ?? (material.isError ? t('tasks.files_unavailable') : '…')}
        subtitle={
          m
            ? [t(`kinds.${m.kind}`), keptInTask ? t('materials.task_only') : null]
                .filter(Boolean)
                .join(' · ')
            : null
        }
        onPress={m ? onOpen : undefined}
        trailing={
          <XStack gap="$1" alignItems="center">
            {m && keptInTask && canShare(m.uploader_id) ? (
              <Button
                size="$2"
                icon={<Ionicons name="people-outline" size={16} />}
                disabled={sharing}
                onPress={onShare}
              >
                {t('materials.share')}
              </Button>
            ) : null}
            {onRemove ? (
              <Button
                size="$2"
                chromeless
                aria-label={t('tasks.files_detach')}
                icon={<Ionicons name="close-circle-outline" size={20} color={theme.color10?.val} />}
                onPress={onRemove}
              />
            ) : null}
          </XStack>
        }
      />
    </YStack>
  );
}

/** Выбор материалов группы: поиск, сначала — по предмету задачи. */
function MaterialPicker({
  subjectId,
  selected,
  disabled,
  full,
  onToggle,
}: {
  subjectId?: string | null;
  selected: string[];
  disabled: boolean;
  full: boolean;
  onToggle: (id: string) => void;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const { groupId, subjectById } = useGroupContext();
  const [query, setQuery] = useState('');
  const [onlySubject, setOnlySubject] = useState(!!subjectId);
  const q = useDeferredValue(query);
  const materials = useMaterials(
    groupId,
    { q, subject_id: onlySubject && subjectId ? subjectId : undefined },
    20,
  );
  const items = materials.data?.pages.flatMap((p) => p.items ?? []) ?? [];

  return (
    <YStack gap="$2" borderWidth={1} borderColor="$borderColor" borderRadius="$4" padding="$2">
      <Input
        size="$4"
        value={query}
        onChangeText={setQuery}
        placeholder={t('tasks.files_search')}
        autoCorrect={false}
      />
      {subjectId ? (
        <Button size="$2" alignSelf="flex-start" onPress={() => setOnlySubject((v) => !v)}>
          {onlySubject ? t('tasks.files_all_subjects') : t('tasks.files_this_subject')}
        </Button>
      ) : null}
      {materials.isPending ? <Spinner /> : null}
      {!materials.isPending && items.length === 0 ? (
        <Paragraph color="$color10">{t('tasks.files_none_found')}</Paragraph>
      ) : null}
      {items.map((m) => {
        const on = selected.includes(m.id);
        return (
          <XStack key={m.id} alignItems="center">
            <YStack flex={1}>
              <MaterialRow
                material={m}
                subject={m.subject_id ? subjectById.get(m.subject_id) : undefined}
                showReview={false}
                onPress={disabled || (full && !on) ? undefined : () => onToggle(m.id)}
              />
            </YStack>
            <Ionicons
              name={on ? 'checkmark-circle' : 'ellipse-outline'}
              size={22}
              color={on ? theme.blue10?.val : theme.color8?.val}
            />
          </XStack>
        );
      })}
      {materials.hasNextPage ? (
        <Button
          size="$3"
          onPress={() => void materials.fetchNextPage()}
          disabled={materials.isFetchingNextPage}
        >
          {t('common.more')}
        </Button>
      ) : null}
    </YStack>
  );
}
