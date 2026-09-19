import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Platform } from 'react-native';

import { useLogin } from '@heatseeker/core';
import { Button, ErrorText, Field, Screen, ScreenTitle } from '@heatseeker/ui';

import { describeError } from '@/lib/errors';

const platform = Platform.OS === 'ios' ? 'IOS' : Platform.OS === 'android' ? 'ANDROID' : 'WEB';

/** Вход для защищённых аккаунтов (email + пароль). */
export default function LoginScreen() {
  const { t } = useTranslation();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const login = useLogin();

  return (
    <Screen scroll>
      <ScreenTitle title={t('auth.login_title')} />
      <Field
        id="email"
        label={t('auth.email')}
        value={email}
        onChangeText={setEmail}
        autoCapitalize="none"
        autoCorrect={false}
        type="email"
        autoComplete="email"
        textContentType="emailAddress"
      />
      <Field
        id="password"
        label={t('auth.password')}
        value={password}
        onChangeText={setPassword}
        password={{ show: t('auth.password_show'), hide: t('auth.password_hide') }}
      />
      <ErrorText>{login.isError ? describeError(t, login.error) : null}</ErrorText>
      <Button
        size="$5"
        theme="accent"
        disabled={login.isPending || !email || !password}
        onPress={() => login.mutate({ email: email.trim(), password, platform })}
      >
        {t('auth.login')}
      </Button>
    </Screen>
  );
}
