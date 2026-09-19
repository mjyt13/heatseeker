import Ionicons from '@expo/vector-icons/Ionicons';
import * as WebBrowser from 'expo-web-browser';
import { useTranslation } from 'react-i18next';
import { Platform } from 'react-native';

import type { DriveStatus } from '@heatseeker/api-client';
import { useDisconnectDrivePublisher, useStartDrivePublisher } from '@heatseeker/core';
import { Button, confirm, ErrorText, H4, ListRow, Paragraph, XStack, YStack } from '@heatseeker/ui';

import { describeError } from '@/lib/errors';

/**
 * Аккаунт Google, от имени которого загрузки публикуются на Диск (D34).
 * Вход идёт в браузере; после возврата статус перечитывается.
 */
export function DrivePublisherCard({
  groupId,
  status,
  onReturn,
}: {
  groupId: string;
  status: DriveStatus;
  onReturn: () => void;
}) {
  const { t } = useTranslation();
  const start = useStartDrivePublisher(groupId);
  const disconnect = useDisconnectDrivePublisher(groupId);
  const pub = status.publisher;

  const signIn = () =>
    start.mutate(undefined, {
      // Native: resolves when the browser is closed. Web: a new tab opens and
      // the status refreshes when this tab regains focus.
      onSuccess: (url) => void WebBrowser.openBrowserAsync(url).then(onReturn, onReturn),
    });

  const confirmDisconnect = async () => {
    const ok = await confirm({
      title: t('drive.publisher.disconnect'),
      message: t('drive.publisher.disconnect_confirm', { email: pub?.email ?? '' }),
      confirmText: t('drive.publisher.disconnect'),
      cancelText: t('common.cancel'),
      destructive: true,
    });
    if (ok) disconnect.mutate();
  };

  return (
    <YStack gap="$3">
      <H4>{t('drive.publisher.title')}</H4>
      {pub ? (
        <ListRow
          leading={<Ionicons name="logo-google" size={22} />}
          title={pub.email}
          subtitle={pub.last_error ? t('drive.publisher.revoked') : t('drive.publisher.active')}
        />
      ) : (
        <Paragraph color="$color10">
          {status.connection?.shared_drive
            ? t('drive.publisher.hint_shared')
            : t('drive.publisher.hint')}
        </Paragraph>
      )}
      {__DEV__ && Platform.OS !== 'web' ? (
        <Paragraph size="$2" color="$color10">
          {t('drive.publisher.dev_hint')}
        </Paragraph>
      ) : null}
      <ErrorText>{start.isError ? describeError(t, start.error) : null}</ErrorText>
      <ErrorText>{disconnect.isError ? describeError(t, disconnect.error) : null}</ErrorText>
      <XStack gap="$2" flexWrap="wrap">
        <Button
          flex={1}
          theme={pub && !pub.last_error ? undefined : 'accent'}
          icon={<Ionicons name="logo-google" size={18} />}
          disabled={start.isPending}
          onPress={signIn}
        >
          {pub ? t('drive.publisher.reconnect') : t('drive.publisher.connect')}
        </Button>
        {pub ? (
          <Button flex={1} theme="red" disabled={disconnect.isPending} onPress={confirmDisconnect}>
            {t('drive.publisher.disconnect')}
          </Button>
        ) : null}
      </XStack>
      {start.isSuccess ? (
        <Paragraph size="$2" color="$color10">
          {t('drive.publisher.after_sign_in')}
        </Paragraph>
      ) : null}
    </YStack>
  );
}
