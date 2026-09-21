import Ionicons from '@expo/vector-icons/Ionicons';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  addDays,
  dueFrom,
  formatDue,
  formatTime,
  isDateOnly,
  monthGrid,
  parseTime,
  sameDay,
  shiftMonth,
  splitDue,
  toCalendarDay,
  type CalendarDay,
  type TimeOfDay,
} from '@heatseeker/core';
import {
  Button,
  Chip,
  Input,
  Paragraph,
  SizableText,
  Switch,
  XStack,
  YStack,
} from '@heatseeker/ui';

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
  const [view, setView] = useState(() => {
    const d = initial?.day ?? today;
    return { year: d.year, month: d.month };
  });
  const [withTime, setWithTime] = useState(!!initial?.time);
  const [timeText, setTimeText] = useState(initial?.time ? formatTime(initial.time) : '');

  const selected = initial?.day ?? null;
  const time = withTime ? parseTime(timeText) : null;
  const timeError = withTime && timeText.trim() !== '' && !time;

  const pick = (day: CalendarDay, nextTime: TimeOfDay | null = time) => {
    setView({ year: day.year, month: day.month });
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

      <YStack borderWidth={1} borderColor="$borderColor" borderRadius="$4" padding="$2" gap="$1">
        <XStack alignItems="center" justifyContent="space-between">
          <Button
            size="$3"
            chromeless
            aria-label={t('calendar.prev')}
            icon={<Ionicons name="chevron-back" size={18} />}
            onPress={() => setView(shiftMonth(view.year, view.month, -1))}
          />
          <SizableText size="$4" fontWeight="600">
            {`${t(`calendar.month_${view.month}`)} ${view.year}`}
          </SizableText>
          <Button
            size="$3"
            chromeless
            aria-label={t('calendar.next')}
            icon={<Ionicons name="chevron-forward" size={18} />}
            onPress={() => setView(shiftMonth(view.year, view.month, 1))}
          />
        </XStack>
        <XStack>
          {Array.from({ length: 7 }, (_, i) => (
            <SizableText key={i} flex={1} textAlign="center" size="$1" color="$color10">
              {t(`calendar.weekday_${i}`)}
            </SizableText>
          ))}
        </XStack>
        {monthGrid(view.year, view.month).map((week, wi) => (
          <XStack key={wi}>
            {week.map((dayNumber, di) => {
              if (dayNumber === null) return <YStack key={di} flex={1} height={38} />;
              const day = { year: view.year, month: view.month, day: dayNumber };
              const isSelected = sameDay(day, selected);
              const isToday = sameDay(day, today);
              return (
                <YStack
                  key={di}
                  flex={1}
                  height={38}
                  alignItems="center"
                  justifyContent="center"
                  role="button"
                  onPress={() => pick(day)}
                >
                  <YStack
                    width={34}
                    height={34}
                    borderRadius={17}
                    alignItems="center"
                    justifyContent="center"
                    backgroundColor={isSelected ? '$blue9' : 'transparent'}
                    borderWidth={isToday && !isSelected ? 1 : 0}
                    borderColor="$blue9"
                    pressStyle={{ backgroundColor: isSelected ? '$blue10' : '$color4' }}
                  >
                    <SizableText size="$3" color={isSelected ? 'white' : undefined}>
                      {dayNumber}
                    </SizableText>
                  </YStack>
                </YStack>
              );
            })}
          </XStack>
        ))}
      </YStack>

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
