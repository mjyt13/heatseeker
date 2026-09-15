import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { unwrap, type GroupWithMembership, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { useSession } from '../session';
import { keys } from './keys';

type Preview = components['schemas']['PreviewOutputBody'];
type MembersBody = components['schemas']['MembersOutputBody'];

/** Группы текущего пользователя. */
export function useMyGroups() {
  const api = useApi();
  const status = useSession((s) => s.status);
  return useQuery({
    queryKey: keys.myGroups(),
    enabled: status === 'authenticated',
    queryFn: async () => unwrap(await api.GET('/me/groups')).items,
  });
}

/** Группа и моё членство в ней. */
export function useGroup(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.group(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async () =>
      unwrap(await api.GET('/groups/{groupId}', { params: { path: { groupId: groupId! } } })),
  });
}

/** Текущая группа из сессии (удобный шорткат). */
export function useCurrentGroup() {
  const currentGroupId = useSession((s) => s.currentGroupId);
  return useGroup(currentGroupId);
}

/** Что стоит за кодом приглашения / группы — без авторизации. */
export function useGroupPreview(code: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.preview(code ?? ''),
    enabled: !!code && code.length >= 4,
    retry: false,
    queryFn: async (): Promise<Preview> =>
      unwrap(await api.GET('/groups/join/{code}', { params: { path: { code: code! } } })),
  });
}

/** Создать группу; становится текущей. */
export function useCreateGroup() {
  const api = useApi();
  const qc = useQueryClient();
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  return useMutation({
    mutationFn: async (body: { name: string; kind?: 'MASTERS' | 'DPO' | 'OTHER' }): Promise<GroupWithMembership> =>
      unwrap(await api.POST('/groups', { body })),
    onSuccess: async (gm) => {
      qc.setQueryData(keys.group(gm.group.id), gm);
      await qc.invalidateQueries({ queryKey: keys.myGroups() });
      await setCurrentGroup(gm.group.id);
    },
  });
}

/** Вступить по коду; группа становится текущей. */
export function useJoinGroup() {
  const api = useApi();
  const qc = useQueryClient();
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  return useMutation({
    mutationFn: async (code: string): Promise<GroupWithMembership> =>
      unwrap(await api.POST('/groups/join/{code}', { params: { path: { code } } })),
    onSuccess: async (gm) => {
      qc.setQueryData(keys.group(gm.group.id), gm);
      await qc.invalidateQueries({ queryKey: keys.myGroups() });
      await setCurrentGroup(gm.group.id);
    },
  });
}

/** Участники группы. */
export function useMembers(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.members(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<MembersBody> =>
      unwrap(await api.GET('/groups/{groupId}/members', { params: { path: { groupId: groupId! } } })),
  });
}
