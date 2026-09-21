import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';

/**
 * Виден ли экран сейчас. Скрытые вкладки остаются смонтированными, и без
 * этого они продолжали бы опрашивать сервер в фоне.
 */
export function useScreenFocused(): boolean {
  const [focused, setFocused] = useState(true);
  useFocusEffect(
    useCallback(() => {
      setFocused(true);
      return () => setFocused(false);
    }, []),
  );
  return focused;
}
