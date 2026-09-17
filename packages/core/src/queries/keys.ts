import type { MaterialFilter } from '../materials';

/** Фабрика ключей TanStack Query. Все ключи группы начинаются с ['group', id]. */
export const keys = {
  meta: () => ['meta'] as const,
  me: () => ['me'] as const,
  myGroups: () => ['me', 'groups'] as const,
  preview: (code: string) => ['preview', code] as const,
  group: (id: string) => ['group', id] as const,
  members: (id: string) => ['group', id, 'members'] as const,
  invites: (id: string) => ['group', id, 'invites'] as const,
  subjects: (id: string, includeArchived = false) => ['group', id, 'subjects', { includeArchived }] as const,
  subject: (id: string, subjectId: string) => ['group', id, 'subjects', subjectId] as const,
  tags: (id: string) => ['group', id, 'tags'] as const,
  quickTags: (id: string) => ['group', id, 'quick-tags'] as const,
  activity: (id: string) => ['group', id, 'activity'] as const,
  sync: (id: string) => ['group', id, 'sync'] as const,
  materials: (id: string, filter: MaterialFilter) => ['group', id, 'materials', filter] as const,
  material: (materialId: string) => ['material', materialId] as const,
  drive: (id: string) => ['group', id, 'drive'] as const,
};
