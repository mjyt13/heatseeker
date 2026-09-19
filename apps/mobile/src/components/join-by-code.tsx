import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useDebouncedValue, useGroupPreview, useJoinGroup } from '@heatseeker/core';
import { Button, ErrorText, Field, H4, YStack } from '@heatseeker/ui';

import { describeError } from '@/lib/errors';

/**
 * Код приглашения или группы, с подсказкой «куда ведёт код». Запрос уходит после
 * паузы в наборе; «не подошёл» показывается, только когда ввод закончен.
 */
export function useCodePreview(code: string) {
  const { t } = useTranslation();
  const trimmed = code.trim();
  const debounced = useDebouncedValue(trimmed);
  const preview = useGroupPreview(debounced || null);
  const settled = debounced === trimmed;
  const hint = !trimmed
    ? t('groups.code_hint')
    : !settled
      ? undefined
      : preview.data
        ? t('groups.join_question', { name: preview.data.group.name })
        : undefined;
  const error = trimmed && settled && preview.isError ? t('groups.code_not_found') : null;
  return { preview: settled ? preview.data : undefined, hint, error };
}

/** Блок «Есть код приглашения?» внизу экрана групп. */
export function JoinByCode({ onJoined }: { onJoined: () => void }) {
  const { t } = useTranslation();
  const [code, setCode] = useState('');
  const { preview, hint, error } = useCodePreview(code);
  const join = useJoinGroup();
  return (
    <YStack gap="$3">
      <H4>{t('groups.have_code')}</H4>
      <Field
        id="join-code"
        label={t('groups.join_code')}
        value={code}
        onChangeText={setCode}
        autoCapitalize="none"
        autoCorrect={false}
        hint={hint}
        error={error}
      />
      <ErrorText>{join.isError ? describeError(t, join.error) : null}</ErrorText>
      <Button
        disabled={!preview || join.isPending}
        onPress={() => join.mutate(code.trim(), { onSuccess: onJoined })}
      >
        {t('groups.join')}
      </Button>
    </YStack>
  );
}
