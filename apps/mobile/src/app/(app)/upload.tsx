import Ionicons from '@expo/vector-icons/Ionicons';
import * as DocumentPicker from 'expo-document-picker';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  checkUpload,
  useDriveStatus,
  useServerMeta,
  useTask,
  useUploadMaterial,
  type MaterialKind,
  type PutFile,
} from '@heatseeker/core';
import {
  Button,
  Chip,
  ErrorText,
  Field,
  Paragraph,
  Screen,
  ScreenTitle,
  Separator,
  Switch,
  TextArea,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { KindPicker, SectionLabel, SubjectPicker, useFormatSize } from '@/components/materials';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';
import { putPickedFile } from '@/lib/upload';

/** Загрузка файла в хранилище группы (и, по желанию, на Google Диск). */
export default function UploadScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  // task: upload straight into a task's files (from the task card).
  const params = useLocalSearchParams<{ subject?: string; task?: string }>();
  const ctx = useGroupContext();
  const meta = useServerMeta();
  const drive = useDriveStatus(ctx.groupId);
  const formatSize = useFormatSize();
  const task = useTask(params.task);
  // A file uploaded into a task stays there unless the author shares it (D43).
  const [taskOnly, setTaskOnly] = useState(true);
  const keptInTask = !!task.data && taskOnly;

  const [asset, setAsset] = useState<DocumentPicker.DocumentPickerAsset | null>(null);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [subjectId, setSubjectId] = useState<string | null>(params.subject ?? null);
  const [kind, setKind] = useState<MaterialKind | null>(null);
  const [toDrive, setToDrive] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);

  // The mutation needs the file-specific sender; it is rebuilt per picked file.
  const putFile = useMemo<PutFile>(
    () => (ticket, onProgress) => {
      if (!asset) return Promise.reject(new Error('no file'));
      return putPickedFile(asset)(ticket, onProgress);
    },
    [asset],
  );
  const upload = useUploadMaterial(ctx.groupId ?? '', putFile);

  const limits = meta.data?.upload;
  const check =
    asset && limits ? checkUpload({ name: asset.name, size: asset.size ?? 0 }, limits) : null;
  const checkMessage =
    check === 'too_large'
      ? t('upload.too_large', { size: formatSize(limits!.max_bytes) })
      : check === 'bad_type'
        ? t('upload.bad_type', { list: (limits!.allowed_ext ?? []).join(', ') })
        : check === 'empty'
          ? t('upload.empty')
          : null;

  const driveAvailable =
    !!meta.data?.features.drive_upload &&
    !!drive.data?.can_publish &&
    ctx.permissions.can('drive.upload');
  // A folder is connected, but nobody can write to it yet (no publishing
  // account for "My Drive", or Google revoked it).
  const driveBlocked =
    !!meta.data?.features.drive_upload &&
    !!drive.data?.connection &&
    !drive.data.can_publish &&
    !!drive.data.publisher_available;
  const driveNeedsSecure =
    !!meta.data?.features.drive_upload &&
    !!drive.data?.can_publish &&
    ctx.permissions.needsSecuring('drive.upload');

  const pick = async () => {
    const res = await DocumentPicker.getDocumentAsync({
      copyToCacheDirectory: true,
      multiple: false,
    });
    if (res.canceled || !res.assets[0]) return;
    setAsset(res.assets[0]);
    upload.reset();
  };

  const send = () => {
    if (!asset) return;
    setProgress(0);
    upload.mutate(
      {
        file_name: asset.name,
        size_bytes: asset.size ?? 0,
        mime: asset.mimeType,
        title: title.trim() || undefined,
        description: description.trim() || undefined,
        subject_id: subjectId ?? undefined,
        kind: kind ?? undefined,
        to_drive: driveAvailable && toDrive && !keptInTask,
        task_id: task.data?.id,
        task_only: keptInTask || undefined,
        onProgress: (sent, total) =>
          setProgress(total > 0 ? Math.round((sent / total) * 100) : null),
      },
      {
        // The server attaches the file to the task; go back to it.
        onSuccess: (m) =>
          task.data
            ? router.replace({ pathname: '/(app)/task/[id]', params: { id: task.data.id } })
            : router.replace({ pathname: '/(app)/material/[id]', params: { id: m.id } }),
        onSettled: () => setProgress(null),
      },
    );
  };

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <XStack alignItems="center" gap="$2">
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="close" size={22} />}
            onPress={() => router.back()}
          />
          <ScreenTitle
            title={t('upload.title')}
            subtitle={task.data ? `${t('thread_targets.TASK')}: ${task.data.title}` : undefined}
          />
        </XStack>

        <Button icon={<Ionicons name="attach-outline" size={18} />} onPress={() => void pick()}>
          {asset
            ? t('upload.picked', { name: asset.name, size: formatSize(asset.size ?? 0) })
            : t('upload.pick')}
        </Button>
        <ErrorText>{checkMessage}</ErrorText>

        <Field
          id="upload-title"
          label={t('materials.name')}
          value={title}
          onChangeText={setTitle}
          placeholder={asset?.name}
          maxLength={200}
        />
        <YStack gap="$1.5">
          <SectionLabel>{t('materials.description')}</SectionLabel>
          <TextArea
            value={description}
            onChangeText={setDescription}
            minHeight={80}
            maxLength={10000}
          />
        </YStack>

        <SectionLabel>{t('materials.subject')}</SectionLabel>
        <SubjectPicker
          subjects={ctx.subjects}
          value={subjectId}
          onChange={setSubjectId}
          emptyLabel={t('upload.subject_auto')}
        />
        <SectionLabel>{t('materials.kind')}</SectionLabel>
        <KindPicker value={kind} onChange={setKind} emptyLabel={t('upload.kind_auto')} />

        {task.data ? (
          <YStack gap="$2">
            <SectionLabel>{t('upload.access')}</SectionLabel>
            <XStack gap="$2" flexWrap="wrap">
              <Chip
                label={t('upload.access_task')}
                selected={taskOnly}
                onPress={() => setTaskOnly(true)}
              />
              <Chip
                label={t('upload.access_group')}
                selected={!taskOnly}
                onPress={() => setTaskOnly(false)}
              />
            </XStack>
            <Paragraph size="$2" color="$color10">
              {taskOnly ? t('upload.access_task_hint') : t('upload.access_group_hint')}
            </Paragraph>
          </YStack>
        ) : null}

        {keptInTask ? null : driveAvailable ? (
          <XStack alignItems="center" gap="$3">
            <Switch checked={toDrive} onCheckedChange={setToDrive} size="$3">
              <Switch.Thumb />
            </Switch>
            <Paragraph flex={1}>{t('upload.to_drive')}</Paragraph>
          </XStack>
        ) : driveNeedsSecure ? (
          <Paragraph color="$color10">{t('upload.to_drive_needs_secure')}</Paragraph>
        ) : driveBlocked ? (
          <Paragraph color="$color10">
            {drive.data?.publisher?.last_error
              ? t('upload.to_drive_publisher_revoked')
              : t('upload.to_drive_needs_publisher')}
          </Paragraph>
        ) : null}

        <Separator />
        {progress !== null ? (
          <Paragraph>{t('upload.progress', { percent: progress })}</Paragraph>
        ) : null}
        <ErrorText>{upload.isError ? describeError(t, upload.error) : null}</ErrorText>
        <Button
          theme="accent"
          disabled={!asset || check !== 'ok' || upload.isPending}
          icon={<Ionicons name="cloud-upload-outline" size={18} />}
          onPress={send}
        >
          {t('upload.send')}
        </Button>
      </Screen>
    </SafeAreaView>
  );
}
