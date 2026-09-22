import Ionicons from '@expo/vector-icons/Ionicons';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { monthGrid, sameDay, shiftMonth, toCalendarDay, type CalendarDay } from '@heatseeker/core';
import { Button, SizableText, XStack, YStack } from '@heatseeker/ui';

/**
 * Месяц с листанием: выбранный день залит, сегодня обведён. Показанный месяц
 * следует за выбранным днём, пока его не перелистнули.
 */
export function CalendarMonth({
  selected,
  onPick,
  marked,
}: {
  selected: CalendarDay | null;
  onPick: (day: CalendarDay) => void;
  /** Дни с точкой под числом (например, дни с занятиями). */
  marked?: (day: CalendarDay) => boolean;
}) {
  const { t } = useTranslation();
  const today = toCalendarDay(new Date());
  const [view, setView] = useState(() => {
    const d = selected ?? today;
    return { year: d.year, month: d.month };
  });
  // Follow a day picked from outside (quick chips) to its month.
  const [shown, setShown] = useState(selected);
  if (selected && !sameDay(selected, shown)) {
    setShown(selected);
    if (selected.year !== view.year || selected.month !== view.month) {
      setView({ year: selected.year, month: selected.month });
    }
  }

  return (
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
                onPress={() => onPick(day)}
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
                  {marked?.(day) ? (
                    <YStack
                      position="absolute"
                      bottom={3}
                      width={4}
                      height={4}
                      borderRadius={2}
                      backgroundColor={isSelected ? 'white' : '$blue9'}
                    />
                  ) : null}
                </YStack>
              </YStack>
            );
          })}
        </XStack>
      ))}
    </YStack>
  );
}
