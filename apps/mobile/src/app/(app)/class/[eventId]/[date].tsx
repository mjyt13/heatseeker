import Ionicons from '@expo/vector-icons/Ionicons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  classTime,
  dayKey,
  toCalendarDay,
  useDeleteScheduleEvent,
  useOccurrence,
  useResetOccurrence,
  useScheduleEvent,
  useSetOccurrence,
  type ScheduleScope,
} from '@heatseeker/core';
import {
  Button,
  confirm,
  EmptyState,
  ErrorText,
  H3,
  ListRow,
  LoadingScreen,
  Paragraph,
  Screen,
  Separator,
  SizableText,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { DiscussionButton } from '@/components/discussion-button';
import {
  describeRepeat,
  formatDayKey,
  formatDayTitle,
  plannedWhen,
  useClassTitle,
} from '@/components/schedule';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Занятие: когда и где, что изменилось; староста отменяет, переносит и правит серию. */
export default function ClassScreen() {
  const { eventId, date } = useLocalSearchParams<{ eventId: string; date: string }>();
  // A hidden tab stays mounted: a fresh view per class resets its state.
  return <ClassView key={`${eventId}:${date}`} eventId={eventId} date={date} />;
}

function ClassView({ eventId, date }: { eventId: string; date: string }) {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, permissions } = useGroupContext();
  const occurrence = useOccurrence(eventId, date);
  const event = useScheduleEvent(eventId);
  const setOccurrence = useSetOccurrence(groupId ?? '');
  const reset = useResetOccurrence(groupId ?? '');
  const remove = useDeleteScheduleEvent(groupId ?? '');
  const classTitle = useClassTitle();

  if (occurrence.isPending) return <LoadingScreen />;
  const o = occurrence.data;
  if (!o) {
    return (
      <SafeAreaView style={{ flex: 1 }} edges={['top']}>
        <Screen>
          <EmptyState
            title={occurrence.isError ? describeError(t, occurrence.error) : t('errors.not_found')}
          />
          <Button onPress={() => router.back()}>{t('common.back')}</Button>
        </Screen>
      </SafeAreaView>
    );
  }

  const title = classTitle(o);
  const canEdit = permissions.can('schedule.edit');
  const cancelled = o.status === 'CANCELLED';
  const e = event.data;
  // "This and following" makes sense from the second class of a series on.
  const first = e ? dayKey(toCalendarDay(new Date(e.starts_at))) : null;
  const splittable = o.recurring && !!first && o.date > first;
  const day = formatDayKey(t, o.date);
  const pending = setOccurrence.isPending || reset.isPending || remove.isPending;
  const error = setOccurrence.error ?? reset.error ?? remove.error;

  const edit = (params: Record<string, string>) =>
    router.push({
      pathname: '/(app)/class/edit',
      params: { eventId, date: o.date, n: String(Date.now()), ...params },
    });
  const cancelClass = () =>
    void confirm({
      title: t('schedule.cancel_class'),
      message: t('schedule.cancel_confirm', { date: day }),
      confirmText: t('schedule.cancel_class'),
      cancelText: t('common.cancel'),
      destructive: true,
    }).then((ok) => ok && setOccurrence.mutate({ eventId, date: o.date, cancelled: true }));
  const deleteClasses = (scope: ScheduleScope) =>
    void confirm({
      title: t('common.delete'),
      message: t('schedule.delete_confirm'),
      confirmText: t('common.delete'),
      cancelText: t('common.cancel'),
      destructive: true,
    }).then(
      (ok) =>
        ok &&
        remove.mutate(
          {
            eventId,
            version: e?.version ?? o.version,
            scope,
            ...(scope === 'FOLLOWING' ? { from: o.date } : {}),
          },
          { onSuccess: () => router.back() },
        ),
    );

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
          <H3 flex={1} numberOfLines={3} textDecorationLine={cancelled ? 'line-through' : 'none'}>
            {title}
          </H3>
        </XStack>

        <YStack gap="$1">
          <SizableText size="$5" fontWeight="600">
            {`${formatDayTitle(t, toCalendarDay(new Date(o.starts_at)))} · ${classTime(o)}`}
          </SizableText>
          <Paragraph color="$color10">{t(`schedule_kinds.${o.kind}`)}</Paragraph>
          {cancelled ? (
            <Paragraph color="$red10" fontWeight="600">
              {o.change_note
                ? `${t('schedule.cancelled')}: ${o.change_note}`
                : t('schedule.cancelled')}
            </Paragraph>
          ) : o.status === 'CHANGED' ? (
            <Paragraph color="$orange10" fontWeight="600">
              {[
                o.planned_starts_at
                  ? t('schedule.moved_from', { when: plannedWhen(t, o) })
                  : t('schedule.changed'),
                o.change_note,
              ]
                .filter(Boolean)
                .join(': ')}
            </Paragraph>
          ) : null}
        </YStack>

        <YStack>
          {o.location ? (
            <ListRow
              leading={<Ionicons name="location-outline" size={20} />}
              title={o.location}
              subtitle={t('schedule.location')}
            />
          ) : null}
          {o.teacher ? (
            <ListRow
              leading={<Ionicons name="person-outline" size={20} />}
              title={o.teacher}
              subtitle={t('schedule.teacher')}
            />
          ) : null}
          <ListRow
            leading={<Ionicons name="repeat-outline" size={20} />}
            title={
              e ? describeRepeat(t, e.repeat, e.until) : o.recurring ? '…' : t('schedule.one_off')
            }
            subtitle={t('schedule.series')}
          />
          {o.note ? (
            <YStack paddingHorizontal="$3" paddingTop="$2" gap="$1">
              <SizableText size="$2" color="$color10">
                {t('schedule.note')}
              </SizableText>
              <Paragraph>{o.note}</Paragraph>
            </YStack>
          ) : null}
        </YStack>

        {o.subject_id ? (
          <DiscussionButton target={{ type: 'SUBJECT', id: o.subject_id }} title={title} />
        ) : null}

        {canEdit ? (
          <>
            <Separator />
            <YStack gap="$2">
              <Button
                icon={<Ionicons name="create-outline" size={18} />}
                disabled={pending}
                onPress={() => edit({ mode: 'one' })}
              >
                {t('schedule.edit_this')}
              </Button>
              {cancelled || o.status === 'CHANGED' ? (
                <Button
                  icon={<Ionicons name="refresh-outline" size={18} />}
                  disabled={pending}
                  onPress={() => reset.mutate({ eventId, date: o.date })}
                >
                  {t('schedule.restore_class')}
                </Button>
              ) : (
                <Button
                  icon={<Ionicons name="close-circle-outline" size={18} />}
                  disabled={pending}
                  onPress={cancelClass}
                >
                  {t('schedule.cancel_class')}
                </Button>
              )}
              {o.recurring ? (
                <Button
                  icon={<Ionicons name="albums-outline" size={18} />}
                  disabled={pending || !e}
                  onPress={() => edit({ scope: 'ALL' })}
                >
                  {t('schedule.edit_series')}
                </Button>
              ) : (
                <Button
                  icon={<Ionicons name="albums-outline" size={18} />}
                  disabled={pending || !e}
                  onPress={() => edit({ scope: 'ALL' })}
                >
                  {t('common.edit')}
                </Button>
              )}
              {splittable ? (
                <Button
                  icon={<Ionicons name="play-forward-outline" size={18} />}
                  disabled={pending}
                  onPress={() => edit({ scope: 'FOLLOWING' })}
                >
                  {t('schedule.edit_following')}
                </Button>
              ) : null}
              {splittable ? (
                <Button
                  theme="red"
                  icon={<Ionicons name="trash-outline" size={18} />}
                  disabled={pending}
                  onPress={() => deleteClasses('FOLLOWING')}
                >
                  {t('schedule.delete_following')}
                </Button>
              ) : null}
              <Button
                theme="red"
                icon={<Ionicons name="trash-outline" size={18} />}
                disabled={pending || !e}
                onPress={() => deleteClasses('ALL')}
              >
                {o.recurring ? t('schedule.delete_series') : t('schedule.delete_one_off')}
              </Button>
              <ErrorText>{error ? describeError(t, error) : null}</ErrorText>
            </YStack>
          </>
        ) : null}
      </Screen>
    </SafeAreaView>
  );
}
