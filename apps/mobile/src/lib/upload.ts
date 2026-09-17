import type { DocumentPickerAsset } from 'expo-document-picker';
import { File, UploadTask } from 'expo-file-system';
import { Platform } from 'react-native';

import { putWithFetch, UploadError, type PutFile } from '@heatseeker/core';

/**
 * Отправляет выбранный файл по тикету загрузки: на устройстве — нативной
 * задачей с прогрессом (файл не читается в память JS), на вебе — через fetch.
 */
export function putPickedFile(asset: DocumentPickerAsset): PutFile {
  if (Platform.OS === 'web') {
    if (!asset.file) throw new Error('picked file is not available on web');
    return putWithFetch(asset.file);
  }
  return async (ticket, onProgress) => {
    const task = new UploadTask(new File(asset.uri), ticket.url, {
      httpMethod: 'PUT',
      headers: ticket.headers ?? {},
      mimeType: ticket.mime,
      onProgress: onProgress
        ? ({ bytesSent, totalBytes }) => onProgress(bytesSent, totalBytes)
        : undefined,
    });
    try {
      const res = await task.uploadAsync();
      if (res.status < 200 || res.status >= 300) throw new UploadError(res.status);
    } finally {
      task.release();
    }
  };
}
