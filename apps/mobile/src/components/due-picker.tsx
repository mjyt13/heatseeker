import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  addDays,
  dueFrom,
  formatDue,
  formatTime,
  isDateOnly,
  parseTime,
  sameDay,
  splitDue,
  toCalendarDay,
  type CalendarDay,
  type TimeOfDay,
} from '@heatseeker/core';
import { Chip, Input, Paragraph, SizableText, Switch, XStack, YStack } from '@heatseeker/ui';

import { CalendarMonth } from '@/components/calendar-month';

const QUICK_TIMES: TimeOfDay[] = [
  { hour: 9, minute: 0 },
  { hour: 12, minute: 0 },
  { hour: 15, minute: 0 },
  { hour: 18, minute: 0 },
  { hour: 21, minute: 0 },
];

/**
 * Срок задачи: календарь месяца, быстрые варианты и необязательное время.
 * Без времени срок — конец выбранного дня. value/onChange — ISO или null.
 */
export function DuePicker({
  value,
  onChange,
}: {
  value: string | null;
  onChange: (iso: string | null) => void;
}) {
  const { t } = useTranslation();
  const today = toCalendarDay(new Date());
  const initial = value ? splitDue(value) : null;
  const [withTime, setWithTime] = useState(!!initial?.time);
  const [timeText, setTimeText] = useState(initial?.time ? formatTime(initial.time) : '');

  const selected = initial?.day ?? null;
  const time = withTime ? parseTime(timeText) : null;
  const timeError = withTime && timeText.trim() !== '' && !time;

  const pick = (day: CalendarDay, nextTime: TimeOfDay | null = time) => {
    onChange(dueFrom(day, nextTime));
  };
  const setTime = (text: string) => {
    setTimeText(text);
    const parsed = parseTime(text);
    if (selected && parsed) onChange(dueFrom(selected, parsed));
  };
  const toggleTime = (on: boolean) => {
    setWithTime(on);
    if (!selected) return;
    const parsed = on ? parseTime(timeText) : null;
    onChange(dueFrom(selected, parsed));
  };

  return (
    <YStack gap="$2">
      <SizableText size="$4" fontWeight="600">
        {value
          ? isDateOnly(value)
            ? `${formatDue(value)} · ${t('calendar.end_of_day')}`
            : formatDue(value)
          : t('calendar.no_due')}
      </SizableText>

      <XStack gap="$2" flexWrap="wrap">
        <Chip
          label={t('calendar.today')}
          selected={sameDay(selected, today)}
          onPress={() => pick(today)}
        />
        <Chip
          label={t('calendar.tomorrow')}
          selected={sameDay(selected, addDays(today, 1))}
          onPress={() => pick(addDays(today, 1))}
        />
        <Chip
          label={t('calendar.in_week')}
          selected={sameDay(selected, addDays(today, 7))}
          onPress={() => pick(addDays(today, 7))}
        />
        <Chip label={t('calendar.no_due')} selected={!value} onPress={() => onChange(null)} />
      </XStack>

      <CalendarMonth selected={selected} onPick={(day) => pick(day)} />

      <XStack alignItems="center" gap="$2">
        <Switch size="$2" checked={withTime} onCheckedChange={toggleTime} disabled={!selected}>
          <Switch.Thumb />
        </Switch>
        <Paragraph color={selected ? undefined : '$color9'}>{t('calendar.with_time')}</Paragraph>
      </XStack>
      {withTime && selected ? (
        <YStack gap="$2">
          <XStack gap="$2" flexWrap="wrap">
            {QUICK_TIMES.map((q) => (
              <Chip
                key={formatTime(q)}
                label={formatTime(q)}
                selected={!!time && time.hour === q.hour && time.minute === q.minute}
                onPress={() => setTime(formatTime(q))}
              />
            ))}
          </XStack>
          <Input
            size="$4"
            width={120}
            value={timeText}
            onChangeText={setTime}
            placeholder={t('calendar.time_placeholder')}
            keyboardType="numbers-and-punctuation"
            maxLength={5}
            borderColor={timeError ? '$red8' : undefined}
          />
          {timeError ? (
            <Paragraph size="$2" color="$red10">
              {t('calendar.time_invalid')}
            </Paragraph>
          ) : null}
        </YStack>
      ) : null}
    </YStack>
  );
}
