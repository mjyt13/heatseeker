import { Platform } from 'react-native';

/**
 * Текст, который можно выделить и скопировать. На телефоне это `selectable`,
 * в вебе — CSS `user-select` (атрибут `selectable` попал бы в DOM с ошибкой).
 */
export const selectableText =
  Platform.OS === 'web' ? ({ userSelect: 'text' } as const) : ({ selectable: true } as const);
