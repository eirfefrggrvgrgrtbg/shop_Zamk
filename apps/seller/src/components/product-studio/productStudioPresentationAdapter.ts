import type {
  ProductPresentationCoreProduct,
  ProductPresentationMediaItem,
  ProductPresentationColorOption,
  ProductPresentationSizeOption,
  ProductPresentationDimensionType,
  ProductPresentationSelectedVariant,
} from "@zamk/shared";
import type { ProductStudioDraft, ProductStudioVariant } from "../../contexts/ProductStudioContext";
import { getProductStudioImageDisplayUrl } from "./productStudioMediaHelper";

export const STUDIO_PREVIEW_PLACEHOLDER_IMAGE =
  "data:image/svg+xml;utf8,<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"900\" height=\"1200\" viewBox=\"0 0 900 1200\"><rect width=\"900\" height=\"1200\" fill=\"%23f1f5f9\"/><rect x=\"140\" y=\"220\" width=\"620\" height=\"760\" rx=\"42\" fill=\"none\" stroke=\"%23cbd5e1\" stroke-width=\"12\" stroke-dasharray=\"28 24\"/><text x=\"450\" y=\"610\" text-anchor=\"middle\" font-family=\"Arial,sans-serif\" font-size=\"38\" fill=\"%2364758b\">Нет изображения</text></svg>";

export interface StudioPresentationModel {
  product: ProductPresentationCoreProduct;
  visibleImages: ProductPresentationMediaItem[];
  colors: ProductPresentationColorOption[];
  dimensionType: ProductPresentationDimensionType;
  uniqueSizes: Array<{ id: string; label: string }>;
  basePrice: number;
  hasVariants: boolean;
}

export const isUuid = (str?: string | null): boolean =>
  Boolean(str && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(str.trim()));

export function getCanonicalSizeLabel(
  sizeValueId: string,
  sizesList?: Array<{ id: string; value?: string; nameRu?: string }> | null,
  fallbackLabel?: string | null
): string {
  if (sizesList && sizesList.length > 0) {
    const found = sizesList.find((s) => s.id === sizeValueId);
    if (found && found.value && !isUuid(found.value)) {
      return found.value;
    }
  }

  if (fallbackLabel && fallbackLabel.trim() !== '' && !isUuid(fallbackLabel)) {
    return fallbackLabel.trim();
  }

  return 'Размер недоступен';
}

/**
 * Pure adapter converting ProductStudioDraft into presentation data models for @zamk/shared.
 */
