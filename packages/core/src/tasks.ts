import type { Task, TaskStatus } from './queries/tasks';

/** Статусы в порядке доски. */
export const TASK_STATUS_ORDER: readonly TaskStatus[] = [
  'TODO',
  'IN_PROGRESS',
  'IN_REVIEW',
  'DONE',
  'CANCELLED',
];

/** Закрытая задача — сделана или отменена. */
export function isClosed(status: TaskStatus): boolean {
  return status === 'DONE' || status === 'CANCELLED';
}

/** Личный статус, если он есть, иначе статус группы. */
export function myStatus(task: Task): TaskStatus {
  return task.my_status ?? task.status;
}

const pad = (n: number) => String(n).padStart(2, '0');

/**
 * Срок без времени — конец дня: «сдать 25.12» значит до 23:59, а не с утра.
 * Такой срок показывается одной датой.
 */
export const END_OF_DAY = { hour: 23, minute: 59 } as const;

/** Срок задан только датой (время — конец дня). */
export function isDateOnly(iso: string): boolean {
  const d = new Date(iso);
  return d.getHours() === END_OF_DAY.hour && d.getMinutes() === END_OF_DAY.minute;
}

/**
 * Срок в виде «25.12.2026» (без времени — конец дня) или «25.12.2026 18:30».
 * Формат разбирается обратно `parseDue`; Intl в Hermes может отсутствовать,
 * поэтому вручную.
 */
export function formatDue(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const date = `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
  return isDateOnly(iso) ? date : `${date} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/**
 * Разбирает «25.12.2026», «25.12.2026 18:30» или «25.12» (текущий год) в
 * локальном времени пользователя; без времени — конец дня. Возвращает
 * ISO-строку или null.
 */
export function parseDue(text: string, now: Date = new Date()): string | null {
  const m = /^\s*(\d{1,2})[.\-/](\d{1,2})(?:[.\-/](\d{2,4}))?(?:[\s,]+(\d{1,2}):(\d{2}))?\s*$/.exec(
    text,
  );
  if (!m) return null;
  const [, rawDay, rawMonth, rawYear, rawHour, rawMinute] = m;
  const day = Number(rawDay);
  const month = Number(rawMonth);
  let year = rawYear ? Number(rawYear) : now.getFullYear();
  if (year < 100) year += 2000;
  const hour = rawHour ? Number(rawHour) : END_OF_DAY.hour;
  const minute = rawMinute ? Number(rawMinute) : END_OF_DAY.minute;
  if (month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59) return null;
  const d = new Date(year, month - 1, day, hour, minute, 0, 0);
  // Reject dates that rolled over (31.02) — the parts must survive the trip.
  if (d.getDate() !== day || d.getMonth() !== month - 1 || d.getFullYear() !== year) return null;
  return d.toISOString();
}

/** Во сколько календарных дней от сегодня попадает срок. */
export function daysUntil(iso: string, now: Date = new Date()): number {
  const due = new Date(iso);
  const a = new Date(due.getFullYear(), due.getMonth(), due.getDate());
  const b = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  return Math.round((a.getTime() - b.getTime()) / 86_400_000);
}

export type DueState =
  | { kind: 'none' }
  | { kind: 'overdue' }
  | { kind: 'today' }
  | { kind: 'tomorrow' }
  | { kind: 'days'; days: number }
  | { kind: 'date' };

/** Как показать срок задачи: просрочен, сегодня, завтра, через N дней или датой. */
export function dueState(task: Task, now: Date = new Date()): DueState {
  if (!task.due_at) return { kind: 'none' };
  if (task.overdue || (!isClosed(task.status) && new Date(task.due_at) < now)) {
    return { kind: 'overdue' };
  }
  const days = daysUntil(task.due_at, now);
  // A closed task keeps its past deadline as a plain date.
  if (days < 0) return { kind: 'date' };
  if (days === 0) return { kind: 'today' };
  if (days === 1) return { kind: 'tomorrow' };
  if (days <= 7) return { kind: 'days', days };
  return { kind: 'date' };
}
