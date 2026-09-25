import { describe, it, expect } from 'vitest';
import {
  reconcileProductStudioVariantMatrix,
  computeRequiredVariantTuples,
  resolveProductStudioDimensionType,
  defaultDraftVariantIdFactory,
  canDeactivateVariantTuple,
  toggleProductStudioVariantTuple,
  BLOCKED_LAST_CELL_TOOLTIP,
  type CanonicalColorInput,
  type CanonicalSizeInput,
  type DraftVariantIdFactory,
} from './productStudioMatrixHelper';
import type { ProductStudioVariant } from '../../contexts/ProductStudioContext';

describe('productStudioMatrixHelper - reconcileProductStudioVariantMatrix', () => {
  const white: CanonicalColorInput = { id: 'col-white', name: 'White', hex: '#FFFFFF' };
  const beige: CanonicalColorInput = { id: 'col-beige', name: 'Beige', hex: '#F5F5DC' };
  const black: CanonicalColorInput = { id: 'col-black', name: 'Black', hex: '#000000' };
  const sizeM: CanonicalSizeInput = { id: 'sz-m', label: 'M' };
  const sizeL: CanonicalSizeInput = { id: 'sz-l', label: 'L' };

  it('A. COLOR_AND_SIZE: White, Beige, Black × M => exactly 3 tuples', () => {
    const colors = [white, beige, black];
    const sizes = [sizeM];
    const result = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', colors, sizes, []);

    expect(result).toHaveLength(3);
    expect(result.map((v) => `${v.colorId}x${v.sizeValueId}`)).toEqual([
      'col-whitexsz-m',
      'col-beigexsz-m',
      'col-blackxsz-m',
    ]);
  });

  it('B. same input twice with deterministic ID factory => same semantic result', () => {
    const colors = [white, black];
    const sizes = [sizeM];
    let counter1 = 0;
    const factory1: DraftVariantIdFactory = (t) => `det-var-${t.colorId}-${t.sizeValueId}-${++counter1}`;
    let counter2 = 0;
    const factory2: DraftVariantIdFactory = (t) => `det-var-${t.colorId}-${t.sizeValueId}-${++counter2}`;

    const run1 = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', colors, sizes, [], undefined, undefined, factory1);
    const run2 = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', colors, sizes, [], undefined, undefined, factory2);

    expect(run1).toEqual(run2);
    // Also verify default factory is deterministic
    const def1 = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', colors, sizes, []);
    const def2 = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', colors, sizes, []);
    expect(def1).toEqual(def2);
  });

  it('C. existing canonical active Black/M => preserve exact canonical ID and owned fields', () => {
    const canonicalBlackM: ProductStudioVariant = {
      id: '00000000-0000-0000-0000-000000000001',
      colorId: black.id,
      colorName: black.name,
      colorHex: black.hex,
      sizeValueId: sizeM.id,
      size: sizeM.label,
      sellerSku: 'SKU-BLACK-M',
      barcode: '1234567890123',
      priceCents: 499000,
      isActive: true,
    };

    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black],
      [sizeM],
      [canonicalBlackM]
    );

    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('00000000-0000-0000-0000-000000000001');
    expect(result[0].sellerSku).toBe('SKU-BLACK-M');
    expect(result[0].barcode).toBe('1234567890123');
    expect(result[0].priceCents).toBe(499000);
    expect(result[0].isActive).toBe(true);
  });

  it('D. missing White/M => create exactly one new draft variant', () => {
    const canonicalBlackM: ProductStudioVariant = {
      id: '00000000-0000-0000-0000-000000000001',
      colorId: black.id,
      sizeValueId: sizeM.id,
      isActive: true,
    };

    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black, white],
      [sizeM],
      [canonicalBlackM]
    );

    expect(result).toHaveLength(2);
    const blackM = result.find((v) => v.colorId === black.id);
    const whiteM = result.find((v) => v.colorId === white.id);

    expect(blackM?.id).toBe('00000000-0000-0000-0000-000000000001');
    expect(whiteM).toBeDefined();
    expect(whiteM?.id).toBe(defaultDraftVariantIdFactory({ colorId: white.id, sizeValueId: sizeM.id }, 1));
    expect(whiteM?.isActive).toBe(true);
  });

  it('E. inactive old White/M => DO NOT reuse inactive ID => create new draft identity', () => {
    const oldInactiveWhiteM: ProductStudioVariant = {
      id: '00000000-0000-0000-0000-old-inactive-uuid',
      colorId: white.id,
      sizeValueId: sizeM.id,
      isActive: false, // Inactive historical variant!
    };

    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [white],
      [sizeM],
      [oldInactiveWhiteM]
    );

    expect(result).toHaveLength(1);
    expect(result[0].colorId).toBe(white.id);
    expect(result[0].sizeValueId).toBe(sizeM.id);
    // MUST NOT reuse old inactive ID!
    expect(result[0].id).not.toBe('00000000-0000-0000-0000-old-inactive-uuid');
    expect(result[0].id).toBe(defaultDraftVariantIdFactory({ colorId: white.id, sizeValueId: sizeM.id }, 0));
    expect(result[0].isActive).toBe(true);
  });

  it('F. remove Beige => Beige tuples removed from current draft', () => {
    const current: ProductStudioVariant[] = [
      { id: 'v-black-m', colorId: black.id, sizeValueId: sizeM.id, isActive: true },
      { id: 'v-beige-m', colorId: beige.id, sizeValueId: sizeM.id, isActive: true },
    ];

    // Seller only configures Black now (Beige removed)
    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black],
      [sizeM],
      current
    );

    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('v-black-m');
    expect(result.some((v) => v.colorId === beige.id)).toBe(false);
  });

  it('G. unknown dimension type => no Cartesian guessing (throws error)', () => {
    expect(() => {
      reconcileProductStudioVariantMatrix(
        'UNKNOWN_DIMENSION_TYPE',
        [white, black],
        [sizeM],
        []
      );
    }).toThrow('Unsupported dimension type: "UNKNOWN_DIMENSION_TYPE"');

    expect(() => {
      computeRequiredVariantTuples('RANDOM_CUSTOM_TYPE', [white], [sizeM]);
    }).toThrow('Unsupported dimension type: "RANDOM_CUSTOM_TYPE"');
  });

  it('H. COLOR_ONLY canonical behavior', () => {
    const colors = [white, black];
    const sizes = [sizeM, sizeL]; // Even if sizes passed, COLOR_ONLY produces color-only variants
    const result = reconcileProductStudioVariantMatrix('COLOR_ONLY', colors, sizes, []);

    expect(result).toHaveLength(2);
    expect(result.every((v) => v.sizeValueId === undefined)).toBe(true);
    expect(result.map((v) => v.colorId)).toEqual([white.id, black.id]);
  });

  it('I. SIZE_ONLY canonical behavior', () => {
    const colors = [white, black]; // Even if colors passed, SIZE_ONLY produces size-only variants
    const sizes = [sizeM, sizeL];
    const result = reconcileProductStudioVariantMatrix('SIZE_ONLY', colors, sizes, []);

    expect(result).toHaveLength(2);
    expect(result.every((v) => v.colorId === undefined)).toBe(true);
    expect(result.map((v) => v.sizeValueId)).toEqual([sizeM.id, sizeL.id]);
  });

  it('J. no-dimension canonical behavior (SINGLE_VARIANT)', () => {
    const result = reconcileProductStudioVariantMatrix('SINGLE_VARIANT', [white], [sizeM], []);

    expect(result).toHaveLength(1);
    expect(result[0].colorId).toBeUndefined();
    expect(result[0].sizeValueId).toBeUndefined();
    expect(result[0].isActive).toBe(true);
  });

  it('K. Sparse matrix preservation: disabled tuples remain disabled', () => {
    const current: ProductStudioVariant[] = [
      { id: 'v-white-m', colorId: white.id, sizeValueId: sizeM.id, isActive: true },
    ];

    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [white, black],
      [sizeM, sizeL],
      current,
      [white],
      [sizeM, sizeL]
    );

    expect(result).toHaveLength(3);
    const hasWhiteL = result.some(v => v.colorId === white.id && v.sizeValueId === sizeL.id);
    expect(hasWhiteL).toBe(false);
    const hasBlackL = result.some(v => v.colorId === black.id && v.sizeValueId === sizeL.id);
    expect(hasBlackL).toBe(true);
  });
});

