import { addDays, formatTime, toCalendarDay, type CalendarDay, type TimeOfDay } from './calendar';
import type { Occurrence, Weekday } from './queries/schedule';

/** Понедельник недели, в которую попадает день. */
export function weekStart(day: CalendarDay): CalendarDay {
  const weekday = new Date(day.year, day.month, day.day).getDay();
  return addDays(day, -((weekday + 6) % 7));
}

/** Семь дней недели с понедельника. */
export function weekDays(monday: CalendarDay): CalendarDay[] {
  return Array.from({ length: 7 }, (_, i) => addDays(monday, i));
}

const pad = (n: number) => String(n).padStart(2, '0');

/** «2026-09-22» — ключ дня и формат дат API. */
export function dayKey(day: CalendarDay): string {
  return `${day.year}-${pad(day.month + 1)}-${pad(day.day)}`;
}

/** День из «2026-09-22». */
export function parseDayKey(key: string): CalendarDay | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(key);
  return m ? { year: Number(m[1]), month: Number(m[2]) - 1, day: Number(m[3]) } : null;
}

/** Начало дня по часам телефона, ISO. */
export function dayStartIso(day: CalendarDay): string {
  return new Date(day.year, day.month, day.day).toISOString();
}

/** Окно запроса расписания: n дней с указанного, по часам телефона. */
export function scheduleWindow(from: CalendarDay, days: number): { from: string; to: string } {
  return { from: dayStartIso(from), to: dayStartIso(addDays(from, days)) };
}

/** Момент времени из дня и времени суток (по часам телефона), ISO. */
export function atTime(day: CalendarDay, time: TimeOfDay): string {
  return new Date(day.year, day.month, day.day, time.hour, time.minute).toISOString();
}

/** Время суток момента по часам телефона. */
export function timeOf(iso: string): TimeOfDay {
  const d = new Date(iso);
  return { hour: d.getHours(), minute: d.getMinutes() };
}

/** Занятия по дням (по часам телефона): ключ — dayKey. */
export function groupByDay(items: readonly Occurrence[]): Map<string, Occurrence[]> {
  const out = new Map<string, Occurrence[]>();
  for (const o of items) {
    const key = dayKey(toCalendarDay(new Date(o.starts_at)));
    const list = out.get(key);
    if (list) list.push(o);
    else out.set(key, [o]);
  }
  return out;
}

/** «10:00–11:30». */
export function classTime(o: Pick<Occurrence, 'starts_at' | 'ends_at'>): string {
  return `${formatTime(timeOf(o.starts_at))}–${formatTime(timeOf(o.ends_at))}`;
}

/** Идёт ли занятие сейчас. */
export function isOngoing(o: Occurrence, now: Date = new Date()): boolean {
  const t = now.getTime();
  return (
    o.status !== 'CANCELLED' &&
    new Date(o.starts_at).getTime() <= t &&
    new Date(o.ends_at).getTime() > t
  );
}

/** Ближайшее занятие, которое ещё не началось (отменённые не в счёт). */
export function nextClass(items: readonly Occurrence[], now: Date = new Date()): Occurrence | null {
  const t = now.getTime();
  let best: Occurrence | null = null;
  for (const o of items) {
    if (o.status === 'CANCELLED' || new Date(o.starts_at).getTime() <= t) continue;
    if (!best || o.starts_at < best.starts_at) best = o;
  }
  return best;
}

/** Адрес занятия в маршрутах и кешах. */
export function occurrenceKey(o: Pick<Occurrence, 'event_id' | 'date'>): string {
  return `${o.event_id}:${o.date}`;
}

/** Дни недели в порядке с понедельника (коды API). */
export const WEEKDAYS: readonly Weekday[] = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU'];

/** Код дня недели для дня. */
export function weekdayOf(day: CalendarDay): Weekday {
  const weekday = new Date(day.year, day.month, day.day).getDay();
  return WEEKDAYS[(weekday + 6) % 7]!;
}

/** Продолжительность в минутах между двумя временами суток (через полночь — +24 ч). */
export function minutesBetween(start: TimeOfDay, end: TimeOfDay): number {
  const diff = end.hour * 60 + end.minute - (start.hour * 60 + start.minute);
  return diff > 0 ? diff : diff + 24 * 60;
}

/** Время суток через n минут. */
export function addMinutes(time: TimeOfDay, minutes: number): TimeOfDay {
  const total = (((time.hour * 60 + time.minute + minutes) % 1440) + 1440) % 1440;
  return { hour: Math.floor(total / 60), minute: total % 60 };
}
