import { describe, expect, test } from 'vitest';

import { firstUnreadId, formatMessageTime, snippet, visibleMessages } from '../src/discussions';
import type { Message } from '../src/queries/discussions';

const now = new Date(2026, 8, 21, 18, 0);

describe('formatMessageTime', () => {
  test('today shows the time only', () => {
    expect(formatMessageTime(new Date(2026, 8, 21, 9, 5).toISOString(), now)).toBe('09:05');
  });
  test('this year adds the date', () => {
    expect(formatMessageTime(new Date(2026, 8, 20, 14, 30).toISOString(), now)).toBe('20.09 14:30');
  });
  test('another year shows the full date', () => {
    expect(formatMessageTime(new Date(2025, 11, 31, 23, 59).toISOString(), now)).toBe('31.12.2025');
  });
  test('garbage gives an empty string', () => {
    expect(formatMessageTime('nope', now)).toBe('');
  });
});

describe('snippet', () => {
  test('collapses whitespace and cuts long text', () => {
    expect(snippet('  a\n\n b  ')).toBe('a b');
    expect(snippet('абвгдежзик', 5)).toBe('абвг…');
  });
});

describe('firstUnreadId', () => {
  const m = (id: string, seq: number, mine = false, deleted = false) =>
    ({ id, seq, mine, deleted }) as Message;
  test('the first message from someone else after the mark', () => {
    const list = [m('a', 1), m('b', 2, true), m('c', 3, false, true), m('d', 4), m('e', 5)];
    expect(firstUnreadId(list, 1)).toBe('d');
    expect(firstUnreadId(list, 5)).toBeNull();
  });
});

describe('visibleMessages', () => {
  const m = (id: string, hidden: boolean) => ({ id, hidden_by_me: hidden }) as Message;
  test('hidden by me are gone unless asked for, and then keep their place', () => {
    const list = [m('a', false), m('b', true), m('c', false)];
    expect(visibleMessages(list, false).map((x) => x.id)).toEqual(['a', 'c']);
    expect(visibleMessages(list, true).map((x) => x.id)).toEqual(['a', 'b', 'c']);
  });
});
