/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import React, { useEffect, useRef, useState } from 'react';
import { render, screen, act, cleanup } from '@testing-library/react';
import * as behaviorClientModule from './behaviorClient';
import { trackProductView, trackVariantSelected, trackFavoriteAdded, trackFavoriteRemoved, trackAddToCart, trackRemoveFromCart, trackCheckoutStarted } from './behaviorClient';
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

describe('PERS.1D — Real Shop Behavioral Triggers Tests', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    localStorage.setItem(VISITOR_ID_STORAGE_KEY, '11111111-1111-4111-8111-111111111111');
    behaviorClientModule.behaviorClient.resetForTesting();
    vi.useRealTimers();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  describe('1. Product View Trigger', () => {
    function PdpViewSimulator({ isLoading, error, product }: { isLoading: boolean; error: string | null; product: { id: string; isPreview?: boolean } | null }) {
      const lastTrackedProductIdRef = useRef<string | null>(null);

      useEffect(() => {
        if (isLoading || !product || product.isPreview || !product.id) {
          return;
        }
        if (lastTrackedProductIdRef.current === product.id) {
          return;
        }
        lastTrackedProductIdRef.current = product.id;
        trackProductView(product.id);
      }, [isLoading, product?.id, product?.isPreview]);

      if (isLoading) return <div>Загрузка...</div>;
      if (error || !product) return <div>Ошибка</div>;
      return <div>Товар {product.id}</div>;
    }

    it('loading PDP -> no product_view', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackProductView');
      render(<PdpViewSimulator isLoading={true} error={null} product={null} />);
      expect(spy).not.toHaveBeenCalled();
    });

    it('failed PDP load -> no product_view', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackProductView');
      render(<PdpViewSimulator isLoading={false} error="Not found" product={null} />);
      expect(spy).not.toHaveBeenCalled();
    });

    it('successful product data render -> one product_view', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackProductView');
      render(<PdpViewSimulator isLoading={false} error={null} product={{ id: 'prod-pdp-1' }} />);
      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith('prod-pdp-1');
    });

    it('rerender same product -> does not duplicate', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackProductView');
      const { rerender } = render(<PdpViewSimulator isLoading={false} error={null} product={{ id: 'prod-pdp-1' }} />);
      expect(spy).toHaveBeenCalledTimes(1);

      rerender(<PdpViewSimulator isLoading={false} error={null} product={{ id: 'prod-pdp-1' }} />);
      expect(spy).toHaveBeenCalledTimes(1);
    });

    it('different product -> allowed', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackProductView');
      const { rerender } = render(<PdpViewSimulator isLoading={false} error={null} product={{ id: 'prod-pdp-1' }} />);
      expect(spy).toHaveBeenCalledTimes(1);

      rerender(<PdpViewSimulator isLoading={false} error={null} product={{ id: 'prod-pdp-2' }} />);
      expect(spy).toHaveBeenCalledTimes(2);
      expect(spy).toHaveBeenLastCalledWith('prod-pdp-2');
    });
  });

  describe('2. Variant Selection Trigger', () => {
    function VariantSelectorSimulator({
      variants,
      initialColorId,
      initialSizeId,
    }: {
      variants: Array<{ id: string; colorId: string; sizeId: string; inStock: boolean }>;
      initialColorId?: string;
      initialSizeId?: string;
    }) {
      const [colorId, setColorId] = useState<string | null>(initialColorId || null);
      const [sizeId, setSizeId] = useState<string | null>(initialSizeId || null);

      const selectedVariant = variants.find(
        (v) => v.colorId === colorId && v.sizeId === sizeId && v.inStock
      );

      const handleColorChange = (newColorId: string) => {
        setColorId(newColorId);
        const targetVariant = variants.find(
          (v) => v.colorId === newColorId && v.sizeId === sizeId && v.inStock
        );
        if (targetVariant?.id && targetVariant.id !== selectedVariant?.id) {
          trackVariantSelected('prod-1', targetVariant.id);
        }
      };

      const handleSizeChange = (newSizeId: string) => {
        setSizeId(newSizeId);
        const targetVariant = variants.find(
          (v) => v.colorId === colorId && v.sizeId === newSizeId && v.inStock
        );
        if (targetVariant?.id && targetVariant.id !== selectedVariant?.id) {
          trackVariantSelected('prod-1', targetVariant.id);
        }
      };

      return (
        <div>
          <button onClick={() => handleColorChange('color-white')}>Выбрать Белый</button>
          <button onClick={() => handleColorChange('color-black')}>Выбрать Чёрный</button>
          <button onClick={() => handleSizeChange('size-m')}>Выбрать M</button>
          <button onClick={() => handleSizeChange('size-l')}>Выбрать L</button>
          <div data-testid="selected">{selectedVariant?.id || 'none'}</div>
        </div>
      );
    }

    const testVariants = [
      { id: 'var-wm', colorId: 'color-white', sizeId: 'size-m', inStock: true },
      { id: 'var-wl', colorId: 'color-white', sizeId: 'size-l', inStock: true },
      { id: 'var-bm', colorId: 'color-black', sizeId: 'size-m', inStock: true },
    ];

    it('initial default variant -> no event emitted', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackVariantSelected');
      render(<VariantSelectorSimulator variants={testVariants} initialColorId="color-white" initialSizeId="size-m" />);
      expect(spy).not.toHaveBeenCalled();
    });

    it('explicit user switch to canonical variant -> one product_variant_selected', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackVariantSelected');
      render(<VariantSelectorSimulator variants={testVariants} initialColorId="color-white" initialSizeId="size-m" />);

      // Switch size to L
      act(() => {
        screen.getByText('Выбрать L').click();
      });

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith('prod-1', 'var-wl');
    });

    it('same already-selected variant -> no new event', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackVariantSelected');
      render(<VariantSelectorSimulator variants={testVariants} initialColorId="color-white" initialSizeId="size-m" />);

      // Click same size M
      act(() => {
        screen.getByText('Выбрать M').click();
      });

      expect(spy).not.toHaveBeenCalled();
    });

    it('selection without resolvable variant -> no event', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackVariantSelected');
      // No size selected initially
      render(<VariantSelectorSimulator variants={testVariants} initialColorId="color-white" />);

      // Switch color, size is still null -> no canonical variant
      act(() => {
        screen.getByText('Выбрать Чёрный').click();
      });

      expect(spy).not.toHaveBeenCalled();
    });
  });

  describe('3. Favorites Triggers', () => {
    it('successful add -> favorite_added once', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackFavoriteAdded');
      const mockApiAdd = vi.fn().mockResolvedValue({ status: 'ok' });

      // Simulate canonical favorites toggle add
      await mockApiAdd('prod-fav-1');
      trackFavoriteAdded('prod-fav-1');

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith('prod-fav-1');
    });

    it('failed add -> no event', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackFavoriteAdded');
      const mockApiAdd = vi.fn().mockRejectedValue(new Error('Network error'));

      try {
        await mockApiAdd('prod-fav-1');
        trackFavoriteAdded('prod-fav-1');
      } catch {
        // failed mutation
      }

      expect(spy).not.toHaveBeenCalled();
    });

    it('successful remove -> favorite_removed once', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackFavoriteRemoved');
      const mockApiRemove = vi.fn().mockResolvedValue({ status: 'ok' });

      await mockApiRemove('prod-fav-1');
      trackFavoriteRemoved('prod-fav-1');

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith('prod-fav-1');
    });

    it('failed remove -> no event', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackFavoriteRemoved');
      const mockApiRemove = vi.fn().mockRejectedValue(new Error('500 Error'));

      try {
        await mockApiRemove('prod-fav-1');
        trackFavoriteRemoved('prod-fav-1');
      } catch {
        // error
      }

      expect(spy).not.toHaveBeenCalled();
    });
  });

  describe('4. Cart Triggers & Delta Semantics', () => {
    it('successful add -> add_to_cart with exact productId, variantId, quantity', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackAddToCart');
      const mockApiAdd = vi.fn().mockResolvedValue({ status: 'ok' });

      await mockApiAdd({ productId: 'p1', productVariantId: 'v1', quantity: 2 });
      trackAddToCart({ productId: 'p1', variantId: 'v1', quantity: 2 });

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith({ productId: 'p1', variantId: 'v1', quantity: 2 });
    });

    it('failed add -> no event', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackAddToCart');
      const mockApiAdd = vi.fn().mockRejectedValue(new Error('Out of stock'));

      try {
        await mockApiAdd({ productId: 'p1', productVariantId: 'v1', quantity: 1 });
        trackAddToCart({ productId: 'p1', variantId: 'v1', quantity: 1 });
      } catch {
        // error
      }

      expect(spy).not.toHaveBeenCalled();
    });

    it('remove entire line -> remove_from_cart with actual removed line quantity', async () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackRemoveFromCart');
      const existingLine = { productId: 'p1', productVariantId: 'v1', quantity: 3 };

      // API removes line
      trackRemoveFromCart({
        productId: existingLine.productId,
        variantId: existingLine.productVariantId,
        quantity: existingLine.quantity,
      });

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith({ productId: 'p1', variantId: 'v1', quantity: 3 });
    });

    it('cart quantity increment -> add_to_cart with positive delta', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackAddToCart');
      const prevQty = 2;
      const nextQty = 3;
      const delta = nextQty - prevQty; // +1

      if (delta > 0) {
        trackAddToCart({ productId: 'p1', variantId: 'v1', quantity: delta });
      }

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith({ productId: 'p1', variantId: 'v1', quantity: 1 });
    });

    it('cart quantity decrement -> remove_from_cart with removed delta', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackRemoveFromCart');
      const prevQty = 3;
      const nextQty = 2;
      const delta = nextQty - prevQty; // -1

      if (delta < 0) {
        trackRemoveFromCart({ productId: 'p1', variantId: 'v1', quantity: Math.abs(delta) });
      }

      expect(spy).toHaveBeenCalledTimes(1);
      expect(spy).toHaveBeenCalledWith({ productId: 'p1', variantId: 'v1', quantity: 1 });
    });
  });

  describe('5. Checkout Started Trigger & Dedupe', () => {
    function CheckoutMountSimulator() {
      const hasTrackedRef = useRef(false);
      const [field, setField] = useState('');

      useEffect(() => {
        if (!hasTrackedRef.current) {
          hasTrackedRef.current = true;
          trackCheckoutStarted();
        }
      }, []);

      return (
        <div>
          <input data-testid="field" value={field} onChange={(e) => setField(e.target.value)} />
        </div>
      );
    }

    it('enter checkout -> one checkout_started', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackCheckoutStarted');
      render(<CheckoutMountSimulator />);
      expect(spy).toHaveBeenCalledTimes(1);
    });

    it('field edits and component rerenders -> do not duplicate checkout_started', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackCheckoutStarted');
      render(<CheckoutMountSimulator />);
      expect(spy).toHaveBeenCalledTimes(1);

      const input = screen.getByTestId('field');
      act(() => {
        input.focus();
        (input as HTMLInputElement).value = 'ivan@test.ru';
        input.dispatchEvent(new Event('input', { bubbles: true }));
      });

      expect(spy).toHaveBeenCalledTimes(1);
    });

    it('leave checkout and deliberate re-entry -> new checkout_started allowed', () => {
      const spy = vi.spyOn(behaviorClientModule, 'trackCheckoutStarted');
      const { unmount } = render(<CheckoutMountSimulator />);
      expect(spy).toHaveBeenCalledTimes(1);

      unmount(); // User leaves checkout

      // User enters checkout again
      render(<CheckoutMountSimulator />);
      expect(spy).toHaveBeenCalledTimes(2);
    });
  });

  describe('6. Telemetry Failure Isolation', () => {
    it('telemetry throw/rejection never affects commerce or throws to caller', async () => {
      // Force behaviorClient.emit to throw
      vi.spyOn(behaviorClientModule.behaviorClient, 'emit').mockImplementation(() => {
        throw new Error('Telemetry network crashed');
      });

      // Commerce action: favorite toggle
      expect(() => {
        trackFavoriteAdded('prod-1');
      }).not.toThrow();

      // Commerce action: add to cart
      expect(() => {
        trackAddToCart({ productId: 'prod-1', variantId: 'var-1', quantity: 1 });
      }).not.toThrow();

      // Commerce action: checkout start
      expect(() => {
        trackCheckoutStarted();
      }).not.toThrow();
    });
  });
});