export function mapStudioDraftToPresentation(draft: ProductStudioDraft): StudioPresentationModel {
  // Title mapping: restrained placeholder if empty
  const name = draft.title?.trim() ? draft.title : "Название товара";

  // Price mapping (cents to rubles, exactly once)
  // Semantics for ProductPresentationCore:
  // - product.price: The original/base price (used for % calculation and cross-out).
  // - product.discountPrice: The current selling price if discounted (red price).
  const draftPrice = draft.priceCents !== undefined ? draft.priceCents / 100 : 0;
  const draftOldPrice = draft.oldPriceCents !== undefined ? draft.oldPriceCents / 100 : undefined;

  const hasDiscount = draftOldPrice !== undefined && draftOldPrice > draftPrice;
  const presentationPrice = hasDiscount ? draftOldPrice : draftPrice;
  const presentationDiscountPrice = hasDiscount ? draftPrice : undefined;

  // basePrice is passed as displayPrice. If discounted, it must be the original price to be crossed out.
  const basePrice = presentationPrice;

  // Media mapping: ordered images with fallback placeholder
  const sortedImages = [...(draft.images || [])].sort((a, b) => {
    if (a.isMain && !b.isMain) return -1;
    if (!a.isMain && b.isMain) return 1;
    if (a.sortOrder !== undefined && b.sortOrder !== undefined) {
      return a.sortOrder - b.sortOrder;
    }
    return 0;
  });

  const visibleImages: ProductPresentationMediaItem[] = sortedImages.map((img) => ({
    url: getProductStudioImageDisplayUrl(img),
    colorId: img.colorId || undefined,
  }));

  const variants = (draft.variants || []).filter((v) => v.isActive !== false);
  const hasVariants = variants.length > 0;

  // Color options mapping from draft.colors and draft variants (keyed strictly by canonical colorId)
  const colorMap = new Map<string, ProductPresentationColorOption>();
  for (const c of draft.colors || []) {
    if (c.id && !colorMap.has(c.id)) {
      colorMap.set(c.id, {
        id: c.id,
        name: c.name || "Цвет",
        hex: c.hex || (c as any).hexValue || undefined,
        hasInStock: true,
      });
    }
  }
  for (const v of variants) {
    if (v.colorId && !colorMap.has(v.colorId)) {
      colorMap.set(v.colorId, {
        id: v.colorId,
        name: v.colorName || "Цвет",
        hex: v.colorHex || undefined,
        hasInStock: true, // all structurally offered draft combinations are preview-enabled
      });
    }
  }
  const colors = Array.from(colorMap.values());

  const sizeMap = new Map<string, { id: string; label: string }>();
  for (const v of draft.variants || []) {
    if (v.sizeValueId && !sizeMap.has(v.sizeValueId)) {
      const label = getCanonicalSizeLabel(v.sizeValueId, null, v.size);
      sizeMap.set(v.sizeValueId, {
        id: v.sizeValueId,
        label,
      });
    }
  }
  const uniqueSizes = Array.from(sizeMap.values());

  // Dimension type derivation: respect explicit draft.dimensionType if present and canonical
  let dimensionType: ProductPresentationDimensionType = "SINGLE_VARIANT";
  if (draft.dimensionType && ["COLOR_AND_SIZE", "COLOR_ONLY", "SIZE_ONLY", "SINGLE_VARIANT"].includes(draft.dimensionType)) {
    dimensionType = draft.dimensionType as ProductPresentationDimensionType;
  } else if (colors.length > 0 && uniqueSizes.length > 0) {
    dimensionType = "COLOR_AND_SIZE";
  } else if (colors.length > 0) {
    dimensionType = "COLOR_ONLY";
  } else if (uniqueSizes.length > 0) {
    dimensionType = "SIZE_ONLY";
  }

  // Product presentation model
  const product: ProductPresentationCoreProduct = {
    id: draft.id,
    name,
    brand: draft.brandName || undefined,
    brandId: draft.brandId || undefined,
    category: draft.categoryName || undefined,
    description: draft.description || "",
    materials: draft.material || undefined,
    materialComposition: draft.materialComposition?.map((m) => ({
      materialName: m.materialName,
      material: m.materialName,
      percentage: m.percentage ?? 0,
    })),
    careInstructions: draft.careInstructions || undefined,
    price: presentationPrice,
    discountPrice: presentationDiscountPrice,
    isNew: false,
    sizeChart: draft.sizeChart,
    attributes: draft.attributes
      ?.map((attr) => ({
        label: attr.name || '',
        value: typeof attr.value === 'boolean' ? (attr.value ? 'Да' : 'Нет') : String(attr.value ?? ''),
      }))
      .filter((attr) => attr.label && attr.value),
  };

  return {
    product,
    visibleImages,
    colors,
    dimensionType,
    uniqueSizes,
    basePrice,
    hasVariants,
  };
}

/**
 * Computes presentation size options given draft structure and current preview color selection.
 * Presentation domain contract (PS.R4B3.1C4C3B2G1):
 * - Always renders every configured product size (complete domain).
 * - If dimensionType is COLOR_ONLY or SINGLE_VARIANT: returns [].
 * - If dimensionType is SIZE_ONLY: all uniqueSizes are AVAILABLE and enabled.
 * - If dimensionType is COLOR_AND_SIZE:
 *   - If no selectedColorId: every size is AVAILABLE and enabled (disabled = false).
 *   - If selectedColorId is set: a size S is enabled iff an existing current variant satisfies
 *     variant.colorId === selectedColorId && variant.sizeValueId === S.
 *     Otherwise size remains visible with state: 'UNAVAILABLE' (or 'NOT_OFFERED') and disabled: true.
 */
export function computePresentationSizes(
  draft: ProductStudioDraft,
  dimensionType: ProductPresentationDimensionType,
  uniqueSizes: Array<{ id: string; label: string }>,
  selectedColorId: string | null
): ProductPresentationSizeOption[] {
  if (dimensionType === "COLOR_ONLY" || dimensionType === "SINGLE_VARIANT") {
    return [];
  }

  if (dimensionType === "SIZE_ONLY" || !selectedColorId) {
    return uniqueSizes.map((s) => ({
      id: s.id,
      label: s.label,
      state: "AVAILABLE",
      disabled: false,
    }));
  }

  // COLOR_AND_SIZE with selectedColorId: all sizes remain visible, incompatible become disabled
  const variants = (draft.variants || []).filter((v) => v.isActive !== false);
  return uniqueSizes.map((s) => {
    const isCompatible = variants.some(
      (v) => v.colorId === selectedColorId && v.sizeValueId === s.id
    );
    return {
      id: s.id,
      label: s.label,
      state: isCompatible ? "AVAILABLE" : "NOT_OFFERED",
      disabled: !isCompatible,
    };
  });
}

