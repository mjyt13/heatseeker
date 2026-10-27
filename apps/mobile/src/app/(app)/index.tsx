import { Redirect } from 'expo-router';

/** Что открывается при входе в приложение: «Ближайшее» — задачи с ближайшими сроками. */
export default function Home() {
  return <Redirect href="/(app)/upcoming" />;
}
