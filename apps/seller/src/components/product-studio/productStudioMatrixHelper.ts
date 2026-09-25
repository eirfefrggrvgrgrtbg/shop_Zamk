import type { ProductStudioVariant } from '../../contexts/ProductStudioContext';

export interface CanonicalColorInput {
  id: string;
  name: string;
  hex?: string;
}

export interface CanonicalSizeInput {
  id: string; // sizeValueId
  label: string; // size display label
}

export type ProductStudioDimensionType =
  | 'COLOR_AND_SIZE'
  | 'COLOR_ONLY'
  | 'SIZE_ONLY'
  | 'SINGLE_VARIANT';

export const CANONICAL_DIMENSION_TYPES: ReadonlySet<ProductStudioDimensionType> = new Set([
  'COLOR_AND_SIZE',
  'COLOR_ONLY',
  'SIZE_ONLY',
  'SINGLE_VARIANT',
]);

export function isCanonicalDimensionType(value: unknown): value is ProductStudioDimensionType {
  return typeof value === 'string' && CANONICAL_DIMENSION_TYPES.has(value as ProductStudioDimensionType);
}

/**
 * Resolves the effective ProductStudioDimensionType strictly and fails closed on missing/unknown values.
 *
 * Rules:
 * A. If draftDimensionType is canonical -> use it.
 * B. If draftDimensionType is absent/null/undefined/empty and categorySchemaDimensionType is canonical -> use categorySchemaDimensionType.
 * C. If both are absent -> return null (unresolved).
 * D. If either supplied value is unsupported/unknown -> fail closed (return null, do NOT guess COLOR_AND_SIZE).
 */
export function resolveProductStudioDimensionType(
  draftDimensionType?: string | null,
  categorySchemaDimensionType?: string | null
): ProductStudioDimensionType | null {
  const hasDraftDim =
    draftDimensionType !== undefined &&
    draftDimensionType !== null &&
    draftDimensionType.trim() !== '';

  if (hasDraftDim) {
    if (isCanonicalDimensionType(draftDimensionType)) {
      return draftDimensionType;
    }
    // draftDimensionType is supplied but unsupported -> fail closed
    return null;
  }

  const hasSchemaDim =
    categorySchemaDimensionType !== undefined &&
    categorySchemaDimensionType !== null &&
    categorySchemaDimensionType.trim() !== '';

  if (hasSchemaDim) {
    if (isCanonicalDimensionType(categorySchemaDimensionType)) {
      return categorySchemaDimensionType;
    }
    // categorySchemaDimensionType is supplied but unsupported -> fail closed
    return null;
  }

  return null;
}

export interface RequiredVariantTuple {
  colorId?: string;
  colorName?: string;
  colorHex?: string;
  sizeValueId?: string;
  size?: string;
}

export type DraftVariantIdFactory = (tuple: RequiredVariantTuple, index: number) => string;

export const defaultDraftVariantIdFactory: DraftVariantIdFactory = (tuple) => {
  const c = tuple.colorId ?? 'none';
  const s = tuple.sizeValueId ?? 'none';
  return `draft-var-${c}-${s}`;
};

/**
 * Expands or updates the draft variant matrix when a new canonical color is added.
 * Invariants:
 * - If there are existing unique sizes, creates combinations for (newColor, each existing size).
 * - If there are NO existing sizes yet, creates a single variant with (newColor, sizeValueId: undefined).
 * - Preserves existing variants and their IDs.
 * - No duplicate (colorId, sizeValueId).
 * - No stock, initialStock, or warehouse semantics.
 */
export function addColorToMatrix(
  currentVariants: ProductStudioVariant[],
  color: CanonicalColorInput
): ProductStudioVariant[] {
  const existingSizes = new Map<string, string>(); // sizeValueId -> size label
  for (const v of currentVariants) {
    if (v.isActive !== false && v.sizeValueId) {
      existingSizes.set(v.sizeValueId, v.size || v.sizeValueId);
    }
  }

  const result = [...currentVariants];

  if (existingSizes.size === 0) {
    const exists = result.some((v) => v.isActive !== false && v.colorId === color.id && !v.sizeValueId);
    if (!exists) {
      const tuple: RequiredVariantTuple = {
        colorId: color.id,
        colorName: color.name,
        colorHex: color.hex,
      };
      result.push({
        id: defaultDraftVariantIdFactory(tuple, result.length),
        ...tuple,
        isActive: true,
      });
    }
  } else {
    for (const [sizeValueId, sizeLabel] of existingSizes.entries()) {
      const exists = result.some(
        (v) => v.isActive !== false && v.colorId === color.id && v.sizeValueId === sizeValueId
      );
      if (!exists) {
        const tuple: RequiredVariantTuple = {
          colorId: color.id,
          colorName: color.name,
          colorHex: color.hex,
          sizeValueId,
          size: sizeLabel,
        };
        result.push({
          id: defaultDraftVariantIdFactory(tuple, result.length),
          ...tuple,
          isActive: true,
        });
      }
    }
  }

  // If we now have color-associated variants, filter out any standalone size-only variants
  const hasColorVariants = result.some((v) => v.isActive !== false && v.colorId !== undefined);
  if (hasColorVariants) {
    return result.filter((v) => v.colorId !== undefined);
  }

  return result;
}

