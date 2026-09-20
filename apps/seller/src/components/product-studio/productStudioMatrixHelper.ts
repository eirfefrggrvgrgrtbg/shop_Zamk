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
    if (v.sizeValueId) {
      existingSizes.set(v.sizeValueId, v.size || v.sizeValueId);
    }
  }

  const result = [...currentVariants];

  if (existingSizes.size === 0) {
    const exists = result.some((v) => v.colorId === color.id && !v.sizeValueId);
    if (!exists) {
      result.push({
        id: `draft-var-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        colorId: color.id,
        colorName: color.name,
        colorHex: color.hex,
        isActive: true,
      });
    }
  } else {
    for (const [sizeValueId, sizeLabel] of existingSizes.entries()) {
      const exists = result.some(
        (v) => v.colorId === color.id && v.sizeValueId === sizeValueId
      );
      if (!exists) {
        result.push({
          id: `draft-var-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          colorId: color.id,
          colorName: color.name,
          colorHex: color.hex,
          sizeValueId,
          size: sizeLabel,
          isActive: true,
        });
      }
    }
  }

  // If we now have color-associated variants, filter out any standalone size-only variants
  const hasColorVariants = result.some((v) => v.colorId !== undefined);
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
    if (v.colorId) {
      existingColors.set(v.colorId, { name: v.colorName, hex: v.colorHex });
    }
  }

  const result = [...currentVariants];

  if (existingColors.size === 0) {
    const exists = result.some((v) => !v.colorId && v.sizeValueId === size.id);
    if (!exists) {
      result.push({
        id: `draft-var-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        sizeValueId: size.id,
        size: size.label,
        isActive: true,
      });
    }
  } else {
    for (const [colorId, colorMeta] of existingColors.entries()) {
      const exists = result.some(
        (v) => v.colorId === colorId && v.sizeValueId === size.id
      );
      if (!exists) {
        result.push({
          id: `draft-var-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          colorId,
          colorName: colorMeta.name,
          colorHex: colorMeta.hex,
          sizeValueId: size.id,
          size: size.label,
          isActive: true,
        });
      }
    }
  }

  // If we now have size-associated variants, filter out any standalone color-only variants
  const hasSizeVariants = result.some((v) => v.sizeValueId !== undefined);
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
      if (v.sizeValueId) {
        existingSizes.set(v.sizeValueId, v.size || v.sizeValueId);
      }
    }
    
    if (existingSizes.size > 0) {
      const fallback: ProductStudioVariant[] = [];
      for (const [sizeValueId, sizeLabel] of existingSizes.entries()) {
        fallback.push({
          id: `draft-var-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          sizeValueId,
          size: sizeLabel,
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
      if (v.colorId) {
        existingColors.set(v.colorId, { name: v.colorName, hex: v.colorHex });
      }
    }

    if (existingColors.size > 0) {
      const fallback: ProductStudioVariant[] = [];
      for (const [colorId, colorMeta] of existingColors.entries()) {
        fallback.push({
          id: `draft-var-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          colorId,
          colorName: colorMeta.name,
          colorHex: colorMeta.hex,
          isActive: true,
        });
      }
      return fallback;
    }
  }

  return filtered;
}
