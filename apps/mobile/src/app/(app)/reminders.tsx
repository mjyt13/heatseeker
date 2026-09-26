import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  dueFrom,
  parseTime,
  toCalendarDay,
  useCreateReminder,
  useDeleteReminder,
  useDoneReminder,
  useReminders,
  useSnoozeReminder,
  type CalendarDay,
  type Reminder,
  type ReminderRepeat,
} from '@heatseeker/core';
import {
  Button,
  Card,
  Chip,
  confirm,
  EmptyState,
  ErrorText,
  Field,
  H3,
  LoadingScreen,
  Paragraph,
  Screen,
  SizableText,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { CalendarMonth } from '@/components/calendar-month';
import { TimeField } from '@/components/schedule';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

const REPEATS: ReminderRepeat[] = ['NONE', 'DAILY', 'WEEKLY', 'MONTHLY'];
const SNOOZES: { key: 'snooze_1h' | 'snooze_3h' | 'snooze_1d'; minutes: number }[] = [
  { key: 'snooze_1h', minutes: 60 },
  { key: 'snooze_3h', minutes: 180 },
  { key: 'snooze_1d', minutes: 60 * 24 },
];

/** Экран «Напоминания»: личные заметки себе на время. */
export default function RemindersScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId } = useGroupContext();
  const [openOnly, setOpenOnly] = useState(true);
  const [adding, setAdding] = useState(false);
  const list = useReminders(groupId, openOnly);
  const snooze = useSnoozeReminder(groupId ?? '');
  const done = useDoneReminder(groupId ?? '');
  const remove = useDeleteReminder(groupId ?? '');

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (list.isPending) return <LoadingScreen />;

  const items = list.data ?? [];
  const error = list.error ?? snooze.error ?? done.error ?? remove.error;

  const drop = (r: Reminder) =>
    void confirm({
      title: t('reminders.delete_confirm'),
      confirmText: t('common.delete'),
      cancelText: t('common.cancel'),
      destructive: true,
    }).then((ok) => ok && remove.mutate(r.id));

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
          <H3 flex={1}>{t('reminders.title')}</H3>
          <Button
            size="$3"
            icon={<Ionicons name="add" size={18} />}
            onPress={() => setAdding((v) => !v)}
          >
            {t('reminders.new')}
          </Button>
        </XStack>

        {adding ? (
          <ReminderForm groupId={groupId} onDone={() => setAdding(false)} />
        ) : (
          <XStack>
            <Chip
              label={t('reminders.open_only')}
              selected={openOnly}
              onPress={() => setOpenOnly((v) => !v)}
            />
          </XStack>
        )}

        <ErrorText>{error ? describeError(t, error) : null}</ErrorText>

        {items.length === 0 ? (
          <EmptyState title={t('reminders.empty')} hint={t('reminders.empty_hint')} />
        ) : (
          items.map((r) => (
            <Card key={r.id} padding="$3" gap="$2" borderWidth={1} borderColor="$borderColor">
              <XStack gap="$2" alignItems="flex-start">
                <Ionicons
                  name={r.status === 'DONE' ? 'checkmark-circle' : 'alarm-outline'}
                  size={20}
                />
                <YStack flex={1} gap="$1">
                  <SizableText
                    size="$4"
                    fontWeight="600"
                    textDecorationLine={r.status === 'DONE' ? 'line-through' : 'none'}
                  >
                    {r.title}
                  </SizableText>
                  {r.note ? (
                    <Paragraph size="$2" color="$color10">
                      {r.note}
                    </Paragraph>
                  ) : null}
                  <SizableText size="$2" color="$color10">
                    {`${new Date(r.remind_at).toLocaleString()} · ${t(`reminders.repeat_${r.repeat}`)} · ${t(`reminders.status_${r.status}`)}`}
                  </SizableText>
                </YStack>
                <Button
                  size="$2"
                  chromeless
                  aria-label={t('common.delete')}
                  icon={<Ionicons name="trash-outline" size={18} />}
                  onPress={() => drop(r)}
                />
              </XStack>
              <XStack gap="$2" flexWrap="wrap">
                {r.status === 'DONE' ? (
                  <Chip
                    label={t('reminders.reopen')}
                    onPress={() => done.mutate({ reminderId: r.id, done: false })}
                  />
                ) : (
                  <>
                    {SNOOZES.map((s) => (
                      <Chip
                        key={s.key}
                        label={`${t('reminders.snooze')}: ${t(`reminders.${s.key}`)}`}
                        onPress={() => snooze.mutate({ reminderId: r.id, minutes: s.minutes })}
                      />
                    ))}
                    <Chip
                      label={t('reminders.done')}
                      onPress={() => done.mutate({ reminderId: r.id, done: true })}
                    />
                  </>
                )}
              </XStack>
            </Card>
          ))
        )}
      </Screen>
    </SafeAreaView>
  );
}

/** Форма нового напоминания: о чём, когда и как часто. */
function ReminderForm({ groupId, onDone }: { groupId: string; onDone: () => void }) {
  const { t } = useTranslation();
  const create = useCreateReminder(groupId);
  const [title, setTitle] = useState('');
  const [note, setNote] = useState('');
  const [day, setDay] = useState<CalendarDay>(() => toCalendarDay(new Date()));
  const [timeText, setTimeText] = useState('09:00');
  const [repeat, setRepeat] = useState<ReminderRepeat>('NONE');
  const [submitted, setSubmitted] = useState(false);

  const time = parseTime(timeText);
  const titleMissing = title.trim() === '';

  const submit = () => {
    setSubmitted(true);
    if (titleMissing || !time) return;
    create.mutate(
      { title: title.trim(), note: note.trim(), remind_at: dueFrom(day, time), repeat },
      {
        onSuccess: () => {
          setTitle('');
          setNote('');
          onDone();
        },
      },
    );
  };

  return (
    <Card padding="$3" gap="$3" borderWidth={1} borderColor="$borderColor">
      <Field
        id="reminder-title"
        label={t('reminders.form_title')}
        value={title}
        onChangeText={setTitle}
        error={submitted && titleMissing ? t('common.form_required') : undefined}
      />
      <Field id="reminder-note" label={t('reminders.form_note')} value={note} onChangeText={setNote} />
      <SizableText size="$2" color="$color10">
        {t('reminders.when')}
      </SizableText>
      <CalendarMonth selected={day} onPick={setDay} />
      <XStack gap="$3">
        <TimeField
          label={t('reminders.time')}
          value={timeText}
          invalid={submitted && !time}
          onChange={setTimeText}
        />
      </XStack>
      <SizableText size="$2" color="$color10">
        {t('reminders.repeat')}
      </SizableText>
      <XStack gap="$2" flexWrap="wrap">
        {REPEATS.map((r) => (
          <Chip
            key={r}
            label={t(`reminders.repeat_${r}`)}
            selected={repeat === r}
            onPress={() => setRepeat(r)}
          />
        ))}
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
