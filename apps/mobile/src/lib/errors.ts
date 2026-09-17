import { ApiError } from '@heatseeker/api-client';
import { UploadError } from '@heatseeker/core';
import type { TFunction } from 'i18next';

/** Человеческое сообщение об ошибке запроса. */
export function describeError(t: TFunction, err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 401:
        return t('errors.unauthorized');
      case 403:
        return t('errors.forbidden');
      case 404:
        return t('errors.not_found');
      case 409:
        return t('errors.conflict');
      case 410:
        return t('errors.gone');
      case 413:
        return t('errors.too_large');
      case 416:
        return t('errors.range');
      case 503:
        return err.body?.detail ?? t('errors.unavailable');
      case 422: {
        const detail = err.body?.errors?.[0]?.message;
        return detail ? `${t('errors.validation')}: ${detail}` : t('errors.validation');
      }
    }
    return err.body?.detail ?? t('errors.unknown');
  }
  if (err instanceof UploadError) return err.status === 413 ? t('errors.too_large') : t('upload.failed');
  if (err instanceof TypeError) return t('errors.network');
  return t('errors.unknown');
}
