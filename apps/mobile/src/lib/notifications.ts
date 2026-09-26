import type Ionicons from '@expo/vector-icons/Ionicons';

import type { NotificationType } from '@heatseeker/core';

type IconName = keyof typeof Ionicons.glyphMap;

/** Куда ведёт уведомление: сервер кладёт ссылку в data. */
interface Deeplink {
  screen?: string;
  thread_id?: string;
  target_type?: string;
  target_id?: string;
  material_id?: string;
  announcement_id?: string;
  reminder_id?: string;
  task_id?: string;
  event_id?: string;
  date?: string;
}

/** Иконка по типу уведомления. */
export function notificationIcon(type: NotificationType): IconName {
  switch (type) {
    case 'MESSAGE_NEW':
    case 'MESSAGE_REPLY':
      return 'chatbubble-ellipses-outline';
    case 'MATERIAL_ADDED':
    case 'MATERIAL_BATCH':
      return 'document-text-outline';
    case 'TASK_CREATED':
    case 'TASK_PINNED':
    case 'TASK_STATUS_CHANGED':
      return 'checkbox-outline';
    case 'TASK_DUE_SOON':
      return 'alarm-outline';
    case 'TASK_OVERDUE':
      return 'warning-outline';
    case 'SCHEDULE_CHANGED':
      return 'calendar-outline';
    case 'MEMBER_JOINED':
      return 'person-add-outline';
    case 'ANNOUNCEMENT':
      return 'megaphone-outline';
    case 'REMINDER':
      return 'alarm-outline';
    default:
      return 'notifications-outline';
  }
}

/**
 * Экран, который открывается по нажатию. Ссылка приходит с сервера, поэтому
 * разбирается осторожно: незнакомое — просто не ведёт никуда.
 */
export function notificationRoute(
  raw: unknown,
): { pathname: string; params?: Record<string, string> } | null {
  const data = (raw ?? {}) as Deeplink;
  switch (data.screen) {
    case 'thread':
      if (!data.target_type || !data.target_id) return null;
      return {
        pathname: '/(app)/discussion/[type]/[id]',
        params: { type: data.target_type, id: data.target_id },
      };
    case 'material':
      return data.material_id
        ? { pathname: '/(app)/material/[id]', params: { id: data.material_id } }
        : null;
    case 'materials':
      return { pathname: '/(app)/feed' };
    case 'task':
      return data.task_id ? { pathname: '/(app)/task/[id]', params: { id: data.task_id } } : null;
    case 'class':
      return data.event_id && data.date
        ? {
            pathname: '/(app)/class/[eventId]/[date]',
            params: { eventId: data.event_id, date: data.date },
          }
        : { pathname: '/(app)/schedule' };
    case 'schedule':
      return { pathname: '/(app)/schedule' };
    case 'announcement':
      return { pathname: '/(app)/announcements' };
    case 'reminders':
      return { pathname: '/(app)/reminders' };
    case 'activity':
      return { pathname: '/(app)/activity' };
    default:
      return null;
  }
}
