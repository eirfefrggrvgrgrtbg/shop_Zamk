import { useState, useMemo, useCallback, useEffect } from 'react';
import type { Product } from '../types/catalog';

export type ProductVariantItem = NonNullable<Product['variants']>[number];

export type DimensionType = 'COLOR_AND_SIZE' | 'SIZE_ONLY' | 'COLOR_ONLY' | 'SINGLE_VARIANT';

export interface ColorOption {
  id: string;
  name: string;
  hex?: string;
  shadeName?: string;
  hasInStock: boolean;
}

export type SizeAvailabilityState = 'AVAILABLE' | 'SOLD_OUT' | 'NOT_OFFERED';

export interface SizeOption {
  id: string;
  label: string;
  inStock: boolean;
  disabled: boolean;
  variantId?: string;
  state: SizeAvailabilityState;
  accessibleLabel: string;
}

export interface VariantSelectionState {
  dimensionType: DimensionType;
  colors: ColorOption[];
  sizes: SizeOption[];
  selectedColorId: string | null;
  selectedSizeId: string | null;
  selectedColor: ColorOption | null;
  selectedSize: SizeOption | null;
  selectedVariant: ProductVariantItem | null;
  isResolved: boolean;
  canAddToCart: boolean;
  requiresColor: boolean;
  requiresSize: boolean;
  ctaText: string;
  sizeSelectionNotice: string | null;
  selectionNotice: string | null;
}

export function isVariantBuyable(v?: ProductVariantItem | null): boolean {
  if (!v) return false;
  if (v.isActive === false) return false;
  return v.inStock === true;
}

export function formatSizeUnavailableNotice(sizeLabel: string, colorName?: string | null): string {
  const cleanSize = (sizeLabel || '').trim();
  const cleanColor = (colorName || '').trim();

  if (!cleanColor) {
    return cleanSize ? `Размер ${cleanSize} недоступен в выбранном цвете` : 'Выбранный размер недоступен в этом цвете';
  }

  const lower = cleanColor.toLowerCase();
  const inflected: Record<string, string> = {
    'белый': 'белом',
    'черный': 'черном',
    'чёрный': 'чёрном',
    'красный': 'красном',
    'синий': 'синем',
    'темно-синий': 'темно-синем',
    'тёмно-синий': 'тёмно-синем',
    'светло-синий': 'светло-синем',
    'серый': 'сером',
    'темно-серый': 'темно-сером',
    'тёмно-серый': 'тёмно-сером',
    'светло-серый': 'светло-сером',
    'зеленый': 'зеленом',
    'зелёный': 'зелёном',
    'темно-зеленый': 'темно-зеленом',
    'тёмно-зелёный': 'тёмно-зелёном',
    'желтый': 'желтом',
    'жёлтый': 'жёлтом',
    'розовый': 'розовом',
    'бежевый': 'бежевом',
    'коричневый': 'коричневом',
    'фиолетовый': 'фиолетовом',
    'голубой': 'голубом',
    'оранжевый': 'оранжевом',
    'бордовый': 'бордовом',
    'мятный': 'мятном',
    'хаки': 'цвете хаки',
    'айвори': 'цвете айвори',
  };

  if (inflected[lower]) {
    const prep = inflected[lower];
    if (prep.startsWith('цвете ')) {
      return `Размер ${cleanSize} недоступен в ${prep}`;
    }
    return `Размер ${cleanSize} недоступен в ${prep} цвете`;
  }

  if (lower.endsWith('ый') || lower.endsWith('ой')) {
    const stem = cleanColor.slice(0, -2);
    return `Размер ${cleanSize} недоступен в ${stem.toLowerCase()}ом цвете`;
  }
  if (lower.endsWith('ий')) {
    const stem = cleanColor.slice(0, -2);
    return `Размер ${cleanSize} недоступен в ${stem.toLowerCase()}ем цвете`;
  }

  return `Размер ${cleanSize} недоступен в цвете «${cleanColor}»`;
}

export function formatStaleSizeNotice(sizeLabel?: string | null): string {
  const clean = (sizeLabel || '').trim();
  if (clean) {
    return `Размер ${clean} только что закончился. Выберите другой размер.`;
  }
  return 'Этот вариант только что закончился.';
}

