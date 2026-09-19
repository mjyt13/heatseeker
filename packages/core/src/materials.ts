import type { FileType, Material, MaterialKind } from './types';

/** Параметры ленты материалов (соответствуют query-параметрам API). */
export interface MaterialFilter {
  subject_id?: string;
  no_subject?: boolean;
  tag_id?: string[];
  kind?: MaterialKind;
  file_type?: FileType;
  mine?: boolean;
  inbox?: boolean;
  archived?: boolean;
  q?: string;
}

/**
 * Превращает ключ быстрого тега (`system:mine`, `subject:<id>`) в фильтр ленты.
 * Для чипов, которые ещё не поддерживаются лентой материалов, возвращает null.
 */
export function filterFromQuickTag(key: string | null | undefined): MaterialFilter | null {
  if (!key) return {};
  const [kind, value] = splitOnce(key, ':');
  if (kind === 'subject' && value) return { subject_id: value };
  if (kind === 'system' && value === 'mine') return { mine: true };
  return null;
}

/** Нормализует фильтр для ключа кеша: без пустых значений и в стабильном порядке. */
export function normalizeFilter(filter: MaterialFilter): MaterialFilter {
  const out: MaterialFilter = {};
  if (filter.subject_id) out.subject_id = filter.subject_id;
  if (filter.no_subject) out.no_subject = true;
  if (filter.tag_id?.length) out.tag_id = [...filter.tag_id].sort();
  if (filter.kind) out.kind = filter.kind;
  if (filter.file_type) out.file_type = filter.file_type;
  if (filter.mine) out.mine = true;
  if (filter.inbox) out.inbox = true;
  if (filter.archived) out.archived = true;
  const q = filter.q?.trim();
  if (q) out.q = q;
  return out;
}

/** «1,4 МБ» — размер файла для людей (единицы — ключи i18n `units.*`). */
export function formatBytes(
  bytes: number,
  locale = 'ru',
): { value: string; unit: 'b' | 'kb' | 'mb' | 'gb' } {
  const units = ['b', 'kb', 'mb', 'gb'] as const;
  let value = Math.max(0, bytes);
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  const digits = i === 0 || value >= 100 ? 0 : 1;
  return {
    value: new Intl.NumberFormat(locale, {
      maximumFractionDigits: digits,
      minimumFractionDigits: 0,
    }).format(value),
    unit: units[i]!,
  };
}

export type FileIcon =
  'pdf' | 'doc' | 'sheet' | 'slides' | 'image' | 'audio' | 'video' | 'archive' | 'text' | 'file';

/** Тип иконки по MIME (включая нативные документы Google). */
export function fileIcon(mime: string): FileIcon {
  if (mime === 'application/pdf') return 'pdf';
  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('audio/')) return 'audio';
  if (mime.startsWith('video/')) return 'video';
  if (/zip|x-7z|rar|gzip|x-tar/.test(mime)) return 'archive';
  if (mime.startsWith('text/')) return 'text';
  if (/spreadsheet|excel|google-apps\.spreadsheet/.test(mime)) return 'sheet';
  if (/presentation|powerpoint|google-apps\.presentation/.test(mime)) return 'slides';
  if (/word|opendocument\.text|rtf|google-apps\.document/.test(mime)) return 'doc';
  return 'file';
}

/** Расширение файла без точки, в нижнем регистре. */
export function fileExtension(name: string): string {
  const dot = name.lastIndexOf('.');
  return dot > 0 && dot < name.length - 1 ? name.slice(dot + 1).toLowerCase() : '';
}

/** Проверяет файл до загрузки теми же правилами, что и сервер. */
export function checkUpload(
  file: { name: string; size: number },
  limits: { max_bytes: number; allowed_ext: readonly string[] | null },
): 'ok' | 'too_large' | 'bad_type' | 'empty' {
  if (file.size <= 0) return 'empty';
  if (file.size > limits.max_bytes) return 'too_large';
  if (!(limits.allowed_ext ?? []).includes(fileExtension(file.name))) return 'bad_type';
  return 'ok';
}

/** Может ли текущий пользователь править или удалять материал (как на сервере). */
export function materialRights(
  material: Pick<Material, 'uploader_id' | 'download_count' | 'status'>,
  userId: string | null | undefined,
  can: { moderate: boolean; editOwn: boolean },
): { edit: boolean; archive: boolean; delete: boolean } {
  const own = !!userId && material.uploader_id === userId && can.editOwn;
  const edit = can.moderate || own;
  return {
    edit: edit && material.status !== 'DELETED',
    archive: edit && material.status === 'ACTIVE',
    delete: can.moderate || (own && material.download_count === 0),
  };
}

function splitOnce(s: string, sep: string): [string, string] {
  const i = s.indexOf(sep);
  return i < 0 ? [s, ''] : [s.slice(0, i), s.slice(i + 1)];
}
