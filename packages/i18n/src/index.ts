import en from './locales/en';
import ru from './locales/ru';

export const DEFAULT_LOCALE = 'ru' as const;
export const SUPPORTED_LOCALES = ['ru', 'en'] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

/** i18next-совместимые ресурсы: { ru: { translation }, en: { translation } }. */
export const resources = {
  ru: { translation: ru },
  en: { translation: en },
} as const;

export type TranslationSchema = typeof ru;

/** Возвращает поддерживаемую локаль по тегу устройства (ru-RU → ru), иначе ru. */
export function resolveLocale(tag: string | undefined | null): Locale {
  const base = (tag ?? '').toLowerCase().split(/[-_]/)[0] ?? '';
  return (SUPPORTED_LOCALES as readonly string[]).includes(base) ? (base as Locale) : DEFAULT_LOCALE;
}

/** Ключ i18n для роли группы. */
export function roleKey(role: string): string {
  return `roles.${role}`;
}