export const PRODUCT_JUST_SOLD_OUT_NOTICE = 'Товар только что закончился.';
export const REFRESH_ERROR_NOTICE = 'Не удалось обновить данные о наличии. Попробуйте обновить страницу.';

export function formatSizeSoldOutAriaLabel(sizeLabel: string): string {
  const cleanSize = (sizeLabel || '').trim();
  return cleanSize ? `Размер ${cleanSize}, закончился` : 'Закончился';
}

export function formatSizeNotOfferedAriaLabel(sizeLabel: string, colorName?: string | null): string {
  const cleanSize = (sizeLabel || '').trim();
  const cleanColor = (colorName || '').trim();

  if (!cleanColor) {
    return cleanSize ? `Размер ${cleanSize}, не представлен в выбранном цвете` : 'Не представлен в выбранном цвете';
  }

  const lower = cleanColor.toLowerCase();
  const inflected: Record<string, string> = {
    'белый': 'белом',
    'черный': 'черном',
    'чёрный': 'чёрном',
    'красный': 'красном',
    'синий': 'синем',
    'темно-синий': 'темно-синем',
    'тёмно-синий': 'тёмно-синем',
    'светло-синий': 'светло-синем',
    'серый': 'сером',
    'темно-серый': 'темно-сером',
    'тёмно-серый': 'тёмно-сером',
    'светло-серый': 'светло-сером',
    'зеленый': 'зеленом',
    'зелёный': 'зелёном',
    'темно-зеленый': 'темно-зеленом',
    'тёмно-зелёный': 'тёмно-зелёном',
    'желтый': 'желтом',
    'жёлтый': 'жёлтом',
    'розовый': 'розовом',
    'бежевый': 'бежевом',
    'коричневый': 'коричневом',
    'фиолетовый': 'фиолетовом',
    'голубой': 'голубом',
    'оранжевый': 'оранжевом',
    'бордовый': 'бордовом',
    'мятный': 'мятном',
    'хаки': 'цвете хаки',
    'айвори': 'цвете айвори',
  };

  if (inflected[lower]) {
    const prep = inflected[lower];
    if (prep.startsWith('цвете ')) {
      return `Размер ${cleanSize}, не представлен в ${prep}`;
    }
    return `Размер ${cleanSize}, не представлен в ${prep} цвете`;
  }

  if (lower.endsWith('ый') || lower.endsWith('ой')) {
    const stem = cleanColor.slice(0, -2);
    return `Размер ${cleanSize}, не представлен в ${stem.toLowerCase()}ом цвете`;
  }
  if (lower.endsWith('ий')) {
    const stem = cleanColor.slice(0, -2);
    return `Размер ${cleanSize}, не представлен в ${stem.toLowerCase()}ем цвете`;
  }

  return `Размер ${cleanSize}, не представлен в цвете «${cleanColor}»`;
}

export function normalizeIdentity(val?: string | null): string {
  return (val || '').trim().toLowerCase();
}

export function getVariantColorId(v: ProductVariantItem): string | null {
  if (v.colorId) return v.colorId;
  const name = normalizeIdentity(v.colorName || v.color);
  return name ? `legacy-color:${name}` : null;
}

export function getVariantSizeId(v: ProductVariantItem): string | null {
  if (v.sizeValueId) return v.sizeValueId;
  const label = normalizeIdentity(v.size);
  return label ? `legacy-size:${label}` : null;
}

export function getDimensionType(variants?: ProductVariantItem[]): DimensionType {
  const active = variants?.filter(v => v.isActive !== false) || [];
  if (active.length === 0) {
    return 'SINGLE_VARIANT';
  }

  const hasColor = active.some(v => Boolean(getVariantColorId(v)));
  const hasSize = active.some(v => {
    const sizeId = getVariantSizeId(v);
    if (!sizeId) return false;
    // Don't treat a single universal size as multi-size dimension if only 1 variant exists
    if (active.length === 1 && (v.size === 'Единый' || v.size === 'Стандарт' || !v.size)) {
      return false;
    }
    return true;
  });

  if (hasColor && hasSize) return 'COLOR_AND_SIZE';
  if (hasSize) return 'SIZE_ONLY';
  if (hasColor) return 'COLOR_ONLY';
  return 'SINGLE_VARIANT';
}

