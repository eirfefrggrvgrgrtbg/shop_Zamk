/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as apiClient from '@zamk/api-client/src/behavior';
import { ApiError } from '@zamk/api-client/src/errors';
import {
  BehaviorEmitter,
  trackProductView,
  trackCatalogImpression,
  trackVariantSelected,
  trackAddToCart,
  trackRemoveFromCart,
  trackFavoriteAdded,
  trackFavoriteRemoved,
  trackCheckoutStarted,
  behaviorClient,
} from './behaviorClient';
import { VISITOR_ID_STORAGE_KEY } from './visitorId';
class MemoryStorage implements Storage {
  private store = new Map<string, string>();
  get length() { return this.store.size; }
  clear() { this.store.clear(); }
  getItem(key: string) { return this.store.has(key) ? this.store.get(key)! : null; }
  key(index: number) { return Array.from(this.store.keys())[index] || null; }
  removeItem(key: string) { this.store.delete(key); }
  setItem(key: string, value: string) { this.store.set(key, String(value)); }
}

const memLocalStorage = new MemoryStorage();
const memSessionStorage = new MemoryStorage();
if (typeof window !== 'undefined' && !window.localStorage) {
  Object.defineProperty(window, 'localStorage', { value: memLocalStorage, writable: true });
}
if (typeof window !== 'undefined' && !window.sessionStorage) {
  Object.defineProperty(window, 'sessionStorage', { value: memSessionStorage, writable: true });
}
if (typeof globalThis.localStorage === 'undefined') {
  (globalThis as any).localStorage = memLocalStorage;
}
if (typeof globalThis.sessionStorage === 'undefined') {
  (globalThis as any).sessionStorage = memSessionStorage;
}

