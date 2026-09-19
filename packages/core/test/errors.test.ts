import { expect, test } from 'vitest';

import { ApiError } from '@heatseeker/api-client';

import { apiErrorCode } from '../src/errors';

test('apiErrorCode reads the RFC 7807 problem type', () => {
  const coded = new ApiError(503, { type: 'urn:heatseeker:error:drive_api_disabled', detail: 'x' });
  expect(apiErrorCode(coded)).toBe('drive_api_disabled');
  expect(apiErrorCode(new ApiError(403, { type: 'about:blank' }))).toBeNull();
  expect(apiErrorCode(new ApiError(422, { type: 'urn:heatseeker:error:unknown_code' }))).toBeNull();
  expect(apiErrorCode(new ApiError(500, undefined))).toBeNull();
  expect(apiErrorCode(new Error('x'))).toBeNull();
});
