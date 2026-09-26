import Ionicons from '@expo/vector-icons/Ionicons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FlatList, KeyboardAvoidingView, Platform } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  firstUnreadId,
  visibleMessages,
  pendingFor,
  snippet,
  useDeleteMessage,
  useEditMessage,
  useHideMessage,
  useMarkThreadRead,
  useModerateMessage,
  useOutbox,
  useRestoreMessage,
  useRetryMessage,
  useSendMessage,
  useThread,
  type DiscussionTarget,
  type Message,
  type OutboxItem,
  type ThreadTargetType,
} from '@heatseeker/core';
import { THREAD_TARGETS } from '@heatseeker/shared';
import {
  Button,
  confirm,
  EmptyState,
  ErrorText,
  H4,
  LoadingScreen,
  Paragraph,
  SizableText,
  Spinner,
  Switch,
  XStack,
  YStack,
} from '@heatseeker/ui';

import {
  Composer,
  MessageActions,
  MessageBubble,
  PendingBubble,
  UnreadDivider,
  type MessageAction,
} from '@/components/discussions';
import { MuteButton } from '@/components/mute-button';
import { describeError } from '@/lib/errors';
import { useScreenFocused } from '@/lib/focus';
import { subjectLabel, useGroupContext } from '@/lib/group';
import { newId } from '@/lib/ids';

type Row =
  | { kind: 'message'; key: string; message: Message; showAuthor: boolean }
  | { kind: 'pending'; key: string; item: OutboxItem }
  | { kind: 'divider'; key: string };

/** Обсуждение: предмета, всей группы, материала или задачи. */
export default function DiscussionScreen() {
  const { type, id, title } = useLocalSearchParams<{ type: string; id: string; title?: string }>();
  // A hidden tab stays mounted: a fresh view per discussion resets the draft,
  // the selection and the unread divider.
  if (!(THREAD_TARGETS as readonly string[]).includes(type)) return null;
  return (
    <DiscussionView
      key={`${type}:${id}`}
      target={{ type: type as ThreadTargetType, id }}
      fallbackTitle={title}
    />
  );
}

