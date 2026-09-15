import type { ReactNode } from 'react';
import { H2, Paragraph, ScrollView, Spinner, YStack } from 'tamagui';

export interface ScreenProps {
  children: ReactNode;
  /** Прокручиваемый контент (формы, списки). */
  scroll?: boolean;
  padded?: boolean;
}

/** Контейнер экрана: фон темы, отступы, опциональная прокрутка. */
export function Screen({ children, scroll = false, padded = true }: ScreenProps) {
  const body = (
    <YStack flex={1} padding={padded ? '$4' : 0} gap="$4" backgroundColor="$background">
      {children}
    </YStack>
  );
  if (!scroll) return body;
  return (
    <ScrollView flex={1} backgroundColor="$background" keyboardShouldPersistTaps="handled">
      {body}
    </ScrollView>
  );
}

/** Заголовок экрана с подписью. */
export function ScreenTitle({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <YStack gap="$1">
      <H2>{title}</H2>
      {subtitle ? <Paragraph color="$color10">{subtitle}</Paragraph> : null}
    </YStack>
  );
}

/** Центрированный индикатор загрузки на весь экран. */
export function LoadingScreen() {
  return (
    <YStack flex={1} alignItems="center" justifyContent="center" backgroundColor="$background">
      <Spinner size="large" />
    </YStack>
  );
}

/** Пустое состояние. */
export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <YStack flex={1} alignItems="center" justifyContent="center" gap="$2" padding="$6">
      <Paragraph fontWeight="600" textAlign="center">
        {title}
      </Paragraph>
      {hint ? (
        <Paragraph color="$color10" textAlign="center">
          {hint}
        </Paragraph>
      ) : null}
    </YStack>
  );
}
