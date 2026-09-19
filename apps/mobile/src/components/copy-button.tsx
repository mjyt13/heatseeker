import Ionicons from '@expo/vector-icons/Ionicons';
import * as Clipboard from 'expo-clipboard';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Button } from '@heatseeker/ui';

/** Кнопка «Скопировать» с подтверждением «Скопировано». */
export function CopyButton({ value, label }: { value: string; label?: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  return (
    <Button
      size="$3"
      alignSelf="flex-start"
      icon={<Ionicons name={copied ? 'checkmark' : 'copy-outline'} size={16} />}
      onPress={() =>
        void Clipboard.setStringAsync(value).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        })
      }
    >
      {copied ? t('groups.copied') : (label ?? t('groups.copy_code'))}
    </Button>
  );
}
