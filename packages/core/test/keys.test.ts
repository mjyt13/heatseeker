import { expect, test } from 'vitest';

import { keys } from '../src/queries/keys';
import { eventLabelKey } from '../src/queries/sync';

test('group-scoped keys share the group prefix so one invalidation clears them all', () => {
  const g = 'g1';
  for (const k of [keys.group(g), keys.members(g), keys.subjects(g), keys.tags(g), keys.quickTags(g), keys.activity(g)]) {
    expect(k.slice(0, 2)).toEqual(['group', g]);
  }
});

test('eventLabelKey', () => {
  expect(eventLabelKey('member.joined')).toBe('events.member_joined');
});
