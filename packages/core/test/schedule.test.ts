import { expect, test } from 'vitest';

import type { Occurrence } from '../src/queries/schedule';
import {
  addMinutes,
  atTime,
  classTime,
  dayKey,
  groupByDay,
  isOngoing,
  minutesBetween,
  nextClass,
  parseDayKey,
  scheduleWindow,
  weekDays,
  weekdayOf,
  weekStart,
} from '../src/schedule';

function occ(start: Date, minutes: number, extra: Partial<Occurrence> = {}): Occurrence {
  return {
    event_id: 'e1',
    date: dayKey({ year: start.getFullYear(), month: start.getMonth(), day: start.getDate() }),
    starts_at: start.toISOString(),
    ends_at: new Date(start.getTime() + minutes * 60_000).toISOString(),
    timezone: 'Europe/Moscow',
    title: 'Матан',
    kind: 'LECTURE',
    location: '',
    teacher: '',
    note: '',
    status: 'SCHEDULED',
    recurring: true,
    version: 1,
    ...extra,
  };
}

test('weekStart goes back to Monday', () => {
  // 2026-09-24 is a Thursday; 2026-09-27 a Sunday.
  expect(weekStart({ year: 2026, month: 8, day: 24 })).toEqual({ year: 2026, month: 8, day: 21 });
  expect(weekStart({ year: 2026, month: 8, day: 27 })).toEqual({ year: 2026, month: 8, day: 21 });
  expect(weekStart({ year: 2026, month: 8, day: 21 })).toEqual({ year: 2026, month: 8, day: 21 });
  // across a month
  expect(weekStart({ year: 2026, month: 9, day: 1 })).toEqual({ year: 2026, month: 8, day: 28 });
});

test('weekDays and weekdayOf', () => {
  const days = weekDays({ year: 2026, month: 8, day: 28 });
  expect(days.map(dayKey)).toEqual([
    '2026-09-28',
    '2026-09-29',
    '2026-09-30',
    '2026-10-01',
    '2026-10-02',
    '2026-10-03',
    '2026-10-04',
  ]);
  expect(days.map(weekdayOf)).toEqual(['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']);
});

test('dayKey round trip', () => {
  expect(dayKey({ year: 2026, month: 0, day: 5 })).toBe('2026-01-05');
  expect(parseDayKey('2026-01-05')).toEqual({ year: 2026, month: 0, day: 5 });
  expect(parseDayKey('05.01.2026')).toBeNull();
});

test('scheduleWindow spans whole local days', () => {
  const w = scheduleWindow({ year: 2026, month: 8, day: 21 }, 7);
  expect(new Date(w.from).getDate()).toBe(21);
  expect(new Date(w.from).getHours()).toBe(0);
  expect(new Date(w.to).getDate()).toBe(28);
});

test('groupByDay and classTime', () => {
  const day = { year: 2026, month: 8, day: 21 };
  const a = occ(new Date(atTime(day, { hour: 10, minute: 0 })), 90);
  const b = occ(new Date(atTime(day, { hour: 13, minute: 30 })), 90);
  const c = occ(new Date(atTime({ ...day, day: 22 }, { hour: 9, minute: 0 })), 45);
  const grouped = groupByDay([a, b, c]);
  expect(grouped.get('2026-09-21')).toEqual([a, b]);
  expect(grouped.get('2026-09-22')).toEqual([c]);
  expect(classTime(a)).toBe('10:00–11:30');
});

test('ongoing and next class skip cancelled ones', () => {
  const now = new Date(2026, 8, 21, 10, 30);
  const running = occ(new Date(2026, 8, 21, 10, 0), 90);
  const cancelled = occ(new Date(2026, 8, 21, 12, 0), 90, { status: 'CANCELLED' });
  const later = occ(new Date(2026, 8, 21, 13, 30), 90);
  expect(isOngoing(running, now)).toBe(true);
  expect(isOngoing(later, now)).toBe(false);
  expect(nextClass([later, cancelled, running], now)).toBe(later);
  expect(nextClass([running], now)).toBeNull();
});

test('minutesBetween and addMinutes', () => {
  expect(minutesBetween({ hour: 10, minute: 0 }, { hour: 11, minute: 30 })).toBe(90);
  expect(minutesBetween({ hour: 23, minute: 0 }, { hour: 0, minute: 30 })).toBe(90);
  expect(addMinutes({ hour: 23, minute: 0 }, 90)).toEqual({ hour: 0, minute: 30 });
  expect(addMinutes({ hour: 8, minute: 30 }, 95)).toEqual({ hour: 10, minute: 5 });
});