export function getColorOptions(variants?: ProductVariantItem[]): ColorOption[] {
  const active = variants?.filter(v => v.isActive !== false) || [];
  const map = new Map<string, ColorOption>();

  for (const v of active) {
    const colorId = getVariantColorId(v);
    if (!colorId) continue;
    const name = v.colorName || v.color;
    if (!name) continue;

    const existing = map.get(colorId);
    const isInStock = isVariantBuyable(v);

    if (!existing) {
      map.set(colorId, {
        id: colorId,
        name,
        hex: v.colorHex || undefined,
        shadeName: (v as any).shadeName || undefined,
        hasInStock: isInStock,
      });
    } else if (isInStock) {
      existing.hasInStock = true;
    }
  }

  return Array.from(map.values());
}

export function getDefaultColorId(colors: ColorOption[]): string | null {
  if (colors.length === 0) return null;
  const inStockColor = colors.find(c => c.hasInStock);
  return inStockColor ? inStockColor.id : colors[0].id;
}

export function getSizeOptions(
  variants?: ProductVariantItem[],
  selectedColorId?: string | null,
  dimensionTypeOrSizeChart?: DimensionType | any,
  maybeSizeChart?: any
): SizeOption[] {
  const active = variants?.filter(v => v.isActive !== false) || [];

  let dimensionType: DimensionType;
  let sizeChart: any;

  if (typeof dimensionTypeOrSizeChart === 'string') {
    dimensionType = dimensionTypeOrSizeChart as DimensionType;
    sizeChart = maybeSizeChart;
  } else {
    dimensionType = getDimensionType(variants);
    sizeChart = dimensionTypeOrSizeChart;
  }

  // 1. Collect all unique sizes across active variants for the product, preserving first-seen order
  interface ProductSizeMeta {
    sizeId: string;
    label: string;
  }
  const productSizes: ProductSizeMeta[] = [];
  const seenSizeIds = new Set<string>();

  for (const v of active) {
    const sizeId = getVariantSizeId(v);
    if (!sizeId) continue;
    if (!seenSizeIds.has(sizeId)) {
      seenSizeIds.add(sizeId);
      productSizes.push({
        sizeId,
        label: v.size || 'Стандарт',
      });
    }
  }

  // 2. Deterministic ordering:
  // Use sizeChart rows if available (matching sizeValueId or label)
  if (sizeChart?.rows && Array.isArray(sizeChart.rows)) {
    const chartRows = sizeChart.rows;
    productSizes.sort((a, b) => {
      const idxA = chartRows.findIndex((r: any) =>
        (r.sizeValueId && r.sizeValueId === a.sizeId) ||
        (r.sizeValueName && r.sizeValueName === a.label) ||
        (r.size && r.size === a.label)
      );
      const idxB = chartRows.findIndex((r: any) =>
        (r.sizeValueId && r.sizeValueId === b.sizeId) ||
        (r.sizeValueName && r.sizeValueName === b.label) ||
        (r.size && r.size === b.label)
      );
      if (idxA !== -1 && idxB !== -1) return idxA - idxB;
      if (idxA !== -1) return -1;
      if (idxB !== -1) return 1;
      return 0;
    });
  }

  // Look up color name for NOT_OFFERED aria label formatting if in COLOR_AND_SIZE mode
  let selectedColorName: string | null = null;
  if (selectedColorId) {
    const colorVar = active.find(v => getVariantColorId(v) === selectedColorId);
    selectedColorName = colorVar?.colorName || colorVar?.color || null;
  }

  // 3. For each size in the product size universe, calculate the exact state
  return productSizes.map(({ sizeId, label }) => {
    if (dimensionType === 'COLOR_AND_SIZE') {
      if (!selectedColorId) {
        // No color selected: neutral disabled
        return {
          id: sizeId,
          label,
          inStock: false,
          disabled: true,
          variantId: undefined,
          state: 'AVAILABLE',
          accessibleLabel: `Размер ${label}`,
        };
      }

      const matchingVariants = active.filter(
        v => getVariantColorId(v) === selectedColorId && getVariantSizeId(v) === sizeId
      );

      if (matchingVariants.length === 0) {
        // Size does not exist under selected color
        return {
          id: sizeId,
          label,
          inStock: false,
          disabled: true,
          variantId: undefined,
          state: 'NOT_OFFERED',
          accessibleLabel: formatSizeNotOfferedAriaLabel(label, selectedColorName),
        };
      }

      // Exact variant exists
      const buyableVariant = matchingVariants.find(v => isVariantBuyable(v));
      if (buyableVariant) {
        return {
          id: sizeId,
          label,
          inStock: true,
          disabled: false,
          variantId: buyableVariant.id,
          state: 'AVAILABLE',
          accessibleLabel: `Размер ${label}`,
        };
      }

      // Variant exists but zero stock / not buyable
      return {
        id: sizeId,
        label,
        inStock: false,
        disabled: true,
        variantId: matchingVariants[0].id,
        state: 'SOLD_OUT',
        accessibleLabel: formatSizeSoldOutAriaLabel(label),
      };
    }

    // SIZE_ONLY or others
    const matchingVariants = active.filter(v => getVariantSizeId(v) === sizeId);
    const buyableVariant = matchingVariants.find(v => isVariantBuyable(v));
    if (buyableVariant) {
      return {
        id: sizeId,
        label,
        inStock: true,
        disabled: false,
        variantId: buyableVariant.id,
        state: 'AVAILABLE',
        accessibleLabel: `Размер ${label}`,
      };
    }

    return {
      id: sizeId,
      label,
      inStock: false,
      disabled: true,
      variantId: matchingVariants[0]?.id,
      state: 'SOLD_OUT',
      accessibleLabel: formatSizeSoldOutAriaLabel(label),
    };
  });
}

