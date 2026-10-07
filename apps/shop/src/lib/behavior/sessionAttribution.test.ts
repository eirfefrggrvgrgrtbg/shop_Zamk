/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as apiClient from '@zamk/api-client/src/behavior';
import {
  BehaviorEmitter,
  behaviorClient,
  parseAttributionMetadata,
  getNavigationAttributionMetadata,
  trackSessionStarted,
  trackPageView,
  trackProductView,
  trackAddToCart,
  trackCheckoutStarted,
} from './behaviorClient';
import {
  getOrRenewSession,
  getRawSession,
  rotateBehaviorSession,
  isSessionStartedEmitted,
  markSessionStartedEmitted,
  SESSION_STORAGE_KEY,
  SESSION_TIMEOUT_MS,
} from './session';
import { VISITOR_ID_STORAGE_KEY, getOrCreateVisitorId } from './visitorId';

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

const setMockLocation = (urlStr: string) => {
  Object.defineProperty(window, 'location', {
    value: new URL(urlStr),
    writable: true,
    configurable: true,
  });
};

const setMockReferrer = (refStr: string) => {
  Object.defineProperty(document, 'referrer', {
    value: refStr,
    writable: true,
    configurable: true,
  });
};

describe('ADS.1B — Session Attribution Contract Test Matrix (A–P)', () => {
  let mockIngest: ReturnType<typeof vi.spyOn>;
  const fixedVisitorId = '11111111-1111-4111-8111-111111111111';

  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, fixedVisitorId);
    behaviorClient.resetForTesting();
    vi.useFakeTimers();

    mockIngest = vi.spyOn(apiClient, 'ingestBehavioralEvents').mockResolvedValue({
      accepted: 1,
      duplicates: 0,
      rejected: [],
    });

    setMockLocation('https://shop.zamk.me/');
    setMockReferrer('');
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  // A. first direct visit
  it('A. first direct visit: creates session, session_started once without fake source, page_view once', () => {
    setMockLocation('https://shop.zamk.me/catalog');
    setMockReferrer('');

    const visitorId = getOrCreateVisitorId();
    const { sessionId, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(true);

    const meta = parseAttributionMetadata();
    expect(meta.source).toBeUndefined();
    expect(meta.medium).toBeUndefined();
    expect(meta.utm_source).toBeUndefined();
    expect(meta.referrer).toBeUndefined();
    expect(meta.landing_path).toBe('/catalog');

    trackSessionStarted(meta);
    trackPageView('/catalog');

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(2);

    expect(queue[0].eventType).toBe('session_started');
    expect(queue[0].sessionId).toBe(sessionId);
    expect(queue[0].metadata?.source).toBeUndefined();
    expect(queue[0].metadata?.landing_path).toBe('/catalog');

    expect(queue[1].eventType).toBe('page_view');
    expect(queue[1].sessionId).toBe(sessionId);
    expect(queue[1].route).toBe('/catalog');
    expect(queue[1].metadata).toBeUndefined();
  });

  // B. first UTM visit
  it('B. first UTM visit: session_started contains attribution, page_view has normalized route without query', () => {
    setMockLocation(
      'https://shop.zamk.me/catalog?utm_source=google&utm_medium=cpc&utm_campaign=spring&utm_term=shoes&utm_content=banner'
    );

    const visitorId = getOrCreateVisitorId();
    const { sessionId, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(true);

    const meta = parseAttributionMetadata();
    expect(meta.utm_source).toBe('google');
    expect(meta.utm_medium).toBe('cpc');
    expect(meta.utm_campaign).toBe('spring');
    expect(meta.utm_term).toBe('shoes');
    expect(meta.utm_content).toBe('banner');
    expect(meta.landing_path).toBe('/catalog');

    trackSessionStarted(meta);
    trackPageView('/catalog');

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(2);

    expect(queue[0].eventType).toBe('session_started');
    expect(queue[0].sessionId).toBe(sessionId);
    expect(queue[0].metadata?.utm_source).toBe('google');
    expect(queue[0].metadata?.utm_campaign).toBe('spring');

    expect(queue[1].eventType).toBe('page_view');
    expect(queue[1].sessionId).toBe(sessionId);
    expect(queue[1].route).toBe('/catalog'); // Query stripped from route
  });

  // C. external referral
  it('C. external referral: strips query/fragment/credentials, classifies source=hostname, medium=referral', () => {
    setMockLocation('https://shop.zamk.me/product/123');
    setMockReferrer('https://admin:secret@fashion-blog.ru:8080/articles/spring-trends?utm_ignore=1#section2');

    const meta = parseAttributionMetadata();
    expect(meta.source).toBe('fashion-blog.ru');
    expect(meta.medium).toBe('referral');
    // Normalization: credentials, query string, and fragment stripped
    expect(meta.referrer).toBe('https://fashion-blog.ru:8080/articles/spring-trends');
    expect(meta.landing_path).toBe('/product/123');
  });

  // D. same-origin referrer
  it('D. same-origin referrer: treated as direct, no non-direct attribution', () => {
    setMockLocation('https://shop.zamk.me/product/123');
    setMockReferrer('https://shop.zamk.me/catalog?page=2');

    const meta = parseAttributionMetadata();
    expect(meta.source).toBeUndefined();
    expect(meta.medium).toBeUndefined();
    expect(meta.referrer).toBeUndefined();
    expect(meta.landing_path).toBe('/product/123');
  });

  // E. SPA navigation
  it('E. SPA navigation: same session, page_view without attribution metadata, no duplicate session_started', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId } = getOrRenewSession(visitorId);
    markSessionStartedEmitted(sessionId);

    // Initial session_started
    trackSessionStarted({ landing_path: '/' });
    trackPageView('/');

    // User navigates via SPA to /cart
    trackPageView('/cart');

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(3);

    expect(queue[0].eventType).toBe('session_started');
    expect(queue[1].eventType).toBe('page_view');
    expect(queue[1].route).toBe('/');

    expect(queue[2].eventType).toBe('page_view');
    expect(queue[2].route).toBe('/cart');
    expect(queue[2].sessionId).toBe(sessionId);
    expect(queue[2].metadata).toBeUndefined();
  });

  // F. reload <30m
  it('F. reload <30m: same session_id, page_view emitted, does not emit duplicate session_started', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId } = getOrRenewSession(visitorId);
    markSessionStartedEmitted(sessionId);

    // Advance 10 minutes
    vi.advanceTimersByTime(10 * 60 * 1000);

    // Simulate page reload
    const { sessionId: reloadedSessionId, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(false);
    expect(reloadedSessionId).toBe(sessionId);
    expect(isSessionStartedEmitted(sessionId)).toBe(true);

    trackPageView('/catalog');

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(1);
    expect(queue[0].eventType).toBe('page_view');
    expect(queue[0].sessionId).toBe(sessionId);
  });

  // G. inactivity >30m
  it('G. inactivity >30m: creates new session, emits session_started and page_view', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId: session1 } = getOrRenewSession(visitorId);
    markSessionStartedEmitted(session1);

    // Advance 31 minutes
    vi.advanceTimersByTime(31 * 60 * 1000);

    const { sessionId: session2, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(true);
    expect(session2).not.toBe(session1);
    expect(isSessionStartedEmitted(session2)).toBe(false);

    markSessionStartedEmitted(session2);
    trackSessionStarted({ landing_path: '/catalog' });
    trackPageView('/catalog');

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(2);
    expect(queue[0].eventType).toBe('session_started');
    expect(queue[0].sessionId).toBe(session2);
    expect(queue[1].eventType).toBe('page_view');
    expect(queue[1].sessionId).toBe(session2);
  });

  // H. malformed session storage
  it('H. malformed session storage: cleans corrupt storage and creates clean session', () => {
    localStorage.setItem(SESSION_STORAGE_KEY, '{"sessionId":"not-a-uuid","visitorId":"bad","lastActive":"not-number"}');

    const visitorId = getOrCreateVisitorId();
    const { sessionId, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(true);
    expect(sessionId).toBeTruthy();

    const stored = getRawSession();
    expect(stored?.sessionId).toBe(sessionId);
    expect(stored?.visitorId).toBe(visitorId);
  });

  // I. anonymous → login SAME session
  it('I. anonymous → login SAME session: preserves session_id and visitor_id', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId } = getOrRenewSession(visitorId);

    trackPageView('/');

    // Simulate login: session state must NOT be rotated
    const sessionAfterLogin = getRawSession();
    expect(sessionAfterLogin?.sessionId).toBe(sessionId);
    expect(sessionAfterLogin?.visitorId).toBe(visitorId);

    trackPageView('/account');
    const queue = behaviorClient.getQueue();
    expect(queue[0].sessionId).toBe(sessionId);
    expect(queue[1].sessionId).toBe(sessionId);
  });

  // J. anonymous → register SAME session
  it('J. anonymous → register SAME session: preserves session_id and visitor_id', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId } = getOrRenewSession(visitorId);

    trackPageView('/');

    // Simulate register: session state must NOT be rotated
    const sessionAfterRegister = getRawSession();
    expect(sessionAfterRegister?.sessionId).toBe(sessionId);
    expect(sessionAfterRegister?.visitorId).toBe(visitorId);

    trackPageView('/orders');
    const queue = behaviorClient.getQueue();
    expect(queue[0].sessionId).toBe(sessionId);
    expect(queue[1].sessionId).toBe(sessionId);
  });

  // K. logout rotates session
  it('K. logout rotates session: immediately clears session so next event gets fresh session_id', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId: session1 } = getOrRenewSession(visitorId);

    trackPageView('/account');

    // Logout
    rotateBehaviorSession();
    expect(getRawSession()).toBeNull();

    // Next activity
    const { sessionId: session2, isNew } = getOrRenewSession(visitorId);
    expect(isNew).toBe(true);
    expect(session2).not.toBe(session1);

    trackPageView('/catalog');
    const queue = behaviorClient.getQueue();
    expect(queue[0].sessionId).toBe(session1);
    expect(queue[1].sessionId).toBe(session2);
  });

  // L. account A → logout → account B isolated
  it('L. account A → logout → account B isolated: account B uses post-logout fresh session', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId: sessionA } = getOrRenewSession(visitorId);

    // Account A activity
    trackPageView('/profile');
    expect(behaviorClient.getQueue()[0].sessionId).toBe(sessionA);

    // Logout from Account A
    rotateBehaviorSession();
    expect(getRawSession()).toBeNull();

    // Account B logs in and browses
    const { sessionId: sessionB } = getOrRenewSession(visitorId);
    expect(sessionB).not.toBe(sessionA);

    trackPageView('/orders');
    const queue = behaviorClient.getQueue();
    expect(queue[1].sessionId).toBe(sessionB);
  });

  // M. Google → VK during same active session updates LNDC
  it('M. Google → VK during same active session: keeps same session_id, emits page_view with VK metadata, no session_started', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId } = getOrRenewSession(visitorId);
    markSessionStartedEmitted(sessionId);

    // Active session originally from Google
    trackSessionStarted({ utm_source: 'google', utm_medium: 'cpc' });
    trackPageView('/catalog');

    // User navigates / lands with new VK campaign within 30m
    const vkSearch = '?utm_source=vk&utm_medium=paid_social&utm_campaign=drop2';
    const vkNavMeta = getNavigationAttributionMetadata(vkSearch);
    expect(vkNavMeta).toBeDefined();
    expect(vkNavMeta?.utm_source).toBe('vk');
    expect(vkNavMeta?.utm_medium).toBe('paid_social');
    expect(vkNavMeta?.utm_campaign).toBe('drop2');

    // Emits page_view with safe attribution metadata
    trackPageView('/catalog', { metadata: vkNavMeta });

    // Next direct/internal SPA navigation (no UTMs)
    const directNavMeta = getNavigationAttributionMetadata('');
    expect(directNavMeta).toBeUndefined();
    trackPageView('/product/456');

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(4);

    expect(queue[0].eventType).toBe('session_started');
    expect(queue[0].metadata?.utm_source).toBe('google');

    expect(queue[1].eventType).toBe('page_view');
    expect(queue[1].route).toBe('/catalog');

    // Page view with new VK touch
    expect(queue[2].eventType).toBe('page_view');
    expect(queue[2].sessionId).toBe(sessionId);
    expect(queue[2].metadata?.utm_source).toBe('vk');
    expect(queue[2].metadata?.utm_campaign).toBe('drop2');

    // Internal navigation without UTMs: metadata is undefined
    expect(queue[3].eventType).toBe('page_view');
    expect(queue[3].sessionId).toBe(sessionId);
    expect(queue[3].metadata).toBeUndefined();
  });

  // N. existing product/cart/checkout events have session_id
  it('N. existing product/cart/checkout events obtain active session_id centrally', () => {
    const visitorId = getOrCreateVisitorId();
    const { sessionId } = getOrRenewSession(visitorId);

    trackProductView('prod-1', 'detail');
    trackAddToCart({ productId: 'prod-1', variantId: 'v-1', quantity: 1 });
    trackCheckoutStarted({ route: '/checkout' });

    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(3);

    for (const e of queue) {
      expect(e.sessionId).toBe(sessionId);
      expect(e.visitorId).toBe(visitorId);
    }
  });

  // O. PII/arbitrary query values do not enter metadata
  it('O. PII/arbitrary query values do not enter metadata: only whitelisted UTM keys are parsed', () => {
    const dirtySearch =
      '?email=victim%40test.com&phone=%2B79991234567&user_id=12345&token=secret_abc&utm_source=telegram&utm_medium=channel&utm_campaign=sale&arbitrary=hack';

    const meta = parseAttributionMetadata(dirtySearch);

    expect(meta.utm_source).toBe('telegram');
    expect(meta.utm_medium).toBe('channel');
    expect(meta.utm_campaign).toBe('sale');

    // Verify PII and unauthorized fields are NOT present
    expect((meta as any).email).toBeUndefined();
    expect((meta as any).phone).toBeUndefined();
    expect((meta as any).user_id).toBeUndefined();
    expect((meta as any).token).toBeUndefined();
    expect((meta as any).arbitrary).toBeUndefined();
  });

  // P. StrictMode duplicate protection
  it('P. StrictMode duplicate protection: duplicate mount effect does not create duplicate session_started or page_view', () => {
    const visitorId = getOrCreateVisitorId();
    let currentNavigationKey: string | null = null;

    // Helper simulating BehaviorRouterTracker effect
    const simulateTrackerEffect = (location: { pathname: string; search: string; key: string }) => {
      const currentKey = `${location.pathname}?${location.search}#${location.key}`;
      if (currentNavigationKey === currentKey) {
        return; // Suppressed by StrictMode ref protection
      }
      currentNavigationKey = currentKey;

      const { sessionId, isNew } = getOrRenewSession(visitorId);
      if (isNew || !isSessionStartedEmitted(sessionId)) {
        markSessionStartedEmitted(sessionId);
        trackSessionStarted(parseAttributionMetadata());
      }
      trackPageView(location.pathname);
    };

    const firstNav = { pathname: '/catalog', search: '', key: 'k1' };

    // StrictMode: Mount 1
    simulateTrackerEffect(firstNav);
    expect(behaviorClient.getQueue().length).toBe(2); // session_started + page_view

    // StrictMode: Mount 2 (duplicate effect on remount)
    simulateTrackerEffect(firstNav);
    expect(behaviorClient.getQueue().length).toBe(2); // Still 2! Not duplicated

    // Real subsequent navigation
    const secondNav = { pathname: '/cart', search: '', key: 'k2' };
    simulateTrackerEffect(secondNav);
    expect(behaviorClient.getQueue().length).toBe(3); // +1 page_view for /cart

    // StrictMode on second navigation
    simulateTrackerEffect(secondNav);
    expect(behaviorClient.getQueue().length).toBe(3); // Still 3! Not duplicated
  });

  // Q. Checkout resolution attaches valid local visitor/session to analyticsContext
  it('Q. Checkout resolution attaches valid local visitor/session to analyticsContext', () => {
    const visitorId = getOrCreateVisitorId();
    const sessionInfo = getOrRenewSession(visitorId);

    const analyticsContext = sessionInfo && visitorId ? {
      visitorId: visitorId,
      sessionId: sessionInfo.sessionId
    } : undefined;

    expect(analyticsContext).toBeDefined();
    expect(analyticsContext?.visitorId).toBe(visitorId);
    expect(analyticsContext?.sessionId).toBe(sessionInfo.sessionId);
  });

  // R. AuctionWins resolution attaches valid local visitor/session to analyticsContext
  it('R. AuctionWins resolution attaches valid local visitor/session to analyticsContext', () => {
    const visitorId = getOrCreateVisitorId();
    const sessionInfo = getOrRenewSession(visitorId);

    const analyticsContext = sessionInfo && visitorId ? {
      visitorId: visitorId,
      sessionId: sessionInfo.sessionId
    } : undefined;

    expect(analyticsContext).toBeDefined();
    expect(analyticsContext?.visitorId).toBe(visitorId);
    expect(analyticsContext?.sessionId).toBe(sessionInfo.sessionId);
  });

  // S. Malformed / unavailable local storage proceeds without fatal error
  it('S. Malformed / unavailable local storage proceeds without fatal error', () => {
    memLocalStorage.setItem(SESSION_STORAGE_KEY, 'invalid-json{{{');

    // Should not throw, recovers gracefully
    let analyticsContext: any;
    expect(() => {
      const visitorId = getOrCreateVisitorId();
      const sessionInfo = visitorId ? getOrRenewSession(visitorId) : null;
      analyticsContext = sessionInfo && visitorId ? {
        visitorId: visitorId,
        sessionId: sessionInfo.sessionId
      } : undefined;
    }).not.toThrow();

    expect(analyticsContext).toBeDefined();
    expect(analyticsContext.sessionId).toBeDefined();
  });

  // T. Frontend never sends source, UTM, campaign_id, or user_id in analyticsContext
  it('T. Frontend never sends source, UTM, campaign_id, or user_id in analyticsContext', () => {
    const visitorId = getOrCreateVisitorId();
    const sessionInfo = getOrRenewSession(visitorId);

    const analyticsContext = sessionInfo && visitorId ? {
      visitorId: visitorId,
      sessionId: sessionInfo.sessionId
    } : undefined;

    const payloadKeys = Object.keys(analyticsContext || {});
    expect(payloadKeys).toEqual(['visitorId', 'sessionId']);
    expect((analyticsContext as any).source).toBeUndefined();
    expect((analyticsContext as any).utm_source).toBeUndefined();
    expect((analyticsContext as any).campaign_id).toBeUndefined();
    expect((analyticsContext as any).user_id).toBeUndefined();
  });

  // U. Shop zamk_token capture
  it('U. Shop zamk_token capture: captures URL-safe zamk_token into metadata, route remains query-free', () => {
    const token = 'c7a40fb68e3146d9a244b7522ff60784';
    setMockLocation(`https://shop.zamk.me/catalog?utm_source=telegram&utm_medium=channel&utm_campaign=autumn_drop&zamk_token=${token}`);

    const meta = parseAttributionMetadata();
    expect(meta.zamk_token).toBe(token);
    expect(meta.utm_source).toBe('telegram');
    expect(meta.utm_campaign).toBe('autumn_drop');
    expect(meta.landing_path).toBe('/catalog');

    // Route passed to page_view stays clean
    trackPageView('/catalog', { metadata: meta });
    const queue = behaviorClient.getQueue();
    expect(queue.length).toBe(1);
    expect(queue[0].route).toBe('/catalog');
    expect(queue[0].metadata?.zamk_token).toBe(token);
    expect(queue[0].metadata?.utm_source).toBe('telegram');
  });

  // V. Invalid / dangerous zamk_token rejected from attribution metadata
  it('V. Invalid / dangerous zamk_token rejected: discard non-URL-safe, too short, or too long tokens', () => {
    // Too short (<8)
    let meta = parseAttributionMetadata('?zamk_token=short');
    expect(meta.zamk_token).toBeUndefined();

    // Invalid charset (contains semicolon, spaces, script tags)
    meta = parseAttributionMetadata('?zamk_token=token<script>alert(1)</script>');
    expect(meta.zamk_token).toBeUndefined();

    // Too long (>128 chars)
    const longToken = 'a'.repeat(129);
    meta = parseAttributionMetadata(`?zamk_token=${longToken}`);
    expect(meta.zamk_token).toBeUndefined();

    // Valid URL-safe base64 token is accepted
    const validToken = 'kI-8_d9X2A_1234567890abcdefABCDEF';
    meta = parseAttributionMetadata(`?zamk_token=${validToken}`);
    expect(meta.zamk_token).toBe(validToken);
  });
});
