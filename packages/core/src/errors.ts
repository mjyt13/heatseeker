import { ApiError } from '@heatseeker/api-client';
import { ERROR_CODES, ERROR_TYPE_PREFIX, type ErrorCode } from '@heatseeker/shared';

/** Машинный код ошибки API (RFC 7807 `type` = `urn:heatseeker:error:<code>`), если он есть. */
export function apiErrorCode(err: unknown): ErrorCode | null {
  if (!(err instanceof ApiError)) return null;
  const type = err.body?.type;
  if (!type?.startsWith(ERROR_TYPE_PREFIX)) return null;
  const code = type.slice(ERROR_TYPE_PREFIX.length);
  return (ERROR_CODES as readonly string[]).includes(code) ? (code as ErrorCode) : null;
}