export function resolveExactVariant(
  variants?: ProductVariantItem[],
  dimensionType: DimensionType = 'SINGLE_VARIANT',
  selectedColorId?: string | null,
  selectedSizeId?: string | null
): ProductVariantItem | null {
  const active = variants?.filter(v => v.isActive !== false) || [];
  if (active.length === 0) return null;

  if (dimensionType === 'SINGLE_VARIANT') {
    return active[0] || null;
  }

  if (dimensionType === 'COLOR_AND_SIZE') {
    if (!selectedColorId || !selectedSizeId) return null;
    const candidates = active.filter(
      v => getVariantColorId(v) === selectedColorId && getVariantSizeId(v) === selectedSizeId
    );
    return candidates.length === 1 ? candidates[0] : null;
  }

  if (dimensionType === 'SIZE_ONLY') {
    if (!selectedSizeId) return null;
    const candidates = active.filter(v => getVariantSizeId(v) === selectedSizeId);
    return candidates.length === 1 ? candidates[0] : null;
  }

  if (dimensionType === 'COLOR_ONLY') {
    if (!selectedColorId) return null;
    const candidates = active.filter(v => getVariantColorId(v) === selectedColorId);
    return candidates.length === 1 ? candidates[0] : null;
  }

  return null;
}

export function selectVariantState(
  variants?: ProductVariantItem[],
  selectedColorId?: string | null,
  selectedSizeId?: string | null,
  sizeChart?: any,
  sizeSelectionNotice: string | null = null
): VariantSelectionState {
  const dimensionType = getDimensionType(variants);
  const colors = getColorOptions(variants);

  const effectiveColorId = (dimensionType === 'COLOR_AND_SIZE' || dimensionType === 'COLOR_ONLY')
    ? (selectedColorId || null)
    : null;

  const sizes = getSizeOptions(variants, effectiveColorId, dimensionType, sizeChart);
  const effectiveSizeId = selectedSizeId || null;

  const selectedColor = colors.find(c => c.id === effectiveColorId) || null;
  const selectedSize = sizes.find(s => s.id === effectiveSizeId) || null;

  const selectedVariant = resolveExactVariant(
    variants,
    dimensionType,
    effectiveColorId,
    effectiveSizeId
  );

  const requiresColor = dimensionType === 'COLOR_AND_SIZE' || dimensionType === 'COLOR_ONLY';
  const requiresSize = dimensionType === 'COLOR_AND_SIZE' || dimensionType === 'SIZE_ONLY';

  const isResolved = selectedVariant !== null;
  const canAddToCart = isResolved && isVariantBuyable(selectedVariant);

  let ctaText = 'Добавить в корзину';
  if (!isResolved) {
    if (requiresColor && !effectiveColorId) {
      ctaText = 'Выберите цвет';
    } else if (selectedColor && !selectedColor.hasInStock) {
      ctaText = 'Нет в наличии';
    } else if (requiresSize && !effectiveSizeId) {
      ctaText = 'Выберите размер';
    } else {
      ctaText = 'Выберите вариант';
    }
  } else if (!canAddToCart) {
    ctaText = 'Нет в наличии';
  }

  return {
    dimensionType,
    colors,
    sizes,
    selectedColorId: effectiveColorId,
    selectedSizeId: effectiveSizeId,
    selectedColor,
    selectedSize,
    selectedVariant,
    isResolved,
    canAddToCart,
    requiresColor,
    requiresSize,
    ctaText,
    sizeSelectionNotice,
    selectionNotice: sizeSelectionNotice,
  };
}

