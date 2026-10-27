import { describe, expect, test } from 'vitest';

import { dueState, formatDue, groupUpcoming, parseDue, upcomingDueBefore } from '../src/tasks';
import type { Task } from '../src/queries/tasks';

function task(patch: Partial<Task>): Task {
  return {
    id: 't',
    group_id: 'g',
    title: 'Задача',
    kind: 'GROUP',
    status: 'TODO',
    priority: 'NORMAL',
    assign_mode: 'ALL',
    visibility: 'GROUP',
    overdue: false,
    assignee_ids: [],
    material_ids: [],
    done_count: 0,
    assigned_count: 0,
    created_at: '2026-09-01T10:00:00Z',
    updated_at: '2026-09-01T10:00:00Z',
    ...patch,
  } as Task;
}

describe('parseDue', () => {
  const now = new Date(2026, 8, 20);

  test('date with and without time', () => {
    const date = parseDue('25.12.2026', now);
    expect(date && new Date(date).getFullYear()).toBe(2026);
    expect(date && formatDue(date)).toBe('25.12.2026');
    const withTime = parseDue('25.12.2026 18:30', now);
    expect(withTime && formatDue(withTime)).toBe('25.12.2026 18:30');
  });

  test('short forms', () => {
    expect(formatDue(parseDue('1.10', now)!)).toBe('01.10.2026');
    expect(formatDue(parseDue('01/10/26', now)!)).toBe('01.10.2026');
  });

  test('rejects nonsense', () => {
    for (const text of ['', 'завтра', '31.02.2026', '25.13.2026', '25.12.2026 25:00']) {
      expect(parseDue(text, now), text).toBeNull();
    }
  });
});

describe('dueState', () => {
  const now = new Date(2026, 8, 20, 12, 0);
  const at = (d: Date) => d.toISOString();

  test('states by distance', () => {
    expect(dueState(task({}), now)).toEqual({ kind: 'none' });
    expect(dueState(task({ due_at: at(new Date(2026, 8, 19)) }), now)).toEqual({ kind: 'overdue' });
    expect(dueState(task({ due_at: at(new Date(2026, 8, 20, 23)) }), now)).toEqual({
      kind: 'today',
    });
    expect(dueState(task({ due_at: at(new Date(2026, 8, 21, 9)) }), now)).toEqual({
      kind: 'tomorrow',
    });
    expect(dueState(task({ due_at: at(new Date(2026, 8, 24)) }), now)).toEqual({
      kind: 'days',
      days: 4,
    });
    expect(dueState(task({ due_at: at(new Date(2026, 9, 24)) }), now)).toEqual({ kind: 'date' });
  });

  test('a closed task is not overdue', () => {
    const closed = task({ due_at: at(new Date(2026, 8, 1)), status: 'DONE' });
    expect(dueState(closed, now)).toEqual({ kind: 'date' });
  });
});

describe('groupUpcoming', () => {
  const now = new Date(2026, 9, 27, 12, 0);
  const at = (day: number, hour = 23, minute = 59) =>
    new Date(2026, 9, day, hour, minute).toISOString();

  test('buckets by deadline, skips undated and my closed tasks', () => {
    const sections = groupUpcoming(
      [
        task({ id: 'later', due_at: at(5 + 31) }),
        task({ id: 'week', due_at: at(30) }),
        task({ id: 'tomorrow', due_at: at(28) }),
        task({ id: 'today', due_at: at(27) }),
        task({ id: 'earlier-today', due_at: at(27, 9, 0) }),
        task({ id: 'overdue', due_at: at(25) }),
        task({ id: 'undated' }),
        task({ id: 'mine-done', due_at: at(27), my_status: 'DONE' }),
      ],
      now,
    );
    expect(sections.map((s) => [s.bucket, s.data.map((t) => t.id)])).toEqual([
      ['overdue', ['overdue', 'earlier-today']],
      ['today', ['today']],
      ['tomorrow', ['tomorrow']],
      ['week', ['week']],
      ['later', ['later']],
    ]);
  });

  test('query bound is stable within a day', () => {
    expect(upcomingDueBefore(new Date(2026, 9, 27, 8))).toBe(
      upcomingDueBefore(new Date(2026, 9, 27, 22)),
    );
  });
});
