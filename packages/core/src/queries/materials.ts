import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query';

import {
  unwrap,
  type ClassifiedMaterial,
  type Material,
  type MaterialDetails,
  type MaterialOpen,
  type ServerMeta,
  type UploadTicket,
  type components,
} from '@heatseeker/api-client';

import { useApi } from '../api-provider';
import { normalizeFilter, type MaterialFilter } from '../materials';
import { keys } from './keys';

type Schemas = components['schemas'];
export type MaterialUpdate = Omit<Schemas['UpdateMaterialInputBody'], '$schema'>;
export type MaterialClassify = Omit<Schemas['ClassifyMaterialInputBody'], '$schema'>;
export type UploadForm = Omit<Schemas['CreateUploadInputBody'], '$schema'>;
export type MaterialsPage = Schemas['MaterialsPageOutputBody'];

/** Лимиты загрузки и возможности сервера (кешируются надолго). */
export function useServerMeta() {
  const api = useApi();
  return useQuery({
    queryKey: keys.meta(),
    staleTime: 60 * 60 * 1000,
    queryFn: async (): Promise<ServerMeta> => unwrap(await api.GET('/meta')),
  });
}

/** Лента материалов с курсорной пагинацией. */
export function useMaterials(
  groupId: string | null | undefined,
  filter: MaterialFilter,
  pageSize = 30,
) {
  const api = useApi();
  const f = normalizeFilter(filter);
  return useInfiniteQuery({
    queryKey: keys.materials(groupId ?? '', f),
    enabled: !!groupId,
    initialPageParam: '',
    getNextPageParam: (last: MaterialsPage) => last.next_cursor || undefined,
    queryFn: async ({ pageParam }): Promise<MaterialsPage> =>
      unwrap(
        await api.GET('/groups/{groupId}/materials', {
          params: {
            path: { groupId: groupId! },
            query: { ...f, limit: pageSize, cursor: pageParam || undefined },
          },
          // the API expects tag_id=a,b (explode: false)
          querySerializer: { array: { style: 'form', explode: false } },
        }),
      ),
  });
}

/** Размер «Входящих» (для модераторов). */
export function useInboxCount(groupId: string | null | undefined, enabled = true) {
  const api = useApi();
  return useQuery({
    queryKey: [...keys.materials(groupId ?? '', { inbox: true }), 'count'],
    enabled: !!groupId && enabled,
    queryFn: async () =>
      unwrap(
        await api.GET('/groups/{groupId}/materials', {
          params: { path: { groupId: groupId! }, query: { inbox: true, limit: 1 } },
        }),
      ).inbox_count ?? 0,
  });
}

/** Материал с историей версий. */
export function useMaterial(materialId: string | null | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: keys.material(materialId ?? ''),
    enabled: !!materialId,
    queryFn: async (): Promise<MaterialDetails> =>
      unwrap(
        await api.GET('/materials/{materialId}', { params: { path: { materialId: materialId! } } }),
      ),
  });
}

/**
 * Запросить PDF-превью офисного файла (pptx/docx/xlsx…): сервер конвертирует
 * его в фоне; готовность видна в `preview_status` версии (перечитать материал).
 */
export function useRequestPreview(materialId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (versionId?: string) =>
      unwrap(
        await api.POST('/materials/{materialId}/preview', {
          params: { path: { materialId }, query: { version_id: versionId } },
        }),
      ).preview_status,
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.material(materialId) }),
  });
}

/** Получить ссылки на файл (не кешируется: ссылки временные). */
export function useOpenMaterial() {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      materialId,
      versionId,
      download,
    }: {
      materialId: string;
      versionId?: string;
      download?: boolean;
    }): Promise<MaterialOpen> =>
      unwrap(
        await api.GET('/materials/{materialId}/open', {
          params: { path: { materialId }, query: { version_id: versionId, download } },
        }),
      ),
    onSuccess: (_, { materialId }) => qc.invalidateQueries({ queryKey: keys.material(materialId) }),
  });
}

/** Изменить название, описание, предмет, тип или теги. */
export function useUpdateMaterial(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      materialId,
      ...body
    }: MaterialUpdate & { materialId: string }): Promise<Material> =>
      unwrap(
        await api.PATCH('/materials/{materialId}', { params: { path: { materialId } }, body }),
      ),
    onSuccess: (m) => invalidateMaterial(qc, groupId, m.id),
  });
}

