import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { unwrap, type Subject, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

export type SubjectInput = components['schemas']['SubjectBody'];

/** Предметы группы. */
export function useSubjects(groupId: string | null | undefined, includeArchived = false) {
  const api = useApi();
  return useQuery({
    queryKey: keys.subjects(groupId ?? '', includeArchived),
    enabled: !!groupId,
    queryFn: async () =>
      unwrap(
        await api.GET('/groups/{groupId}/subjects', {
          params: { path: { groupId: groupId! }, query: { include_archived: includeArchived } },
        }),
      ).items,
  });
}

/** Создать предмет (тег создаётся автоматически). */
export function useCreateSubject(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: SubjectInput): Promise<Subject> =>
      unwrap(await api.POST('/groups/{groupId}/subjects', { params: { path: { groupId } }, body })),
    onSuccess: () => invalidateSubjectViews(qc, groupId),
  });
}

/** Изменить предмет. */
export function useUpdateSubject(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ subjectId, ...body }: SubjectInput & { subjectId: string }): Promise<Subject> =>
      unwrap(
        await api.PUT('/groups/{groupId}/subjects/{subjectId}', { params: { path: { groupId, subjectId } }, body }),
      ),
    onSuccess: () => invalidateSubjectViews(qc, groupId),
  });
}

/** Архивировать / восстановить предмет. */
export function useArchiveSubject(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ subjectId, restore = false }: { subjectId: string; restore?: boolean }) => {
      const path = { params: { path: { groupId, subjectId } } };
      const res = restore
        ? await api.POST('/groups/{groupId}/subjects/{subjectId}/restore', path)
        : await api.POST('/groups/{groupId}/subjects/{subjectId}/archive', path);
      unwrap(res);
    },
    onSuccess: () => invalidateSubjectViews(qc, groupId),
  });
}

async function invalidateSubjectViews(qc: ReturnType<typeof useQueryClient>, groupId: string) {
  await Promise.all([
    qc.invalidateQueries({ queryKey: ['group', groupId, 'subjects'] }),
    qc.invalidateQueries({ queryKey: keys.tags(groupId) }),
    qc.invalidateQueries({ queryKey: keys.quickTags(groupId) }),
  ]);
}
