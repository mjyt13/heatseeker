import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { unwrap, type GroupWithMembership, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { useSession } from '../session';
import { keys } from './keys';

type Preview = components['schemas']['PreviewOutputBody'];
export type GroupSearchItem = components['schemas']['GroupSearchItemDTO'];
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

/** Открытые группы до регистрации (D44): первый экран, без токена. */
export function useDiscoverGroups(query: string) {
  const api = useApi();
  const q = query.trim();
  return useQuery({
    queryKey: keys.groupDiscover(q),
    staleTime: 30_000,
    placeholderData: (previous) => previous,
    queryFn: async (): Promise<GroupSearchItem[]> =>
      unwrap(await api.GET('/groups/discover', { params: { query: q ? { q } : {} } })).items ?? [],
  });
}

/** Можно ли работать в группе: я в ней состою и членство активно. */
export function isActiveMemberOf(groups: readonly GroupWithMembership[], groupId: string): boolean {
  return groups.some((g) => g.group.id === groupId && g.membership.status === 'ACTIVE');
}

/**
 * Сбрасывает выбранную группу, если текущий аккаунт в ней не состоит: вошли
 * под другим именем на том же телефоне, исключили из группы. Без этого все
 * экраны получали бы 403. Решает только по свежему списку групп.
 */
export function useCurrentGroupGuard() {
  const groups = useMyGroups();
  const currentGroupId = useSession((s) => s.currentGroupId);
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  const fresh = groups.isFetchedAfterMount && !groups.isFetching;
  const data = groups.data;
  useEffect(() => {
    if (!currentGroupId || !fresh || !data) return;
    if (!isActiveMemberOf(data, currentGroupId)) void setCurrentGroup(null);
  }, [currentGroupId, fresh, data, setCurrentGroup]);
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

/** Открытые группы по части названия (пустая строка — все открытые). */
export function useGroupSearch(query: string, enabled = true) {
  const api = useApi();
  const status = useSession((s) => s.status);
  const q = query.trim();
  return useQuery({
    queryKey: keys.groupSearch(q),
    enabled: enabled && status === 'authenticated',
    staleTime: 30_000,
    placeholderData: (previous) => previous,
    queryFn: async (): Promise<GroupSearchItem[]> =>
      unwrap(await api.GET('/groups/search', { params: { query: q ? { q } : {} } })).items ?? [],
  });
}

/** Создать группу; становится текущей. */
export function useCreateGroup() {
  const api = useApi();
  const qc = useQueryClient();
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  return useMutation({
    mutationFn: async (body: {
      name: string;
      kind?: 'MASTERS' | 'DPO' | 'OTHER';
    }): Promise<GroupWithMembership> => unwrap(await api.POST('/groups', { body })),
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

/** Вступить в открытую группу из поиска; группа становится текущей. */
export function useJoinOpenGroup() {
  const api = useApi();
  const qc = useQueryClient();
  const setCurrentGroup = useSession((s) => s.setCurrentGroup);
  return useMutation({
    mutationFn: async (groupId: string): Promise<GroupWithMembership> =>
      unwrap(await api.POST('/groups/{groupId}/join', { params: { path: { groupId } } })),
    onSuccess: async (gm) => {
      qc.setQueryData(keys.group(gm.group.id), gm);
      await qc.invalidateQueries({ queryKey: keys.myGroups() });
      await qc.invalidateQueries({ queryKey: ['groups', 'search'] });
      await setCurrentGroup(gm.group.id);
    },
  });
}

/** Сменить код группы (старый перестаёт работать). */
export function useRotateJoinCode(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      unwrap(
        await api.POST('/groups/{groupId}/join-code/rotate', { params: { path: { groupId } } }),
      ),
    onSuccess: (group) => {
      qc.setQueryData<GroupWithMembership>(keys.group(groupId), (prev) =>
        prev ? { ...prev, group } : prev,
      );
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
      unwrap(
        await api.GET('/groups/{groupId}/members', { params: { path: { groupId: groupId! } } }),
      ),
  });
}
