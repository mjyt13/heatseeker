import Ionicons from '@expo/vector-icons/Ionicons';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useMute, useMutes, useUnmute, type MuteScope } from '@heatseeker/core';
import { Button, Chip, Paragraph, SizableText, XStack, YStack } from '@heatseeker/ui';

/** Пресеты тишины: она всегда заканчивается (D-решение по уведомлениям). */
const PRESETS: { key: 'mute_1h' | 'mute_8h' | 'mute_1d' | 'mute_1w'; hours: number }[] = [
  { key: 'mute_1h', hours: 1 },
  { key: 'mute_8h', hours: 8 },
  { key: 'mute_1d', hours: 24 },
  { key: 'mute_1w', hours: 24 * 7 },
];

/** Момент, до которого молчим. */
function until(hours: number): string {
  return new Date(Date.now() + hours * 3600_000).toISOString();
}

/**
 * Кнопка «приглушить» рядом с заголовком обсуждения или предмета: пока
 * молчит — колокольчик перечёркнут, нажатие возвращает звук.
 */
export function MuteButton({
  groupId,
  scopeType,
  scopeId,
}: {
  groupId: string;
  scopeType: MuteScope;
  scopeId: string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const mutes = useMutes(groupId);
  const mute = useMute(groupId);
  const unmute = useUnmute(groupId);
  const active = (mutes.data ?? []).find(
    (m) => m.scope_type === scopeType && m.scope_id === scopeId,
  );

  const silence = (hours: number) => {
    setOpen(false);
    mute.mutate({ scopeType, scopeId, until: until(hours) });
  };

  return (
    <YStack>
      <Button
        size="$3"
        chromeless
        aria-label={t('notifications.mute')}
        disabled={mute.isPending || unmute.isPending}
        icon={
          <Ionicons name={active ? 'notifications-off' : 'notifications-outline'} size={20} />
        }
        onPress={() => (active ? unmute.mutate({ scopeType, scopeId }) : setOpen((v) => !v))}
      />
      {open ? (
        <YStack
          position="absolute"
          top="$4"
          right={0}
          zIndex={10}
          padding="$3"
          gap="$2"
          minWidth={220}
          borderRadius="$4"
          borderWidth={1}
          borderColor="$borderColor"
          backgroundColor="$background"
        >
          <SizableText size="$3" fontWeight="600">
            {t('notifications.mute')}
          </SizableText>
          <Paragraph size="$1" color="$color10">
            {t('notifications.mute_hint')}
          </Paragraph>
          <XStack gap="$2" flexWrap="wrap">
            {PRESETS.map((p) => (
              <Chip key={p.key} label={t(`notifications.${p.key}`)} onPress={() => silence(p.hours)} />
            ))}
          </XStack>
        </YStack>
      ) : null}
    </YStack>
  );
}