describe('productStudioMatrixHelper - resolveProductStudioDimensionType', () => {
  it('1. returns canonical draft.dimensionType directly', () => {
    expect(resolveProductStudioDimensionType('COLOR_AND_SIZE')).toBe('COLOR_AND_SIZE');
    expect(resolveProductStudioDimensionType('COLOR_ONLY')).toBe('COLOR_ONLY');
    expect(resolveProductStudioDimensionType('SIZE_ONLY')).toBe('SIZE_ONLY');
    expect(resolveProductStudioDimensionType('SINGLE_VARIANT')).toBe('SINGLE_VARIANT');
  });

  it('2. falls back to canonical categorySchema.dimensionType when draft dimension is absent', () => {
    expect(resolveProductStudioDimensionType(undefined, 'COLOR_AND_SIZE')).toBe('COLOR_AND_SIZE');
    expect(resolveProductStudioDimensionType(null, 'COLOR_ONLY')).toBe('COLOR_ONLY');
    expect(resolveProductStudioDimensionType('', 'SIZE_ONLY')).toBe('SIZE_ONLY');
    expect(resolveProductStudioDimensionType(undefined, 'SINGLE_VARIANT')).toBe('SINGLE_VARIANT');
  });

  it('3. returns null when both draft and schema dimensions are absent', () => {
    expect(resolveProductStudioDimensionType(undefined, undefined)).toBeNull();
    expect(resolveProductStudioDimensionType(null, null)).toBeNull();
    expect(resolveProductStudioDimensionType('', '')).toBeNull();
  });

  it('4. fails closed (returns null) when draft.dimensionType is invalid/unknown (no schema fallback)', () => {
    expect(resolveProductStudioDimensionType('UNKNOWN', 'SIZE_ONLY')).toBeNull();
    expect(resolveProductStudioDimensionType('INVALID_TYPE', 'COLOR_AND_SIZE')).toBeNull();
  });

  it('5. fails closed (returns null) when schema dimension is invalid/unknown and draft is absent', () => {
    expect(resolveProductStudioDimensionType(undefined, 'UNKNOWN_SCHEMA_DIM')).toBeNull();
    expect(resolveProductStudioDimensionType(null, 'CUSTOM_DIM')).toBeNull();
  });
});

