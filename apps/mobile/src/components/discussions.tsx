import Ionicons from '@expo/vector-icons/Ionicons';
import type { ComponentProps } from 'react';
import { useTranslation } from 'react-i18next';

import {
  formatMessageTime,
  snippet,
  type Discussion,
  type Message,
  type OutboxItem,
} from '@heatseeker/core';
import {
  Button,
  ListRow,
  Paragraph,
  SizableText,
  TextArea,
  XStack,
  YStack,
  useTheme,
} from '@heatseeker/ui';

type IconName = ComponentProps<typeof Ionicons>['name'];

/** Кружок с числом непрочитанных. */
export function UnreadBadge({ count, muted = false }: { count: number; muted?: boolean }) {
  if (count <= 0) return null;
  return (
    <XStack
      minWidth={22}
      height={22}
      paddingHorizontal="$1.5"
      borderRadius={11}
      backgroundColor={muted ? '$color6' : '$blue9'}
      alignItems="center"
      justifyContent="center"
    >
      <SizableText size="$1" fontWeight="700" color={muted ? '$color11' : 'white'}>
        {count > 99 ? '99+' : count}
      </SizableText>
    </XStack>
  );
}

/**
 * Строка экрана «Обсуждения»: название, последнее сообщение и непрочитанное.
 * `nested` — непрочитанное в обсуждениях материалов и задач предмета.
 */
