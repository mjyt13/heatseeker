import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  useAnnouncements,
  useCreateAnnouncement,
  useDeleteAnnouncement,
  type Announcement,
} from '@heatseeker/core';
import {
  Button,
  Card,
  confirm,
  EmptyState,
  ErrorText,
  Field,
  H3,
  LoadingScreen,
  Paragraph,
  Screen,
  SizableText,
  Switch,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Экран «Объявления»: односторонние сообщения старосты всей группе. */
export default function AnnouncementsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, permissions } = useGroupContext();
  const [adding, setAdding] = useState(false);
  const list = useAnnouncements(groupId);
  const remove = useDeleteAnnouncement(groupId ?? '');

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (list.isPending) return <LoadingScreen />;

  const items = list.data ?? [];
  const canSend = permissions.can('announcement.send');
  const error = list.error ?? remove.error;

  const drop = (a: Announcement) =>
    void confirm({
      title: t('announcements.delete_confirm'),
      confirmText: t('common.delete'),
      cancelText: t('common.cancel'),
      destructive: true,
    }).then((ok) => ok && remove.mutate(a.id));

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <XStack alignItems="center" gap="$2">
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="chevron-back" size={20} />}
            onPress={() => router.back()}
          />
          <H3 flex={1}>{t('announcements.title')}</H3>
          {canSend ? (
            <Button
              size="$3"
              icon={<Ionicons name="megaphone-outline" size={18} />}
              onPress={() => setAdding((v) => !v)}
            >
              {t('announcements.new')}
            </Button>
          ) : null}
        </XStack>

        {permissions.needsSecuring('announcement.send') ? (
          <Paragraph size="$2" color="$color10">
            {t('announcements.secure_to_send')}
          </Paragraph>
        ) : null}

        {adding && groupId ? (
          <AnnouncementForm groupId={groupId} onDone={() => setAdding(false)} />
        ) : null}

        <ErrorText>{error ? describeError(t, error) : null}</ErrorText>

        {items.length === 0 ? (
          <EmptyState title={t('announcements.empty')} hint={t('announcements.empty_hint')} />
        ) : (
          items.map((a) => (
            <Card
              key={a.id}
              padding="$3"
              gap="$2"
              borderWidth={1}
              borderColor={a.urgent ? '$red8' : '$borderColor'}
            >
              <XStack gap="$2" alignItems="flex-start">
                <Ionicons name={a.urgent ? 'warning-outline' : 'megaphone-outline'} size={20} />
                <YStack flex={1} gap="$1">
                  <SizableText size="$4" fontWeight="600">
                    {a.title}
                  </SizableText>
                  {a.body ? <Paragraph size="$3">{a.body}</Paragraph> : null}
                  <SizableText size="$1" color="$color10">
                    {[
                      a.author_name,
                      new Date(a.created_at).toLocaleString(),
                      a.pinned ? t('announcements.pinned_label') : null,
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                  </SizableText>
                </YStack>
                {canSend ? (
                  <Button
                    size="$2"
                    chromeless
                    aria-label={t('common.delete')}
                    icon={<Ionicons name="trash-outline" size={18} />}
                    onPress={() => drop(a)}
                  />
                ) : null}
              </XStack>
            </Card>
          ))
        )}
      </Screen>
    </SafeAreaView>
  );
}

/** Форма объявления: заголовок, текст, срочность и закрепление. */
function AnnouncementForm({ groupId, onDone }: { groupId: string; onDone: () => void }) {
  const { t } = useTranslation();
  const create = useCreateAnnouncement(groupId);
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [urgent, setUrgent] = useState(false);
  const [pinned, setPinned] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const titleMissing = title.trim() === '';

  const submit = () => {
    setSubmitted(true);
    if (titleMissing) return;
    create.mutate(
      { title: title.trim(), body: body.trim(), urgent, pinned },
      {
        onSuccess: () => {
          setTitle('');
          setBody('');
          onDone();
        },
      },
    );
  };

  return (
    <Card padding="$3" gap="$3" borderWidth={1} borderColor="$borderColor">
      <Field
        id="announcement-title"
        label={t('announcements.form_title')}
        value={title}
        onChangeText={setTitle}
        error={submitted && titleMissing ? t('common.form_required') : undefined}
      />
      <Field
        id="announcement-body"
        label={t('announcements.form_body')}
        value={body}
        onChangeText={setBody}
        multiline
        numberOfLines={4}
      />
      <XStack alignItems="center" gap="$3">
        <YStack flex={1}>
          <SizableText size="$4">{t('announcements.urgent')}</SizableText>
          <SizableText size="$1" color="$color10">
            {t('announcements.urgent_hint')}
          </SizableText>
        </YStack>
        <Switch size="$2" checked={urgent} onCheckedChange={setUrgent}>
          <Switch.Thumb />
        </Switch>
      </XStack>
      <XStack alignItems="center" gap="$3">
        <SizableText flex={1} size="$4">
          {t('announcements.pinned')}
        </SizableText>
        <Switch size="$2" checked={pinned} onCheckedChange={setPinned}>
          <Switch.Thumb />
        </Switch>
      </XStack>
      <ErrorText>{create.isError ? describeError(t, create.error) : null}</ErrorText>
      <XStack gap="$2">
        <Button flex={1} chromeless onPress={onDone}>
          {t('common.cancel')}
        </Button>
        <Button flex={1} theme="blue" disabled={create.isPending} onPress={submit}>
          {t('common.save')}
        </Button>
      </XStack>
    </Card>
  );
}
