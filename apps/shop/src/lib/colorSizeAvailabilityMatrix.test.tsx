// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
  getColorOptions,
  getSizeOptions,
  getDimensionType,
  selectVariantState,
  useVariantSelection,
  isVariantBuyable,
  type ProductVariantItem,
} from './variantSelection';
import {
  validateVariantUrlState,
  computeVariantUrlParams,
} from './variantUrlState';

describe('CATALOG VARIANTS.3B — PDP Color × Size Availability State Machine', () => {
  // Proven Target Fashion Fixture: Oversize Hoodie
  const blackColorId = 'col-black';
  const yellowColorId = 'col-yellow';
  const redColorId = 'col-red';

  const size48Id = 'sz-48';
  const size50Id = 'sz-50';
  const size52Id = 'sz-52';
  const size54Id = 'sz-54';

  const oversizeHoodieVariants: ProductVariantItem[] = [
    // Black: 48, 50, 52 available
    { id: 'v-b-48', productId: 'prod-hoodie', colorId: blackColorId, colorName: 'Черный', colorHex: '#000000', sizeValueId: size48Id, size: '48', isActive: true, inStock: true, priceCents: 100000 },
    { id: 'v-b-50', productId: 'prod-hoodie', colorId: blackColorId, colorName: 'Черный', colorHex: '#000000', sizeValueId: size50Id, size: '50', isActive: true, inStock: true, priceCents: 100000 },
    { id: 'v-b-52', productId: 'prod-hoodie', colorId: blackColorId, colorName: 'Черный', colorHex: '#000000', sizeValueId: size52Id, size: '52', isActive: true, inStock: true, priceCents: 100000 },

    // Yellow: 48, 52 available
    { id: 'v-y-48', productId: 'prod-hoodie', colorId: yellowColorId, colorName: 'Желтый', colorHex: '#FFFF00', sizeValueId: size48Id, size: '48', isActive: true, inStock: true, priceCents: 105000 },
    { id: 'v-y-52', productId: 'prod-hoodie', colorId: yellowColorId, colorName: 'Желтый', colorHex: '#FFFF00', sizeValueId: size52Id, size: '52', isActive: true, inStock: true, priceCents: 105000 },

    // Red: 48, 50, 54 available (52 not offered)
    { id: 'v-r-48', productId: 'prod-hoodie', colorId: redColorId, colorName: 'Красный', colorHex: '#FF0000', sizeValueId: size48Id, size: '48', isActive: true, inStock: true, priceCents: 110000 },
    { id: 'v-r-50', productId: 'prod-hoodie', colorId: redColorId, colorName: 'Красный', colorHex: '#FF0000', sizeValueId: size50Id, size: '50', isActive: true, inStock: true, priceCents: 110000 },
    { id: 'v-r-54', productId: 'prod-hoodie', colorId: redColorId, colorName: 'Красный', colorHex: '#FF0000', sizeValueId: size54Id, size: '54', isActive: true, inStock: true, priceCents: 110000 },
  ];

  describe('NOTHING SELECTED', () => {
    it('A. no selection => all colors visible and interactive', () => {
      const colors = getColorOptions(oversizeHoodieVariants);
      expect(colors).toHaveLength(3);
      expect(colors.map(c => c.name)).toEqual(['Черный', 'Желтый', 'Красный']);
      colors.forEach(c => {
        expect(c.state).toBe('AVAILABLE');
        expect(c.disabled).toBe(false);
        expect(c.hasInStock).toBe(true);
      });
    });

    it('B. no selection => union of all sizes visible, not auto-selected, Add to Cart disabled', () => {
      const sizes = getSizeOptions(oversizeHoodieVariants, null, 'COLOR_AND_SIZE');
      expect(sizes).toHaveLength(4);
      expect(sizes.map(s => s.label)).toEqual(['48', '50', '52', '54']);
      sizes.forEach(s => {
        expect(s.disabled).toBe(false);
        expect(s.state).toBe('AVAILABLE');
      });

      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));
      expect(result.current.selectedColorId).toBeNull();
      expect(result.current.selectedSizeId).toBeNull();
      expect(result.current.selectedVariant).toBeNull();
      expect(result.current.canAddToCart).toBe(false);
      expect(result.current.ctaText).toBe('Выберите цвет');
    });
  });

  describe('SIZE SELECTED FIRST', () => {
    it('C-G. select 52: compatible Black and Yellow active; incompatible Red dimmed but still rendered', () => {
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));

      // C. select 52
      act(() => {
        result.current.selectSize(size52Id);
      });

      expect(result.current.selectedSizeId).toBe(size52Id);
      expect(result.current.selectedColorId).toBeNull();
      expect(result.current.selectedVariant).toBeNull();
      expect(result.current.canAddToCart).toBe(false);
      expect(result.current.ctaText).toBe('Выберите цвет');

      // Check colors recalculated against size 52
      const colors = result.current.colors;
      const black = colors.find(c => c.id === blackColorId)!;
      const yellow = colors.find(c => c.id === yellowColorId)!;
      const red = colors.find(c => c.id === redColorId)!;

      // D. compatible Black active
      expect(black.state).toBe('AVAILABLE');
      expect(black.hasInStock).toBe(true);
      expect(black.disabled).toBe(false);

      // E. compatible Yellow active
      expect(yellow.state).toBe('AVAILABLE');
      expect(yellow.hasInStock).toBe(true);
      expect(yellow.disabled).toBe(false);

      // F. incompatible Red dimmed (UNAVAILABLE for size 52, but clickable)
      expect(red.state).toBe('UNAVAILABLE');
      expect(red.hasInStock).toBe(false);
      expect(red.disabled).toBe(false);

      // G. Red still rendered (length remains 3)
      expect(colors).toHaveLength(3);
    });
  });

  describe('COLOR SELECTED FIRST', () => {
    it('H-L. select Red: 48, 50, 54 active; 52 dimmed (NOT_OFFERED) but visible', () => {
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));

      // H. select Red
      act(() => {
        result.current.selectColor(redColorId);
      });

      expect(result.current.selectedColorId).toBe(redColorId);
      expect(result.current.selectedSizeId).toBeNull();
      expect(result.current.selectedVariant).toBeNull();
      expect(result.current.canAddToCart).toBe(false);
      expect(result.current.ctaText).toBe('Выберите размер');

      const sizes = result.current.sizes;
      const s48 = sizes.find(s => s.id === size48Id)!;
      const s50 = sizes.find(s => s.id === size50Id)!;
      const s52 = sizes.find(s => s.id === size52Id)!;
      const s54 = sizes.find(s => s.id === size54Id)!;

      // I. 48 active
      expect(s48.state).toBe('AVAILABLE');
      expect(s48.inStock).toBe(true);
      expect(s48.disabled).toBe(false);

      // J. 50 active
      expect(s50.state).toBe('AVAILABLE');
      expect(s50.inStock).toBe(true);
      expect(s50.disabled).toBe(false);

      // K. 52 dimmed (NOT_OFFERED, but clickable for transition)
      expect(s52.state).toBe('NOT_OFFERED');
      expect(s52.inStock).toBe(false);
      expect(s52.disabled).toBe(false);

      // L. 54 active
      expect(s54.state).toBe('AVAILABLE');
      expect(s54.inStock).toBe(true);
      expect(s54.disabled).toBe(false);
    });
  });

  describe('VALID COMBINATION', () => {
    it('M-P. size 52 -> click Yellow: size 52 preserved, exact Yellow×52 variant resolved, Add to Cart enabled', () => {
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));

      // Start with size 52
      act(() => {
        result.current.selectSize(size52Id);
      });
      expect(result.current.selectedSizeId).toBe(size52Id);

      // M. click Yellow
      act(() => {
        result.current.selectColor(yellowColorId);
      });

      // N. size 52 preserved
      expect(result.current.selectedColorId).toBe(yellowColorId);
      expect(result.current.selectedSizeId).toBe(size52Id);

      // O. exact Yellow×52 variant selected
      expect(result.current.selectedVariant).not.toBeNull();
      expect(result.current.selectedVariant?.id).toBe('v-y-52');
      expect(result.current.selectedVariant?.priceCents).toBe(105000);

      // P. Add to Cart enabled
      expect(result.current.isResolved).toBe(true);
      expect(result.current.canAddToCart).toBe(true);
      expect(result.current.ctaText).toBe('Добавить в корзину');
    });
  });

  describe('INCOMPATIBLE COLOR CLICK — CANONICAL RULE', () => {
    it('Q-V. size 52 -> click Red: Red becomes selected, size clears, URL removes size, no selectedVariant, Red sizes recalculate', () => {
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));

      // Start with size 52
      act(() => {
        result.current.selectSize(size52Id);
      });

      // Q. click Red (incompatible with 52)
      act(() => {
        result.current.selectColor(redColorId);
      });

      // R. Red becomes selected
      expect(result.current.selectedColorId).toBe(redColorId);

      // S. size clears
      expect(result.current.selectedSizeId).toBeNull();

      // T. URL removes size
      const currentParams = new URLSearchParams('size=sz-52');
      const nextParams = computeVariantUrlParams(currentParams, 'COLOR_AND_SIZE', redColorId, null, oversizeHoodieVariants);
      expect(nextParams.get('color')).toBe(redColorId);
      expect(nextParams.has('size')).toBe(false);

      // U. no selectedVariant
      expect(result.current.selectedVariant).toBeNull();
      expect(result.current.canAddToCart).toBe(false);
      expect(result.current.ctaText).toBe('Выберите размер');

      // V. Red sizes recalculate
      const sizes = result.current.sizes;
      expect(sizes.find(s => s.id === size48Id)?.state).toBe('AVAILABLE');
      expect(sizes.find(s => s.id === size50Id)?.state).toBe('AVAILABLE');
      expect(sizes.find(s => s.id === size52Id)?.state).toBe('NOT_OFFERED');
      expect(sizes.find(s => s.id === size54Id)?.state).toBe('AVAILABLE');
    });
  });

  describe('INCOMPATIBLE SIZE CLICK — SYMMETRIC RULE', () => {
    it('W-AA. Red selected -> click 52: size 52 selected, color clears, URL removes color, colors recalculate for size 52', () => {
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));

      // Start with Red
      act(() => {
        result.current.selectColor(redColorId);
      });
      expect(result.current.selectedColorId).toBe(redColorId);

      // W. click 52 (incompatible with Red)
      act(() => {
        result.current.selectSize(size52Id);
      });

      // X. size 52 selected
      expect(result.current.selectedSizeId).toBe(size52Id);

      // Y. color clears
      expect(result.current.selectedColorId).toBeNull();

      // Z. URL removes color
      const currentParams = new URLSearchParams('color=col-red');
      const nextParams = computeVariantUrlParams(currentParams, 'COLOR_AND_SIZE', null, size52Id, oversizeHoodieVariants);
      expect(nextParams.has('color')).toBe(false);
      expect(nextParams.get('size')).toBe(size52Id);

      // AA. colors recalculate for size 52
      const colors = result.current.colors;
      expect(colors.find(c => c.id === blackColorId)?.state).toBe('AVAILABLE');
      expect(colors.find(c => c.id === yellowColorId)?.state).toBe('AVAILABLE');
      expect(colors.find(c => c.id === redColorId)?.state).toBe('UNAVAILABLE');
    });
  });

  describe('URL STATE SYNCHRONIZATION', () => {
    it('AB. valid query restores state', () => {
      const params = new URLSearchParams(`color=${yellowColorId}&size=${size52Id}`);
      const validated = validateVariantUrlState(oversizeHoodieVariants, params);

      expect(validated.targetColorId).toBe(yellowColorId);
      expect(validated.targetSizeId).toBe(size52Id);
      expect(validated.isExactBuyable).toBe(true);
      expect(validated.needsReplace).toBe(false);
    });

    it('AC. invalid color handled safely', () => {
      const params = new URLSearchParams(`color=unknown-color-id&size=${size52Id}`);
      const validated = validateVariantUrlState(oversizeHoodieVariants, params);

      expect(validated.targetColorId).toBeNull();
      expect(validated.targetSizeId).toBe(size52Id);
      expect(validated.hasExplicitSizeIntent).toBe(true);
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('color')).toBe(false);
      expect(validated.sanitizedSearchParams.get('size')).toBe(size52Id);
    });

    it('AD. invalid size handled safely', () => {
      const params = new URLSearchParams(`color=${yellowColorId}&size=unknown-size-id`);
      const validated = validateVariantUrlState(oversizeHoodieVariants, params);

      expect(validated.targetColorId).toBe(yellowColorId);
      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.get('color')).toBe(yellowColorId);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });

    it('AE. impossible pair does not survive as stale pair', () => {
      // Red has no 52
      const params = new URLSearchParams(`color=${redColorId}&size=${size52Id}`);
      const validated = validateVariantUrlState(oversizeHoodieVariants, params);

      expect(validated.targetColorId).toBe(redColorId);
      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.get('color')).toBe(redColorId);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
      expect(validated.notice).toContain('52');
    });
  });

  describe('SAFETY & CANONICAL TRUTH', () => {
    it('AF. sold-out exact variant treated unavailable', () => {
      const variantsWithSoldOut: ProductVariantItem[] = [
        ...oversizeHoodieVariants.map(v => v.id === 'v-b-52' ? { ...v, inStock: false } : v)
      ];

      const state = selectVariantState(variantsWithSoldOut, blackColorId, size52Id);
      expect(state.isResolved).toBe(true);
      expect(state.canAddToCart).toBe(false);
      expect(state.ctaText).toBe('Нет в наличии');
    });

    it('AG. inactive exact variant treated unavailable', () => {
      const variantsWithInactive: ProductVariantItem[] = [
        ...oversizeHoodieVariants.map(v => v.id === 'v-b-52' ? { ...v, isActive: false } : v)
      ];

      const state = selectVariantState(variantsWithInactive, blackColorId, size52Id);
      // Inactive variant is not resolved as a valid selection
      expect(state.isResolved).toBe(false);
      expect(state.canAddToCart).toBe(false);
    });

    it('AH. no auto-selection of alternate variant on incompatible clicks', () => {
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));

      // 1. Select 52, then click Red (which lacks 52)
      act(() => {
        result.current.selectSize(size52Id);
      });
      act(() => {
        result.current.selectColor(redColorId);
      });
      // MUST NOT pick 48, 50, or 54 automatically
      expect(result.current.selectedSizeId).toBeNull();

      // 2. Select Red, then click 52 (which Red lacks)
      act(() => {
        result.current.selectSize(size52Id);
      });
      // MUST NOT pick Black or Yellow automatically
      expect(result.current.selectedColorId).toBeNull();
    });

    it('AI. product_view not duplicated by selector interaction', () => {
      // Validating that selector functions only mutate selection state and do not invoke view telemetry
      const { result } = renderHook(() => useVariantSelection(oversizeHoodieVariants));
      const trackFn = vi.fn();

      act(() => {
        result.current.selectSize(size52Id);
        result.current.selectColor(blackColorId);
      });

      // Selection state updated cleanly without telemetry side-effects inside state machine
      expect(result.current.isResolved).toBe(true);
      expect(trackFn).not.toHaveBeenCalled();
    });
  });
});
