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

export interface SizeOption {
  id: string;
  label: string;
  inStock: boolean;
  disabled: boolean;
  variantId?: string;
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
    const isInStock = Boolean(v.inStock ?? v.isActive);

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
  dimensionType: DimensionType = 'COLOR_AND_SIZE',
  sizeChart?: any
): SizeOption[] {
  const active = variants?.filter(v => v.isActive !== false) || [];
  const matchingVariants = active.filter(v => {
    if (dimensionType === 'COLOR_AND_SIZE') {
      if (!selectedColorId) return false;
      return getVariantColorId(v) === selectedColorId;
    }
    return true;
  });

  const map = new Map<string, SizeOption>();
  for (const v of matchingVariants) {
    const sizeId = getVariantSizeId(v);
    if (!sizeId) continue;
    const label = v.size || 'Стандарт';
    const inStock = Boolean(v.inStock ?? v.isActive);

    const existing = map.get(sizeId);
    if (!existing) {
      map.set(sizeId, {
        id: sizeId,
        label,
        inStock,
        disabled: !inStock,
        variantId: v.id,
      });
    } else if (inStock) {
      existing.inStock = true;
      existing.disabled = false;
      existing.variantId = v.id;
    }
  }

  const list = Array.from(map.values());
  if (sizeChart?.rows) {
    const sortOrder = sizeChart.rows.map((r: any) => r.sizeValueName);
    list.sort((a, b) => {
      const idxA = sortOrder.indexOf(a.label);
      const idxB = sortOrder.indexOf(b.label);
      if (idxA !== -1 && idxB !== -1) return idxA - idxB;
      if (idxA !== -1) return -1;
      if (idxB !== -1) return 1;
      return 0;
    });
  }
  return list;
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
  sizeChart?: any
): VariantSelectionState {
  const dimensionType = getDimensionType(variants);
  const colors = getColorOptions(variants);

  const effectiveColorId = (dimensionType === 'COLOR_AND_SIZE' || dimensionType === 'COLOR_ONLY')
    ? (selectedColorId !== undefined && selectedColorId !== null ? selectedColorId : getDefaultColorId(colors))
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
  const canAddToCart = isResolved && Boolean(selectedVariant.inStock ?? selectedVariant.isActive);

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
    return getDefaultColorId(colors);
  });

  const [selectedSizeId, setSelectedSizeId] = useState<string | null>(() => {
    return initialSizeId ?? null;
  });

  useEffect(() => {
    if (colors.length > 0 && !selectedColorId) {
      setSelectedColorId(getDefaultColorId(colors));
    }
  }, [colors, selectedColorId]);

  const selectColor = useCallback((colorId: string) => {
    setSelectedColorId(colorId);
    // CANONICAL INVARIANT: Switching color clears the selected size!
    setSelectedSizeId(null);
  }, []);

  const selectSize = useCallback((sizeId: string) => {
    setSelectedSizeId(sizeId);
  }, []);

  const state = useMemo(() => {
    return selectVariantState(variants, selectedColorId, selectedSizeId, sizeChart);
  }, [variants, selectedColorId, selectedSizeId, sizeChart]);

  return {
    ...state,
    selectColor,
    selectSize,
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