export function useVariantSelection(
  variants?: ProductVariantItem[],
  sizeChart?: any,
  initialColorId?: string | null,
  initialSizeId?: string | null
) {
  const colors = useMemo(() => getColorOptions(variants), [variants]);

  const [selectedColorId, setSelectedColorId] = useState<string | null>(() => {
    if (initialColorId !== undefined && initialColorId !== null) return initialColorId;
    return null;
  });

  const [selectedSizeId, setSelectedSizeId] = useState<string | null>(() => {
    return initialSizeId ?? null;
  });

  const [sizeSelectionNotice, setSizeSelectionNotice] = useState<string | null>(null);

  const selectColor = useCallback((colorId: string) => {
    if (!colorId || colorId === selectedColorId) return;

    setSelectedColorId(colorId);

    const dim = getDimensionType(variants);
    if (dim !== 'COLOR_AND_SIZE') {
      setSizeSelectionNotice(null);
      return;
    }

    if (!selectedSizeId) {
      // Case D: no size was selected before color change -> just switch color, no warning
      setSizeSelectionNotice(null);
      return;
    }

    // Look up target variant under new color + selectedSizeId
    const targetVariant = variants?.find(
      v => v.isActive !== false &&
           getVariantColorId(v) === colorId &&
           getVariantSizeId(v) === selectedSizeId
    );

    if (isVariantBuyable(targetVariant)) {
      // Case A: valid & buyable in new color -> KEEP selected size
      setSizeSelectionNotice(null);
    } else {
      // Case B or C: variant is unavailable (sold out) or does not exist in new color
      const sizeLabel = variants?.find(v => getVariantSizeId(v) === selectedSizeId)?.size || 'выбранного размера';
      const colorObj = colors.find(c => c.id === colorId);
      const colorName = colorObj?.name || variants?.find(v => getVariantColorId(v) === colorId)?.colorName || variants?.find(v => getVariantColorId(v) === colorId)?.color;

      // Clear selected size (do NOT auto-pick S, L or nearest size)
      setSelectedSizeId(null);
      // Set contextual explanation
      setSizeSelectionNotice(formatSizeUnavailableNotice(sizeLabel, colorName));
    }
  }, [selectedColorId, selectedSizeId, variants, colors]);

  const selectSize = useCallback((sizeId: string) => {
    const dimType = getDimensionType(variants);
    const effColorId = (dimType === 'COLOR_AND_SIZE' || dimType === 'COLOR_ONLY')
      ? (selectedColorId || null)
      : null;
    const currentSizes = getSizeOptions(variants, effColorId, dimType, sizeChart);
    const targetSize = currentSizes.find(s => s.id === sizeId);
    if (targetSize && targetSize.disabled) {
      return;
    }
    setSelectedSizeId(sizeId);
    // User made an explicit size choice -> clear contextual notice
    setSizeSelectionNotice(null);
  }, [variants, selectedColorId, colors, sizeChart]);

  const clearNotice = useCallback(() => {
    setSizeSelectionNotice(null);
  }, []);

  const clearSelectedSize = useCallback(() => {
    setSelectedSizeId(null);
  }, []);

  const restoreSelection = useCallback((colorId: string | null, sizeId: string | null, notice: string | null = null) => {
    setSelectedColorId(colorId);
    setSelectedSizeId(sizeId);
    setSizeSelectionNotice(notice);
  }, []);

  const state = useMemo(() => {
    return selectVariantState(variants, selectedColorId, selectedSizeId, sizeChart, sizeSelectionNotice);
  }, [variants, selectedColorId, selectedSizeId, sizeChart, sizeSelectionNotice]);

  return {
    ...state,
    selectColor,
    selectSize,
    clearNotice,
    clearSelectedSize,
    setSelectedColorId,
    setSelectedSizeId,
    setSizeSelectionNotice,
    restoreSelection,
  };
}

