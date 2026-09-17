import Ionicons from '@expo/vector-icons/Ionicons';
import { useTranslation } from 'react-i18next';

import type { Material, Subject } from '@heatseeker/api-client';
import { fileIcon, formatBytes, type FileIcon, type MaterialKind } from '@heatseeker/core';
import { Chip, ListRow, Paragraph, ScrollView, XStack, YStack, useTheme } from '@heatseeker/ui';

import { subjectLabel } from '@/lib/group';

type IconName = keyof typeof Ionicons.glyphMap;

const icons: Record<FileIcon, IconName> = {
  pdf: 'document-text-outline',
  doc: 'document-outline',
  sheet: 'grid-outline',
  slides: 'easel-outline',
  image: 'image-outline',
  archive: 'archive-outline',
  text: 'reader-outline',
  file: 'document-attach-outline',
};

export const MATERIAL_KINDS: readonly MaterialKind[] = [
  'LECTURE',
  'NOTES',
  'REPORT',
  'CALC',
  'ASSIGNMENT',
  'OTHER',
];

/** Иконка файла по MIME. */
export function FileGlyph({ mime, size = 24 }: { mime: string; size?: number }) {
  const theme = useTheme();
  return <Ionicons name={icons[fileIcon(mime)]} size={size} color={theme.color10?.val} />;
}

/** «2,4 МБ». */
export function useFormatSize() {
  const { t, i18n } = useTranslation();
  return (bytes: number) => {
    const { value, unit } = formatBytes(bytes, i18n.language);
    return `${value} ${t(`units.${unit}`)}`;
  };
}

export interface MaterialRowProps {
  material: Material;
  subject?: Subject;
  uploaderName?: string;
  onPress?: () => void;
}

/** Строка ленты: тип файла, название, предмет · тип · источник. */
export function MaterialRow({ material, subject, uploaderName, onPress }: MaterialRowProps) {
  const { t } = useTranslation();
  const theme = useTheme();
  const formatSize = useFormatSize();
  const parts = [
    subjectLabel(subject) ??
      (material.needs_review ? t('materials.needs_review') : t('materials.no_subject')),
    t(`kinds.${material.kind}`),
    material.source === 'GDRIVE'
      ? t('materials.source_drive')
      : uploaderName
        ? t('materials.source_upload', { name: uploaderName })
        : null,
    material.file.size_bytes > 0 ? formatSize(material.file.size_bytes) : null,
  ].filter(Boolean);
  return (
    <ListRow
      leading={<FileGlyph mime={material.file.mime} />}
      title={material.title}
      subtitle={parts.join(' · ')}
      trailing={
        material.needs_review ? (
          <Ionicons name="alert-circle-outline" size={18} color={theme.yellow10?.val} />
        ) : undefined
      }
      onPress={onPress}
    />
  );
}

/** Горизонтальный выбор предмета (null — «без предмета» или «авто»). */
export function SubjectPicker({
  subjects,
  value,
  onChange,
  emptyLabel,
}: {
  subjects: readonly Subject[];
  value: string | null;
  onChange: (id: string | null) => void;
  emptyLabel: string;
}) {
  return (
    <ScrollView horizontal showsHorizontalScrollIndicator={false} flexGrow={0} flexShrink={0}>
      <XStack gap="$2" paddingVertical="$1">
        <Chip label={emptyLabel} selected={value === null} onPress={() => onChange(null)} />
        {subjects
          .filter((s) => !s.archived_at)
          .map((s) => (
            <Chip
              key={s.id}
              label={subjectLabel(s)!}
              color={s.color}
              selected={value === s.id}
              onPress={() => onChange(s.id)}
            />
          ))}
      </XStack>
    </ScrollView>
  );
}

/** Выбор типа материала (null — «авто»/«все»). */
export function KindPicker({
  value,
  onChange,
  emptyLabel,
}: {
  value: MaterialKind | null;
  onChange: (kind: MaterialKind | null) => void;
  emptyLabel?: string;
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$2" flexWrap="wrap">
      {emptyLabel ? (
        <Chip label={emptyLabel} selected={value === null} onPress={() => onChange(null)} />
      ) : null}
      {MATERIAL_KINDS.map((k) => (
        <Chip key={k} label={t(`kinds.${k}`)} selected={value === k} onPress={() => onChange(k)} />
      ))}
    </XStack>
  );
}

/** Подпись под заголовком раздела. */
export function SectionLabel({ children }: { children: string }) {
  return (
    <YStack paddingTop="$2">
      <Paragraph size="$2" color="$color10" textTransform="uppercase" letterSpacing={0.5}>
        {children}
      </Paragraph>
    </YStack>
  );
}
