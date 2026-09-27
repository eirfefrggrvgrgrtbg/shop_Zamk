/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import React from 'react';
import { render, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useCatalogImpression } from './useCatalogImpression';
import * as behaviorClientModule from './behaviorClient';
import { VISITOR_ID_STORAGE_KEY } from './visitorId';
import { ProductCard } from '../../components/product/ProductCard';

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
if (typeof window !== 'undefined' && !window.sessionStorage) {
  Object.defineProperty(window, 'sessionStorage', { value: new MemoryStorage(), writable: true });
}
if (typeof globalThis.localStorage === 'undefined') {
  (globalThis as any).localStorage = memStorage;
}
if (typeof globalThis.sessionStorage === 'undefined') {
  (globalThis as any).sessionStorage = new MemoryStorage();
}

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: null, openAuthModal: vi.fn() }),
}));
vi.mock('../../contexts/FavoritesContext', () => ({
  useFavorites: () => ({ isFavorite: () => false, toggleFavorite: vi.fn() }),
}));
vi.mock('../../contexts/ToastContext', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));
vi.mock('../../contexts/CartContext', () => ({
  useCart: () => ({ addItem: vi.fn() }),
}));

function TestCard({ productId, placement }: { productId: string; placement: string }) {
  const ref = useCatalogImpression<HTMLDivElement>({ productId, placement });
  return <div ref={ref} data-testid="card">Card</div>;
}