export interface StaleStockReconciliationResult {
  nextSizeId: string | null;
  notice: string | null;
  isBuyable: boolean;
}

export function reconcileSelectionAfterStaleStock(
  dimensionType: DimensionType,
  selectedColorId: string | null,
  selectedSizeId: string | null,
  refreshedVariants: ProductVariantItem[] | undefined,
  previousSizeLabel?: string | null
): StaleStockReconciliationResult {
  if (dimensionType === 'COLOR_AND_SIZE') {
    const targetVariant = refreshedVariants?.find(
      v => v.isActive !== false &&
           getVariantColorId(v) === selectedColorId &&
           getVariantSizeId(v) === selectedSizeId
    );
    if (isVariantBuyable(targetVariant)) {
      return { nextSizeId: selectedSizeId, notice: null, isBuyable: true };
    }
    return {
      nextSizeId: null,
      notice: formatStaleSizeNotice(previousSizeLabel),
      isBuyable: false,
    };
  }

  if (dimensionType === 'SIZE_ONLY') {
    const targetVariant = refreshedVariants?.find(
      v => v.isActive !== false &&
           getVariantSizeId(v) === selectedSizeId
    );
    if (isVariantBuyable(targetVariant)) {
      return { nextSizeId: selectedSizeId, notice: null, isBuyable: true };
    }
    return {
      nextSizeId: null,
      notice: formatStaleSizeNotice(previousSizeLabel),
      isBuyable: false,
    };
  }

  if (dimensionType === 'COLOR_ONLY') {
    const targetVariant = refreshedVariants?.find(
      v => v.isActive !== false &&
           getVariantColorId(v) === selectedColorId
    );
    if (isVariantBuyable(targetVariant)) {
      return { nextSizeId: null, notice: null, isBuyable: true };
    }
    return {
      nextSizeId: null,
      notice: 'Этот вариант только что закончился.',
      isBuyable: false,
    };
  }

  // SINGLE_VARIANT
  const targetVariant = refreshedVariants?.[0];
  if (isVariantBuyable(targetVariant)) {
    return { nextSizeId: null, notice: null, isBuyable: true };
  }
  return {
    nextSizeId: null,
    notice: 'Этот вариант только что закончился.',
    isBuyable: false,
  };
}

export function isLightColor(hex?: string): boolean {
  if (!hex) return false;
  let c = hex.trim().replace('#', '');
  if (c.length === 3) {
    c = c.split('').map(x => x + x).join('');
  }
  if (c.length !== 6) return false;
  const r = parseInt(c.substring(0, 2), 16);
  const g = parseInt(c.substring(2, 4), 16);
  const b = parseInt(c.substring(4, 6), 16);
  if (isNaN(r) || isNaN(g) || isNaN(b)) return false;
  const brightness = (r * 299 + g * 587 + b * 114) / 1000;
  return brightness > 190;
}

export function formatVariantDetails(color?: string | null, size?: string | null): string | null {
  const parts: string[] = [];
  if (color && color.trim()) {
    parts.push(color.trim());
  }
  if (size && size.trim()) {
    parts.push(size.trim());
  }
  if (parts.length === 0) return null;
  return parts.join(' · ');
}

export function getCartItemImageUrl(
  imageUrl?: string | null,
  fallbackImage?: string | null,
  placeholderImage: string = 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image'
): string {
  if (imageUrl && imageUrl.trim()) {
    return imageUrl.trim();
  }
  if (fallbackImage && fallbackImage.trim()) {
    return fallbackImage.trim();
  }
  return placeholderImage;
}