describe('BehaviorEmitter & behaviorClient', () => {
  let mockIngest: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, '11111111-1111-4111-8111-111111111111');
    behaviorClient.resetForTesting();
    vi.useFakeTimers();

    mockIngest = vi.spyOn(apiClient, 'ingestBehavioralEvents').mockResolvedValue({
      accepted: 1,
      duplicates: 0,
      rejected: [],
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('event creation: generates stable UUID eventId, visitorId, occurredAt, route, and placement', () => {
    const emitter = new BehaviorEmitter();
    emitter.emit('product_view', {
      productId: 'prod-123',
      placement: 'detail_page',
      route: '/product/prod-123?utm_source=tg#reviews',
    });

    expect(emitter.getQueueLength()).toBe(1);
    const queued = (emitter as any).queue[0].event;

    expect(queued.eventType).toBe('product_view');
    expect(queued.productId).toBe('prod-123');
    expect(queued.visitorId).toBe('11111111-1111-4111-8111-111111111111');
    expect(queued.placement).toBe('detail_page');
    expect(queued.route).toBe('/product/prod-123'); // query string and hash stripped
    expect(queued.occurredAt).toBeTruthy();
    expect(queued.eventId).toBeTruthy();

    // Verify forbidden authority fields are NOT present
    expect((queued as any).userId).toBeUndefined();
    expect((queued as any).source).toBeUndefined();
    expect((queued as any).receivedAt).toBeUndefined();
    expect((queued as any).categoryId).toBeUndefined();
  });

  it('batching: single event stays in queue until timer or threshold flush occurs', async () => {
    const emitter = new BehaviorEmitter({ flushThreshold: 5, flushIntervalMs: 2000 });
    emitter.start();

    emitter.emit('product_view', { productId: 'prod-1' });
    expect(emitter.getQueueLength()).toBe(1);
    expect(mockIngest).not.toHaveBeenCalled();

    // Advance timer by 1 second (less than 2s) -> still not flushed
    vi.advanceTimersByTime(1000);
    expect(mockIngest).not.toHaveBeenCalled();

    // Advance timer to 2 seconds -> flushed
    await vi.advanceTimersByTimeAsync(1000);
    expect(mockIngest).toHaveBeenCalledTimes(1);
    expect(emitter.getQueueLength()).toBe(0);

    emitter.stop();
  });

  it('batching: threshold flush triggers immediately when queue reaches threshold', async () => {
    const emitter = new BehaviorEmitter({ flushThreshold: 3, flushIntervalMs: 10000 });

    emitter.emit('add_to_cart', { productId: 'prod-1', variantId: 'v-1', quantity: 1 });
    emitter.emit('add_to_cart', { productId: 'prod-2', variantId: 'v-2', quantity: 1 });
    expect(mockIngest).not.toHaveBeenCalled();

    // 3rd event reaches threshold -> triggers flush
    emitter.emit('add_to_cart', { productId: 'prod-3', variantId: 'v-3', quantity: 1 });
    await vi.runAllTimersAsync();

    expect(mockIngest).toHaveBeenCalledTimes(1);
    const sentEvents = mockIngest.mock.calls[0][0].events;
    expect(sentEvents.length).toBe(3);
  });

  it('single flush in flight: events emitted while flush is running are not lost or duplicated', async () => {
    let resolveFirstFlush: (res: any) => void = () => {};
    mockIngest.mockImplementationOnce(
      () =>
        new Promise((res) => {
          resolveFirstFlush = res;
        })
    );

    const emitter = new BehaviorEmitter({ flushThreshold: 2 });
    emitter.emit('product_view', { productId: 'prod-1' });
    emitter.emit('product_view', { productId: 'prod-2' }); // triggers first flush

    expect(mockIngest).toHaveBeenCalledTimes(1);

    // Emit more events while first flush is in flight
    emitter.emit('product_view', { productId: 'prod-3' });
    emitter.emit('product_view', { productId: 'prod-4' });

    // Complete first flush
    resolveFirstFlush({ accepted: 2, duplicates: 0, rejected: [] });
    await vi.runAllTimersAsync();

    // Flush remaining events
    await emitter.flush();
    expect(mockIngest).toHaveBeenCalledTimes(2);

    const firstCallEvents = mockIngest.mock.calls[0][0].events;
    const secondCallEvents = mockIngest.mock.calls[1][0].events;

    expect(firstCallEvents.map((e: any) => e.productId)).toEqual(['prod-1', 'prod-2']);
    expect(secondCallEvents.map((e: any) => e.productId)).toEqual(['prod-3', 'prod-4']);
  });

  it('failure & retry: network failure or 5xx requeues events preserving exact eventId', async () => {
    mockIngest.mockRejectedValueOnce(new Error('Network offline'));

    const emitter = new BehaviorEmitter();
    emitter.emit('product_view', { productId: 'prod-1' });

    const originalEventId = (emitter as any).queue[0].event.eventId;

    // Flush fails
    await emitter.flush();
    expect(mockIngest).toHaveBeenCalledTimes(1);

    // Events must be requeued, not lost
    expect(emitter.getQueueLength()).toBe(1);
    expect((emitter as any).queue[0].event.eventId).toBe(originalEventId);

    // Next successful flush sends same eventId
    mockIngest.mockResolvedValueOnce({ accepted: 1, duplicates: 0, rejected: [] });
    await emitter.flush();

    expect(mockIngest).toHaveBeenCalledTimes(2);
    expect(mockIngest.mock.calls[1][0].events[0].eventId).toBe(originalEventId);
    expect(emitter.getQueueLength()).toBe(0);
  });

  it('failure & retry: structural 400 or 413 error is dropped immediately and not retried', async () => {
    const apiError = new ApiError('Structural error', 'STRUCTURAL_ERROR', 400);
    mockIngest.mockRejectedValueOnce(apiError);

    const emitter = new BehaviorEmitter();
    emitter.emit('product_view', { productId: 'poison-prod' });

    await emitter.flush();
    expect(mockIngest).toHaveBeenCalledTimes(1);

    // Poison batch was dropped
    expect(emitter.getQueueLength()).toBe(0);
  });

  it('202 partial response: accepted, duplicates, and entity-rejected events are all completed and not retried', async () => {
    mockIngest.mockResolvedValueOnce({
      accepted: 1,
      duplicates: 1,
      rejected: [{ eventId: 'evt-3', code: 'unknown_product' }],
    });

    const emitter = new BehaviorEmitter();
    emitter.emit('product_view', { productId: 'prod-1' });
    emitter.emit('product_view', { productId: 'prod-2' });
    emitter.emit('product_view', { productId: 'prod-3' });

    await emitter.flush();
    expect(mockIngest).toHaveBeenCalledTimes(1);
    expect(emitter.getQueueLength()).toBe(0);
  });

  it('queue bound: drops oldest events when maxQueueSize is reached to protect memory', () => {
    const emitter = new BehaviorEmitter({ maxQueueSize: 3, flushThreshold: 100 });

    emitter.emit('product_view', { productId: 'prod-1' });
    emitter.emit('product_view', { productId: 'prod-2' });
    emitter.emit('product_view', { productId: 'prod-3' });
    emitter.emit('product_view', { productId: 'prod-4' });

    expect(emitter.getQueueLength()).toBe(3);
    const queuedIds = (emitter as any).queue.map((item: any) => item.event.productId);
    expect(queuedIds).toEqual(['prod-2', 'prod-3', 'prod-4']);
  });

  it('typed convenience helpers emit corresponding event types', () => {
    trackProductView('p-1');
    trackCatalogImpression('p-2', { placement: 'hero' });
    trackVariantSelected('p-3', 'v-1');
    trackAddToCart('p-4', 'v-2', 2);
    trackRemoveFromCart('p-5', 'v-3', 1);
    trackFavoriteAdded('p-6');
    trackFavoriteRemoved('p-7');
    trackCheckoutStarted({ route: '/checkout' });

    expect(behaviorClient.getQueueLength()).toBe(8);

    const queue = (behaviorClient as any).queue.map((item: any) => ({
      type: item.event.eventType,
      productId: item.event.productId,
      variantId: item.event.variantId,
      quantity: item.event.quantity,
      placement: item.event.placement,
      route: item.event.route,
    }));

    expect(queue).toEqual([
      { type: 'product_view', productId: 'p-1', variantId: undefined, quantity: undefined, placement: undefined, route: '/' },
      { type: 'catalog_impression', productId: 'p-2', variantId: undefined, quantity: undefined, placement: 'hero', route: '/' },
      { type: 'product_variant_selected', productId: 'p-3', variantId: 'v-1', quantity: undefined, placement: undefined, route: '/' },
      { type: 'add_to_cart', productId: 'p-4', variantId: 'v-2', quantity: 2, placement: undefined, route: '/' },
      { type: 'remove_from_cart', productId: 'p-5', variantId: 'v-3', quantity: 1, placement: undefined, route: '/' },
      { type: 'favorite_added', productId: 'p-6', variantId: undefined, quantity: undefined, placement: undefined, route: '/' },
      { type: 'favorite_removed', productId: 'p-7', variantId: undefined, quantity: undefined, placement: undefined, route: '/' },
      { type: 'checkout_started', productId: undefined, variantId: undefined, quantity: undefined, placement: undefined, route: '/checkout' },
    ]);
  });
});
