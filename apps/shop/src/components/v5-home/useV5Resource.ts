import { useEffect, useRef, useState } from 'react';

export interface V5Resource<T> {
  data: T | undefined;
  error: boolean;
  isLoading: boolean;
}

interface Settled<T> {
  key: string;
  data?: T;
  error: boolean;
}

/**
 * Loads one resource per `key` (auth state, retry counter, ...).
 * State is written only from the promise callbacks; `isLoading` is derived (the stored result belongs to
 * another key), so there is no synchronous setState in the effect. Answers for a stale key are dropped.
 */
export function useV5Resource<T>(key: string, load: () => Promise<T>): V5Resource<T> {
  const [settled, setSettled] = useState<Settled<T> | null>(null);
  const loadRef = useRef(load);

  useEffect(() => {
    loadRef.current = load;
  });

  useEffect(() => {
    let cancelled = false;
    loadRef
      .current()
      .then((data) => {
        if (!cancelled) setSettled({ key, data, error: false });
      })
      .catch(() => {
        if (!cancelled) setSettled({ key, error: true });
      });
    return () => {
      cancelled = true;
    };
  }, [key]);

  const current = settled?.key === key ? settled : null;
  return { data: current?.data, error: current?.error ?? false, isLoading: current === null };
}
