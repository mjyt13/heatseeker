import { expect, test } from 'vitest';

import {
  ERROR_CODES,
  EVENT_KINDS,
  FILE_TYPES,
  TASK_ASSIGN_MODES,
  TASK_KINDS,
  TASK_PRIORITYS,
  TASK_STATUSS,
  THREAD_TARGETS,
} from '@heatseeker/shared';

import { resolveLocale, resources } from '../src';

function keys(obj: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    v && typeof v === 'object'
      ? keys(v as Record<string, unknown>, `${prefix}${k}.`)
      : [`${prefix}${k}`],
  );
}

test('en covers every ru key (plural forms may differ)', () => {
  const ru = keys(resources.ru.translation);
  const en = new Set(keys(resources.en.translation));
  const missing = ru.filter((k) => !en.has(k) && !/_(one|few|many|other)$/.test(k));
  expect(missing).toEqual([]);
});

test('resolveLocale', () => {
  expect(resolveLocale('ru-RU')).toBe('ru');
  expect(resolveLocale('en_US')).toBe('en');
  expect(resolveLocale('de')).toBe('ru');
  expect(resolveLocale(undefined)).toBe('ru');
});

test('every plural has an _other form (fallback when plural rules are unavailable)', () => {
  for (const [lng, { translation }] of Object.entries(resources)) {
    const all = new Set(keys(translation));
    const bases = [...all].filter((k) => k.endsWith('_one')).map((k) => k.slice(0, -'_one'.length));
    const missing = bases.filter((b) => !all.has(`${b}_other`));
    expect(missing, lng).toEqual([]);
    if (lng === 'ru') {
      expect(bases.filter((b) => !all.has(`${b}_few`) || !all.has(`${b}_many`))).toEqual([]);
    }
  }
});

test('server enums have labels: event kinds, error codes, file, task and thread types', () => {
  for (const [lng, { translation }] of Object.entries(resources)) {
    const all = new Set(keys(translation));
    const missing = [
      ...EVENT_KINDS.map((k) => `events.${k.replace('.', '_')}`),
      ...ERROR_CODES.map((c) => `errors.codes.${c}`),
      ...FILE_TYPES.map((f) => `file_types.${f}`),
      ...TASK_STATUSS.map((s) => `task_status.${s}`),
      ...TASK_KINDS.map((k) => `task_kinds.${k}`),
      ...TASK_PRIORITYS.map((p) => `task_priority.${p}`),
      ...TASK_ASSIGN_MODES.map((m) => `task_assign.${m}`),
      ...THREAD_TARGETS.map((t) => `thread_targets.${t}`),
    ].filter((k) => !all.has(k));
    expect(missing, lng).toEqual([]);
  }
});
