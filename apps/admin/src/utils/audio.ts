// Lazy-initialized singleton AudioContext to prevent exceeding browser context limits
let sharedAudioCtx: AudioContext | null = null;
let isAudioUnlocked = false;

function getAudioContext(): AudioContext | null {
  if (typeof window === 'undefined') return null;
  const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
  if (!AudioCtx) return null;

  if (!sharedAudioCtx || sharedAudioCtx.state === 'closed') {
    try {
      sharedAudioCtx = new AudioCtx();
    } catch {
      return null;
    }
  }

  return sharedAudioCtx;
}

/**
 * Synchronously unlocks Web Audio during a user activation (click/keydown/submit).
 * Plays a 1-sample silent buffer to fully prime WebKit/Safari audio hardware.
 * Safe to call repeatedly. Returns true if audio context is ready.
 */
export function unlockScannerAudio(): boolean {
  try {
    const ctx = getAudioContext();
    if (!ctx) return false;

    if (ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }

    if (!isAudioUnlocked || ctx.state === 'suspended') {
      try {
        const buffer = ctx.createBuffer(1, 1, 22050);
        const source = ctx.createBufferSource();
        source.buffer = buffer;
        source.connect(ctx.destination);
        source.start(0);
        isAudioUnlocked = true;
      } catch {
        // Silently continue if buffer creation fails
      }
    }

    return true;
  } catch {
    return false;
  }
}

// Backward-compatible alias
export const unlockAudioContext = unlockScannerAudio;

// Global auto-registration on window for first user gesture
if (typeof window !== 'undefined') {
  const unlockEvents = ['keydown', 'pointerdown', 'touchstart', 'click'];
  const onFirstGesture = () => {
    unlockScannerAudio();
    unlockEvents.forEach((ev) => window.removeEventListener(ev, onFirstGesture, true));
  };
  unlockEvents.forEach((ev) => {
    window.addEventListener(ev, onFirstGesture, { capture: true, passive: true });
  });
}

/**
 * Plays scanner audio feedback. Never throws or halts execution.
 */
export function playBeepSound(type: 'success' | 'error' | 'click' = 'success'): void {
  try {
    const ctx = getAudioContext();
    if (!ctx) return;

    if (ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }

    const now = ctx.currentTime;

    if (type === 'success') {
      // Crisp high-pitched confirmation tone (1000 Hz / ~C6)
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(1000, now);
      gain.gain.setValueAtTime(0.25, now);
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.12);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start(now);
      osc.stop(now + 0.12);
    } else if (type === 'error') {
      // Double low buzz tone (280 Hz)
      // Pulse 1: 0 to 0.08s
      const osc1 = ctx.createOscillator();
      const gain1 = ctx.createGain();
      osc1.type = 'sawtooth';
      osc1.frequency.setValueAtTime(280, now);
      gain1.gain.setValueAtTime(0.3, now);
      gain1.gain.exponentialRampToValueAtTime(0.001, now + 0.08);
      osc1.connect(gain1);
      gain1.connect(ctx.destination);
      osc1.start(now);
      osc1.stop(now + 0.08);

      // Pulse 2: 0.11s to 0.22s
      const osc2 = ctx.createOscillator();
      const gain2 = ctx.createGain();
      osc2.type = 'sawtooth';
      osc2.frequency.setValueAtTime(280, now + 0.11);
      gain2.gain.setValueAtTime(0.3, now + 0.11);
      gain2.gain.exponentialRampToValueAtTime(0.001, now + 0.22);
      osc2.connect(gain2);
      gain2.connect(ctx.destination);
      osc2.start(now + 0.11);
      osc2.stop(now + 0.22);
    } else if (type === 'click') {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'triangle';
      osc.frequency.setValueAtTime(600, now);
      gain.gain.setValueAtTime(0.1, now);
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.04);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start(now);
      osc.stop(now + 0.04);
    }
  } catch {
    // Silently continue
  }
}
