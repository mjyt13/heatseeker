import { expect, test } from 'vitest';

import { resolveLocale, resources } from '../src';

function keys(obj: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    v && typeof v === 'object' ? keys(v as Record<string, unknown>, `${prefix}${k}.`) : [`${prefix}${k}`],
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
