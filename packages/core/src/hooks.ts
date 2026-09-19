import { useEffect, useState } from 'react';

/** Значение, которое догоняет `value` после паузы в `delayMs` (для поиска по мере ввода). */
export function useDebouncedValue<T>(value: T, delayMs = 350): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);
  return debounced;
}
