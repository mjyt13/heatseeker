import type { Message } from './queries/discussions';

const pad = (n: number) => String(n).padStart(2, '0');

/**
 * Время сообщения: «14:05» сегодня, «20.09 14:05» в этом году, иначе
 * «20.09.2025». Вручную — Intl в Hermes может отсутствовать.
 */
export function formatMessageTime(iso: string, now: Date = new Date()): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  const sameYear = d.getFullYear() === now.getFullYear();
  if (sameYear && d.getMonth() === now.getMonth() && d.getDate() === now.getDate()) return time;
  const date = `${pad(d.getDate())}.${pad(d.getMonth() + 1)}`;
  return sameYear ? `${date} ${time}` : `${date}.${d.getFullYear()}`;
}

/** Одна строка для превью: пробелы и переводы строк схлопнуты, длинное обрезано. */
export function snippet(body: string, max = 80): string {
  const line = body.replace(/\s+/g, ' ').trim();
  return line.length > max ? `${line.slice(0, max - 1).trimEnd()}…` : line;
}

/**
 * Где в треде провести черту «Новые сообщения»: id первого чужого сообщения
 * после моей отметки прочтения, или null, если нового нет.
 */
export function firstUnreadId(messages: readonly Message[], lastReadSeq: number): string | null {
  const first = messages.find((m) => m.seq > lastReadSeq && !m.mine && !m.deleted);
  return first?.id ?? null;
}

/**
 * Что показывать в треде: скрытые мной сообщения исчезают совсем; в режиме
 * «Показывать скрытые» они стоят на своих местах среди остальных.
 */
export function visibleMessages(messages: readonly Message[], showHidden: boolean): Message[] {
  return showHidden ? [...messages] : messages.filter((m) => !m.hidden_by_me);
}
