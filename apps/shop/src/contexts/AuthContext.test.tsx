/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import React from 'react';
import { renderHook, act } from '@testing-library/react';
import { AuthProvider, useAuth } from './AuthContext';
import * as authApi from '@zamk/api-client/src/auth';
import {
  getOrRenewSession,
  getRawSession,
  rotateBehaviorSession,
  SESSION_STORAGE_KEY,
} from '../lib/behavior/session';
import { VISITOR_ID_STORAGE_KEY, getOrCreateVisitorId } from '../lib/behavior/visitorId';

class MemoryStorage implements Storage {
  private store = new Map<string, string>();
  get length() {
    return this.store.size;
  }
  clear() {
    this.store.clear();
  }
  getItem(key: string) {
    return this.store.has(key) ? this.store.get(key)! : null;
  }
  key(index: number) {
    return Array.from(this.store.keys())[index] || null;
  }
  removeItem(key: string) {
    this.store.delete(key);
  }
  setItem(key: string, value: string) {
    this.store.set(key, String(value));
  }
}

const memLocalStorage = new MemoryStorage();
if (typeof window !== 'undefined') {
  Object.defineProperty(window, 'localStorage', { value: memLocalStorage, writable: true });
}

describe('AuthContext — Session Attribution Continuity & Isolation', () => {
  const visitorId = '11111111-1111-4111-8111-111111111111';

  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, visitorId);
    vi.restoreAllMocks();

    // Default: not authenticated on init
    vi.spyOn(authApi, 'refresh').mockRejectedValue(new Error('unauthorized'));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('anonymous → login: keeps the same session_id and visitor_id for attribution continuity', async () => {
    // 1. Establish anonymous session before login
    const { sessionId: anonymousSessionId } = getOrRenewSession(visitorId);
    expect(anonymousSessionId).toBeTruthy();

    const mockLoginUser = {
      id: '22222222-2222-4222-8222-222222222222',
      email: 'customer@zamk.me',
      role: 'customer',
      name: 'Customer One',
    };
    vi.spyOn(authApi, 'login').mockResolvedValue({
      user: mockLoginUser,
      token: 'jwt-token',
    } as any);

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await act(async () => {
      await result.current.login('customer@zamk.me', 'secret123');
    });

    expect(result.current.user?.id).toBe(mockLoginUser.id);

    // Verify session was NOT rotated
    const sessionAfterLogin = getRawSession();
    expect(sessionAfterLogin?.sessionId).toBe(anonymousSessionId);
    expect(sessionAfterLogin?.visitorId).toBe(visitorId);
  });

  it('anonymous → register: keeps the same session_id and visitor_id', async () => {
    const { sessionId: anonymousSessionId } = getOrRenewSession(visitorId);

    const mockRegisterUser = {
      id: '33333333-3333-4333-8333-333333333333',
      email: 'newuser@zamk.me',
      role: 'customer',
      name: 'New Customer',
    };
    vi.spyOn(authApi, 'register').mockResolvedValue({
      user: mockRegisterUser,
      token: 'jwt-token',
    } as any);

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await act(async () => {
      await result.current.register(
        'New',
        'Customer',
        '',
        '+79991234567',
        'newuser@zamk.me',
        'secret123',
        'secret123'
      );
    });

    expect(result.current.user?.id).toBe(mockRegisterUser.id);

    // Verify session was NOT rotated
    const sessionAfterRegister = getRawSession();
    expect(sessionAfterRegister?.sessionId).toBe(anonymousSessionId);
    expect(sessionAfterRegister?.visitorId).toBe(visitorId);
  });

  it('logout: rotates/clears analytics session immediately to isolate future anonymous sessions', async () => {
    const { sessionId: userSessionId } = getOrRenewSession(visitorId);

    vi.spyOn(authApi, 'logout').mockResolvedValue({} as any);

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await act(async () => {
      await result.current.logout();
    });

    expect(result.current.user).toBeNull();

    // Verify session was cleared
    expect(getRawSession()).toBeNull();

    // Next activity gets a clean, new session
    const { sessionId: nextSessionId, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(true);
    expect(nextSessionId).not.toBe(userSessionId);
  });

  it('account switch: switching between different authenticated accounts rotates analytics session', async () => {
    const userA = {
      id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
      email: 'userA@zamk.me',
      role: 'customer',
    };
    const userB = {
      id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
      email: 'userB@zamk.me',
      role: 'customer',
    };

    // Hydrate User A initially
    vi.spyOn(authApi, 'refresh').mockResolvedValue({ user: userA } as any);

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    // Wait for hydration
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });
    expect(result.current.user?.id).toBe(userA.id);

    // Create session for User A
    const { sessionId: sessionA } = getOrRenewSession(visitorId);
    expect(sessionA).toBeTruthy();

    // User B logs in without explicit logout (account switch)
    vi.spyOn(authApi, 'login').mockResolvedValue({ user: userB, token: 'jwt' } as any);
    await act(async () => {
      await result.current.login('userB@zamk.me', 'secret123');
    });

    expect(result.current.user?.id).toBe(userB.id);

    // Session A should have been rotated
    expect(getRawSession()).toBeNull();

    // Next event for User B creates new session
    const { sessionId: sessionB } = getOrRenewSession(visitorId);
    expect(sessionB).not.toBe(sessionA);
  });
});
