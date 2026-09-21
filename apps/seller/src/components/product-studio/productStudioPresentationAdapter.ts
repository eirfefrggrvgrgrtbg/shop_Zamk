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

  const variants = draft.variants || [];
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

  // Size options mapping from draft variants (keyed by canonical sizeValueId)
  const sizeMap = new Map<string, { id: string; label: string }>();
  for (const v of variants) {
    if (v.sizeValueId && !sizeMap.has(v.sizeValueId)) {
      sizeMap.set(v.sizeValueId, {
        id: v.sizeValueId,
        label: v.size || v.sizeValueId,
      });
    }
  }
  const uniqueSizes = Array.from(sizeMap.values());

  // Dimension type derivation
  let dimensionType: ProductPresentationDimensionType = "SINGLE_VARIANT";
  if (colors.length > 0 && uniqueSizes.length > 0) {
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
 * Invariants:
 * - Before color selection in COLOR_AND_SIZE: all sizes are visible but disabled.
 * - After color selection: offered combinations become AVAILABLE; absent combinations become NOT_OFFERED.
 * - No SOLD_OUT is derived from draft (stock is warehouse truth, not draft truth).
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

  if (dimensionType === "SIZE_ONLY") {
    return uniqueSizes.map((s) => ({
      id: s.id,
      label: s.label,
      state: "AVAILABLE",
      disabled: false,
    }));
  }

  // COLOR_AND_SIZE
  if (!selectedColorId) {
    // Before color selection: all sizes visible but disabled
    return uniqueSizes.map((s) => ({
      id: s.id,
      label: s.label,
      state: "AVAILABLE",
      disabled: true,
    }));
  }

  // After color selection
  return uniqueSizes.map((s) => {
    const isOffered = (draft.variants || []).some(
      (v) => v.colorId === selectedColorId && v.sizeValueId === s.id
    );

    return {
      id: s.id,
      label: s.label,
      state: isOffered ? "AVAILABLE" : "NOT_OFFERED",
      disabled: !isOffered,
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
  const variants = draft.variants || [];

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