/**
 * Expands or updates the draft variant matrix when a new canonical size is added.
 * Invariants:
 * - If there are existing unique colors, creates combinations for (each existing color, newSize).
 * - If there are NO existing colors yet, creates a single variant with (colorId: undefined, newSize).
 * - Preserves existing variants and their IDs.
 * - No duplicate (colorId, sizeValueId).
 * - No stock, initialStock, or warehouse semantics.
 */
export function addSizeToMatrix(
  currentVariants: ProductStudioVariant[],
  size: CanonicalSizeInput
): ProductStudioVariant[] {
  const existingColors = new Map<string, { name?: string; hex?: string }>();
  for (const v of currentVariants) {
    if (v.isActive !== false && v.colorId) {
      existingColors.set(v.colorId, { name: v.colorName, hex: v.colorHex });
    }
  }

  const result = [...currentVariants];

  if (existingColors.size === 0) {
    const exists = result.some((v) => v.isActive !== false && !v.colorId && v.sizeValueId === size.id);
    if (!exists) {
      const tuple: RequiredVariantTuple = {
        sizeValueId: size.id,
        size: size.label,
      };
      result.push({
        id: defaultDraftVariantIdFactory(tuple, result.length),
        ...tuple,
        isActive: true,
      });
    }
  } else {
    for (const [colorId, colorMeta] of existingColors.entries()) {
      const exists = result.some(
        (v) => v.isActive !== false && v.colorId === colorId && v.sizeValueId === size.id
      );
      if (!exists) {
        const tuple: RequiredVariantTuple = {
          colorId,
          colorName: colorMeta.name,
          colorHex: colorMeta.hex,
          sizeValueId: size.id,
          size: size.label,
        };
        result.push({
          id: defaultDraftVariantIdFactory(tuple, result.length),
          ...tuple,
          isActive: true,
        });
      }
    }
  }

  // If we now have size-associated variants, filter out any standalone color-only variants
  const hasSizeVariants = result.some((v) => v.isActive !== false && v.sizeValueId !== undefined);
  if (hasSizeVariants) {
    return result.filter((v) => v.sizeValueId !== undefined);
  }

  return result;
}

/**
 * Removes a color from the variant matrix.
 * If this is the last color being removed but sizes exist, fallback to size-only variants.
 */
export function removeColorFromMatrix(
  currentVariants: ProductStudioVariant[],
  colorId: string
): ProductStudioVariant[] {
  const filtered = currentVariants.filter((v) => v.colorId !== colorId);
  const hasRemainingColors = filtered.some((v) => v.colorId !== undefined);

  if (!hasRemainingColors) {
    // If no colors left, but we had sizes in the removed color variants,
    // we need to rebuild size-only variants for any sizes that existed.
    const existingSizes = new Map<string, string>();
    for (const v of currentVariants) {
      if (v.isActive !== false && v.sizeValueId) {
        existingSizes.set(v.sizeValueId, v.size || v.sizeValueId);
      }
    }

    if (existingSizes.size > 0) {
      const fallback: ProductStudioVariant[] = [];
      for (const [sizeValueId, sizeLabel] of existingSizes.entries()) {
        const tuple: RequiredVariantTuple = {
          sizeValueId,
          size: sizeLabel,
        };
        fallback.push({
          id: defaultDraftVariantIdFactory(tuple, fallback.length),
          ...tuple,
          isActive: true,
        });
      }
      return fallback;
    }
  }

  return filtered;
}

/**
 * Removes a size from the variant matrix.
 * If this is the last size being removed but colors exist, fallback to color-only variants.
 */
