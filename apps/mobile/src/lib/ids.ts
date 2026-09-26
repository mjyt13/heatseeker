import * as Crypto from 'expo-crypto';

/**
 * Случайный UUID для `client_id` (идемпотентность создания).
 *
 * В браузере `crypto.randomUUID` есть только в защищённом контексте — по
 * `http://<IP>` (как в dev с телефона и планшета) его нет, поэтому собираем
 * UUID v4 из случайных байт: `getRandomValues` работает и без https.
 */
export function newId(): string {
  const bytes = Crypto.getRandomBytes(16);
  // Версия 4 и вариант RFC 4122.
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