/**
 * Finds matching variant in draft for resolved preview state.
 */
export function findMatchingDraftVariant(
  draft: ProductStudioDraft,
  dimensionType: ProductPresentationDimensionType,
  selectedColorId: string | null,
  selectedSizeId: string | null
): ProductStudioVariant | null {
  const variants = (draft.variants || []).filter((v) => v.isActive !== false);

  if (dimensionType === "COLOR_AND_SIZE") {
    if (!selectedColorId || !selectedSizeId) return null;
    return (
      variants.find(
        (v) => v.colorId === selectedColorId && v.sizeValueId === selectedSizeId
      ) || null
    );
  }

  if (dimensionType === "COLOR_ONLY") {
    if (!selectedColorId) return null;
    return variants.find((v) => v.colorId === selectedColorId) || null;
  }

  if (dimensionType === "SIZE_ONLY") {
    if (!selectedSizeId) return null;
    return variants.find((v) => v.sizeValueId === selectedSizeId) || null;
  }

  // SINGLE_VARIANT
  return variants[0] || null;
}

/**
 * Maps matched variant to presentation selected variant model.
 */
export function mapToPresentationSelectedVariant(
  variant: ProductStudioVariant | null
): ProductPresentationSelectedVariant | null {
  if (!variant) return null;
  return {
    id: variant.id,
    sellerSku: variant.sellerSku,
    sku: variant.sellerSku,
    priceCents: variant.priceCents,
    size: variant.size,
  };
}

/**
 * Computes presentation color options given draft structure and current preview size selection.
 * Presentation domain contract (PS.R4B3.1C4C3B2G1):
 * - Always renders every configured product color (complete domain).
 * - If no selectedSizeId is set: every color is AVAILABLE and enabled (disabled = false).
 * - If selectedSizeId is set: a color C is enabled iff an existing current variant satisfies
 *   variant.colorId === C && variant.sizeValueId === selectedSizeId.
 *   Otherwise color remains visible with state: 'UNAVAILABLE' and disabled: true.
 */
export function computePresentationColors(
  draft: ProductStudioDraft,
  allColors: ProductPresentationColorOption[],
  selectedSizeId: string | null
): ProductPresentationColorOption[] {
  if (!selectedSizeId) {
    return allColors.map((color) => ({
      ...color,
      state: "AVAILABLE" as const,
      disabled: false,
    }));
  }

  const variants = (draft.variants || []).filter((v) => v.isActive !== false);
  return allColors.map((color) => {
    const isCompatible = variants.some(
      (v) => v.sizeValueId === selectedSizeId && v.colorId === color.id
    );
    return {
      ...color,
      state: (isCompatible ? "AVAILABLE" : "UNAVAILABLE") as "AVAILABLE" | "UNAVAILABLE",
      disabled: !isCompatible,
    };
  });
}

/**
 * Focuses the first media item matching the given colorId.
 * Fallback to 0 (canonical/main media).
 */
export function findFirstMediaIndexForColor(
  images: ProductPresentationMediaItem[],
  colorId: string | null
): number {
  if (!colorId) return 0;
  const index = images.findIndex((img) => img.colorId === colorId);
  return index !== -1 ? index : 0;
}

/**
 * Extracts canonical unique color IDs available in the draft.
 */
export function getCanonicalColorIds(draft: ProductStudioDraft): string[] {
  const colorIds = new Set<string>();
  for (const c of draft.colors || []) {
    if (c.id) {
      colorIds.add(c.id);
    }
  }
  for (const v of draft.variants || []) {
    if (v.isActive !== false && v.colorId) {
      colorIds.add(v.colorId);
    }
  }
  return Array.from(colorIds);
}

/**
 * Deterministically resolves the preview selectedColorId:
 * - Does NOT auto-select when 1 color exists.
 * - Preserves current selectedColorId if it is still valid in draft colors.
 * - If invalid or not present: returns null.
 */
export function resolveDeterministicPreviewColorId(
  draft: ProductStudioDraft,
  currentColorId: string | null
): string | null {
  if (!currentColorId) {
    return null;
  }
  const available = getCanonicalColorIds(draft);
  if (available.includes(currentColorId)) {
    return currentColorId;
  }
  return null;
}
