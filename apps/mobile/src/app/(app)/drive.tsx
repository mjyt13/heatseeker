import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Linking } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  useConnectDrive,
  useDisconnectDrive,
  useDriveStatus,
  useSyncDrive,
} from '@heatseeker/core';
import {
  Button,
  confirm,
  ErrorText,
  Field,
  H4,
  ListRow,
  LoadingScreen,
  Paragraph,
  Screen,
  ScreenTitle,
  Separator,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { CopyButton } from '@/components/copy-button';
import { DrivePublisherCard } from '@/components/drive-publisher';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Подключение папки группы на Google Диске. */
export default function DriveScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const groupId = ctx.groupId ?? '';
  const status = useDriveStatus(ctx.groupId);
  const connect = useConnectDrive(groupId);
  const disconnect = useDisconnectDrive(groupId);
  const sync = useSyncDrive(groupId);
  const [folder, setFolder] = useState('');
  const [changing, setChanging] = useState(false);

  const canManage = ctx.permissions.can('drive.manage');
  const needsSecure = ctx.permissions.needsSecuring('drive.manage');
  const s = status.data;
  const conn = s?.connection;

  if (status.isPending) return <LoadingScreen />;

  const confirmDisconnect = async () => {
    const ok = await confirm({
      title: t('drive.disconnect'),
      message: t('drive.disconnect_confirm'),
      confirmText: t('drive.disconnect'),
      cancelText: t('common.cancel'),
      destructive: true,
    });
    if (ok) disconnect.mutate();
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
          <ScreenTitle title={t('drive.title')} />
        </XStack>
        <ErrorText>{status.isError ? describeError(t, status.error) : null}</ErrorText>

        {s && !s.configured ? <Paragraph>{t('drive.not_configured')}</Paragraph> : null}

        {conn ? (
          <YStack gap="$2">
            <ListRow
              leading={<Ionicons name="folder-outline" size={24} />}
              title={conn.root_folder_name || conn.root_folder_id}
              subtitle={[
                t(`drive.status.${conn.status}`),
                conn.shared_drive ? t('drive.shared_drive') : null,
              ]
                .filter(Boolean)
                .join(' · ')}
              onPress={() => void Linking.openURL(conn.folder_url)}
            />
            <Paragraph color="$color10">
              {t('drive.last_sync', {
                when: conn.last_sync_at
                  ? new Date(conn.last_sync_at).toLocaleString()
                  : t('drive.never'),
              })}
            </Paragraph>
            {s?.stats ? (
              <Paragraph color="$color10">
                {t('drive.stats', {
                  files: s.stats.files,
                  folders: s.stats.folders,
                  skipped: s.stats.skipped,
                  inbox: s.stats.inbox_size,
                })}
              </Paragraph>
            ) : null}
            {s?.upload_enabled ? (
              <Paragraph color="$color10">
                {s.can_publish
                  ? t('drive.writable')
                  : s.publisher_available
                    ? t('drive.needs_publisher')
                    : t('drive.read_only')}
              </Paragraph>
            ) : null}
            {conn.last_error ? (
              <ErrorText>
                {conn.last_error_code
                  ? t(`errors.codes.${conn.last_error_code}`, {
                      email: s?.service_account_email ?? '',
                      defaultValue: conn.last_error,
                    })
                  : conn.last_error}
              </ErrorText>
            ) : null}
          </YStack>
        ) : null}

        {canManage && conn ? (
          <XStack gap="$2" flexWrap="wrap">
            <Button
              flex={1}
              disabled={sync.isPending || conn.status === 'SYNCING'}
              onPress={() => sync.mutate(false)}
            >
              {t('drive.sync_now')}
            </Button>
            <Button
              flex={1}
              disabled={sync.isPending || conn.status === 'SYNCING'}
              onPress={() => sync.mutate(true)}
            >
              {t('drive.full_rescan')}
            </Button>
          </XStack>
        ) : null}
        <ErrorText>{sync.isError ? describeError(t, sync.error) : null}</ErrorText>

        {s?.configured && canManage && (!conn || changing) ? (
          <YStack gap="$3">
            <Separator />
            <H4>{t('drive.how_to_title')}</H4>
            <Paragraph>1. {t('drive.how_to_1')}</Paragraph>
            <Paragraph userSelect="text">
              2. {t('drive.how_to_2', { email: s.service_account_email })}
            </Paragraph>
            {s.service_account_email ? <CopyButton value={s.service_account_email} /> : null}
            <Paragraph>3. {t('drive.how_to_3')}</Paragraph>
            <Field
              id="drive-folder"
              label={t('drive.folder_link')}
              value={folder}
              onChangeText={setFolder}
              autoCapitalize="none"
              autoCorrect={false}
              placeholder="https://drive.google.com/drive/folders/…"
            />
            <ErrorText>
              {connect.isError
                ? describeError(t, connect.error, { email: s.service_account_email ?? '' })
                : null}
            </ErrorText>
            <Button
              theme="accent"
              disabled={folder.trim().length < 10 || connect.isPending}
              onPress={() =>
                connect.mutate(folder.trim(), {
                  onSuccess: () => {
                    setFolder('');
                    setChanging(false);
                  },
                })
              }
            >
              {t('drive.connect')}
            </Button>
          </YStack>
        ) : null}

        {canManage && conn && !changing ? (
          <XStack gap="$2">
            <Button flex={1} onPress={() => setChanging(true)}>
              {t('drive.reconnect')}
            </Button>
            <Button
              flex={1}
              theme="red"
              disabled={disconnect.isPending}
              onPress={confirmDisconnect}
            >
              {t('drive.disconnect')}
            </Button>
          </XStack>
        ) : null}
        <ErrorText>{disconnect.isError ? describeError(t, disconnect.error) : null}</ErrorText>

        {s && canManage && s.upload_enabled && s.publisher_available ? (
          <>
            <Separator />
            <DrivePublisherCard
              groupId={groupId}
              status={s}
              onReturn={() => void status.refetch()}
            />
          </>
        ) : null}

        {!canManage && (needsSecure || !conn) ? (
          <Paragraph color="$color10">{t('drive.needs_manage')}</Paragraph>
        ) : null}
      </Screen>
    </SafeAreaView>
  );
}
