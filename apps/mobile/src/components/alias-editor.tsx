import Ionicons from '@expo/vector-icons/Ionicons';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Button, Input, Label, Paragraph, XStack, YStack } from '@heatseeker/ui';

/**
 * Синонимы предмета списком: каждый удаляется отдельно, так что имена папок
 * с запятыми не ломаются при редактировании.
 */
export function AliasEditor({
  value,
  onChange,
}: {
  value: readonly string[];
  onChange: (aliases: string[]) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');

  const add = () => {
    const alias = draft.trim().toLowerCase();
    if (alias && !value.includes(alias)) onChange([...value, alias]);
    setDraft('');
  };

  return (
    <YStack gap="$1.5">
      <Label htmlFor="subject-alias" size="$3">
        {t('subjects.aliases')}
      </Label>
      {value.length > 0 ? (
        <XStack gap="$2" flexWrap="wrap">
          {value.map((alias) => (
            <Button
              key={alias}
              size="$2"
              iconAfter={<Ionicons name="close" size={14} />}
              aria-label={t('subjects.alias_remove', { alias })}
              onPress={() => onChange(value.filter((a) => a !== alias))}
            >
              {alias}
            </Button>
          ))}
        </XStack>
      ) : null}
      <XStack gap="$2" alignItems="center">
        <Input
          id="subject-alias"
          flex={1}
          size="$4"
          value={draft}
          onChangeText={setDraft}
          placeholder={t('subjects.alias_placeholder')}
          autoCapitalize="none"
          returnKeyType="done"
          onSubmitEditing={add}
        />
        <Button size="$4" disabled={!draft.trim()} onPress={add}>
          {t('subjects.alias_add')}
        </Button>
      </XStack>
      <Paragraph size="$2" color="$color10">
        {t('subjects.aliases_hint')}
      </Paragraph>
    </YStack>
  );
}
