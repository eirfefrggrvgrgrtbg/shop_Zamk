export const VISITOR_ID_STORAGE_KEY = 'zamk_visitor_id';

const UUID_V4_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export const isValidUUID = (val: unknown): val is string => {
  return typeof val === 'string' && UUID_V4_REGEX.test(val);
};

const generateUUID = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  // Fallback for older environments
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
};

/**
 * Returns the durable anonymous visitor ID from localStorage.
 * Lazily creates and stores a new UUID if missing or malformed.
 */
export const getOrCreateVisitorId = (): string => {
  if (typeof window === 'undefined' || !window.localStorage) {
    return generateUUID();
  }

  try {
    const stored = window.localStorage.getItem(VISITOR_ID_STORAGE_KEY);
    if (isValidUUID(stored)) {
      return stored;
    }

    const newId = generateUUID();
    window.localStorage.setItem(VISITOR_ID_STORAGE_KEY, newId);
    return newId;
  } catch {
    // LocalStorage might be disabled in private browsing
    return generateUUID();
  }
};
