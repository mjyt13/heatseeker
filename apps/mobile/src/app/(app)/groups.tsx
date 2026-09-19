import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  useCreateGroup,
  useDebouncedValue,
  useGroupSearch,
  useJoinOpenGroup,
  useMyGroups,
  useSession,
} from '@heatseeker/core';
import { LIMITS } from '@heatseeker/shared';
import {
  Avatar,
  Button,
  ErrorText,
  Field,
  H4,
  Input,
  ListRow,
  Paragraph,
  Screen,
  ScreenTitle,
  Separator,
  Spinner,
  YStack,
} from '@heatseeker/ui';

import { JoinByCode } from '@/components/join-by-code';
import { describeError } from '@/lib/errors';

/** Мои группы, поиск группы по названию (D33), создание и — внизу — вступление по коду. */
export default function GroupsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const groups = useMyGroups();
  const currentGroupId = useSession((s) => s.currentGroupId);
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  const [query, setQuery] = useState('');
  const debouncedQuery = useDebouncedValue(query);
  const search = useGroupSearch(debouncedQuery);
  const joinOpen = useJoinOpenGroup();
  const [name, setName] = useState('');
  const create = useCreateGroup();

  const goHome = () => router.replace('/(app)');
  const open = (groupId: string) => void setCurrentGroup(groupId).then(goHome);

  const mine = groups.data ?? [];
  const found = search.data ?? [];
  const typing = query.trim() !== debouncedQuery.trim();

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <ScreenTitle title={t('groups.title')} />

        {mine.length > 0 ? (
          <YStack>
            <H4>{t('groups.mine')}</H4>
            {mine.map((gm) => (
              <ListRow
                key={gm.group.id}
                leading={<Avatar name={gm.group.name} />}
                title={gm.group.name}
                subtitle={(gm.membership.roles ?? []).map((r) => t(`roles.${r}`)).join(', ')}
                trailing={
                  gm.group.id === currentGroupId ? (
                    <Ionicons name="checkmark" size={20} />
                  ) : undefined
                }
                onPress={() => open(gm.group.id)}
              />
            ))}
          </YStack>
        ) : null}

        <YStack gap="$2">
          <H4>{t('groups.find')}</H4>
          <Input
            size="$4"
            value={query}
            onChangeText={setQuery}
            placeholder={t('groups.find_placeholder')}
            autoCorrect={false}
            returnKeyType="search"
            clearButtonMode="while-editing"
          />
          <YStack>
            {found.map((g) => (
              <ListRow
                key={g.id}
                leading={<Avatar name={g.name} />}
                title={g.name}
                subtitle={[
                  t(`groups.kind.${g.kind}`),
                  t('groups.members_count', { count: g.member_count }),
                ].join(' · ')}
                trailing={
                  g.is_member ? (
                    <Paragraph size="$2" color="$color10">
                      {t('groups.member_badge')}
                    </Paragraph>
                  ) : (
                    <Button
                      size="$3"
                      theme="accent"
                      disabled={joinOpen.isPending}
                      onPress={() => joinOpen.mutate(g.id, { onSuccess: goHome })}
                    >
                      {t('groups.join_action')}
                    </Button>
                  )
                }
                onPress={g.is_member ? () => open(g.id) : undefined}
              />
            ))}
          </YStack>
          {search.isPending || (typing && found.length === 0) ? (
            <Spinner alignSelf="flex-start" />
          ) : search.isError ? (
            <ErrorText>{describeError(t, search.error)}</ErrorText>
          ) : found.length === 0 ? (
            <Paragraph color="$color10">{t('groups.find_empty')}</Paragraph>
          ) : null}
          <ErrorText>{joinOpen.isError ? describeError(t, joinOpen.error) : null}</ErrorText>
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

        <Separator />

        <JoinByCode onJoined={goHome} />
      </Screen>
    </SafeAreaView>
  );
}
