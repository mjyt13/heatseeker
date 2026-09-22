import type { TFunction } from 'i18next';
import { useId } from 'react';
import { useTranslation } from 'react-i18next';

import {
  WEEKDAYS,
  classTime,
  formatTime,
  parseDayKey,
  timeOf,
  toCalendarDay,
  type CalendarDay,
  type Occurrence,
  type ScheduleKind,
  type ScheduleRepeat,
  type Weekday,
} from '@heatseeker/core';
import { Chip, Input, Paragraph, SizableText, XStack, YStack } from '@heatseeker/ui';

import { useGroupContext } from '@/lib/group';

export const SCHEDULE_KINDS: readonly ScheduleKind[] = [
  'LECTURE',
  'SEMINAR',
  'LAB',
  'EXAM',
  'CONSULTATION',
  'OTHER',
];

/** «Понедельник, 21 сентября». */
export function formatDayTitle(t: TFunction, day: CalendarDay): string {
  const weekday = (new Date(day.year, day.month, day.day).getDay() + 6) % 7;
  return t('schedule.day_title', {
    weekday: t(`calendar.weekday_long_${weekday}`),
    day: day.day,
    month: t(`calendar.month_gen_${day.month}`),
  });
}

/** «21 сентября» по ключу дня «2026-09-21». */
export function formatDayKey(t: TFunction, key: string): string {
  const day = parseDayKey(key);
  return day ? `${day.day} ${t(`calendar.month_gen_${day.month}`)}` : key;
}

/** «Через неделю: Пн, Чт · до 28 декабря». */
export function describeRepeat(
  t: TFunction,
  repeat: ScheduleRepeat | null | undefined,
  until: string | null | undefined,
): string {
  if (!repeat) return t('schedule.one_off');
  const rule =
    repeat.interval_weeks === 2 ? t('schedule.repeat_biweekly') : t('schedule.repeat_weekly');
  const days = (repeat.weekdays ?? [])
    .map((d) => t(`calendar.weekday_${WEEKDAYS.indexOf(d)}`))
    .join(', ');
  const base = days ? t('schedule.repeat_on', { rule, days }) : rule;
  return until ? `${base} · ${t('schedule.repeat_until', { date: formatDayKey(t, until) })}` : base;
}

/** Название занятия: своё, иначе предмета, иначе тип. */
export function useClassTitle() {
  const { t } = useTranslation();
  const { subjectById } = useGroupContext();
  return (o: { title: string; subject_id?: string | null; kind: ScheduleKind }) =>
    o.title ||
    (o.subject_id ? subjectById.get(o.subject_id)?.name : undefined) ||
    t(`schedule_kinds.${o.kind}`);
}

/** Когда занятие должно было быть по плану: «пн 21 сентября, 10:00». */
export function plannedWhen(t: TFunction, o: Occurrence): string | null {
  if (!o.planned_starts_at) return null;
  const d = new Date(o.planned_starts_at);
  const day = toCalendarDay(d);
  return `${day.day} ${t(`calendar.month_gen_${day.month}`)}, ${formatTime(timeOf(o.planned_starts_at))}`;
}

/** Строка занятия в списке: время, название и тип, аудитория и преподаватель, статус. */
export function OccurrenceRow({
  occurrence: o,
  badge,
  onPress,
}: {
  occurrence: Occurrence;
  /** «Идёт сейчас» / «Следующее». */
  badge?: 'now' | 'next' | null;
  onPress: () => void;
}) {
  const { t } = useTranslation();
  const title = useClassTitle()(o);
  const cancelled = o.status === 'CANCELLED';
  const changed = o.status === 'CHANGED';
  const [start, end] = classTime(o).split('–');
  const details = [t(`schedule_kinds.${o.kind}`), o.location, o.teacher]
    .filter(Boolean)
    .join(' · ');
  const status = cancelled
    ? t('schedule.cancelled')
    : changed
      ? o.planned_starts_at
        ? t('schedule.moved_from', { when: plannedWhen(t, o) })
        : t('schedule.changed')
      : null;
  return (
    <XStack
      gap="$3"
      paddingVertical="$2.5"
      paddingHorizontal="$3"
      borderRadius="$4"
      borderWidth={badge === 'now' ? 1 : 0}
      borderColor="$green8"
      backgroundColor="$background"
      pressStyle={{ backgroundColor: '$backgroundPress' }}
      role="button"
      onPress={onPress}
      opacity={cancelled ? 0.6 : 1}
    >
      <YStack width={48} alignItems="flex-end">
        <SizableText
          size="$4"
          fontWeight="600"
          textDecorationLine={cancelled ? 'line-through' : 'none'}
        >
          {start}
        </SizableText>
        <SizableText size="$2" color="$color10">
          {end}
        </SizableText>
      </YStack>
      <YStack flex={1} gap="$0.5">
        <SizableText
          size="$4"
          numberOfLines={2}
          textDecorationLine={cancelled ? 'line-through' : 'none'}
        >
          {title}
        </SizableText>
        {details ? (
          <Paragraph size="$2" color="$color10" numberOfLines={2}>
            {details}
          </Paragraph>
        ) : null}
        {status ? (
          <Paragraph size="$2" color={cancelled ? '$red10' : '$orange10'} numberOfLines={2}>
            {o.change_note ? `${status}: ${o.change_note}` : status}
          </Paragraph>
        ) : null}
        {badge ? (
          <Paragraph size="$2" color={badge === 'now' ? '$green10' : '$blue10'} fontWeight="600">
            {badge === 'now' ? t('schedule.now') : t('schedule.next')}
          </Paragraph>
        ) : null}
      </YStack>
    </XStack>
  );
}

/** Тип занятия — чипами. */
export function ScheduleKindPicker({
  value,
  onChange,
}: {
  value: ScheduleKind;
  onChange: (kind: ScheduleKind) => void;
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$2" flexWrap="wrap">
      {SCHEDULE_KINDS.map((k) => (
        <Chip
          key={k}
          label={t(`schedule_kinds.${k}`)}
          selected={value === k}
          onPress={() => onChange(k)}
        />
      ))}
    </XStack>
  );
}

/** Дни недели для повтора: выбранные залиты. */
export function WeekdayPicker({
  value,
  onToggle,
}: {
  value: readonly Weekday[];
  onToggle: (day: Weekday) => void;
}) {
  const { t } = useTranslation();
  return (
    <XStack gap="$1.5" flexWrap="wrap">
      {WEEKDAYS.map((d, i) => (
        <Chip
          key={d}
          label={t(`calendar.weekday_${i}`)}
          selected={value.includes(d)}
          onPress={() => onToggle(d)}
        />
      ))}
    </XStack>
  );
}

/** Поле времени «10:00» с подписью. */
export function TimeField({
  label,
  value,
  invalid,
  onChange,
}: {
  label: string;
  value: string;
  invalid: boolean;
  onChange: (text: string) => void;
}) {
  const { t } = useTranslation();
  // Hidden tabs keep a form mounted twice: ids must differ.
  const id = useId();
  return (
    <YStack gap="$1" flex={1}>
      <SizableText size="$2" color="$color10">
        {label}
      </SizableText>
      <Input
        id={id}
        size="$4"
        value={value}
        onChangeText={onChange}
        placeholder={t('calendar.time_placeholder')}
        keyboardType="numbers-and-punctuation"
        maxLength={5}
        borderColor={invalid ? '$red8' : undefined}
      />
    </YStack>
  );
}
