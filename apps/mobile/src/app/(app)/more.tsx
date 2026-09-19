import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useInboxCount, useLogout, useMe, useSetCredentials } from '@heatseeker/core';
import { LIMITS } from '@heatseeker/shared';
import {
  Avatar,
  Button,
  ErrorText,
  Field,
  H4,
  ListRow,
  Paragraph,
  Screen,
  ScreenTitle,
  Separator,
  YStack,
} from '@heatseeker/ui';

import { InviteCard } from '@/components/invite-card';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';
import { tokenStore } from '@/lib/storage';

/** Разделы группы, профиль: защита аккаунта, выход. */
export default function MoreScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const me = useMe();
  const ctx = useGroupContext();
  const moderator = ctx.permissions.can('material.moderate');
  const inbox = useInboxCount(ctx.groupId, moderator);
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
            subtitle={user.secured ? t('auth.secured') : (user.email ?? null)}
          />
        ) : null}

        {ctx.groupId ? (
          <YStack>
            <H4>{t('more.section_group')}</H4>
            <ListRow
              leading={<Ionicons name="book-outline" size={22} />}
              title={t('subjects.manage')}
              onPress={() => router.push('/(app)/subjects')}
            />
            <ListRow
              leading={<Ionicons name="logo-google" size={22} />}
              title={t('drive.title')}
              onPress={() => router.push('/(app)/drive')}
            />
            {moderator ? (
              <ListRow
                leading={<Ionicons name="file-tray-full-outline" size={22} />}
                title={t('inbox.title')}
                trailing={inbox.data ? <H4>{inbox.data}</H4> : undefined}
                onPress={() => router.push('/(app)/inbox')}
              />
            ) : null}
            <ListRow
              leading={<Ionicons name="pulse-outline" size={22} />}
              title={t('activity.title')}
              onPress={() => router.push('/(app)/activity')}
            />
          </YStack>
        ) : null}

        {ctx.group.data ? (
          <>
            <Separator />
            <InviteCard
              group={ctx.group.data.group}
              canRotate={ctx.permissions.can('group.settings')}
            />
          </>
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
              type="email"
              autoComplete="email"
            />
            <Field
              id="secure-password"
              label={t('auth.password')}
              value={password}
              onChangeText={setPassword}
              password={{
                show: t('auth.password_show'),
                hide: t('auth.password_hide'),
                isNew: true,
              }}
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
