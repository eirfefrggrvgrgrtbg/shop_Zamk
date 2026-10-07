import { isValidUUID } from './visitorId';

export const SESSION_STORAGE_KEY = 'zamk_behavior_session_v1';
export const SESSION_TIMEOUT_MS = 30 * 60 * 1000; // 30 minutes

export interface SessionState {
  sessionId: string;
  visitorId: string;
  lastActive: number;
  sessionStartedEmitted: boolean;
}

const generateUUID = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
};

export const getRawSession = (): SessionState | null => {
  if (typeof window === 'undefined' || !window.localStorage) {
    return null;
  }
  try {
    const stored = window.localStorage.getItem(SESSION_STORAGE_KEY);
    if (!stored) return null;
    const parsed = JSON.parse(stored);
    if (
      typeof parsed === 'object' &&
      parsed !== null &&
      isValidUUID(parsed.sessionId) &&
      isValidUUID(parsed.visitorId) &&
      typeof parsed.lastActive === 'number' &&
      !isNaN(parsed.lastActive)
    ) {
      return {
        sessionId: parsed.sessionId,
        visitorId: parsed.visitorId,
        lastActive: parsed.lastActive,
        sessionStartedEmitted: Boolean(parsed.sessionStartedEmitted),
      };
    }
    // Clean up malformed session
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
    return null;
  } catch {
    try {
      window.localStorage.removeItem(SESSION_STORAGE_KEY);
    } catch {
      // Best effort
    }
    return null;
  }
};

export const saveSession = (state: SessionState): void => {
  if (typeof window === 'undefined' || !window.localStorage) return;
  try {
    window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Best effort
  }
};

export const rotateBehaviorSession = (): void => {
  if (typeof window === 'undefined' || !window.localStorage) return;
  try {
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
  } catch {
    // Best effort
  }
};

export const isSessionStartedEmitted = (sessionId: string): boolean => {
  const session = getRawSession();
  if (!session || session.sessionId !== sessionId) return false;
  return session.sessionStartedEmitted;
};

export const markSessionStartedEmitted = (sessionId: string): void => {
  const session = getRawSession();
  if (session && session.sessionId === sessionId) {
    session.sessionStartedEmitted = true;
    saveSession(session);
  }
};

/**
 * Gets the current active session ID for the given visitor, or creates a new one.
 * Touches lastActive timestamp with current time to refresh the 30-minute inactivity clock.
 * If a new session is created, `isNew` will be true.
 */
export const getOrRenewSession = (
  visitorId: string,
  now = Date.now()
): { sessionId: string; isNew: boolean } => {
  const existing = getRawSession();

  if (existing) {
    const isExpired = now - existing.lastActive > SESSION_TIMEOUT_MS;
    const isMismatched = existing.visitorId !== visitorId;

    if (!isExpired && !isMismatched) {
      // Valid session, touch lastActive clock
      existing.lastActive = now;
      saveSession(existing);
      return { sessionId: existing.sessionId, isNew: false };
    }
  }

  // Create new session
  const newSession: SessionState = {
    sessionId: generateUUID(),
    visitorId,
    lastActive: now,
    sessionStartedEmitted: false,
  };
  saveSession(newSession);
  return { sessionId: newSession.sessionId, isNew: true };
};
