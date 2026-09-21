import { END_OF_DAY } from './tasks';

/** Календарный день в локальном времени; month — 0…11, как в Date. */
export interface CalendarDay {
  year: number;
  month: number;
  day: number;
}

/**
 * Сетка месяца для календаря: недели с понедельника, пустые клетки — null.
 */
export function monthGrid(year: number, month: number): (number | null)[][] {
  const first = new Date(year, month, 1);
  const days = new Date(year, month + 1, 0).getDate();
  // Date.getDay: 0 = Sunday; the grid starts on Monday.
  const lead = (first.getDay() + 6) % 7;
  const cells: (number | null)[] = [
    ...Array<null>(lead).fill(null),
    ...Array.from({ length: days }, (_, i) => i + 1),
  ];
  while (cells.length % 7) cells.push(null);
  const weeks: (number | null)[][] = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));
  return weeks;
}

/** Соседний месяц: delta = -1 назад, +1 вперёд. */
export function shiftMonth(
  year: number,
  month: number,
  delta: number,
): { year: number; month: number } {
  const d = new Date(year, month + delta, 1);
  return { year: d.getFullYear(), month: d.getMonth() };
}

export function sameDay(a: CalendarDay | null, b: CalendarDay | null): boolean {
  return !!a && !!b && a.year === b.year && a.month === b.month && a.day === b.day;
}

/** День из даты (локальное время). */
export function toCalendarDay(date: Date): CalendarDay {
  return { year: date.getFullYear(), month: date.getMonth(), day: date.getDate() };
}

/** День через n дней от указанного. */
export function addDays(day: CalendarDay, n: number): CalendarDay {
  return toCalendarDay(new Date(day.year, day.month, day.day + n));
}

export interface TimeOfDay {
  hour: number;
  minute: number;
}

/** «18:30», «9:05», «18.30» → время; иначе null. */
export function parseTime(text: string): TimeOfDay | null {
  const m = /^\s*(\d{1,2})[:.](\d{2})\s*$/.exec(text);
  if (!m) return null;
  const hour = Number(m[1]);
  const minute = Number(m[2]);
  return hour <= 23 && minute <= 59 ? { hour, minute } : null;
}

export function formatTime(time: TimeOfDay): string {
  return `${String(time.hour).padStart(2, '0')}:${String(time.minute).padStart(2, '0')}`;
}

/** Срок из дня и необязательного времени (без времени — конец дня), ISO. */
export function dueFrom(day: CalendarDay, time: TimeOfDay | null): string {
  const t = time ?? END_OF_DAY;
  return new Date(day.year, day.month, day.day, t.hour, t.minute, 0, 0).toISOString();
}

/** Разложить срок обратно на день и время (null — срок без времени). */
export function splitDue(iso: string): { day: CalendarDay; time: TimeOfDay | null } {
  const d = new Date(iso);
  const time = { hour: d.getHours(), minute: d.getMinutes() };
  const endOfDay = time.hour === END_OF_DAY.hour && time.minute === END_OF_DAY.minute;
  return { day: toCalendarDay(d), time: endOfDay ? null : time };
}
