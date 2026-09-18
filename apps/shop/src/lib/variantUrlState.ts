import type { ProductVariantItem, DimensionType } from './variantSelection';
import {
  getDimensionType,
  getColorOptions,
  getVariantColorId,
  getVariantSizeId,
  isVariantBuyable,
  formatSizeUnavailableNotice,
  formatStaleSizeNotice,
  normalizeIdentity,
} from './variantSelection';

function matchesColor(v: ProductVariantItem, colorParam: string): boolean {
  return (
    getVariantColorId(v) === colorParam ||
    v.colorId === colorParam ||
    (Boolean(v.colorName) && normalizeIdentity(v.colorName) === normalizeIdentity(colorParam)) ||
    (Boolean(v.color) && normalizeIdentity(v.color) === normalizeIdentity(colorParam))
  );
}

function matchesSize(v: ProductVariantItem, sizeParam: string): boolean {
  return (
    getVariantSizeId(v) === sizeParam ||
    v.sizeValueId === sizeParam ||
    (Boolean(v.size) && normalizeIdentity(v.size) === normalizeIdentity(sizeParam)) ||
    getVariantSizeId(v) === `legacy-size:${normalizeIdentity(sizeParam)}`
  );
}

export interface VariantUrlParams {
  colorParam: string | null;
  sizeParam: string | null;
}

export function parseVariantUrlParams(searchParams: URLSearchParams): VariantUrlParams {
  const color = searchParams.get('color')?.trim() || null;
  const size = searchParams.get('size')?.trim() || null;
  return {
    colorParam: color,
    sizeParam: size,
  };
}

export interface ValidatedVariantUrlState {
  dimensionType: DimensionType;
  targetColorId: string | null;
  targetSizeId: string | null;
  hasExplicitColorIntent: boolean;
  hasExplicitSizeIntent: boolean;
  isExactBuyable: boolean;
  needsReplace: boolean;
  sanitizedSearchParams: URLSearchParams;
  notice: string | null;
}

export function validateVariantUrlState(
  variants: ProductVariantItem[] | undefined,
  searchParams: URLSearchParams
): ValidatedVariantUrlState {
  const { colorParam, sizeParam } = parseVariantUrlParams(searchParams);
  const dimensionType = getDimensionType(variants);
  const activeVariants = variants?.filter(v => v.isActive !== false) || [];
  const colors = getColorOptions(variants);

  const sanitized = new URLSearchParams(searchParams);
  let needsReplace = false;
  let targetColorId: string | null = null;
  let targetSizeId: string | null = null;
  let hasExplicitColorIntent = false;
  let hasExplicitSizeIntent = false;
  let isExactBuyable = false;
  let notice: string | null = null;

  if (dimensionType === 'COLOR_AND_SIZE') {
    if (colorParam) {
      const validColor = colors.find(
        c => c.id === colorParam || normalizeIdentity(c.name) === normalizeIdentity(colorParam)
      );
      if (validColor) {
        targetColorId = validColor.id;
        hasExplicitColorIntent = true;

        if (sizeParam) {
          const exactVariant = activeVariants.find(
            v => matchesColor(v, targetColorId!) && matchesSize(v, sizeParam)
          );

          if (exactVariant && isVariantBuyable(exactVariant)) {
            targetSizeId = getVariantSizeId(exactVariant);
            hasExplicitSizeIntent = true;
            isExactBuyable = true;
          } else {
            targetSizeId = null;
            sanitized.delete('size');
            needsReplace = true;

            const sizeLabel =
              exactVariant?.size ||
              activeVariants.find(v => matchesSize(v, sizeParam))?.size ||
              sizeParam;

            if (exactVariant && !isVariantBuyable(exactVariant)) {
              // Section 12: Exact variant exists but inStock !== true (SOLD OUT)
              notice = formatStaleSizeNotice(sizeLabel);
            } else {
              // Section 13: Size not offered in this color (NOT OFFERED)
              notice = formatSizeUnavailableNotice(sizeLabel, validColor.name);
            }
          }
        }
      } else {
        sanitized.delete('color');
        if (sizeParam) {
          sanitized.delete('size');
        }
        needsReplace = true;
        targetColorId = null;
        targetSizeId = null;
      }
    } else if (sizeParam) {
      // COLOR_AND_SIZE product with only sizeParam: must NOT guess color
      sanitized.delete('size');
      needsReplace = true;
      targetColorId = null;
      targetSizeId = null;
    }
  } else if (dimensionType === 'SIZE_ONLY') {
    if (colorParam) {
      sanitized.delete('color');
      needsReplace = true;
    }
    if (sizeParam) {
      const exactVariant = activeVariants.find(v => matchesSize(v, sizeParam));
      if (exactVariant && isVariantBuyable(exactVariant)) {
        targetSizeId = getVariantSizeId(exactVariant);
        hasExplicitSizeIntent = true;
        isExactBuyable = true;
      } else {
        sanitized.delete('size');
        needsReplace = true;
        targetSizeId = null;
        if (exactVariant && !isVariantBuyable(exactVariant)) {
          const sizeLabel = exactVariant.size || sizeParam;
          notice = formatStaleSizeNotice(sizeLabel);
        }
      }
    }
  } else if (dimensionType === 'COLOR_ONLY') {
    if (sizeParam) {
      sanitized.delete('size');
      needsReplace = true;
    }
    if (colorParam) {
      const validColor = colors.find(
        c => c.id === colorParam || normalizeIdentity(c.name) === normalizeIdentity(colorParam)
      );
      if (validColor) {
        targetColorId = validColor.id;
        hasExplicitColorIntent = true;
      } else {
        sanitized.delete('color');
        needsReplace = true;
      }
    }
  } else {
    // SINGLE_VARIANT
    if (colorParam) {
      sanitized.delete('color');
      needsReplace = true;
    }
    if (sizeParam) {
      sanitized.delete('size');
      needsReplace = true;
    }
  }

  return {
    dimensionType,
    targetColorId,
    targetSizeId,
    hasExplicitColorIntent,
    hasExplicitSizeIntent,
    isExactBuyable,
    needsReplace,
    sanitizedSearchParams: sanitized,
    notice,
  };
}

