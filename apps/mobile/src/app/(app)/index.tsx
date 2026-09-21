import { Redirect } from 'expo-router';

/**
 * Что открывается при входе в приложение. Пока — задачи; какой экран должен
 * встречать, решим отдельно (docs/FIXES.md).
 */
export default function Home() {
  return <Redirect href="/(app)/tasks" />;
}
