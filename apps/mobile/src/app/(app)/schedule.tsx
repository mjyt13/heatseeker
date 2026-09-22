import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { RefreshControl } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  addDays,
  dayKey,
  groupByDay,
  isOngoing,
  nextClass,
  sameDay,
  scheduleWindow,
  toCalendarDay,
  useCalendarLink,
  useSchedule,
  weekDays,
  weekStart,
  type CalendarDay,
  type Occurrence,
} from '@heatseeker/core';
import {
  Button,
  Card,
  Chip,
  EmptyState,
  ErrorText,
  H3,
  LoadingScreen,
  Paragraph,
  ScrollView,
  SizableText,
  Spinner,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { CopyButton } from '@/components/copy-button';
import { formatDayTitle, OccurrenceRow } from '@/components/schedule';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';
import { selectableText } from '@/lib/text';

/** Вкладка «Расписание»: неделя по дням, листание недель, один день по нажатию. */
export default function ScheduleScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, permissions } = useGroupContext();
  const today = toCalendarDay(new Date());
  const [monday, setMonday] = useState(() => weekStart(today));
  // A day picked in the strip; null — the whole week.
  const [day, setDay] = useState<CalendarDay | null>(null);
  const [calendarOpen, setCalendarOpen] = useState(false);
  const range = useMemo(() => scheduleWindow(monday, 7), [monday]);
  const schedule = useSchedule(groupId, range.from, range.to);

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (schedule.isPending) return <LoadingScreen />;

  const items = schedule.data ?? [];
  const byDay = groupByDay(items);
  const days = weekDays(monday);
  const now = new Date();
  const upcoming = nextClass(items, now);
  const shown = (day ? [day] : days).filter((d) => byDay.has(dayKey(d)));
  const canEdit = permissions.can('schedule.edit');
  const thisWeek = sameDay(monday, weekStart(today));

  const flip = (weeks: number) => {
    setMonday(addDays(monday, weeks * 7));
    setDay(null);
  };
  const open = (o: Occurrence) =>
    router.push({
      pathname: '/(app)/class/[eventId]/[date]',
      params: { eventId: o.event_id, date: o.date },
    });
  const last = days[6]!;
  const weekLabel = t('schedule.week_range', {
    from: `${monday.day} ${t(`calendar.month_gen_${monday.month}`)}`,
    to: `${last.day} ${t(`calendar.month_gen_${last.month}`)}`,
  });

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack paddingHorizontal="$4" paddingTop="$2" alignItems="center" gap="$2">
          <H3 flex={1}>{t('schedule.title')}</H3>
          <Button
            size="$3"
            chromeless
            aria-label={t('schedule.calendar_link')}
            icon={<Ionicons name="share-outline" size={20} />}
            onPress={() => setCalendarOpen((v) => !v)}
          />
          {canEdit ? (
            <Button
              size="$3"
              icon={<Ionicons name="add" size={18} />}
              onPress={() =>
                router.push({
                  pathname: '/(app)/class/edit',
                  params: {
                    date: dayKey(day ?? (thisWeek ? today : monday)),
                    n: String(Date.now()),
                  },
                })
              }
            >
              {t('schedule.add')}
            </Button>
          ) : null}
        </XStack>

        <XStack alignItems="center" paddingHorizontal="$2" paddingTop="$1">
          <Button
            size="$3"
            chromeless
            aria-label={t('schedule.prev_week')}
            icon={<Ionicons name="chevron-back" size={20} />}
            onPress={() => flip(-1)}
          />
          <SizableText flex={1} textAlign="center" size="$4" fontWeight="600">
            {weekLabel}
          </SizableText>
          {schedule.isFetching ? <Spinner size="small" /> : null}
          <Button
            size="$3"
            chromeless
            aria-label={t('schedule.next_week')}
            icon={<Ionicons name="chevron-forward" size={20} />}
            onPress={() => flip(1)}
          />
        </XStack>

        <XStack paddingHorizontal="$2" gap="$1">
          {days.map((d, i) => {
            const isSelected = sameDay(d, day);
            const isToday = sameDay(d, today);
            const has = byDay.has(dayKey(d));
            return (
              <YStack
                key={dayKey(d)}
                flex={1}
                alignItems="center"
                paddingVertical="$1.5"
                borderRadius="$4"
                backgroundColor={isSelected ? '$blue9' : 'transparent'}
                borderWidth={isToday && !isSelected ? 1 : 0}
                borderColor="$blue9"
                pressStyle={{ backgroundColor: isSelected ? '$blue10' : '$color4' }}
                role="button"
                aria-pressed={isSelected}
                onPress={() => setDay(isSelected ? null : d)}
              >
                <SizableText size="$1" color={isSelected ? 'white' : '$color10'}>
                  {t(`calendar.weekday_${i}`)}
                </SizableText>
                <SizableText size="$4" fontWeight="600" color={isSelected ? 'white' : undefined}>
                  {d.day}
                </SizableText>
                <YStack
                  width={5}
                  height={5}
                  borderRadius={3}
                  backgroundColor={has ? (isSelected ? 'white' : '$blue9') : 'transparent'}
                />
              </YStack>
            );
          })}
        </XStack>
        <XStack paddingHorizontal="$4" paddingTop="$2" gap="$2">
          {day ? <Chip label={t('schedule.week')} onPress={() => setDay(null)} /> : null}
          {!thisWeek ? (
            <Chip
              label={t('schedule.today')}
              onPress={() => {
                setMonday(weekStart(today));
                setDay(null);
              }}
            />
          ) : null}
        </XStack>

        <ScrollView
          contentContainerStyle={{ padding: 12, paddingBottom: 32 }}
          refreshControl={
            <RefreshControl
              refreshing={schedule.isRefetching && !schedule.isPlaceholderData}
              onRefresh={() => void schedule.refetch()}
            />
          }
        >
          {calendarOpen && groupId ? (
            <CalendarLinkCard groupId={groupId} onClose={() => setCalendarOpen(false)} />
          ) : null}
          <ErrorText>{schedule.isError ? describeError(t, schedule.error) : null}</ErrorText>
          {permissions.needsSecuring('schedule.edit') ? (
            <Paragraph size="$2" color="$color10" paddingBottom="$2">
              {t('schedule.secure_to_edit')}
            </Paragraph>
          ) : null}
          {shown.length === 0 ? (
            <EmptyState
              title={day ? t('schedule.empty_day') : t('schedule.empty_week')}
              hint={canEdit ? t('schedule.empty_hint_editor') : t('schedule.empty_hint')}
            />
          ) : (
            shown.map((d) => (
              <YStack key={dayKey(d)} gap="$1" paddingBottom="$3">
                <SizableText
                  size="$3"
                  fontWeight="600"
                  color={sameDay(d, today) ? '$blue10' : '$color10'}
                  paddingHorizontal="$1"
                >
                  {formatDayTitle(t, d)}
                </SizableText>
                {byDay.get(dayKey(d))!.map((o) => (
                  <OccurrenceRow
                    key={`${o.event_id}:${o.date}`}
                    occurrence={o}
                    badge={isOngoing(o, now) ? 'now' : o === upcoming ? 'next' : null}
                    onPress={() => open(o)}
                  />
                ))}
              </YStack>
            ))
          )}
        </ScrollView>
      </YStack>
    </SafeAreaView>
  );
}

/** Личная ссылка на расписание для календаря телефона. */
function CalendarLinkCard({ groupId, onClose }: { groupId: string; onClose: () => void }) {
  const { t } = useTranslation();
  const link = useCalendarLink(groupId);
  return (
    <Card padding="$3" gap="$2" marginBottom="$3" borderWidth={1} borderColor="$borderColor">
      <XStack alignItems="center">
        <SizableText flex={1} size="$4" fontWeight="600">
          {t('schedule.calendar_title')}
        </SizableText>
        <Button
          size="$2"
          chromeless
          aria-label={t('common.close')}
          icon={<Ionicons name="close" size={18} />}
          onPress={onClose}
        />
      </XStack>
      <Paragraph size="$2" color="$color10">
        {t('schedule.calendar_hint')}
      </Paragraph>
      {link.isPending ? <Spinner /> : null}
      <ErrorText>{link.isError ? describeError(t, link.error) : null}</ErrorText>
      {link.data ? (
        <>
          <Paragraph size="$1" color="$color10" numberOfLines={2} {...selectableText}>
            {link.data}
          </Paragraph>
          <CopyButton value={link.data} label={t('schedule.calendar_copy')} />
        </>
      ) : null}
    </Card>
  );
}
