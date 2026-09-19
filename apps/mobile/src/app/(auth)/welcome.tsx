import { randomUUID } from 'expo-crypto';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Platform } from 'react-native';

import { useRegister } from '@heatseeker/core';
import { LIMITS } from '@heatseeker/shared';
import { Button, ErrorText, Field, H1, Paragraph, Screen, YStack } from '@heatseeker/ui';

import { useCodePreview } from '@/components/join-by-code';
import { API_URL } from '@/lib/api';
import { describeError } from '@/lib/errors';

const platform = Platform.OS === 'ios' ? 'IOS' : Platform.OS === 'android' ? 'ANDROID' : 'WEB';

/** Первый экран: только имя (+ код группы, если пришли по ссылке). */
export default function WelcomeScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const params = useLocalSearchParams<{ code?: string }>();
  const [name, setName] = useState('');
  const [code, setCode] = useState(params.code ?? '');
  // One id per screen: repeated taps and retries map to the same account.
  const [clientId] = useState(randomUUID);
  const codePreview = useCodePreview(code);
  const register = useRegister();

  const nameError = name.trim().length === 0 && register.isError ? t('errors.validation') : null;

  const submit = () => {
    // An unrecognised code must not block sign-up: the group can be found by name later.
    const inviteCode = codePreview.preview ? code.trim() : undefined;
    register.mutate({ name: name.trim(), invite_code: inviteCode, client_id: clientId, platform });
  };

  return (
    <Screen scroll>
      <YStack gap="$2" paddingTop="$8">
        <H1>{t('auth.welcome_title')}</H1>
        <Paragraph color="$color10">{t('auth.welcome_hint')}</Paragraph>
      </YStack>
      <Field
        id="name"
        label={t('auth.name_placeholder')}
        value={name}
        onChangeText={setName}
        maxLength={LIMITS.userNameMax}
        autoFocus
        autoCapitalize="words"
        error={nameError}
        returnKeyType="next"
      />
      <Field
        id="code"
        label={t('auth.code_optional')}
        value={code}
        onChangeText={setCode}
        autoCapitalize="none"
        autoCorrect={false}
        hint={code.trim() ? codePreview.hint : t('auth.code_hint')}
        error={codePreview.error}
      />
      <ErrorText>{register.isError ? describeError(t, register.error) : null}</ErrorText>
      <Button
        size="$5"
        theme="accent"
        disabled={register.isPending || !name.trim()}
        onPress={submit}
      >
        {t('auth.continue')}
      </Button>
      <Button chromeless onPress={() => router.push('/(auth)/login')}>
        {t('auth.have_account')}
      </Button>
      {__DEV__ ? (
        <Paragraph size="$2" color="$color9" textAlign="center" userSelect="text">
          {t('auth.dev_api', { url: API_URL })}
        </Paragraph>
      ) : null}
    </Screen>
  );
}
