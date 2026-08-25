// Reusable HTMLAudioElement instances for scanner feedback
let successAudio: HTMLAudioElement | null = null;
let errorAudio: HTMLAudioElement | null = null;

function getSuccessAudio(): HTMLAudioElement | null {
  if (typeof window === 'undefined' || typeof Audio === 'undefined') return null;
  if (!successAudio) {
    successAudio = new Audio('/sounds/scan-success.wav');
    successAudio.preload = 'auto';
  }
  return successAudio;
}

function getErrorAudio(): HTMLAudioElement | null {
  if (typeof window === 'undefined' || typeof Audio === 'undefined') return null;
  if (!errorAudio) {
    errorAudio = new Audio('/sounds/scan-error.wav');
    errorAudio.preload = 'auto';
  }
  return errorAudio;
}

/**
 * Prime audio elements on user gesture (optional Safari optimization).
 */
export function primeScannerAudio(): void {
  try {
    const s = getSuccessAudio();
    const e = getErrorAudio();
    if (s && s.readyState < 2) s.load();
    if (e && e.readyState < 2) e.load();
  } catch {
    // Non-blocking
  }
}

// Global auto-prime on first user gesture
if (typeof window !== 'undefined') {
  const gestureEvents = ['keydown', 'pointerdown', 'touchstart', 'click'];
  const onFirstGesture = () => {
    primeScannerAudio();
    gestureEvents.forEach((ev) => window.removeEventListener(ev, onFirstGesture, true));
  };
  gestureEvents.forEach((ev) => {
    window.addEventListener(ev, onFirstGesture, { capture: true, passive: true });
  });
}

/**
 * Plays scanner audio feedback for regular receiving flow.
 * Never throws, never halts execution or network flow.
 */
export function playBeepSound(type: 'success' | 'error' | 'click' = 'success'): void {
  try {
    const audio = type === 'error' ? getErrorAudio() : getSuccessAudio();
    if (!audio) return;

    audio.currentTime = 0;
    const playPromise = audio.play();
    if (playPromise !== undefined) {
      playPromise.catch((_) => {
        // Non-blocking background catch
      });
    }
  } catch {
    // Non-blocking
  }
}

// Canonical alias
export const playScannerSound = playBeepSound;