describe('PERS.1D — useCatalogImpression Tests', () => {
  let mockTrackCatalogImpression: ReturnType<typeof vi.spyOn>;
  let observerCallback: IntersectionObserverCallback;
  let observedElements: Element[] = [];
  let disconnectCount = 0;

  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, '11111111-1111-4111-8111-111111111111');
    vi.useFakeTimers();

    observedElements = [];
    disconnectCount = 0;

    class MockIntersectionObserver {
      constructor(cb: IntersectionObserverCallback) {
        observerCallback = cb;
      }
      observe(el: Element) {
        observedElements.push(el);
      }
      unobserve() {}
      disconnect() {
        disconnectCount++;
      }
    }

    (globalThis as any).IntersectionObserver = MockIntersectionObserver;
    if (typeof window !== 'undefined') {
      (window as any).IntersectionObserver = MockIntersectionObserver;
    }

    mockTrackCatalogImpression = vi.spyOn(behaviorClientModule, 'trackCatalogImpression').mockImplementation(() => {});
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('1. <50% visible -> no event emitted', () => {
    const { getByTestId } = render(<TestCard productId="prod-1" placement="catalog_grid" />);
    const div = getByTestId('card');

    // Simulate entry with 49% intersection ratio
    observerCallback(
      [
        {
          isIntersecting: true,
          intersectionRatio: 0.49,
          target: div,
        } as unknown as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver
    );

    vi.advanceTimersByTime(2000);
    expect(mockTrackCatalogImpression).not.toHaveBeenCalled();
  });

  it('2. >=50% but <1 second -> no event emitted', () => {
    const { getByTestId } = render(<TestCard productId="prod-1" placement="catalog_grid" />);
    const div = getByTestId('card');

    // Becomes 60% visible
    observerCallback(
      [
        {
          isIntersecting: true,
          intersectionRatio: 0.6,
          target: div,
        } as unknown as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver
    );

    vi.advanceTimersByTime(800); // Only 800ms
    expect(mockTrackCatalogImpression).not.toHaveBeenCalled();
  });

  it('3. >=50% continuously for 1 second -> emits one catalog_impression', () => {
    const { getByTestId } = render(<TestCard productId="prod-1" placement="catalog_grid" />);
    const div = getByTestId('card');

    // Becomes 75% visible
    observerCallback(
      [
        {
          isIntersecting: true,
          intersectionRatio: 0.75,
          target: div,
        } as unknown as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver
    );

    vi.advanceTimersByTime(1000);
    expect(mockTrackCatalogImpression).toHaveBeenCalledTimes(1);
    expect(mockTrackCatalogImpression).toHaveBeenCalledWith('prod-1', 'catalog_grid');
  });

  it('4. card leaves viewport before 1 second -> timer cancelled, no event emitted', () => {
    const { getByTestId } = render(<TestCard productId="prod-1" placement="catalog_grid" />);
    const div = getByTestId('card');

    // Becomes 100% visible
    observerCallback(
      [
        {
          isIntersecting: true,
          intersectionRatio: 1.0,
          target: div,
        } as unknown as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver
    );

    vi.advanceTimersByTime(500);

    // Leaves viewport at 500ms
    observerCallback(
      [
        {
          isIntersecting: false,
          intersectionRatio: 0,
          target: div,
        } as unknown as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver
    );

    vi.advanceTimersByTime(1000);
    expect(mockTrackCatalogImpression).not.toHaveBeenCalled();
  });

  it('5. ordinary rerender does not recreate observer or duplicate events', () => {
    const { getByTestId, rerender } = render(<TestCard productId="prod-1" placement="catalog_grid" />);
    const div = getByTestId('card');

    // Rerender with identical props
    rerender(<TestCard productId="prod-1" placement="catalog_grid" />);
    expect(disconnectCount).toBe(0);

    // Trigger visibility
    observerCallback(
      [
        {
          isIntersecting: true,
          intersectionRatio: 0.8,
          target: div,
        } as unknown as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver
    );

    vi.advanceTimersByTime(1000);
    expect(mockTrackCatalogImpression).toHaveBeenCalledTimes(1);
  });

  it('6. semantic dedupe integration suppresses duplicate within 5 min, allows different placement', () => {
    mockTrackCatalogImpression.mockRestore();
    behaviorClientModule.behaviorClient.resetForTesting();

    const { getByTestId, unmount } = render(<TestCard productId="prod-10" placement="catalog_grid" />);
    const div = getByTestId('card');

    // First impression -> queued
    observerCallback(
      [{ isIntersecting: true, intersectionRatio: 0.9, target: div } as any],
      {} as any
    );
    vi.advanceTimersByTime(1000);
    expect(behaviorClientModule.behaviorClient.getQueueLength()).toBe(1);

    // Second impression within 5 minutes for same product & placement -> suppressed by dedupe
    observerCallback(
      [{ isIntersecting: true, intersectionRatio: 0.9, target: div } as any],
      {} as any
    );
    vi.advanceTimersByTime(1000);
    expect(behaviorClientModule.behaviorClient.getQueueLength()).toBe(1); // Still 1!

    unmount();

    // Different placement -> allowed immediately
    const { getByTestId: getByTestId2 } = render(<TestCard productId="prod-10" placement="home_recommendations" />);
    const div2 = getByTestId2('card');

    observerCallback(
      [{ isIntersecting: true, intersectionRatio: 0.9, target: div2 } as any],
      {} as any
    );
    vi.advanceTimersByTime(1000);
    expect(behaviorClientModule.behaviorClient.getQueueLength()).toBe(2);
  });

  it('7. ProductCard observes the visual Polaroid card container rather than the outer wrapper', () => {
    const mockProduct: any = {
      id: 'prod-test-visual',
      name: 'Test Jacket',
      price: 100,
      image: '/test.jpg',
    };

    render(
      <MemoryRouter>
        <ProductCard product={mockProduct} placement="catalog_grid" />
      </MemoryRouter>
    );

    expect(observedElements.length).toBe(1);
    const observed = observedElements[0];

    // The observed node must NOT be the outer group wrapper
    expect(observed.classList.contains('group')).toBe(false);

    // The observed node must be the Polaroid card containing the image container
    expect(observed.querySelector('img')).not.toBeNull();
    expect(observed.classList.contains('backdrop-blur-xl') || observed.classList.contains('rounded-2xl')).toBe(true);
  });
});
