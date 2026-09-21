import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

import { expect, test } from 'vitest';

import { resources } from '../src';

const root = join(__dirname, '..', '..', '..');
const sources = ['apps/mobile/src', 'packages/core/src', 'packages/ui/src'];

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return files(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

function keys(obj: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    v && typeof v === 'object'
      ? keys(v as Record<string, unknown>, `${prefix}${k}.`)
      : [`${prefix}${k}`],
  );
}

// Only literal keys are checked: t('tabs.tasks'). Keys built at runtime
// (t(`kinds.${kind}`)) are covered by the enum tests in locales.test.ts.
test('every literal t() key used by the apps exists in every locale', () => {
  const used = new Map<string, string>();
  for (const dir of sources) {
    for (const file of files(join(root, dir))) {
      for (const m of readFileSync(file, 'utf8').matchAll(
        /\bt\(\s*'([a-z0-9_.]+)'(\s*,\s*\{[^}]*context)?/g,
      )) {
        // A context key (t('player.listen', { context })) resolves to listen_<context>.
        if (!m[2]) used.set(m[1]!, file.slice(root.length + 1));
      }
    }
  }
  expect(used.size).toBeGreaterThan(100);
  for (const [lng, { translation }] of Object.entries(resources)) {
    const all = new Set(keys(translation).map((k) => k.replace(/_(one|few|many|other)$/, '')));
    const missing = [...used].filter(([k]) => !all.has(k)).map(([k, f]) => `${k} (${f})`);
    expect(missing, lng).toEqual([]);
  }
});
