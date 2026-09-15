// Константы, которые клиенты должны знать заранее (лимиты валидации и
// системные ключи). Держать в соответствии с проверками в apps/api.

export const LIMITS = {
  userNameMax: 80,
  groupNameMin: 2,
  groupNameMax: 80,
  subjectNameMax: 120,
  subjectShortNameMax: 30,
  tagNameMax: 60,
  passwordMin: 8,
  syncPageDefault: 200,
  syncPageMax: 1000,
} as const;

/** Ключи системных быстрых тегов, которые API возвращает в /quick-tags. */
export const SYSTEM_QUICK_TAGS = ['system:mine', 'system:saved', 'system:unread'] as const;
export type SystemQuickTag = (typeof SYSTEM_QUICK_TAGS)[number];

/** Пресеты «отключить уведомления на …», в минутах. 0 — до указанной даты. */
export const MUTE_PRESETS_MINUTES = [60, 8 * 60, 24 * 60, 7 * 24 * 60] as const;

/** Роли, которые могут выдаваться приглашением обычным старостой. */
export const HEADMAN_INVITABLE_ROLES = ['STUDENT', 'GUEST'] as const;
