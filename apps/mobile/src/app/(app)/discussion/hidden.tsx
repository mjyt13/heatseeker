import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { FlatList, RefreshControl } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  formatMessageTime,
  snippet,
  useHiddenMessages,
  useHideMessage,
  type Message,
} from '@heatseeker/core';
import {
  Button,
  EmptyState,
  ErrorText,
  H3,
  ListRow,
  LoadingScreen,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { describeError } from '@/lib/errors';
import { discussionHref } from '@/lib/discussion';
import { useGroupContext } from '@/lib/group';

/** «Скрытые мной»: вернуть сообщение или перейти в его обсуждение. */
export default function HiddenMessagesScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, subjectById } = useGroupContext();
  const hidden = useHiddenMessages(groupId);
  const unhide = useHideMessage(groupId ?? '');

  if (hidden.isPending) return <LoadingScreen />;

  const where = (m: Message) => {
    const thread = m.thread;
    if (!thread) return '';
    if (thread.target_type === 'GENERAL') return t('discussions.general');
    if (thread.target_type === 'SUBJECT') {
      return subjectById.get(thread.target_id)?.name ?? t('thread_targets.SUBJECT');
    }
    return thread.title ?? t(`thread_targets.${thread.target_type}`);
  };

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack alignItems="center" gap="$2" paddingHorizontal="$2" paddingTop="$2">
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="chevron-back" size={20} />}
            onPress={() => router.back()}
          />
          <H3>{t('discussions.hidden_by_me')}</H3>
        </XStack>
        <ErrorText>
          {hidden.isError
            ? describeError(t, hidden.error)
            : unhide.isError
              ? describeError(t, unhide.error)
              : null}
        </ErrorText>
        <FlatList
          data={hidden.data ?? []}
          keyExtractor={(m) => m.id}
          contentContainerStyle={{ padding: 12, paddingBottom: 32 }}
          refreshControl={
            <RefreshControl
              refreshing={hidden.isRefetching}
              onRefresh={() => void hidden.refetch()}
            />
          }
          ListEmptyComponent={
            <EmptyState
              title={t('discussions.hidden_empty')}
              hint={t('discussions.hidden_empty_hint')}
            />
          }
          renderItem={({ item: m }) => (
            <ListRow
              title={`${m.author_name}: ${snippet(m.body, 60)}`}
              subtitle={`${where(m)} · ${formatMessageTime(m.created_at)}`}
              onPress={
                m.thread
                  ? () =>
                      router.push(
                        discussionHref(
                          { type: m.thread!.target_type, id: m.thread!.target_id },
                          where(m),
                        ),
                      )
                  : undefined
              }
              trailing={
                <Button
                  size="$2"
                  icon={<Ionicons name="eye-outline" size={16} />}
                  disabled={unhide.isPending}
                  onPress={() => unhide.mutate({ messageId: m.id, hidden: false })}
                >
                  {t('discussions.unhide')}
                </Button>
              }
            />
          )}
        />
      </YStack>
    </SafeAreaView>
  );
}
