import { expect, test } from 'vitest';

import { ACTIONS, PERMISSIONS, ROLES, can, requiresSecured } from '../src';

test('every action has at least one role', () => {
  for (const action of ACTIONS) {
    expect(PERMISSIONS[action].length, `${action} grants nothing`).toBeGreaterThan(0);
  }
});

test('union semantics across roles', () => {
  expect(can(['STUDENT'], 'task.pin')).toBe(false);
  expect(can(['STUDENT', 'HEADMAN'], 'task.pin')).toBe(true);
  expect(can(['GUEST'], 'group.read')).toBe(true);
  expect(can(['GUEST'], 'thread.write')).toBe(false);
});

test('secured actions are a subset of actions', () => {
  expect(requiresSecured('member.manage')).toBe(true);
  expect(requiresSecured('group.read')).toBe(false);
});

test('roles list matches the server', () => {
  expect([...ROLES]).toEqual(['OWNER', 'ADMIN', 'MODERATOR', 'HEADMAN', 'STUDENT', 'GUEST']);
});
