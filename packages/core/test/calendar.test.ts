import { describe, expect, test } from 'vitest';

import { addDays, dueFrom, monthGrid, parseTime, shiftMonth, splitDue } from '../src/calendar';
import { formatDue } from '../src/tasks';

describe('monthGrid', () => {
  test('weeks start on Monday', () => {
    // 1 September 2026 is a Tuesday.
    const weeks = monthGrid(2026, 8);
    expect(weeks[0]).toEqual([null, 1, 2, 3, 4, 5, 6]);
    expect(weeks.at(-1)).toEqual([28, 29, 30, null, null, null, null]);
    expect(weeks.every((w) => w.length === 7)).toBe(true);
  });
  test('a month starting on Monday has no leading gap', () => {
    // 1 June 2026 is a Monday.
    expect(monthGrid(2026, 5)[0]?.[0]).toBe(1);
  });
});

describe('shiftMonth and addDays', () => {
  test('cross the year boundary', () => {
    expect(shiftMonth(2026, 11, 1)).toEqual({ year: 2027, month: 0 });
    expect(shiftMonth(2026, 0, -1)).toEqual({ year: 2025, month: 11 });
    expect(addDays({ year: 2026, month: 11, day: 31 }, 1)).toEqual({
      year: 2027,
      month: 0,
      day: 1,
    });
  });
});

describe('parseTime', () => {
  test('accepts HH:MM and HH.MM', () => {
    expect(parseTime('9:05')).toEqual({ hour: 9, minute: 5 });
    expect(parseTime('18.30')).toEqual({ hour: 18, minute: 30 });
    expect(parseTime('24:00')).toBeNull();
    expect(parseTime('ab')).toBeNull();
  });
});

describe('dueFrom / splitDue', () => {
  const day = { year: 2026, month: 11, day: 25 };
  test('without time the deadline is the end of the day, shown as a date', () => {
    const iso = dueFrom(day, null);
    expect(formatDue(iso)).toBe('25.12.2026');
    expect(splitDue(iso)).toEqual({ day, time: null });
  });
  test('with time', () => {
    const iso = dueFrom(day, { hour: 18, minute: 30 });
    expect(formatDue(iso)).toBe('25.12.2026 18:30');
    expect(splitDue(iso)).toEqual({ day, time: { hour: 18, minute: 30 } });
  });
});
