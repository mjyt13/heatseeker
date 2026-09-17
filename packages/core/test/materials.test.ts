import { describe, expect, test } from 'vitest';

import {
  checkUpload,
  fileExtension,
  fileIcon,
  filterFromQuickTag,
  formatBytes,
  materialRights,
  normalizeFilter,
} from '../src/materials';
import { keys } from '../src/queries/keys';
import { putWithFetch, UploadError } from '../src/queries/materials';

describe('filterFromQuickTag', () => {
  test('maps chips to feed filters', () => {
    expect(filterFromQuickTag(null)).toEqual({});
    expect(filterFromQuickTag('subject:abc')).toEqual({ subject_id: 'abc' });
    expect(filterFromQuickTag('system:mine')).toEqual({ mine: true });
  });
  test('chips the feed cannot filter by yet', () => {
    expect(filterFromQuickTag('system:saved')).toBeNull();
    expect(filterFromQuickTag('system:unread')).toBeNull();
    expect(filterFromQuickTag('weird')).toBeNull();
  });
});

test('normalizeFilter drops empties and sorts tags so cache keys are stable', () => {
  expect(normalizeFilter({ q: '  ', mine: false, tag_id: [] })).toEqual({});
  expect(normalizeFilter({ q: ' лекция ', tag_id: ['b', 'a'], inbox: true })).toEqual({
    q: 'лекция',
    tag_id: ['a', 'b'],
    inbox: true,
  });
});

test('materials keys live under the group prefix', () => {
  expect(keys.materials('g', {}).slice(0, 3)).toEqual(['group', 'g', 'materials']);
  expect(keys.drive('g').slice(0, 2)).toEqual(['group', 'g']);
});

test('formatBytes', () => {
  expect(formatBytes(512)).toEqual({ value: '512', unit: 'b' });
  expect(formatBytes(1536)).toEqual({ value: '1,5', unit: 'kb' });
  expect(formatBytes(200 * 1024 * 1024)).toEqual({ value: '200', unit: 'mb' });
  expect(formatBytes(-1)).toEqual({ value: '0', unit: 'b' });
  expect(formatBytes(1536, 'en').value).toBe('1.5');
});

test('fileIcon', () => {
  expect(fileIcon('application/pdf')).toBe('pdf');
  expect(fileIcon('application/vnd.google-apps.document')).toBe('doc');
  expect(
    fileIcon('application/vnd.openxmlformats-officedocument.presentationml.presentation'),
  ).toBe('slides');
  expect(fileIcon('application/vnd.ms-excel')).toBe('sheet');
  expect(fileIcon('image/png')).toBe('image');
  expect(fileIcon('text/csv')).toBe('text');
  expect(fileIcon('application/octet-stream')).toBe('file');
});

test('fileExtension and checkUpload', () => {
  expect(fileExtension('Лекция.PDF')).toBe('pdf');
  expect(fileExtension('.hidden')).toBe('');
  expect(fileExtension('trailing.')).toBe('');
  const limits = { max_bytes: 100, allowed_ext: ['pdf'] };
  expect(checkUpload({ name: 'a.pdf', size: 10 }, limits)).toBe('ok');
  expect(checkUpload({ name: 'a.pdf', size: 101 }, limits)).toBe('too_large');
  expect(checkUpload({ name: 'a.exe', size: 10 }, limits)).toBe('bad_type');
  expect(checkUpload({ name: 'a.pdf', size: 0 }, limits)).toBe('empty');
  expect(checkUpload({ name: 'a.pdf', size: 1 }, { max_bytes: 100, allowed_ext: null })).toBe(
    'bad_type',
  );
});

describe('materialRights mirrors the server rules', () => {
  const mine = { uploader_id: 'me', download_count: 0, status: 'ACTIVE' as const };
  const student = { moderate: false, editOwn: true };
  test('owner before and after someone opened the file', () => {
    expect(materialRights(mine, 'me', student)).toEqual({
      edit: true,
      archive: true,
      delete: true,
    });
    expect(materialRights({ ...mine, download_count: 3 }, 'me', student).delete).toBe(false);
  });
  test('others and Drive files', () => {
    expect(materialRights(mine, 'other', student)).toEqual({
      edit: false,
      archive: false,
      delete: false,
    });
    expect(materialRights({ ...mine, uploader_id: undefined }, 'me', student).edit).toBe(false);
    expect(
      materialRights({ ...mine, download_count: 9 }, 'other', { moderate: true, editOwn: true }),
    ).toEqual({
      edit: true,
      archive: true,
      delete: true,
    });
  });
  test('archived and deleted', () => {
    expect(materialRights({ ...mine, status: 'ARCHIVED' }, 'me', student).archive).toBe(false);
    expect(materialRights({ ...mine, status: 'DELETED' }, 'me', student).edit).toBe(false);
  });
});

test('putWithFetch sends the ticket headers and reports failures', async () => {
  const calls: RequestInit[] = [];
  const ok = (async (_url: string, init: RequestInit) => {
    calls.push(init);
    return new Response(null, { status: 200 });
  }) as unknown as typeof fetch;
  const ticket = {
    upload_id: 'u',
    method: 'PUT' as const,
    url: 'http://s3/x',
    headers: { 'Content-Type': 'application/pdf' },
    expires_at: '',
    max_bytes: 1,
    file_name: 'a.pdf',
    mime: 'application/pdf',
  };
  await putWithFetch('data', ok)(ticket);
  expect(calls[0]).toMatchObject({
    method: 'PUT',
    headers: { 'Content-Type': 'application/pdf' },
    body: 'data',
  });

  const failing = (async () => new Response(null, { status: 403 })) as unknown as typeof fetch;
  await expect(putWithFetch('data', failing)(ticket)).rejects.toBeInstanceOf(UploadError);
});
