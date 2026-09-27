import type { ClientSafeEventType } from '@zamk/api-client/src/behavior';

export const DEDUPE_STORAGE_KEY = 'zamk_semantic_dedupe';

export const PRODUCT_VIEW_DEDUPE_WINDOW_MS = 30 * 60 * 1000; // 30 minutes
export const CATALOG_IMPRESSION_DEDUPE_WINDOW_MS = 5 * 60 * 1000; // 5 minutes
const MAX_DEDUPE_ENTRIES = 200;

interface DedupeState {
  [key: string]: number; // key -> timestamp of last emit
}

const getDedupeKey = (
  visitorId: string,
  eventType: ClientSafeEventType,
  productId?: string,
  placement?: string
): string | null => {
  if (eventType === 'product_view' && productId) {
    return `pv:${visitorId}:${productId}`;
  }
  if (eventType === 'catalog_impression' && productId) {
    return `ci:${visitorId}:${productId}:${placement || ''}`;
  }
  return null;
};

const getWindowForEventType = (eventType: ClientSafeEventType): number => {
  if (eventType === 'product_view') {
    return PRODUCT_VIEW_DEDUPE_WINDOW_MS;
  }
  if (eventType === 'catalog_impression') {
    return CATALOG_IMPRESSION_DEDUPE_WINDOW_MS;
  }
  return 0;
};

const loadDedupeState = (): DedupeState => {
  if (typeof window === 'undefined' || !window.sessionStorage) {
    return {};
  }
  try {
    const raw = window.sessionStorage.getItem(DEDUPE_STORAGE_KEY);
    if (!raw) return {};
    return JSON.parse(raw);
  } catch {
    return {};
  }
};

const saveDedupeState = (state: DedupeState): void => {
  if (typeof window === 'undefined' || !window.sessionStorage) {
    return;
  }
  try {
    window.sessionStorage.setItem(DEDUPE_STORAGE_KEY, JSON.stringify(state));
  } catch {
    // SessionStorage full or disabled
  }
};

const pruneExpiredEntries = (state: DedupeState, now: number): DedupeState => {
  const nextState: DedupeState = {};
  const entries = Object.entries(state);

  for (const [key, ts] of entries) {
    // Max window across all rules is 30 minutes
    if (now - ts <= PRODUCT_VIEW_DEDUPE_WINDOW_MS) {
      nextState[key] = ts;
    }
  }

  // If still above max entries, sort and retain most recent
  const keys = Object.keys(nextState);
  if (keys.length > MAX_DEDUPE_ENTRIES) {
    const sorted = keys.sort((a, b) => nextState[b] - nextState[a]);
    const trimmedState: DedupeState = {};
    for (let i = 0; i < MAX_DEDUPE_ENTRIES; i++) {
      trimmedState[sorted[i]] = nextState[sorted[i]];
    }
    return trimmedState;
  }

  return nextState;
};

/**
 * Checks whether an event should be suppressed by semantic deduplication.
 */
export const shouldSuppressEvent = (
  visitorId: string,
  eventType: ClientSafeEventType,
  productId?: string,
  placement?: string,
  now = Date.now()
): boolean => {
  const windowMs = getWindowForEventType(eventType);
  if (windowMs === 0) {
    return false; // No dedupe rule for this event type
  }

  const key = getDedupeKey(visitorId, eventType, productId, placement);
  if (!key) {
    return false;
  }

  const state = loadDedupeState();
  const lastTs = state[key];
  if (typeof lastTs === 'number' && now - lastTs < windowMs) {
    return true; // Suppress
  }

  return false;
};

/**
 * Records a successful queue addition for an event into the semantic dedupe storage.
 */
export const recordEventDeduped = (
  visitorId: string,
  eventType: ClientSafeEventType,
  productId?: string,
  placement?: string,
  now = Date.now()
): void => {
  const windowMs = getWindowForEventType(eventType);
  if (windowMs === 0) {
    return;
  }

  const key = getDedupeKey(visitorId, eventType, productId, placement);
  if (!key) {
    return;
  }

  const state = loadDedupeState();
  const pruned = pruneExpiredEntries(state, now);
  pruned[key] = now;
  saveDedupeState(pruned);
};
