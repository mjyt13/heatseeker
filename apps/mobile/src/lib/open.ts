import * as WebBrowser from 'expo-web-browser';
import { Linking } from 'react-native';

import type { MaterialOpen } from '@heatseeker/api-client';

/**
 * Показывает файл: из нашего хранилища или через сервер — во встроенном
 * браузере; иначе — ссылкой на Google Диск (откроется приложение Диска).
 */
export async function showMaterial(links: MaterialOpen, preferDrive = false): Promise<void> {
  // Office files are viewed through their PDF preview once it is ready.
  const direct = links.preview_url ?? links.download_url ?? links.stream_url;
  if (preferDrive && links.drive_web_view_link) {
    await Linking.openURL(links.drive_web_view_link);
    return;
  }
  if (direct) {
    await WebBrowser.openBrowserAsync(direct);
    return;
  }
  if (links.drive_web_view_link) await Linking.openURL(links.drive_web_view_link);
}