export function serializeSizeParam(sizeId: string, variants?: ProductVariantItem[]): string {
  if (variants) {
    const matching = variants.find(
      v =>
        getVariantSizeId(v) === sizeId ||
        v.sizeValueId === sizeId ||
        (Boolean(v.size) && normalizeIdentity(v.size) === normalizeIdentity(sizeId)) ||
        getVariantSizeId(v) === `legacy-size:${normalizeIdentity(sizeId)}`
    );
    if (matching?.sizeValueId) return matching.sizeValueId;
    if (matching?.size) return matching.size;
  }
  if (sizeId.startsWith('legacy-size:')) {
    return sizeId.slice('legacy-size:'.length);
  }
  return sizeId;
}

export function computeVariantUrlParams(
  currentParams: URLSearchParams,
  dimensionType: DimensionType,
  nextColorId?: string | null,
  nextSizeId?: string | null,
  variants?: ProductVariantItem[]
): URLSearchParams {
  const next = new URLSearchParams(currentParams);

  const cleanSize = nextSizeId
    ? serializeSizeParam(nextSizeId, variants)
    : null;

  if (dimensionType === 'COLOR_AND_SIZE') {
    if (nextColorId) {
      next.set('color', nextColorId);
    } else {
      next.delete('color');
    }
    if (cleanSize) {
      next.set('size', cleanSize);
    } else {
      next.delete('size');
    }
  } else if (dimensionType === 'SIZE_ONLY') {
    next.delete('color');
    if (cleanSize) {
      next.set('size', cleanSize);
    } else {
      next.delete('size');
    }
  } else if (dimensionType === 'COLOR_ONLY') {
    if (nextColorId) {
      next.set('color', nextColorId);
    } else {
      next.delete('color');
    }
    next.delete('size');
  } else {
    next.delete('color');
    next.delete('size');
  }

  return next;
}

export function areSearchParamsEqual(a: URLSearchParams, b: URLSearchParams): boolean {
  return a.toString() === b.toString();
}
