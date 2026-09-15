import { defaultConfig } from '@tamagui/config/v4';
import { createTamagui } from 'tamagui';

/**
 * Единая тема для mobile и web на базе v4-конфигурации Tamagui (системные
 * шрифты, светлая/тёмная темы). Отличия от дефолта:
 * - разрешены longhand-имена стилей (`backgroundColor`, `alignItems`) — они
 *   привычнее в React Native, чем shorthand'ы `bg`/`items`;
 * - разрешены сырые значения (цвета предметов приходят из API строкой).
 * Фирменные токены добавим, когда появится дизайн.
 */
export const config = createTamagui({
  ...defaultConfig,
  settings: {
    ...defaultConfig.settings,
    onlyAllowShorthands: false,
    allowedStyleValues: false,
  },
});

export type AppTamaguiConfig = typeof config;

declare module 'tamagui' {
  // eslint-disable-next-line @typescript-eslint/no-empty-object-type
  interface TamaguiCustomConfig extends AppTamaguiConfig {}
}

export default config;
