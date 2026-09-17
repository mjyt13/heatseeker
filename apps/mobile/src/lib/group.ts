import { useMemo } from 'react';

import type { Subject } from '@heatseeker/api-client';
import {
  useCurrentGroup,
  useMe,
  useMembers,
  usePermissions,
  useSession,
  useSubjects,
} from '@heatseeker/core';

/**
 * Всё, что экранам нужно знать о текущей группе: права, предметы и имена
 * участников (для подписи «Загружено: …»).
 */
export function useGroupContext() {
  const groupId = useSession((s) => s.currentGroupId);
  const group = useCurrentGroup();
  const me = useMe();
  const permissions = usePermissions(group.data?.membership, !!me.data?.secured);
  const subjects = useSubjects(groupId);
  const members = useMembers(groupId);

  const subjectById = useMemo(() => {
    const map = new Map<string, Subject>();
    for (const s of subjects.data ?? []) map.set(s.id, s);
    return map;
  }, [subjects.data]);

  const memberName = useMemo(() => {
    const map = new Map<string, string>();
    for (const m of members.data?.items ?? []) map.set(m.user.id, m.user.name);
    return (id: string | null | undefined) => (id ? map.get(id) : undefined);
  }, [members.data]);

  return {
    groupId,
    group,
    me: me.data ?? null,
    permissions,
    subjects: subjects.data ?? [],
    subjectById,
    memberName,
  };
}

/** Короткое имя предмета для чипов и подписей. */
export function subjectLabel(subject: Subject | undefined): string | undefined {
  return subject ? subject.short_name || subject.name : undefined;
}
