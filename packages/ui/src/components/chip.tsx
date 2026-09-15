import { SizableText, XStack, styled } from 'tamagui';

const ChipFrame = styled(XStack, {
  name: 'Chip',
  alignItems: 'center',
  gap: '$1.5',
  paddingHorizontal: '$3',
  paddingVertical: '$1.5',
  borderRadius: '$10',
  borderWidth: 1,
  borderColor: '$borderColor',
  backgroundColor: '$background',
  cursor: 'pointer',
  hoverStyle: { backgroundColor: '$backgroundHover' },
  pressStyle: { backgroundColor: '$backgroundPress', scale: 0.97 },
  variants: {
    selected: {
      true: {
        backgroundColor: '$color8',
        borderColor: '$color8',
        hoverStyle: { backgroundColor: '$color9' },
      },
    },
  } as const,
});

export interface ChipProps {
  label: string;
  selected?: boolean;
  /** Цвет метки (например, цвет предмета). */
  color?: string | null;
  /** Количество непрочитанного и т.п. */
  badge?: number;
  onPress?: () => void;
}

/** Чип фильтра — быстрые теги на главном экране. */
export function Chip({ label, selected = false, color, badge, onPress }: ChipProps) {
  return (
    <ChipFrame selected={selected} onPress={onPress} accessibilityRole="button" accessibilityState={{ selected }}>
      {color ? <XStack width={8} height={8} borderRadius={4} backgroundColor={color} /> : null}
      <SizableText size="$3" color={selected ? '$color1' : '$color'} fontWeight={selected ? '600' : '400'}>
        {label}
      </SizableText>
      {badge ? (
        <SizableText size="$1" color={selected ? '$color1' : '$color10'}>
          {badge}
        </SizableText>
      ) : null}
    </ChipFrame>
  );
}