export function removeSizeFromMatrix(
  currentVariants: ProductStudioVariant[],
  sizeValueId: string
): ProductStudioVariant[] {
  const filtered = currentVariants.filter((v) => v.sizeValueId !== sizeValueId);
  const hasRemainingSizes = filtered.some((v) => v.sizeValueId !== undefined);

  if (!hasRemainingSizes) {
    // If no sizes left, but we had colors in the removed size variants,
    // we need to rebuild color-only variants for any colors that existed.
    const existingColors = new Map<string, { name?: string; hex?: string }>();
    for (const v of currentVariants) {
      if (v.isActive !== false && v.colorId) {
        existingColors.set(v.colorId, { name: v.colorName, hex: v.colorHex });
      }
    }

    if (existingColors.size > 0) {
      const fallback: ProductStudioVariant[] = [];
      for (const [colorId, colorMeta] of existingColors.entries()) {
        const tuple: RequiredVariantTuple = {
          colorId,
          colorName: colorMeta.name,
          colorHex: colorMeta.hex,
        };
        fallback.push({
          id: defaultDraftVariantIdFactory(tuple, fallback.length),
          ...tuple,
          isActive: true,
        });
      }
      return fallback;
    }
  }

  return filtered;
}

/**
 * Computes required variant tuples based on dimension type and configured dimensions.
 * Fails closed for unknown/unsupported dimension types without inventing Cartesian tuples.
 */
export function computeRequiredVariantTuples(
  dimensionType: ProductStudioDimensionType | string,
  colors: CanonicalColorInput[],
  sizes: CanonicalSizeInput[]
): RequiredVariantTuple[] {
  switch (dimensionType) {
    case 'COLOR_AND_SIZE': {
      if (colors.length === 0 && sizes.length === 0) {
        return [];
      }
      if (colors.length > 0 && sizes.length === 0) {
        return colors.map((c) => ({
          colorId: c.id,
          colorName: c.name,
          colorHex: c.hex,
        }));
      }
      if (colors.length === 0 && sizes.length > 0) {
        return sizes.map((s) => ({
          sizeValueId: s.id,
          size: s.label,
        }));
      }
      const tuples: RequiredVariantTuple[] = [];
      for (const c of colors) {
        for (const s of sizes) {
          tuples.push({
            colorId: c.id,
            colorName: c.name,
            colorHex: c.hex,
            sizeValueId: s.id,
            size: s.label,
          });
        }
      }
      return tuples;
    }
    case 'COLOR_ONLY': {
      return colors.map((c) => ({
        colorId: c.id,
        colorName: c.name,
        colorHex: c.hex,
      }));
    }
    case 'SIZE_ONLY': {
      return sizes.map((s) => ({
        sizeValueId: s.id,
        size: s.label,
      }));
    }
    case 'SINGLE_VARIANT': {
      return [{}];
    }
    default:
      throw new Error(`Unsupported dimension type: "${dimensionType}"`);
  }
}

/**
 * Reconciles a variant matrix to enforce EXACT Cartesian completeness and pure state.
 *
 * Invariants:
 * 1. Pure & deterministic: given identical inputs, produces identical semantic output.
 * 2. Active preservation: ONLY variants with v.isActive !== false are eligible for preservation.
 * 3. Inactive variants are never resurrected or reused: if a tuple was previously inactive,
 *    a NEW draft variant identity is created with createDraftVariantId.
 * 4. Canonical dimension types only: fails closed on unknown dimension types.
 */
