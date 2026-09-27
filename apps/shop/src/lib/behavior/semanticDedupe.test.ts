/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  shouldSuppressEvent,
  recordEventDeduped,
  DEDUPE_STORAGE_KEY,
} from './semanticDedupe';

describe('semanticDedupe', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.useRealTimers();
  });

  it('product_view: allows first emit, suppresses within 30 minutes, allows again after 30 minutes', () => {
    const visitorId = 'v-111';
    const productId = 'prod-123';
    const now = 1000000;

    // 1. First emit -> not suppressed
    expect(shouldSuppressEvent(visitorId, 'product_view', productId, undefined, now)).toBe(false);

    // Record dedupe
    recordEventDeduped(visitorId, 'product_view', productId, undefined, now);

    // 2. Same product within 30 min (e.g. at 29 min) -> suppressed
    const at29Min = now + (30 * 60 * 1000 - 1000);
    expect(shouldSuppressEvent(visitorId, 'product_view', productId, undefined, at29Min)).toBe(true);

    // 3. Different product -> allowed immediately
    expect(shouldSuppressEvent(visitorId, 'product_view', 'prod-456', undefined, at29Min)).toBe(false);

    // 4. Same product after 30 min -> allowed again
    const at31Min = now + (30 * 60 * 1000 + 1000);
    expect(shouldSuppressEvent(visitorId, 'product_view', productId, undefined, at31Min)).toBe(false);
  });

  it('catalog_impression: allows first emit, suppresses within 5 minutes for same placement, allows different placement or after 5 minutes', () => {
    const visitorId = 'v-111';
    const productId = 'prod-123';
    const placement = 'main_banner';
    const now = 1000000;

    // 1. First emit -> not suppressed
    expect(shouldSuppressEvent(visitorId, 'catalog_impression', productId, placement, now)).toBe(false);

    // Record dedupe
    recordEventDeduped(visitorId, 'catalog_impression', productId, placement, now);

    // 2. Same product + same placement within 5 min (e.g. at 4 min) -> suppressed
    const at4Min = now + (5 * 60 * 1000 - 1000);
    expect(shouldSuppressEvent(visitorId, 'catalog_impression', productId, placement, at4Min)).toBe(true);

    // 3. Same product + different placement -> allowed immediately
    expect(shouldSuppressEvent(visitorId, 'catalog_impression', productId, 'sidebar', at4Min)).toBe(false);

    // 4. Same product + same placement after 5 min -> allowed again
    const at6Min = now + (5 * 60 * 1000 + 1000);
    expect(shouldSuppressEvent(visitorId, 'catalog_impression', productId, placement, at6Min)).toBe(false);
  });

  it('survives simulated component rerenders and reloads through sessionStorage', () => {
    const visitorId = 'v-111';
    const productId = 'prod-123';
    const now = 1000000;

    recordEventDeduped(visitorId, 'product_view', productId, undefined, now);

    // Verify raw state in sessionStorage
    const raw = sessionStorage.getItem(DEDUPE_STORAGE_KEY);
    expect(raw).toBeTruthy();
    expect(JSON.parse(raw!)[`pv:${visitorId}:${productId}`]).toBe(now);

    // Next query reads directly from sessionStorage
    expect(shouldSuppressEvent(visitorId, 'product_view', productId, undefined, now + 1000)).toBe(true);
  });

  it('does not suppress event types without dedupe rules (e.g. add_to_cart, favorite_added)', () => {
    const visitorId = 'v-111';
    const productId = 'prod-123';
    const now = 1000000;

    expect(shouldSuppressEvent(visitorId, 'add_to_cart', productId, undefined, now)).toBe(false);
    recordEventDeduped(visitorId, 'add_to_cart', productId, undefined, now);
    expect(shouldSuppressEvent(visitorId, 'add_to_cart', productId, undefined, now + 1000)).toBe(false);
  });
});
