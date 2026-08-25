// Lazy-initialized singleton AudioContext to prevent exceeding browser context limits
let sharedAudioCtx: AudioContext | null = null;

function getAudioContext(): AudioContext | null {
  if (typeof window === 'undefined') return null;
  const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
  if (!AudioCtx) return null;

  if (!sharedAudioCtx) {
    try {
      sharedAudioCtx = new AudioCtx();
    } catch {
      return null;
    }
  }

  return sharedAudioCtx;
}

/**
 * Ensures the AudioContext is resumed upon user interaction if suspended by browser autoplay policy.
 */
export function unlockAudioContext(): void {
  try {
    const ctx = getAudioContext();
    if (ctx && ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }
  } catch {
    // Silently ignore
  }
}

// Auto-register user gesture listeners to unlock audio on first interaction
if (typeof window !== 'undefined') {
  const unlockEvents = ['keydown', 'pointerdown', 'touchstart', 'click'];
  const handleFirstInteraction = () => {
    unlockAudioContext();
    unlockEvents.forEach((ev) => window.removeEventListener(ev, handleFirstInteraction));
  };
  unlockEvents.forEach((ev) => {
    window.addEventListener(ev, handleFirstInteraction, { passive: true, once: true });
  });
}

/**
 * Plays a short synthetic beep sound using Web Audio API.
 * Never throws or interrupts application execution.
 */
export function playBeepSound(type: 'success' | 'error' | 'click' = 'success'): void {
  try {
    const ctx = getAudioContext();
    if (!ctx) return;

    const playTone = () => {
      try {
        const now = ctx.currentTime;
        const gain = ctx.createGain();
        gain.connect(ctx.destination);

        if (type === 'success') {
          // High-pitched bright tone (880Hz / A5)
          const osc = ctx.createOscillator();
          osc.type = 'sine';
          osc.frequency.setValueAtTime(880, now);
          gain.gain.setValueAtTime(0.15, now);
          gain.gain.exponentialRampToValueAtTime(0.001, now + 0.12);
          osc.connect(gain);
          osc.start(now);
          osc.stop(now + 0.12);
        } else if (type === 'error') {
          // Low distinct buzz tone (220Hz / A3)
          const osc = ctx.createOscillator();
          osc.type = 'sawtooth';
          osc.frequency.setValueAtTime(220, now);
          gain.gain.setValueAtTime(0.2, now);
          gain.gain.exponentialRampToValueAtTime(0.001, now + 0.28);
          osc.connect(gain);
          osc.start(now);
          osc.stop(now + 0.28);
        } else if (type === 'click') {
          // Short subtle click (600Hz)
          const osc = ctx.createOscillator();
          osc.type = 'triangle';
          osc.frequency.setValueAtTime(600, now);
          gain.gain.setValueAtTime(0.08, now);
          gain.gain.exponentialRampToValueAtTime(0.001, now + 0.05);
          osc.connect(gain);
          osc.start(now);
          osc.stop(now + 0.05);
        }
      } catch {
        // Silently ignore audio rendering errors
      }
    };

    if (ctx.state === 'suspended') {
      ctx.resume().then(playTone).catch(() => {});
    } else {
      playTone();
    }
  } catch {
    // Silently continue
  }
}
