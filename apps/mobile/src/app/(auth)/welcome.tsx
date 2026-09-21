import Ionicons from '@expo/vector-icons/Ionicons';
import { randomUUID } from 'expo-crypto';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Platform } from 'react-native';

import {
  useDebouncedValue,
  useDiscoverGroups,
  useRegister,
  type GroupSearchItem,
} from '@heatseeker/core';
import { LIMITS } from '@heatseeker/shared';
import {
  Avatar,
  Button,
  ErrorText,
  Field,
  H1,
  H4,
  Input,
  ListRow,
  Paragraph,
  Screen,
  Spinner,
  YStack,
} from '@heatseeker/ui';

import { useCodePreview } from '@/components/join-by-code';
import { API_URL } from '@/lib/api';
import { describeError } from '@/lib/errors';

const platform = Platform.OS === 'ios' ? 'IOS' : Platform.OS === 'android' ? 'ANDROID' : 'WEB';

/** Сколько открытых групп предлагать на первом экране. */
const SUGGESTIONS = 6;

/**
 * Первый экран: имя и группа — открытые группы по названию (D33, D44), код
 * приглашения — по ссылке внизу (или сразу, если пришли по приглашению).
 */
export default function WelcomeScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const params = useLocalSearchParams<{ code?: string }>();
  const [name, setName] = useState('');
  const [code, setCode] = useState(params.code ?? '');
  const [showCode, setShowCode] = useState(!!params.code);
  const [query, setQuery] = useState('');
  const [picked, setPicked] = useState<GroupSearchItem | null>(null);
  const discover = useDiscoverGroups(useDebouncedValue(query));
  // One id per screen: repeated taps and retries map to the same account.
  const [clientId] = useState(randomUUID);
  const codePreview = useCodePreview(code);
  const register = useRegister();

  const nameError = name.trim().length === 0 && register.isError ? t('errors.validation') : null;

  const submit = () => {
    // An unrecognised code must not block sign-up: the group can be found by name later.
    const inviteCode = codePreview.preview ? code.trim() : undefined;
    register.mutate({
      name: name.trim(),
      invite_code: inviteCode,
      // A recognised code wins over a picked group.
      group_id: inviteCode ? undefined : picked?.id,
      client_id: clientId,
      platform,
    });
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
      <YStack gap="$2">
        <H4>{t('auth.group_title')}</H4>
        <Input
          size="$4"
          value={query}
          onChangeText={setQuery}
          placeholder={t('auth.group_search')}
          autoCorrect={false}
          returnKeyType="search"
        />
        {discover.isPending ? <Spinner /> : null}
        {(discover.data ?? []).slice(0, SUGGESTIONS).map((g) => {
          const selected = picked?.id === g.id;
          return (
            <ListRow
              key={g.id}
              leading={<Avatar name={g.name} />}
              title={g.name}
              subtitle={[
                t(`groups.kind.${g.kind}`),
                t('groups.members_count', { count: g.member_count }),
              ].join(' · ')}
              trailing={
                <Ionicons name={selected ? 'checkmark-circle' : 'ellipse-outline'} size={22} />
              }
              onPress={() => setPicked(selected ? null : g)}
            />
          );
        })}
        {discover.data && discover.data.length === 0 && query.trim() ? (
          <Paragraph size="$2" color="$color10">
            {t('auth.group_none_found')}
          </Paragraph>
        ) : null}
        <Paragraph size="$2" color="$color10">
          {picked ? t('auth.group_selected', { name: picked.name }) : t('auth.group_hint')}
        </Paragraph>
      </YStack>

      {showCode ? (
        <Field
          id="code"
          label={t('auth.code_optional')}
          value={code}
          onChangeText={setCode}
          autoCapitalize="none"
          autoCorrect={false}
          hint={
            code.trim()
              ? codePreview.preview && picked
                ? t('auth.code_wins')
                : codePreview.hint
              : t('auth.code_hint')
          }
          error={codePreview.error}
        />
      ) : (
        <Button chromeless size="$3" alignSelf="flex-start" onPress={() => setShowCode(true)}>
          {t('auth.have_code')}
        </Button>
      )}
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
