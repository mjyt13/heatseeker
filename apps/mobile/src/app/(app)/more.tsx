import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useLogout, useMe, useSetCredentials } from '@heatseeker/core';
import { LIMITS } from '@heatseeker/shared';
import { Avatar, Button, ErrorText, Field, H4, ListRow, Paragraph, Screen, ScreenTitle, Separator, YStack } from '@heatseeker/ui';

import { describeError } from '@/lib/errors';
import { tokenStore } from '@/lib/storage';

/** Профиль: защита аккаунта, выход. */
export default function MoreScreen() {
  const { t } = useTranslation();
  const me = useMe();
  const secure = useSetCredentials();
  const logout = useLogout(async () => (await tokenStore.get())?.refreshToken ?? null);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const user = me.data;

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <ScreenTitle title={t('tabs.more')} />
        {user ? (
          <ListRow
            leading={<Avatar name={user.name} size={44} />}
            title={user.name}
            subtitle={user.secured ? t('auth.secured') : user.email ?? null}
          />
        ) : null}

        <Separator />

        {user && !user.secured ? (
          <YStack gap="$3">
            <H4>{t('auth.secure_title')}</H4>
            <Paragraph color="$color10">{t('auth.secure_hint')}</Paragraph>
            <Field
              id="secure-email"
              label={t('auth.email')}
              value={email}
              onChangeText={setEmail}
              autoCapitalize="none"
              keyboardType="email-address"
            />
            <Field
              id="secure-password"
              label={t('auth.password')}
              value={password}
              onChangeText={setPassword}
              secureTextEntry
              hint={t('auth.password_hint', { min: LIMITS.passwordMin })}
            />
            <ErrorText>{secure.isError ? describeError(t, secure.error) : null}</ErrorText>
            <Button
              theme="accent"
              disabled={secure.isPending || (!email.trim() && password.length < LIMITS.passwordMin)}
              onPress={() =>
                secure.mutate({
                  email: email.trim() || undefined,
                  password: password.length >= LIMITS.passwordMin ? password : undefined,
                })
              }
            >
              {t('auth.secure_title')}
            </Button>
          </YStack>
        ) : null}

        <Separator />

        <Button onPress={() => logout.mutate()} disabled={logout.isPending}>
          {t('auth.logout')}
        </Button>
      </Screen>
    </SafeAreaView>
  );
}
