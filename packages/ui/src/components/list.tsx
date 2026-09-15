import type { ReactNode } from 'react';
import { Paragraph, SizableText, XStack, YStack, styled } from 'tamagui';

const RowFrame = styled(XStack, {
  name: 'ListRow',
  alignItems: 'center',
  gap: '$3',
  paddingVertical: '$3',
  paddingHorizontal: '$3',
  borderRadius: '$4',
  backgroundColor: '$background',
  hoverStyle: { backgroundColor: '$backgroundHover' },
  pressStyle: { backgroundColor: '$backgroundPress' },
});

export interface ListRowProps {
  title: string;
  subtitle?: string | null;
  leading?: ReactNode;
  trailing?: ReactNode;
  onPress?: () => void;
}

/** Строка списка: иконка/аватар слева, заголовок и подзаголовок, действие справа. */
export function ListRow({ title, subtitle, leading, trailing, onPress }: ListRowProps) {
  return (
    <RowFrame onPress={onPress} accessibilityRole={onPress ? 'button' : undefined}>
      {leading}
      <YStack flex={1} gap="$0.5">
        <SizableText size="$4" numberOfLines={1}>
          {title}
        </SizableText>
        {subtitle ? (
          <Paragraph size="$2" color="$color10" numberOfLines={2}>
            {subtitle}
          </Paragraph>
        ) : null}
      </YStack>
      {trailing}
    </RowFrame>
  );
}

/** Круглый аватар-заглушка с инициалом. */
export function Avatar({ name, size = 36 }: { name: string; size?: number }) {
  const initial = name.trim().charAt(0).toUpperCase() || '?';
  return (
    <XStack
      width={size}
      height={size}
      borderRadius={size / 2}
      backgroundColor="$color5"
      alignItems="center"
      justifyContent="center"
    >
      <SizableText size="$3" fontWeight="600">
        {initial}
      </SizableText>
    </XStack>
  );
}
