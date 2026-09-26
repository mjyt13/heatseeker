import type { MaterialFilter } from '../materials';
import type { TaskFilter } from './tasks';

/** Фабрика ключей TanStack Query. Все ключи группы начинаются с ['group', id]. */
export const keys = {
  meta: () => ['meta'] as const,
  me: () => ['me'] as const,
  myGroups: () => ['me', 'groups'] as const,
  preview: (code: string) => ['preview', code] as const,
  groupSearch: (q: string) => ['groups', 'search', q] as const,
  groupDiscover: (q: string) => ['groups', 'discover', q] as const,
  group: (id: string) => ['group', id] as const,
  members: (id: string) => ['group', id, 'members'] as const,
  invites: (id: string) => ['group', id, 'invites'] as const,
  subjects: (id: string, includeArchived = false) =>
    ['group', id, 'subjects', { includeArchived }] as const,
  subject: (id: string, subjectId: string) => ['group', id, 'subjects', subjectId] as const,
  tags: (id: string) => ['group', id, 'tags'] as const,
  quickTags: (id: string) => ['group', id, 'quick-tags'] as const,
  activity: (id: string) => ['group', id, 'activity'] as const,
  sync: (id: string) => ['group', id, 'sync'] as const,
  materials: (id: string, filter: MaterialFilter) => ['group', id, 'materials', filter] as const,
  material: (materialId: string) => ['material', materialId] as const,
  tasks: (id: string, filter: TaskFilter) => ['group', id, 'tasks', filter] as const,
  taskBoard: (id: string) => ['group', id, 'tasks', 'board'] as const,
  task: (taskId: string) => ['task', taskId] as const,
  drive: (id: string) => ['group', id, 'drive'] as const,
  discussions: (id: string) => ['group', id, 'discussions'] as const,
  thread: (id: string, targetType: string, targetId: string, includeHidden = false) =>
    ['group', id, 'thread', targetType, targetId, { includeHidden }] as const,
  hiddenMessages: (id: string) => ['group', id, 'messages', 'hidden'] as const,
  scheduleAll: (id: string) => ['group', id, 'schedule'] as const,
  schedule: (id: string, from: string, to: string) =>
    ['group', id, 'schedule', { from, to }] as const,
  scheduleEvent: (eventId: string) => ['schedule-event', eventId] as const,
  occurrence: (eventId: string, date: string) =>
    ['schedule-event', eventId, 'occurrence', date] as const,
  calendarLink: (id: string) => ['group', id, 'calendar-link'] as const,
  notificationsAll: (id: string) => ['group', id, 'notifications'] as const,
  notifications: (id: string, unreadOnly = false) =>
    ['group', id, 'notifications', { unreadOnly }] as const,
  notificationsUnread: (id: string) => ['group', id, 'notifications', 'unread'] as const,
  notificationPrefs: (id: string) => ['group', id, 'notification-prefs'] as const,
  notificationSettings: () => ['me', 'notification-settings'] as const,
  mutes: (id: string) => ['group', id, 'mutes'] as const,
  announcements: (id: string) => ['group', id, 'announcements'] as const,
  remindersAll: (id: string) => ['group', id, 'reminders'] as const,
  reminders: (id: string, openOnly = false) => ['group', id, 'reminders', { openOnly }] as const,
};
