// Примитивы Tamagui, которые используют экраны, — один импорт вместо двух.
export {
  Button,
  Card,
  H1,
  H2,
  H3,
  H4,
  Input,
  Label,
  Paragraph,
  ScrollView,
  Separator,
  SizableText,
  Spinner,
  Text,
  TextArea,
  Theme,
  View,
  XStack,
  YStack,
  TamaguiProvider,
  useTheme,
} from 'tamagui';

export { config as tamaguiConfig } from './tamagui.config';
export type { AppTamaguiConfig } from './tamagui.config';

export * from './components/chip';
export * from './components/screen';
export * from './components/form';
export * from './components/list';
