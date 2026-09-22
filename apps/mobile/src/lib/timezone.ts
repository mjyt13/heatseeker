import { getCalendars } from 'expo-localization';

/**
 * Часовой пояс телефона (IANA, «Europe/Moscow»): по его часам серия занятий
 * держит время. Без сведений — UTC; для городов без перехода на летнее время
 * это не сдвигает занятия.
 */
export function deviceTimeZone(): string {
  try {
    const zone = getCalendars()[0]?.timeZone;
    if (zone) return zone;
  } catch {
    // Web or an old runtime without the module: fall back below.
  }
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}