describe('productStudioMatrixHelper - Sparse Matrix Domain & Last-Cell Guard (PS.R4B3.1C4C3B3B-R2)', () => {
  const black: CanonicalColorInput = { id: 'col-black', name: 'Black', hex: '#000000' };
  const grey: CanonicalColorInput = { id: 'col-grey', name: 'Grey', hex: '#808080' };
  const white: CanonicalColorInput = { id: 'col-white', name: 'White', hex: '#FFFFFF' };
  const sizeM: CanonicalSizeInput = { id: 'sz-m', label: 'M' };
  const sizeL: CanonicalSizeInput = { id: 'sz-l', label: 'L' };

  // Section 11 Tests
  it('A. Initial dense default: Black, Grey × M, L => all 4 ON', () => {
    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black, grey],
      [sizeM, sizeL],
      []
    );

    expect(result).toHaveLength(4);
    expect(result.every((v) => v.isActive === true)).toBe(true);
    expect(result.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-l')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-m')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-l')).toBe(true);
  });

  it('B. Disable ordinary cell: turn Grey/M OFF via toggleProductStudioVariantTuple', () => {
    const initial = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black, grey],
      [sizeM, sizeL],
      []
    );

    const toggleRes = toggleProductStudioVariantTuple(
      initial,
      { colorId: 'col-grey', sizeValueId: 'sz-m' },
      true // currently active -> turn OFF
    );

    expect(toggleRes.allowed).toBe(true);
    expect(toggleRes.variants).toHaveLength(3);
    expect(toggleRes.variants.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-m')).toBe(false);
    expect(toggleRes.variants.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-l')).toBe(true);
    expect(toggleRes.variants.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m')).toBe(true);
    expect(toggleRes.variants.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-l')).toBe(true);
  });

  it('C. Preserve OFF when adding size: Black/M ON, Grey/M OFF, add L => Grey/M remains OFF, L tuples default ON', () => {
    // Starting state: Black/M ON, Grey/M OFF (only Black/M exists)
    const currentVariants: ProductStudioVariant[] = [
      { id: 'v-black-m', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M', isActive: true },
    ];
    const oldColors = [black, grey];
    const oldSizes = [sizeM];

    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black, grey],
      [sizeM, sizeL], // add L
      currentVariants,
      oldColors,
      oldSizes
    );

    // Expected: Black/M ON, Grey/M OFF (absent), Black/L ON, Grey/L ON
    expect(result).toHaveLength(3);
    expect(result.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-m')).toBe(false); // remains OFF
    expect(result.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-l')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-l')).toBe(true);
  });

  it('D. Preserve OFF when adding color: Black/M ON, Black/L OFF, add White => Black/L remains OFF, White tuples default ON', () => {
    // Starting state: Black/M ON, Black/L OFF
    const currentVariants: ProductStudioVariant[] = [
      { id: 'v-black-m', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M', isActive: true },
    ];
    const oldColors = [black];
    const oldSizes = [sizeM, sizeL];

    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black, white], // add White
      [sizeM, sizeL],
      currentVariants,
      oldColors,
      oldSizes
    );

    // Expected: Black/M ON, Black/L OFF (absent), White/M ON, White/L ON
    expect(result).toHaveLength(3);
    expect(result.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-l')).toBe(false); // remains OFF
    expect(result.some((v) => v.colorId === 'col-white' && v.sizeValueId === 'sz-m')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-white' && v.sizeValueId === 'sz-l')).toBe(true);
  });

  it('E. Remove size axis: removing L prunes all */L tuples and preserves other sparse state', () => {
    const currentVariants: ProductStudioVariant[] = [
      { id: 'v-black-m', colorId: 'col-black', sizeValueId: 'sz-m', isActive: true },
      { id: 'v-black-l', colorId: 'col-black', sizeValueId: 'sz-l', isActive: true },
      { id: 'v-grey-l', colorId: 'col-grey', sizeValueId: 'sz-l', isActive: true },
    ];
    const oldColors = [black, grey];
    const oldSizes = [sizeM, sizeL];

    // Remove L -> sizes now only [sizeM]
    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black, grey],
      [sizeM],
      currentVariants,
      oldColors,
      oldSizes
    );

    expect(result).toHaveLength(1);
    expect(result[0].colorId).toBe('col-black');
    expect(result[0].sizeValueId).toBe('sz-m');
    expect(result.some((v) => v.sizeValueId === 'sz-l')).toBe(false);
  });

  it('F. Remove color axis: removing Grey prunes all Grey/* tuples and preserves other sparse state', () => {
    const currentVariants: ProductStudioVariant[] = [
      { id: 'v-black-m', colorId: 'col-black', sizeValueId: 'sz-m', isActive: true },
      { id: 'v-black-l', colorId: 'col-black', sizeValueId: 'sz-l', isActive: true },
      { id: 'v-grey-l', colorId: 'col-grey', sizeValueId: 'sz-l', isActive: true },
    ];
    const oldColors = [black, grey];
    const oldSizes = [sizeM, sizeL];

    // Remove Grey -> colors now only [black]
    const result = reconcileProductStudioVariantMatrix(
      'COLOR_AND_SIZE',
      [black],
      [sizeM, sizeL],
      currentVariants,
      oldColors,
      oldSizes
    );

    expect(result).toHaveLength(2);
    expect(result.every((v) => v.colorId === 'col-black')).toBe(true);
    expect(result.some((v) => v.colorId === 'col-grey')).toBe(false);
  });

  // Section 12 Tests: Last-Cell Regressions
  it('CASE 1 — last cell of size column: Black/M ON, Black/XXL ON -> Black/XXL OFF blocked', () => {
    const variants: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M', isActive: true },
      { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-xxl', size: 'XXL', isActive: true },
    ];

    expect(canDeactivateVariantTuple(variants, 'col-black', 'sz-xxl')).toBe(false);

    const toggleRes = toggleProductStudioVariantTuple(
      variants,
      { colorId: 'col-black', sizeValueId: 'sz-xxl' },
      true
    );

    expect(toggleRes.allowed).toBe(false);
    expect(toggleRes.reason).toBe('LAST_CELL_IN_AXIS');
    expect(toggleRes.variants).toEqual(variants); // Unchanged
    expect(BLOCKED_LAST_CELL_TOOLTIP).toContain('Нельзя отключить последний вариант');
  });

  it('CASE 2 — last cell of color row: Black/M ON, Grey/M ON -> Grey/M OFF blocked', () => {
    const variants: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M', isActive: true },
      { id: 'v2', colorId: 'col-grey', sizeValueId: 'sz-m', size: 'M', isActive: true },
    ];

    expect(canDeactivateVariantTuple(variants, 'col-grey', 'sz-m')).toBe(false);

    const toggleRes = toggleProductStudioVariantTuple(
      variants,
      { colorId: 'col-grey', sizeValueId: 'sz-m' },
      true
    );

    expect(toggleRes.allowed).toBe(false);
    expect(toggleRes.reason).toBe('LAST_CELL_IN_AXIS');
    expect(toggleRes.variants).toEqual(variants); // Unchanged
  });

  it('CASE 3 — ordinary OFF remains allowed when both axes retain >= 1 active variant', () => {
    const variants: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-m', isActive: true },
      { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-l', isActive: true },
      { id: 'v3', colorId: 'col-grey', sizeValueId: 'sz-m', isActive: true },
      { id: 'v4', colorId: 'col-grey', sizeValueId: 'sz-l', isActive: true },
    ];

    // Grey still has Grey/L (length=2 initially). M still has Black/M (length=2 initially).
    expect(canDeactivateVariantTuple(variants, 'col-grey', 'sz-m')).toBe(true);

    const toggleRes = toggleProductStudioVariantTuple(
      variants,
      { colorId: 'col-grey', sizeValueId: 'sz-m' },
      true
    );

    expect(toggleRes.allowed).toBe(true);
    expect(toggleRes.variants).toHaveLength(3);
    expect(toggleRes.variants.some((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-m')).toBe(false);
  });
});