export function DiscussionRow({
  title,
  icon,
  discussion,
  emptyHint,
  onPress,
}: {
  title: string;
  icon: IconName;
  discussion: Discussion;
  emptyHint?: string;
  onPress: () => void;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const last = discussion.last_message;
  const subtitle = last
    ? `${last.author_name}: ${snippet(last.body, 70)} · ${formatMessageTime(last.created_at)}`
    : (emptyHint ?? t('discussions.nobody_yet'));
  return (
    <ListRow
      title={title}
      subtitle={subtitle}
      onPress={onPress}
      leading={<Ionicons name={icon} size={22} color={theme.color10?.val} />}
      trailing={
        <XStack gap="$1.5" alignItems="center">
          <UnreadBadge count={discussion.nested_unread} muted />
          <UnreadBadge count={discussion.unread} />
        </XStack>
      }
    />
  );
}

/** Цитата сообщения, на которое отвечают. */
function Quote({ author, body, onPress }: { author: string; body: string; onPress?: () => void }) {
  return (
    <YStack
      borderLeftWidth={3}
      borderLeftColor="$blue9"
      paddingLeft="$2"
      marginBottom="$1"
      onPress={onPress}
    >
      <SizableText size="$2" fontWeight="600" numberOfLines={1}>
        {author}
      </SizableText>
      <Paragraph size="$2" color="$color10" numberOfLines={2}>
        {body}
      </Paragraph>
    </YStack>
  );
}

/**
 * Сообщение в треде. Текст скрытого или удалённого заменён пометкой; нажатие
 * выделяет сообщение и открывает действия.
 */
export function MessageBubble({
  message,
  selected,
  showAuthor,
  onPress,
}: {
  message: Message;
  selected: boolean;
  showAuthor: boolean;
  onPress: () => void;
}) {
  const { t } = useTranslation();
  const mine = message.mine;
  let note: string | null = null;
  if (message.deleted) note = t('discussions.deleted');
  else if (message.hidden_for_all && !message.body) note = t('discussions.hidden_by_moderator');
  const reply = message.reply_to;
  const replyBody = reply
    ? reply.deleted
      ? t('discussions.deleted')
      : reply.body || t('discussions.hidden_by_moderator')
    : '';

  return (
    <XStack
      justifyContent={mine ? 'flex-end' : 'flex-start'}
      paddingHorizontal="$3"
      paddingVertical="$1"
    >
      <YStack
        maxWidth="85%"
        paddingHorizontal="$3"
        paddingVertical="$2"
        borderRadius="$5"
        backgroundColor={mine ? '$color4' : '$color2'}
        borderWidth={selected ? 2 : 1}
        borderColor={selected ? '$blue9' : '$borderColor'}
        opacity={note ? 0.7 : 1}
        onPress={onPress}
        onLongPress={onPress}
      >
        {showAuthor && !mine ? (
          <SizableText size="$2" fontWeight="700" color="$blue11" numberOfLines={1}>
            {message.author_name}
          </SizableText>
        ) : null}
        {reply && !note ? <Quote author={reply.author_name} body={replyBody} /> : null}
        {note ? (
          <Paragraph size="$3" fontStyle="italic" color="$color10">
            {note}
          </Paragraph>
        ) : (
          <Paragraph size="$4" selectable>
            {message.body}
          </Paragraph>
        )}
        {message.hidden_by_me ? (
          <SizableText size="$1" color="$color10">
            {t('discussions.hidden_mine')}
          </SizableText>
        ) : null}
        {message.hidden_for_all && message.body ? (
          <SizableText size="$1" color="$red10">
            {t('discussions.hidden_by_moderator_visible')}
          </SizableText>
        ) : null}
        <SizableText size="$1" color="$color9" alignSelf="flex-end">
          {[
            message.edited_at && !message.deleted ? t('discussions.edited') : null,
            formatMessageTime(message.created_at),
          ]
            .filter(Boolean)
            .join(' · ')}
        </SizableText>
      </YStack>
    </XStack>
  );
}

/** Сообщение из офлайн-очереди: ещё не на сервере. */
export function PendingBubble({
  item,
  onRetry,
  onDiscard,
}: {
  item: OutboxItem;
  onRetry: () => void;
  onDiscard: () => void;
}) {
  const { t } = useTranslation();
  const failed = item.status === 'failed';
  return (
    <XStack justifyContent="flex-end" paddingHorizontal="$3" paddingVertical="$1">
      <YStack
        maxWidth="85%"
        paddingHorizontal="$3"
        paddingVertical="$2"
        borderRadius="$5"
        backgroundColor="$color4"
        borderWidth={1}
        borderStyle="dashed"
        borderColor={failed ? '$red8' : '$borderColor'}
        gap="$1"
      >
        {item.reply_to ? (
          <Quote author={item.reply_to.author_name} body={item.reply_to.body} />
        ) : null}
        <Paragraph size="$4">{item.body}</Paragraph>
        <XStack gap="$1.5" alignItems="center" alignSelf="flex-end">
          <Ionicons name={failed ? 'alert-circle-outline' : 'time-outline'} size={14} />
          <SizableText size="$1" color={failed ? '$red10' : '$color9'}>
            {failed
              ? t('discussions.failed', { error: item.error ?? '' })
              : t('discussions.waiting_network')}
          </SizableText>
        </XStack>
        {failed ? (
          <XStack gap="$2" alignSelf="flex-end">
            <Button size="$2" onPress={onRetry}>
              {t('discussions.retry')}
            </Button>
            <Button size="$2" theme="red" onPress={onDiscard}>
              {t('discussions.discard')}
            </Button>
          </XStack>
        ) : null}
      </YStack>
    </XStack>
  );
}

/** Черта «Новые сообщения». */
export function UnreadDivider() {
  const { t } = useTranslation();
  return (
    <XStack alignItems="center" gap="$2" paddingHorizontal="$4" paddingVertical="$2">
      <YStack flex={1} height={1} backgroundColor="$blue9" />
      <SizableText size="$1" color="$blue11">
        {t('discussions.new_messages')}
      </SizableText>
      <YStack flex={1} height={1} backgroundColor="$blue9" />
    </XStack>
  );
}

/** Действие над выделенным сообщением. */
export interface MessageAction {
  key: string;
  label: string;
  icon: IconName;
  destructive?: boolean;
  onPress: () => void;
}

/** Панель действий над выделенным сообщением — над полем ввода. */
export function MessageActions({
  actions,
  onClose,
}: {
  actions: MessageAction[];
  onClose: () => void;
}) {
  return (
    <XStack
      gap="$1.5"
      paddingHorizontal="$3"
      paddingVertical="$2"
      flexWrap="wrap"
      alignItems="center"
      borderTopWidth={1}
      borderColor="$borderColor"
      backgroundColor="$background"
    >
      {actions.map((a) => (
        <Button
          key={a.key}
          size="$2"
          theme={a.destructive ? 'red' : undefined}
          icon={<Ionicons name={a.icon} size={16} />}
          onPress={a.onPress}
        >
          {a.label}
        </Button>
      ))}
      <Button
        size="$2"
        chromeless
        marginLeft="auto"
        icon={<Ionicons name="close" size={18} />}
        onPress={onClose}
      />
    </XStack>
  );
}

/** Поле ввода с кнопкой отправки; над ним — к чему относится текст (ответ, правка). */
export function Composer({
  value,
  onChange,
  onSend,
  context,
  onCancelContext,
  pending,
}: {
  value: string;
  onChange: (text: string) => void;
  onSend: () => void;
  context?: string | null;
  onCancelContext?: () => void;
  pending?: boolean;
}) {
  const { t } = useTranslation();
  const canSend = value.trim().length > 0 && !pending;
  return (
    <YStack
      borderTopWidth={1}
      borderColor="$borderColor"
      backgroundColor="$background"
      paddingBottom="$2"
    >
      {context ? (
        <XStack paddingHorizontal="$3" paddingTop="$2" alignItems="center" gap="$2">
          <Ionicons name="return-down-forward-outline" size={16} />
          <SizableText flex={1} size="$2" color="$color10" numberOfLines={1}>
            {context}
          </SizableText>
          <Button
            size="$2"
            chromeless
            icon={<Ionicons name="close" size={16} />}
            onPress={onCancelContext}
          />
        </XStack>
      ) : null}
      <XStack paddingHorizontal="$3" paddingTop="$2" gap="$2" alignItems="flex-end">
        <TextArea
          flex={1}
          value={value}
          onChangeText={onChange}
          placeholder={t('discussions.placeholder')}
          maxLength={4000}
          minHeight={44}
          maxHeight={140}
          numberOfLines={1}
        />
        <Button
          size="$4"
          circular
          theme={canSend ? 'accent' : undefined}
          disabled={!canSend}
          icon={<Ionicons name="send" size={18} />}
          aria-label={t('discussions.send')}
          onPress={onSend}
        />
      </XStack>
    </YStack>
  );
}
