// Hermes (Android) has no Intl.PluralRules: without it i18next silently falls
// back to one/other and Russian plurals (_few/_many) render as raw keys.
import '@formatjs/intl-pluralrules/polyfill.js';
import '@formatjs/intl-pluralrules/locale-data/ru.js';
import '@formatjs/intl-pluralrules/locale-data/en.js';
import { getLocales } from 'expo-localization';
import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';

import { DEFAULT_LOCALE, resolveLocale, resources } from '@heatseeker/i18n';

const deviceTag = getLocales()[0]?.languageTag;

void i18next.use(initReactI18next).init({
  resources,
  lng: resolveLocale(deviceTag),
  fallbackLng: DEFAULT_LOCALE,
  interpolation: { escapeValue: false },
  returnNull: false,
  showSupportNotice: false,
});

export default i18next;
