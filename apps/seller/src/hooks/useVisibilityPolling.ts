import { useEffect, useRef } from 'react';

/**
 * Periodically executes a callback only when document is visible.
 * Pauses when tab is hidden, and triggers an immediate silent execution when tab becomes visible.
 */
export function useVisibilityPolling(
  callback: () => Promise<void> | void,
  intervalMs: number = 4000,
  enabled: boolean = true
) {
  const savedCallback = useRef(callback);
  savedCallback.current = callback;

  useEffect(() => {
    if (!enabled) return;

    let isSubscribed = true;
    let timerId: ReturnType<typeof setInterval> | null = null;

    const execute = async () => {
      if (document.hidden || !isSubscribed) return;
      try {
        await savedCallback.current();
      } catch (err) {
        // Silently swallow background polling errors
      }
    };

    const startTimer = () => {
      if (timerId) clearInterval(timerId);
      timerId = setInterval(() => {
        if (!document.hidden && isSubscribed) {
          execute();
        }
      }, intervalMs);
    };

    const handleVisibilityChange = () => {
      if (!document.hidden && isSubscribed) {
        execute();
        startTimer();
      } else if (document.hidden && timerId) {
        clearInterval(timerId);
        timerId = null;
      }
    };

    const handleFocus = () => {
      if (!document.hidden && isSubscribed) {
        execute();
      }
    };

    startTimer();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    window.addEventListener('focus', handleFocus);

    return () => {
      isSubscribed = false;
      if (timerId) clearInterval(timerId);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      window.removeEventListener('focus', handleFocus);
    };
  }, [intervalMs, enabled]);
}
