import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { unwrap, type components } from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { keys } from './keys';

type Schemas = components['schemas'];

export type Task = Schemas['TaskDTO'];
export type TaskCounts = Schemas['TaskCountsDTO'];
export type TaskInput = Omit<Schemas['TaskBody'], '$schema'>;
export type TaskStatus = Task['status'];
export type TaskKind = Task['kind'];
export type TaskPriority = Task['priority'];
export type TaskAssignMode = Task['assign_mode'];

/** Фильтр списка задач (совпадает с query-параметрами API). */
export interface TaskFilter {
  subject_id?: string;
  no_subject?: boolean;
  kind?: TaskKind;
  status?: TaskStatus[];
  mine?: boolean;
  open?: boolean;
  overdue?: boolean;
  due_before?: string;
  q?: string;
}

/** Убирает пустые значения, чтобы ключ запроса не зависел от их формы. */
function normalize(filter: TaskFilter): TaskFilter {
  const out: TaskFilter = {};
  if (filter.subject_id) out.subject_id = filter.subject_id;
  if (filter.no_subject) out.no_subject = true;
  if (filter.kind) out.kind = filter.kind;
  if (filter.status?.length) out.status = [...filter.status].sort();
  if (filter.mine) out.mine = true;
  if (filter.open) out.open = true;
  if (filter.overdue) out.overdue = true;
  if (filter.due_before) out.due_before = filter.due_before;
  const q = filter.q?.trim();
  if (q) out.q = q;
  return out;
}

/** Задачи группы: сначала закреплённые, затем по сроку. */
export function useTasks(groupId: string | null | undefined, filter: TaskFilter = {}) {
  const api = useApi();
  const f = normalize(filter);
  return useQuery({
    queryKey: keys.tasks(groupId ?? '', f),
    enabled: !!groupId,
    queryFn: async (): Promise<Task[]> =>
      unwrap(
        await api.GET('/groups/{groupId}/tasks', {
          params: { path: { groupId: groupId! }, query: f },
        }),
      ).items ?? [],
  });
}

/** Счётчики доски задач: по статусам, открытые, просроченные, ближайшие, мои. */
export function useTaskBoard(groupId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.taskBoard(groupId ?? ''),
    enabled: !!groupId,
    queryFn: async (): Promise<TaskCounts> =>
      unwrap(
        await api.GET('/groups/{groupId}/tasks/board', { params: { path: { groupId: groupId! } } }),
      ),
  });
}

/** Одна задача. */
export function useTask(taskId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.task(taskId ?? ''),
    enabled: !!taskId,
    queryFn: async (): Promise<Task> =>
      unwrap(await api.GET('/tasks/{taskId}', { params: { path: { taskId: taskId! } } })),
  });
}

/** Создать задачу (client_id делает повтор безопасным). */
export function useCreateTask(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: TaskInput): Promise<Task> =>
      unwrap(await api.POST('/groups/{groupId}/tasks', { params: { path: { groupId } }, body })),
    onSuccess: (task) => invalidateTask(qc, groupId, task.id),
  });
}

/** Изменить задачу (поля заменяются целиком). */
export function useUpdateTask(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, ...body }: TaskInput & { taskId: string }): Promise<Task> =>
      unwrap(await api.PATCH('/tasks/{taskId}', { params: { path: { taskId } }, body })),
    onSuccess: (task) => invalidateTask(qc, groupId, task.id),
  });
}

/** Статус задачи для всей группы (автор, староста, модератор). */
export function useSetTaskStatus(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, status }: { taskId: string; status: TaskStatus }): Promise<Task> =>
      unwrap(
        await api.PATCH('/tasks/{taskId}/status', {
          params: { path: { taskId } },
          body: { status },
        }),
      ),
    onSuccess: (task) => invalidateTask(qc, groupId, task.id),
  });
}

/** Мой личный прогресс по задаче. */
export function useSetMyTaskStatus(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, status }: { taskId: string; status: TaskStatus }): Promise<Task> =>
      unwrap(
        await api.PATCH('/tasks/{taskId}/me/status', {
          params: { path: { taskId } },
          body: { status },
        }),
      ),
    onSuccess: (task) => invalidateTask(qc, groupId, task.id),
  });
}

/** Закрепить или открепить задачу. */
export function usePinTask(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, pinned }: { taskId: string; pinned: boolean }): Promise<Task> => {
      const params = { path: { taskId } };
      return unwrap(
        pinned
          ? await api.POST('/tasks/{taskId}/pin', { params })
          : await api.DELETE('/tasks/{taskId}/pin', { params }),
      );
    },
    onSuccess: (task) => invalidateTask(qc, groupId, task.id),
  });
}

/** Прикреплённые к задаче материалы (набор заменяется целиком). */
export function useSetTaskMaterials(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, materialIds }: { taskId: string; materialIds: string[] }): Promise<Task> =>
      unwrap(
        await api.PUT('/tasks/{taskId}/materials', {
          params: { path: { taskId } },
          body: { material_ids: materialIds },
        }),
      ),
    onSuccess: (task) => invalidateTask(qc, groupId, task.id),
  });
}

/** Удалить задачу. */
export function useDeleteTask(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (taskId: string): Promise<void> => {
      unwrap(await api.DELETE('/tasks/{taskId}', { params: { path: { taskId } } }));
    },
    onSuccess: (_, taskId) => invalidateTask(qc, groupId, taskId),
  });
}

/** Перечитать списки, счётчики и саму задачу. */
export async function invalidateTask(qc: QueryClient, groupId: string, taskId: string) {
  await Promise.all([
    qc.invalidateQueries({ queryKey: ['group', groupId, 'tasks'] }),
    qc.invalidateQueries({ queryKey: keys.task(taskId) }),
  ]);
}