export function reconcileProductStudioVariantMatrix(
  dimensionType: ProductStudioDimensionType | string,
  colors: CanonicalColorInput[],
  sizes: CanonicalSizeInput[],
  currentVariants: ProductStudioVariant[],
  oldColors?: CanonicalColorInput[],
  oldSizes?: CanonicalSizeInput[],
  createDraftVariantId: DraftVariantIdFactory = defaultDraftVariantIdFactory
): ProductStudioVariant[] {
  const tuples = computeRequiredVariantTuples(dimensionType, colors, sizes);

  const isSparse = dimensionType === 'COLOR_AND_SIZE' && colors.length > 0 && sizes.length > 0;
  const oldColorIds = new Set((oldColors || []).map(c => c.id));
  const oldSizeIds = new Set((oldSizes || []).map(s => s.id));
  const hasOldData = oldColors !== undefined && oldSizes !== undefined;

  return (tuples.map((tuple, index) => {
    // Only current ACTIVE variants may be candidates for preservation.
    // Inactive historical records (isActive === false) are treated as nonexistent.
    const existing = currentVariants.find(
      (v) =>
        v.isActive !== false &&
        (v.colorId || undefined) === (tuple.colorId || undefined) &&
        (v.sizeValueId || undefined) === (tuple.sizeValueId || undefined)
    );

    if (existing) {
      return { ...existing, isActive: true };
    }

    // Sparse matrix logic: if this tuple doesn't exist, we only create it if it's forced by a NEW color or NEW size,
    // or if we're not in sparse mode (dense), or if no oldData was provided.
    if (isSparse && hasOldData) {
      const isNewColor = tuple.colorId && !oldColorIds.has(tuple.colorId);
      const isNewSize = tuple.sizeValueId && !oldSizeIds.has(tuple.sizeValueId);

      if (!isNewColor && !isNewSize) {
         // It's an existing color and existing size, but no existing variant.
         // This means the seller explicitly disabled this cell previously. We must NOT self-heal.
         return null;
      }
    }

    return {
      id: createDraftVariantId(tuple, index),
      colorId: tuple.colorId,
      colorName: tuple.colorName,
      colorHex: tuple.colorHex,
      sizeValueId: tuple.sizeValueId,
      size: tuple.size,
      isActive: true,
    };
  }) as (ProductStudioVariant | null)[]).filter((v): v is ProductStudioVariant => v !== null);
}

export const BLOCKED_LAST_CELL_TOOLTIP =
  'Нельзя отключить последний вариант цвета или размера. Удалите цвет или размер отдельно.';

/**
 * Determines whether an active variant tuple (colorId, sizeValueId) can be turned OFF.
 * Invariant:
 * Every configured color must have >= 1 active variant.
 * Every configured size must have >= 1 active variant.
 * Turning OFF a tuple is blocked if it would leave 0 active variants for that color OR size.
 */
export function canDeactivateVariantTuple(
  currentVariants: ProductStudioVariant[],
  colorId?: string | null,
  sizeValueId?: string | null
): boolean {
  const activeVariants = (currentVariants || []).filter((v) => v.isActive !== false);

  if (colorId) {
    const activeForColor = activeVariants.filter(
      (v) => (v.colorId || undefined) === colorId
    );
    if (activeForColor.length <= 1) {
      return false;
    }
  }

  if (sizeValueId) {
    const activeForSize = activeVariants.filter(
      (v) => (v.sizeValueId || undefined) === sizeValueId
    );
    if (activeForSize.length <= 1) {
      return false;
    }
  }

  return true;
}

export interface ToggleVariantTupleResult {
  allowed: boolean;
  variants: ProductStudioVariant[];
  reason?: 'LAST_CELL_IN_AXIS';
}

/**
 * Toggles a variant tuple between active and inactive.
 * If deactivating, verifies that the mutation does not violate the axis invariant.
 * If invariant would be violated, the mutation is blocked and current variants returned unchanged.
 */
export function toggleProductStudioVariantTuple(
  currentVariants: ProductStudioVariant[],
  target: {
    colorId?: string;
    colorName?: string;
    colorHex?: string;
    sizeValueId?: string;
    size?: string;
  },
  currentlyActive: boolean,
  createDraftVariantId: DraftVariantIdFactory = defaultDraftVariantIdFactory
): ToggleVariantTupleResult {
  const activeVariants = (currentVariants || []).filter((v) => v.isActive !== false);

  if (currentlyActive) {
    const canDeactivate = canDeactivateVariantTuple(
      activeVariants,
      target.colorId,
      target.sizeValueId
    );

    if (!canDeactivate) {
      return {
        allowed: false,
        variants: currentVariants,
        reason: 'LAST_CELL_IN_AXIS',
      };
    }

    const nextVariants = activeVariants.filter(
      (v) =>
        !(
          (v.colorId || undefined) === (target.colorId || undefined) &&
          (v.sizeValueId || undefined) === (target.sizeValueId || undefined)
        )
    );

    return {
      allowed: true,
      variants: nextVariants,
    };
  }

  const newVariant: ProductStudioVariant = {
    id: createDraftVariantId(
      {
        colorId: target.colorId,
        colorName: target.colorName,
        colorHex: target.colorHex,
        sizeValueId: target.sizeValueId,
        size: target.size,
      },
      activeVariants.length
    ),
    colorId: target.colorId,
    colorName: target.colorName,
    colorHex: target.colorHex,
    sizeValueId: target.sizeValueId,
    size: target.size,
    isActive: true,
  };

  return {
    allowed: true,
    variants: [...activeVariants, newVariant],
  };
}
