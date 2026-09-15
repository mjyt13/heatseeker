import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useCreateGroup, useGroupPreview, useJoinGroup, useMyGroups, useSession } from '@heatseeker/core';
import { LIMITS } from '@heatseeker/shared';
import {
  Avatar,
  Button,
  ErrorText,
  Field,
  H4,
  ListRow,
  Screen,
  ScreenTitle,
  Separator,
  YStack,
} from '@heatseeker/ui';

import { describeError } from '@/lib/errors';

/** Выбор группы, создание и вступление по коду. */
export default function GroupsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const groups = useMyGroups();
  const currentGroupId = useSession((s) => s.currentGroupId);
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  const [name, setName] = useState('');
  const [code, setCode] = useState('');
  const preview = useGroupPreview(code.trim() || null);
  const create = useCreateGroup();
  const join = useJoinGroup();

  const goHome = () => router.replace('/(app)');

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <ScreenTitle title={t('groups.title')} />

        <YStack>
          {(groups.data ?? []).map((gm) => (
            <ListRow
              key={gm.group.id}
              leading={<Avatar name={gm.group.name} />}
              title={gm.group.name}
              subtitle={(gm.membership.roles ?? []).map((r) => t(`roles.${r}`)).join(', ')}
              trailing={gm.group.id === currentGroupId ? <H4>✓</H4> : null}
              onPress={() => {
                void setCurrentGroup(gm.group.id).then(goHome);
              }}
            />
          ))}
        </YStack>

        <Separator />

        <YStack gap="$3">
          <H4>{t('groups.join')}</H4>
          <Field
            id="join-code"
            label={t('groups.join_code')}
            value={code}
            onChangeText={setCode}
            autoCapitalize="none"
            autoCorrect={false}
            hint={preview.data ? t('groups.join_question', { name: preview.data.group.name }) : undefined}
          />
          <ErrorText>{join.isError ? describeError(t, join.error) : null}</ErrorText>
          <Button disabled={!preview.data || join.isPending} onPress={() => join.mutate(code.trim(), { onSuccess: goHome })}>
            {t('groups.join')}
          </Button>
        </YStack>

        <Separator />

        <YStack gap="$3">
          <H4>{t('groups.create')}</H4>
          <Field
            id="group-name"
            label={t('groups.name')}
            value={name}
            onChangeText={setName}
            maxLength={LIMITS.groupNameMax}
          />
          <ErrorText>{create.isError ? describeError(t, create.error) : null}</ErrorText>
          <Button
            theme="accent"
            disabled={name.trim().length < LIMITS.groupNameMin || create.isPending}
            onPress={() => create.mutate({ name: name.trim() }, { onSuccess: goHome })}
          >
            {t('groups.create')}
          </Button>
        </YStack>
      </Screen>
    </SafeAreaView>
  );
}
