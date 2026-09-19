import { useState, type ComponentProps } from 'react';
import { Button, Input, Label, Paragraph, XStack, YStack } from 'tamagui';

export interface FieldProps extends ComponentProps<typeof Input> {
  label: string;
  error?: string | null;
  hint?: string;
  /**
   * Password field: the text is hidden (native `secureTextEntry` and web
   * `type="password"`) with a show/hide toggle. Labels come from the app's i18n.
   */
  password?: { show: string; hide: string; isNew?: boolean };
}

/** Поле формы: подпись, ввод, подсказка/ошибка. */
export function Field({ label, error, hint, id, password, ...input }: FieldProps) {
  const [revealed, setRevealed] = useState(false);
  const secret: ComponentProps<typeof Input> = password
    ? {
        type: revealed ? 'text' : 'password',
        secureTextEntry: !revealed,
        autoCapitalize: 'none',
        autoCorrect: false,
        autoComplete: password.isNew ? 'new-password' : 'current-password',
        textContentType: password.isNew ? 'newPassword' : 'password',
      }
    : {};
  return (
    <YStack gap="$1.5">
      <Label htmlFor={id} size="$3">
        {label}
      </Label>
      <XStack alignItems="center" gap="$2">
        <Input
          id={id}
          size="$4"
          flex={1}
          borderColor={error ? '$red8' : undefined}
          {...input}
          {...secret}
        />
        {password ? (
          <Button size="$3" chromeless onPress={() => setRevealed((v) => !v)}>
            {revealed ? password.hide : password.show}
          </Button>
        ) : null}
      </XStack>
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