/** Разобрать материал из «Входящих». */
/** Через сколько перечитать ленту после выученного синонима (сервер разбирает похожие файлы в фоне). */
const RECLASSIFY_REFRESH_MS = 4000;

export function useClassifyMaterial(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      materialId,
      ...body
    }: MaterialClassify & { materialId: string }): Promise<ClassifiedMaterial> =>
      unwrap(
        await api.POST('/materials/{materialId}/classify', {
          params: { path: { materialId } },
          body,
        }),
      ),
    onSuccess: async (m) => {
      await invalidateMaterial(qc, groupId, m.id);
      if (!m.learned_alias) return;
      // A learned alias changes the subject, and the server re-sorts similar
      // Inbox files in the background (a couple of seconds later).
      await qc.invalidateQueries({ queryKey: ['group', groupId, 'subjects'] });
      setTimeout(() => {
        void qc.invalidateQueries({ queryKey: ['group', groupId, 'materials'] });
      }, RECLASSIFY_REFRESH_MS);
    },
  });
}

export type MaterialBulkClassify = Omit<Schemas['BulkClassifyInputBody'], '$schema'>;

/** Массовый разбор во «Входящих»: один предмет (и тип) для выбранных. */
export function useBulkClassify(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: MaterialBulkClassify): Promise<number> =>
      unwrap(
        await api.POST('/groups/{groupId}/materials/classify', {
          params: { path: { groupId } },
          body,
        }),
      ).classified,
    onSuccess: async (_n, body) => {
      await qc.invalidateQueries({ queryKey: ['group', groupId, 'materials'] });
      await Promise.all(
        (body.material_ids ?? []).map((id) =>
          qc.invalidateQueries({ queryKey: keys.material(id) }),
        ),
      );
    },
  });
}

export type MaterialTransition = 'archive' | 'restore' | 'delete';

/** Архивировать / вернуть / удалить материал. */
export function useMaterialTransition(groupId: string) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      materialId,
      action,
    }: {
      materialId: string;
      action: MaterialTransition;
    }) => {
      const params = { params: { path: { materialId } } };
      const res =
        action === 'delete'
          ? await api.DELETE('/materials/{materialId}', params)
          : action === 'archive'
            ? await api.POST('/materials/{materialId}/archive', params)
            : await api.POST('/materials/{materialId}/restore', params);
      unwrap(res);
    },
    onSuccess: (_, { materialId }) => invalidateMaterial(qc, groupId, materialId),
  });
}

/** Отправляет байты по выданному тикету; реализуется платформой (expo-file-system, fetch + Blob). */
export type PutFile = (
  ticket: UploadTicket,
  onProgress?: (sent: number, total: number) => void,
) => Promise<void>;

export interface UploadInput extends UploadForm {
  onProgress?: (sent: number, total: number) => void;
}

/** Загрузка материала: тикет → PUT файла → завершение. */
export function useUploadMaterial(groupId: string, putFile: PutFile) {
  const api = useApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ onProgress, ...form }: UploadInput): Promise<Material> => {
      const ticket = unwrap(
        await api.POST('/groups/{groupId}/materials/uploads', {
          params: { path: { groupId } },
          body: form,
        }),
      );
      await putFile(ticket, onProgress);
      return unwrap(
        await api.POST('/uploads/{uploadId}/complete', {
          params: { path: { uploadId: ticket.upload_id } },
        }),
      );
    },
    onSuccess: (m) => invalidateMaterial(qc, groupId, m.id),
  });
}

/** Реализация PutFile через fetch (веб и тесты). */
export function putWithFetch(body: BodyInit, fetchImpl: typeof fetch = fetch): PutFile {
  return async (ticket) => {
    const res = await fetchImpl(ticket.url, {
      method: ticket.method,
      headers: ticket.headers,
      body,
    });
    if (!res.ok) throw new UploadError(res.status);
  };
}

/** Хранилище отклонило файл. */
export class UploadError extends Error {
  readonly status: number;
  constructor(status: number) {
    super(`upload failed with HTTP ${status}`);
    this.name = 'UploadError';
    this.status = status;
  }
}

async function invalidateMaterial(qc: QueryClient, groupId: string, materialId: string) {
  await Promise.all([
    qc.invalidateQueries({ queryKey: ['group', groupId, 'materials'] }),
    qc.invalidateQueries({ queryKey: keys.material(materialId) }),
    qc.invalidateQueries({ queryKey: keys.activity(groupId) }),
  ]);
}
