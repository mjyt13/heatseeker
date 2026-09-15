import type { ComponentProps } from 'react';
import { Input, Label, Paragraph, YStack } from 'tamagui';

export interface FieldProps extends ComponentProps<typeof Input> {
  label: string;
  error?: string | null;
  hint?: string;
}

/** Поле формы: подпись, ввод, подсказка/ошибка. */
export function Field({ label, error, hint, id, ...input }: FieldProps) {
  return (
    <YStack gap="$1.5">
      <Label htmlFor={id} size="$3">
        {label}
      </Label>
      <Input id={id} size="$4" borderColor={error ? '$red8' : undefined} {...input} />
      {error ? (
        <Paragraph size="$2" color="$red10">
          {error}
        </Paragraph>
      ) : hint ? (
        <Paragraph size="$2" color="$color10">
          {hint}
        </Paragraph>
      ) : null}
    </YStack>
  );
}

/** Текст ошибки формы/запроса. */
export function ErrorText({ children }: { children?: string | null }) {
  if (!children) return null;
  return (
    <Paragraph size="$2" color="$red10">
      {children}
    </Paragraph>
  );
}