function DiscussionView({
  target,
  fallbackTitle,
}: {
  target: DiscussionTarget;
  fallbackTitle?: string;
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const focused = useScreenFocused();
  const ctx = useGroupContext();
  const groupId = ctx.groupId ?? '';
  const [showHidden, setShowHidden] = useState(false);
  const thread = useThread(groupId, target, { includeHidden: showHidden, live: focused });
  const outbox = useOutbox((s) => s.items);
  const send = useSendMessage(groupId, target);
  const retry = useRetryMessage();
  const markRead = useMarkThreadRead(groupId);
  const edit = useEditMessage(groupId);
  const remove = useDeleteMessage(groupId);
  const restore = useRestoreMessage(groupId);
  const hide = useHideMessage(groupId);
  const moderate = useModerateMessage(groupId);

  const [draft, setDraft] = useState('');
  const [replyTo, setReplyTo] = useState<Message | null>(null);
  const [editing, setEditing] = useState<Message | null>(null);
  const [selected, setSelected] = useState<Message | null>(null);
  // Where "new messages" started when the discussion was opened; it stays put while reading.
  const [readMark, setReadMark] = useState<number | null>(null);
  const markedSeq = useRef(0);

  const newest = thread.data?.pages[0];
  const messages = thread.messages;
  const lastSeq = newest?.last_seq ?? 0;

  // Captured once, on the first page: adjusting state while rendering is fine here.
  if (newest && readMark === null) setReadMark(newest.last_read_seq);

  // Reading the newest page marks the discussion read.
  useEffect(() => {
    if (!focused || !lastSeq || lastSeq <= markedSeq.current) return;
    if (newest && lastSeq <= newest.last_read_seq) {
      markedSeq.current = lastSeq;
      return;
    }
    markedSeq.current = lastSeq;
    markRead.mutate({ target, seq: lastSeq });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focused, lastSeq]);

  const pending = useMemo(
    () => pendingFor(outbox, groupId, target, messages),
    [outbox, groupId, target, messages],
  );
  // Messages I hid are not shown at all, unless "show hidden" is on.
  const shown = useMemo(() => visibleMessages(messages, showHidden), [messages, showHidden]);
  const dividerBefore = readMark === null ? null : firstUnreadId(shown, readMark);

  // FlatList is inverted: the newest row comes first.
  const rows = useMemo(() => {
    const out: Row[] = [];
    shown.forEach((m, i) => {
      const prev = shown[i - 1];
      if (m.id === dividerBefore) out.push({ kind: 'divider', key: 'divider' });
      out.push({
        kind: 'message',
        key: m.id,
        message: m,
        showAuthor: !prev || prev.author_id !== m.author_id || prev.id === dividerBefore,
      });
    });
    for (const item of pending) out.push({ kind: 'pending', key: item.client_id, item });
    return out.reverse();
  }, [shown, pending, dividerBefore]);

  const header = useHeader(target, thread.data?.pages[0]?.thread?.title ?? fallbackTitle);
  const canWrite = ctx.permissions.can('thread.write');
  const canModerate = ctx.permissions.can('message.moderate');
  const actionError = edit.error ?? remove.error ?? restore.error ?? hide.error ?? moderate.error;

  const submit = () => {
    const body = draft.trim();
    if (!body) return;
    if (editing) {
      edit.mutate(
        { messageId: editing.id, body },
        {
          onSuccess: () => {
            setEditing(null);
            setDraft('');
          },
        },
      );
      return;
    }
    send(
      newId(),
      body,
      replyTo
        ? { id: replyTo.id, author_name: replyTo.author_name, body: snippet(replyTo.body, 200) }
        : undefined,
    );
    setDraft('');
    setReplyTo(null);
  };

  const actions = (m: Message): MessageAction[] => {
    const close = () => setSelected(null);
    const list: MessageAction[] = [];
    if (m.deleted) {
      if (m.mine) {
        list.push({
          key: 'restore',
          label: t('discussions.restore'),
          icon: 'arrow-undo-circle-outline',
          onPress: () => {
            restore.mutate(m.id);
            close();
          },
        });
      }
      return list;
    }
    if (canWrite && m.body) {
      list.push({
        key: 'reply',
        label: t('discussions.reply'),
        icon: 'arrow-undo-outline',
        onPress: () => {
          setEditing(null);
          setReplyTo(m);
          close();
        },
      });
    }
    if (m.mine && !m.hidden_for_all) {
      list.push({
        key: 'edit',
        label: t('discussions.edit'),
        icon: 'create-outline',
        onPress: () => {
          setReplyTo(null);
          setEditing(m);
          setDraft(m.body);
          close();
        },
      });
    }
    if (m.mine) {
      list.push({
        key: 'delete',
        label: t('discussions.delete'),
        icon: 'trash-outline',
        destructive: true,
        onPress: async () => {
          close();
          const ok = await confirm({
            title: t('discussions.delete'),
            message: t('discussions.delete_confirm'),
            confirmText: t('common.delete'),
            cancelText: t('common.cancel'),
            destructive: true,
          });
          if (ok) remove.mutate(m.id);
        },
      });
    } else {
      list.push({
        key: 'hide',
        label: m.hidden_by_me ? t('discussions.unhide') : t('discussions.hide_for_me'),
        icon: m.hidden_by_me ? 'eye-outline' : 'eye-off-outline',
        onPress: () => {
          hide.mutate({ messageId: m.id, hidden: !m.hidden_by_me });
          close();
        },
      });
    }
    if (canModerate) {
      list.push({
        key: 'moderate',
        label: m.hidden_for_all ? t('discussions.unhide_for_all') : t('discussions.hide_for_all'),
        icon: m.hidden_for_all ? 'shield-checkmark-outline' : 'shield-outline',
        destructive: !m.hidden_for_all,
        onPress: () => {
          moderate.mutate({ messageId: m.id, hidden: !m.hidden_for_all });
          close();
        },
      });
    }
    return list;
  };

  if (thread.isPending) return <LoadingScreen />;

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top', 'bottom']}>
      <KeyboardAvoidingView
        style={{ flex: 1 }}
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
      >
        <YStack flex={1} backgroundColor="$background">
          <XStack alignItems="center" gap="$2" paddingHorizontal="$2" paddingVertical="$2">
            <Button
              size="$3"
              chromeless
              icon={<Ionicons name="chevron-back" size={20} />}
              onPress={() => router.back()}
            />
            <YStack flex={1} onPress={header.onPress}>
              <H4 numberOfLines={1}>{header.title}</H4>
              {header.subtitle ? (
                <SizableText size="$2" color="$color10" numberOfLines={1}>
                  {header.subtitle}
                </SizableText>
              ) : null}
            </YStack>
            <XStack alignItems="center" gap="$1.5">
              {groupId && (target.type === 'SUBJECT' || newest?.thread) ? (
                <MuteButton
                  groupId={groupId}
                  scopeType={target.type === 'SUBJECT' ? 'SUBJECT' : 'THREAD'}
                  scopeId={target.type === 'SUBJECT' ? target.id : newest!.thread!.id}
                />
              ) : null}
              <Paragraph size="$1" color="$color10">
                {t('discussions.show_hidden')}
              </Paragraph>
              <Switch size="$2" checked={showHidden} onCheckedChange={setShowHidden}>
                <Switch.Thumb />
              </Switch>
            </XStack>
          </XStack>

          {thread.isError ? <ErrorText>{describeError(t, thread.error)}</ErrorText> : null}

          {rows.length || messages.length ? (
            <FlatList
              inverted
              data={rows}
              keyExtractor={(r) => r.key}
              contentContainerStyle={{ paddingVertical: 8 }}
              onEndReachedThreshold={0.3}
              onEndReached={() => {
                if (thread.hasNextPage && !thread.isFetchingNextPage) void thread.fetchNextPage();
              }}
              ListFooterComponent={thread.isFetchingNextPage ? <Spinner margin="$3" /> : null}
              keyboardShouldPersistTaps="handled"
              renderItem={({ item: row }) => {
                switch (row.kind) {
                  case 'divider':
                    return <UnreadDivider />;
                  case 'pending':
                    return (
                      <PendingBubble
                        item={row.item}
                        onRetry={() => retry(row.item.client_id)}
                        onDiscard={() => useOutbox.getState().remove(row.item.client_id)}
                      />
                    );
                  case 'message':
                    return (
                      <MessageBubble
                        message={row.message}
                        showAuthor={row.showAuthor}
                        selected={selected?.id === row.message.id}
                        onPress={() =>
                          setSelected(selected?.id === row.message.id ? null : row.message)
                        }
                      />
                    );
                }
              }}
            />
          ) : (
            <YStack flex={1} justifyContent="center">
              <EmptyState
                title={t('discussions.thread_empty')}
                hint={t('discussions.thread_empty_hint')}
              />
            </YStack>
          )}

          {actionError ? <ErrorText>{describeError(t, actionError)}</ErrorText> : null}
          {selected && actions(selected).length ? (
            <MessageActions actions={actions(selected)} onClose={() => setSelected(null)} />
          ) : null}

          {canWrite ? (
            <Composer
              value={draft}
              onChange={setDraft}
              onSend={submit}
              pending={edit.isPending}
              context={
                editing
                  ? t('discussions.editing')
                  : replyTo
                    ? `${t('discussions.replying_to', { name: replyTo.author_name })}: ${snippet(replyTo.body, 60)}`
                    : null
              }
              onCancelContext={() => {
                if (editing) setDraft('');
                setEditing(null);
                setReplyTo(null);
              }}
            />
          ) : (
            <Paragraph size="$2" color="$color10" textAlign="center" padding="$3">
              {t('discussions.guest_readonly')}
            </Paragraph>
          )}
        </YStack>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

/** Шапка: что обсуждаем; для материала и задачи — переход к ним. */
function useHeader(target: DiscussionTarget, knownTitle?: string | null) {
  const { t } = useTranslation();
  const router = useRouter();
  const { subjectById } = useGroupContext();
  switch (target.type) {
    case 'GENERAL':
      return { title: t('discussions.general'), subtitle: t('discussions.general_hint') };
    case 'SUBJECT': {
      const subject = subjectById.get(target.id);
      return {
        title: subject?.name ?? knownTitle ?? t('thread_targets.SUBJECT'),
        subtitle: subject?.teacher ?? subjectLabel(subject) ?? null,
      };
    }
    case 'MATERIAL':
      return {
        title: knownTitle ?? t('thread_targets.MATERIAL'),
        subtitle: t('thread_targets.MATERIAL'),
        onPress: () => router.push({ pathname: '/(app)/material/[id]', params: { id: target.id } }),
      };
    case 'TASK':
      return {
        title: knownTitle ?? t('thread_targets.TASK'),
        subtitle: t('thread_targets.TASK'),
        onPress: () => router.push({ pathname: '/(app)/task/[id]', params: { id: target.id } }),
      };
    default:
      return { title: knownTitle ?? t(`thread_targets.${target.type}`), subtitle: null };
  }
}
