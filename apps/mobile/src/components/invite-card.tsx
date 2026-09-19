import { useTranslation } from 'react-i18next';
import { Share } from 'react-native';

import type { Group } from '@heatseeker/api-client';
import { useRotateJoinCode } from '@heatseeker/core';
import {
  Button,
  confirm,
  ErrorText,
  H4,
  Paragraph,
  SizableText,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { CopyButton } from '@/components/copy-button';
import { describeError } from '@/lib/errors';

/** «Пригласить в группу»: код группы, копирование, системное «Поделиться», смена кода. */
export function InviteCard({ group, canRotate }: { group: Group; canRotate: boolean }) {
  const { t } = useTranslation();
  const rotate = useRotateJoinCode(group.id);
  // A join code only works while the group is open.
  const code = group.join_policy === 'OPEN' ? group.join_code : undefined;

  const share = () => {
    if (!code) return;
    void Share.share({ message: t('groups.share_message', { name: group.name, code }) });
  };

  const askRotate = async () => {
    const ok = await confirm({
      title: t('groups.rotate_code'),
      message: t('groups.rotate_confirm'),
      confirmText: t('groups.rotate_code'),
      cancelText: t('common.cancel'),
      destructive: true,
    });
    if (ok) rotate.mutate();
  };

  return (
    <YStack gap="$2">
      <H4>{t('groups.invite_title')}</H4>
      <Paragraph color="$color10">{t('groups.invite_hint')}</Paragraph>
      {code ? (
        <>
          <SizableText size="$7" fontWeight="600" letterSpacing={2}>
            {code}
          </SizableText>
          <XStack gap="$2" flexWrap="wrap">
            <CopyButton value={code} />
            <Button size="$3" onPress={share}>
              {t('groups.share_code')}
            </Button>
            {canRotate ? (
              <Button
                size="$3"
                chromeless
                disabled={rotate.isPending}
                onPress={() => void askRotate()}
              >
                {t('groups.rotate_code')}
              </Button>
            ) : null}
          </XStack>
          <ErrorText>{rotate.isError ? describeError(t, rotate.error) : null}</ErrorText>
        </>
      ) : (
        <Paragraph>{t('groups.no_code')}</Paragraph>
      )}
    </YStack>
  );
}
