import { describe, it, expect } from 'vitest';
import {
  parseVariantUrlParams,
  validateVariantUrlState,
  computeVariantUrlParams,
  areSearchParamsEqual,
} from './variantUrlState';
import type { ProductVariantItem } from './variantSelection';

describe('SHOP PDP.2D2 — variantUrlState Unit Tests', () => {
  const whiteColorId = 'c-white-uuid';
  const blackColorId = 'c-black-uuid';
  const sizeMId = 's-m-uuid';
  const sizeLId = 's-l-uuid';
  const sizeXLId = 's-xl-uuid';

  const sampleVariants: ProductVariantItem[] = [
    {
      id: 'var-white-m',
      colorId: whiteColorId,
      colorName: 'Белый',
      sizeValueId: sizeMId,
      size: 'M',
      priceCents: 10000,
      inStock: true,
      isActive: true,
    },
    {
      id: 'var-white-l',
      colorId: whiteColorId,
      colorName: 'Белый',
      sizeValueId: sizeLId,
      size: 'L',
      priceCents: 10000,
      inStock: false, // SOLD OUT
      isActive: true,
    },
    // White does NOT offer XL (NOT_OFFERED)
    {
      id: 'var-black-m',
      colorId: blackColorId,
      colorName: 'Чёрный',
      sizeValueId: sizeMId,
      size: 'M',
      priceCents: 10000,
      inStock: true,
      isActive: true,
    },
    {
      id: 'var-black-xl',
      colorId: blackColorId,
      colorName: 'Чёрный',
      sizeValueId: sizeXLId,
      size: 'XL',
      priceCents: 11000,
      inStock: true,
      isActive: true,
    },
  ];

  describe('1. Clean URL Parsing & Validation', () => {
    it('returns nulls and requires no replace for clean URL', () => {
      const params = new URLSearchParams('');
      const parsed = parseVariantUrlParams(params);
      expect(parsed.colorParam).toBeNull();
      expect(parsed.sizeParam).toBeNull();

      const validated = validateVariantUrlState(sampleVariants, params);
      expect(validated.dimensionType).toBe('COLOR_AND_SIZE');
      expect(validated.targetColorId).toBeNull();
      expect(validated.targetSizeId).toBeNull();
      expect(validated.hasExplicitColorIntent).toBe(false);
      expect(validated.hasExplicitSizeIntent).toBe(false);
      expect(validated.needsReplace).toBe(false);
      expect(validated.sanitizedSearchParams.toString()).toBe('');
    });

    it('preserves unrelated query parameters on clean URL', () => {
      const params = new URLSearchParams('utm_source=telegram&ref=sale');
      const validated = validateVariantUrlState(sampleVariants, params);
      expect(validated.needsReplace).toBe(false);
      expect(validated.sanitizedSearchParams.get('utm_source')).toBe('telegram');
      expect(validated.sanitizedSearchParams.get('ref')).toBe('sale');
    });
  });

  describe('2. COLOR_AND_SIZE Product URL Scenarios', () => {
    it('restores valid color and buyable size', () => {
      const params = new URLSearchParams(`color=${whiteColorId}&size=${sizeMId}`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBe(whiteColorId);
      expect(validated.targetSizeId).toBe(sizeMId);
      expect(validated.hasExplicitColorIntent).toBe(true);
      expect(validated.hasExplicitSizeIntent).toBe(true);
      expect(validated.isExactBuyable).toBe(true);
      expect(validated.needsReplace).toBe(false);
    });

    it('restores valid color only (no size in URL)', () => {
      const params = new URLSearchParams(`color=${whiteColorId}`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBe(whiteColorId);
      expect(validated.targetSizeId).toBeNull();
      expect(validated.hasExplicitColorIntent).toBe(true);
      expect(validated.hasExplicitSizeIntent).toBe(false);
      expect(validated.needsReplace).toBe(false);
    });

    it('sanitizes sold-out size in URL: keeps color, removes size, sets notice', () => {
      // White + L exists but is inStock = false
      const params = new URLSearchParams(`color=${whiteColorId}&size=${sizeLId}`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBe(whiteColorId);
      expect(validated.targetSizeId).toBeNull(); // Do NOT select sold-out size
      expect(validated.hasExplicitColorIntent).toBe(true);
      expect(validated.hasExplicitSizeIntent).toBe(false);
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.get('color')).toBe(whiteColorId);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
      expect(validated.notice).toContain('Размер L');
    });

    it('sanitizes NOT_OFFERED size in URL: keeps color, removes size', () => {
      // White + XL does NOT exist in sampleVariants
      const params = new URLSearchParams(`color=${whiteColorId}&size=${sizeXLId}`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBe(whiteColorId);
      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.get('color')).toBe(whiteColorId);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });

    it('sanitizes completely invalid size UUID: keeps color, removes size', () => {
      const params = new URLSearchParams(`color=${whiteColorId}&size=non-existent-size-uuid`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBe(whiteColorId);
      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.get('color')).toBe(whiteColorId);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });

    it('sanitizes invalid color UUID: removes color and size params', () => {
      const params = new URLSearchParams(`color=unknown-color-uuid&size=${sizeMId}`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBeNull();
      expect(validated.targetSizeId).toBeNull();
      expect(validated.hasExplicitColorIntent).toBe(false);
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('color')).toBe(false);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });

    it('sanitizes size-only URL on COLOR_AND_SIZE product without guessing color', () => {
      // Standalone size param must NOT guess color
      const params = new URLSearchParams(`size=${sizeMId}`);
      const validated = validateVariantUrlState(sampleVariants, params);

      expect(validated.targetColorId).toBeNull();
      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });
  });

  describe('3. SIZE_ONLY Product URL Scenarios', () => {
    const sizeOnlyVariants: ProductVariantItem[] = [
      {
        id: 'var-s1',
        sizeValueId: sizeMId,
        size: 'M',
        priceCents: 5000,
        inStock: true,
        isActive: true,
      },
      {
        id: 'var-s2',
        sizeValueId: sizeLId,
        size: 'L',
        priceCents: 5000,
        inStock: false,
        isActive: true,
      },
    ];

    it('restores buyable size and strips spurious color param', () => {
      const params = new URLSearchParams(`color=some-color&size=${sizeMId}`);
      const validated = validateVariantUrlState(sizeOnlyVariants, params);

      expect(validated.dimensionType).toBe('SIZE_ONLY');
      expect(validated.targetSizeId).toBe(sizeMId);
      expect(validated.hasExplicitSizeIntent).toBe(true);
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('color')).toBe(false);
      expect(validated.sanitizedSearchParams.get('size')).toBe(sizeMId);
    });

    it('sanitizes sold-out size for SIZE_ONLY product', () => {
      const params = new URLSearchParams(`size=${sizeLId}`);
      const validated = validateVariantUrlState(sizeOnlyVariants, params);

      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });
  });

  describe('4. COLOR_ONLY Product URL Scenarios', () => {
    const colorOnlyVariants: ProductVariantItem[] = [
      {
        id: 'var-c1',
        colorId: whiteColorId,
        colorName: 'Белый',
        priceCents: 7000,
        inStock: true,
        isActive: true,
      },
    ];

    it('restores valid color and strips spurious size param', () => {
      const params = new URLSearchParams(`color=${whiteColorId}&size=${sizeMId}`);
      const validated = validateVariantUrlState(colorOnlyVariants, params);

      expect(validated.dimensionType).toBe('COLOR_ONLY');
      expect(validated.targetColorId).toBe(whiteColorId);
      expect(validated.hasExplicitColorIntent).toBe(true);
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
      expect(validated.sanitizedSearchParams.get('color')).toBe(whiteColorId);
    });
  });

  describe('5. SINGLE_VARIANT Product URL Scenarios', () => {
    const singleVariants: ProductVariantItem[] = [
      {
        id: 'var-single',
        priceCents: 3000,
        inStock: true,
        isActive: true,
      },
    ];

    it('strips color and size params for SINGLE_VARIANT product', () => {
      const params = new URLSearchParams(`color=${whiteColorId}&size=${sizeMId}`);
      const validated = validateVariantUrlState(singleVariants, params);

      expect(validated.dimensionType).toBe('SINGLE_VARIANT');
      expect(validated.targetColorId).toBeNull();
      expect(validated.targetSizeId).toBeNull();
      expect(validated.needsReplace).toBe(true);
      expect(validated.sanitizedSearchParams.has('color')).toBe(false);
      expect(validated.sanitizedSearchParams.has('size')).toBe(false);
    });
  });

  describe('6. computeVariantUrlParams', () => {
    it('computes color and size for COLOR_AND_SIZE', () => {
      const initial = new URLSearchParams('ref=123');
      const next = computeVariantUrlParams(initial, 'COLOR_AND_SIZE', whiteColorId, sizeMId);
      expect(next.get('color')).toBe(whiteColorId);
      expect(next.get('size')).toBe(sizeMId);
      expect(next.get('ref')).toBe('123');
    });

    it('clears size when nextSizeId is null', () => {
      const initial = new URLSearchParams(`color=${blackColorId}&size=${sizeMId}`);
      const next = computeVariantUrlParams(initial, 'COLOR_AND_SIZE', whiteColorId, null);
      expect(next.get('color')).toBe(whiteColorId);
      expect(next.has('size')).toBe(false);
    });

    it('only sets size for SIZE_ONLY', () => {
      const initial = new URLSearchParams(`color=foo`);
      const next = computeVariantUrlParams(initial, 'SIZE_ONLY', null, sizeMId);
      expect(next.has('color')).toBe(false);
      expect(next.get('size')).toBe(sizeMId);
    });

    it('only sets color for COLOR_ONLY', () => {
      const initial = new URLSearchParams(`size=foo`);
      const next = computeVariantUrlParams(initial, 'COLOR_ONLY', whiteColorId, null);
      expect(next.get('color')).toBe(whiteColorId);
      expect(next.has('size')).toBe(false);
    });
  });

  describe('7. areSearchParamsEqual', () => {
    it('compares identical params', () => {
      const p1 = new URLSearchParams('color=c1&size=s1');
      const p2 = new URLSearchParams('color=c1&size=s1');
      expect(areSearchParamsEqual(p1, p2)).toBe(true);
    });

    it('detects different params', () => {
      const p1 = new URLSearchParams('color=c1&size=s1');
      const p2 = new URLSearchParams('color=c1&size=s2');
      expect(areSearchParamsEqual(p1, p2)).toBe(false);
    });
  });
});
