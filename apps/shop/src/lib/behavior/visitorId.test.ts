/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest';
import {
  getOrCreateVisitorId,
  isValidUUID,
  VISITOR_ID_STORAGE_KEY,
} from './visitorId';

class MemoryStorage implements Storage {
  private store = new Map<string, string>();
  get length() { return this.store.size; }
  clear() { this.store.clear(); }
  getItem(key: string) { return this.store.has(key) ? this.store.get(key)! : null; }
  key(index: number) { return Array.from(this.store.keys())[index] || null; }
  removeItem(key: string) { this.store.delete(key); }
  setItem(key: string, value: string) { this.store.set(key, String(value)); }
}

const memStorage = new MemoryStorage();
if (typeof window !== 'undefined' && !window.localStorage) {
  Object.defineProperty(window, 'localStorage', { value: memStorage, writable: true });
}
if (typeof globalThis.localStorage === 'undefined') {
  (globalThis as any).localStorage = memStorage;
}

describe('visitorId', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('generates a valid UUID v4 when zamk_visitor_id is missing and stores it in localStorage', () => {
    expect(localStorage.getItem(VISITOR_ID_STORAGE_KEY)).toBeNull();

    const id = getOrCreateVisitorId();
    expect(isValidUUID(id)).toBe(true);
    expect(localStorage.getItem(VISITOR_ID_STORAGE_KEY)).toBe(id);
  });

  it('reuses existing valid stored UUID across calls and page loads', () => {
    const existing = '12345678-1234-4234-8234-1234567890ab';
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, existing);

    const id1 = getOrCreateVisitorId();
    const id2 = getOrCreateVisitorId();

    expect(id1).toBe(existing);
    expect(id2).toBe(existing);
  });

  it('replaces malformed or corrupted stored value with a new valid UUID', () => {
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, 'corrupted-non-uuid-string');

    const id = getOrCreateVisitorId();
    expect(isValidUUID(id)).toBe(true);
    expect(id).not.toBe('corrupted-non-uuid-string');
    expect(localStorage.getItem(VISITOR_ID_STORAGE_KEY)).toBe(id);
  });

  it('remains independent of user identity (does not change on auth state shifts)', () => {
    const initialId = getOrCreateVisitorId();
    expect(isValidUUID(initialId)).toBe(true);

    // Simulate user login / logout in AuthContext
    const loggedInUserId = 'user-9999';
    const idAfterLogin = getOrCreateVisitorId();
    expect(idAfterLogin).toBe(initialId);
    expect(idAfterLogin).not.toContain(loggedInUserId);

    const idAfterLogout = getOrCreateVisitorId();
    expect(idAfterLogout).toBe(initialId);
  });
});
