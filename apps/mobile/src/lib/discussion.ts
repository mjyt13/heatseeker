import type { DiscussionTarget } from '@heatseeker/core';

/**
 * Путь к обсуждению. title — подпись в шапке, пока сервер её не знает
 * (в треде ещё никто не писал).
 */
export function discussionHref(target: DiscussionTarget, title?: string | null) {
  return {
    pathname: '/(app)/discussion/[type]/[id]' as const,
    params: { type: target.type, id: target.id, ...(title ? { title } : {}) },
  };
}
