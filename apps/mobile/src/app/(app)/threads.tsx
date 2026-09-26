import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { RefreshControl } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useDiscussions, type Discussion } from '@heatseeker/core';
import {
  Button,
  ErrorText,
  H3,
  LoadingScreen,
  ScrollView,
  Separator,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { NotificationsBell } from '@/components/notifications-bell';
import { DiscussionRow } from '@/components/discussions';
import { SectionLabel } from '@/components/materials';
import { describeError } from '@/lib/errors';
import { discussionHref } from '@/lib/discussion';
import { subjectLabel, useGroupContext } from '@/lib/group';

/** Вкладка «Обсуждения»: общий чат, строка на каждый предмет, материалы и задачи. */
export default function ThreadsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, subjectById } = useGroupContext();
  const discussions = useDiscussions(groupId);

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (discussions.isPending) return <LoadingScreen />;

  const data = discussions.data;
  const open = (d: Discussion, title?: string | null) =>
    router.push(discussionHref({ type: d.target_type, id: d.target_id }, title));

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack
          paddingHorizontal="$4"
          paddingTop="$2"
          alignItems="center"
          justifyContent="space-between"
        >
          <H3 flex={1}>{t('discussions.title')}</H3>
          <NotificationsBell groupId={groupId} />
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="eye-off-outline" size={18} />}
            onPress={() => router.push('/(app)/discussion/hidden')}
          >
            {t('discussions.hidden_by_me')}
          </Button>
        </XStack>
        <ScrollView
          contentContainerStyle={{ padding: 12, paddingBottom: 32 }}
          refreshControl={
            <RefreshControl
              refreshing={discussions.isRefetching}
              onRefresh={() => void discussions.refetch()}
            />
          }
        >
          <ErrorText>{discussions.isError ? describeError(t, discussions.error) : null}</ErrorText>
          {data ? (
            <YStack gap="$1">
              <DiscussionRow
                title={t('discussions.general')}
                icon="people-outline"
                discussion={data.general}
                emptyHint={t('discussions.general_hint')}
                onPress={() => open(data.general)}
              />
              <Separator marginVertical="$2" />
              {(data.subjects ?? []).map((d) => {
                const subject = d.subject_id ? subjectById.get(d.subject_id) : undefined;
                const title = subject?.name ?? t('thread_targets.SUBJECT');
                return (
                  <DiscussionRow
                    key={d.target_id}
                    title={title}
                    icon="book-outline"
                    discussion={d}
                    onPress={() => open(d, title)}
                  />
                );
              })}
              {(data.others ?? []).length ? (
                <>
                  <SectionLabel>{t('discussions.others')}</SectionLabel>
                  {(data.others ?? []).map((d) => {
                    const subject = d.subject_id ? subjectById.get(d.subject_id) : undefined;
                    const title =
                      d.target_type === 'SUBJECT'
                        ? (subject?.name ?? t('thread_targets.SUBJECT'))
                        : (d.title ?? t(`thread_targets.${d.target_type}`));
                    const kind = t(`thread_targets.${d.target_type}`);
                    return (
                      <DiscussionRow
                        key={`${d.target_type}:${d.target_id}`}
                        title={`${[kind, subjectLabel(subject)].filter(Boolean).join(' · ')}: ${title}`}
                        icon={
                          d.target_type === 'TASK' ? 'checkbox-outline' : 'document-text-outline'
                        }
                        discussion={d}
                        onPress={() => open(d, title)}
                      />
                    );
                  })}
                </>
              ) : null}
            </YStack>
          ) : null}
        </ScrollView>
      </YStack>
    </SafeAreaView>
  );
}
