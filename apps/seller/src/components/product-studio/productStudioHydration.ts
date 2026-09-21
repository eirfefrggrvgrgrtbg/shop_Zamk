import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerSizeValue,
  SellerDictionaryValue,
  ProductImage,
  ProductVariant,
  ProductAttributeValue,
  AdminProductMaterialComposition,
  AdminProductSizeChartRow,
} from '@zamk/api-client';
import type {
  ProductStudioDraft,
  ProductStudioImage,
  ProductStudioVariant,
} from '../../contexts/ProductStudioContext';
import type { ProductStudioAttributeItem } from './ProductStudioCharacteristicsModal';

export interface HydrateProductStudioDraftParams {
  product: SellerProduct;
  categorySchema: SellerCategorySchema;
  canonicalColors: SellerColor[];
  dictionaryValuesMap?: Record<string, SellerDictionaryValue[]>;
}

/**
 * Pure mapper: transforms a persisted SellerProduct and its canonical references
 * into a single hydrated ProductStudioDraft.
 *
 * Invariants:
 * - Brand: draft.brandId = product.brandId, draft.brandName = product.brandName.
 *   NEVER falls back to sellerName.
 * - Identity: existing variant.id, image.id, sizeValueId, colorId are strictly preserved.
 * - Colors: hydrated strictly through canonical IDs matching canonicalColors. ZERO label matching.
 * - Media: sorted deterministically by sortOrder with original API index tie-breaker.
 * - Composition: structured materialComposition rows mapped directly; legacy product.material preserved.
 * - ZeroCartesian: does NOT call Cartesian generators or mutation helpers.
 */
export function hydrateProductStudioDraft({
  product,
  categorySchema,
  canonicalColors,
  dictionaryValuesMap = {},
}: HydrateProductStudioDraftParams): ProductStudioDraft {
  if (!product) {
    throw new Error('Hydration error: product data is required');
  }

  // 1. Basics & Identity
  const draftId = product.id;
  const title = product.title ?? '';
  const description = product.description ?? '';
  const categoryId = product.categoryId;
  const categoryName = product.categoryName || categorySchema?.name || '';

  // HARD RULE: brandId and brandName come strictly from product truth, never from sellerName
  const brandId = product.brandId;
  const brandName = product.brandName || '';

  const status = product.status;
  const priceCents = product.priceCents;
  const oldPriceCents = product.oldPriceCents;
  const careInstructions = product.careInstructions ?? '';
  const gender = product.gender;
  const currency = product.currency || 'RUB';
  const dimensionType = categorySchema?.dimensionType || 'COLOR_AND_SIZE';

  // 2. Media Hydration
  const images: ProductStudioImage[] = (product.images || [])
    .map((img: ProductImage, originalIndex: number) => {
      const url = img.imageUrl || img.url || '';
      const imageId = img.id || '';
      return {
        uiKey: imageId,
        sortOrder: typeof img.sortOrder === 'number' ? img.sortOrder : originalIndex,
        colorId: img.colorId || null,
        isMain: Boolean(img.isMain),
        altText: img.altText ?? null,
        source: {
          kind: 'canonical' as const,
          imageId,
          url,
        },
        originalIndex,
      };
    })
    .sort((a, b) => {
      const sortA = typeof a.sortOrder === 'number' ? a.sortOrder : a.originalIndex;
      const sortB = typeof b.sortOrder === 'number' ? b.sortOrder : b.originalIndex;
      if (sortA !== sortB) {
        return sortA - sortB;
      }
      return a.originalIndex - b.originalIndex;
    })
    .map(({ originalIndex: _, ...img }) => img);

  // 3. Colors Hydration (Strictly canonical IDs)
  const referencedColorIds = new Set<string>();
  (product.variants || []).forEach((v: ProductVariant) => {
    if (v.colorId) {
      referencedColorIds.add(v.colorId);
    }
  });
  (product.images || []).forEach((img: ProductImage) => {
    if (img.colorId) {
      referencedColorIds.add(img.colorId);
    }
  });

  const colors: Array<{ id: string; nameRu: string; hex?: string; code?: string }> = [];
  for (const cId of Array.from(referencedColorIds)) {
    const matched = (canonicalColors || []).find((c: SellerColor) => c.id === cId);
    if (!matched) {
      throw new Error(
        `Data integrity error: product references unknown colorId "${cId}" not found in canonical reference dictionary`
      );
    }
    colors.push({
      id: matched.id,
      nameRu: matched.nameRu,
      hex: matched.hex || (matched as any).hexValue,
      code: matched.code,
    });
  }

  // 4. Variants Hydration (Preserve exact variant IDs)
  const variants: ProductStudioVariant[] = (product.variants || []).map((v: ProductVariant) => {
    return {
      id: v.id,
      colorId: v.colorId || undefined,
      colorName: v.colorName || undefined,
      colorHex: v.colorHex || undefined,
      sizeValueId: v.sizeValueId || undefined,
      size: v.size || (v as any).sizeName || undefined,
      sellerSku: v.sellerSku || (v as any).sku || undefined,
      barcode: v.barcode || undefined,
      priceCents: typeof v.priceCents === 'number' ? v.priceCents : product.priceCents,
      isActive: v.isActive !== false,
    };
  });

  // 5. Attributes Hydration
  const schemaAttrs = categorySchema?.attributes || [];
  const attributes: ProductStudioAttributeItem[] = (product.attributes || []).map((attr: ProductAttributeValue) => {
    const def = schemaAttrs.find((a: any) => a.id === attr.attributeDefinitionId);
    if (!def) {
      throw new Error(
        `Data integrity error: product references attributeDefinitionId "${attr.attributeDefinitionId}" not found in category schema`
      );
    }

    let val: any = undefined;

    if (attr.textValue !== undefined && attr.textValue !== null) {
      val = attr.textValue;
    } else if (attr.numberValue !== undefined && attr.numberValue !== null) {
      val = attr.numberValue;
    } else if (attr.boolValue !== undefined && attr.boolValue !== null) {
      val = attr.boolValue;
    }

    const dictValId = attr.enumValueId || undefined;
    if (dictValId) {
      if (!def.dictionaryId) {
        throw new Error(
          `Data integrity error: attribute "${attr.attributeDefinitionId}" has enumValueId "${dictValId}" but category schema does not define a dictionaryId`
        );
      }
      const dictOptions = dictionaryValuesMap[def.dictionaryId];
      if (!dictOptions) {
        throw new Error(
          `Data integrity error: missing canonical dictionary data for dictionaryId "${def.dictionaryId}"`
        );
      }
      const matchedOpt = dictOptions.find((opt: SellerDictionaryValue) => opt.id === dictValId);
      if (!matchedOpt) {
        throw new Error(
          `Data integrity error: product references unknown enumValueId "${dictValId}" not found in canonical dictionary "${def.dictionaryId}"`
        );
      }
      val = matchedOpt.nameRu;
    }

    return {
      attributeDefinitionId: attr.attributeDefinitionId,
      code: def.code,
      name: def.nameRu,
      dictionaryValueId: dictValId,
      value: val,
    };
  });

  // 6. Composition Hydration
  const materialComposition =
    product.materialComposition && product.materialComposition.length > 0
      ? product.materialComposition.map((row: AdminProductMaterialComposition) => ({
          materialId: row.materialId,
          materialName: row.materialName,
          percentage: row.percentage,
        }))
      : [];

  const legacyMaterial = product.material || '';

  // 7. Size Chart Hydration
  const sizeChart = product.sizeChart
    ? {
        id: product.sizeChart.id,
        categoryId: product.sizeChart.categoryId,
        rows: (product.sizeChart.rows || []).map((r: AdminProductSizeChartRow) => ({
          sizeChartId: r.sizeChartId,
          sizeValueId: r.sizeValueId,
          sizeValueName: r.sizeValueName,
          measurements: r.measurements || {},
        })),
      }
    : undefined;

  return {
    id: draftId,
    title,
    description,
    categoryId,
    categoryName,
    brandId,
    brandName,
    status,
    priceCents,
    oldPriceCents,
    images,
    colors,
    dimensionType,
    variants,
    attributes,
    materialComposition,
    material: legacyMaterial,
    careInstructions,
    sizeChart,
    gender,
    currency,
  };
}

/**
 * Resolves the active size system for an existing product.
 *
 * Invariants:
 * 1. Product with NO existing sizeValueIds:
 *    - If schema has exactly one default system, return it.
 *    - If no default (or multiple/none), return systemId = null.
 *    - NEVER arbitrarily select allowedSizeSystems[0].
 * 2. Product WITH existing sizeValueIds:
 *    - Fetch canonical values for EVERY allowed size system.
 *    - If any required allowed-system fetch fails, return explicit hydration error.
 *      NEVER silently ignore or warn.
 *    - Find systems containing ALL existing sizeValueIds.
 *    - Exactly 1 match: return that systemId.
 *    - 0 matches: integrity error.
 *    - >1 matches: integrity error: "Размерная система товара определяется неоднозначно".
 *      NEVER choose default, NEVER choose first, NEVER label-match.
 */
export async function resolveSizeSystemForProduct(
  sizeValueIds: string[],
  categorySchema: SellerCategorySchema,
  fetchSizeValues: (systemId: string) => Promise<SellerSizeValue[]>
): Promise<{ systemId: string | null; error?: string }> {
  const allowedSystems = categorySchema?.allowedSizeSystems || [];

  if (sizeValueIds.length === 0) {
    const defaultSystems = allowedSystems.filter((s) => s.isDefault);
    if (defaultSystems.length === 1) {
      return { systemId: defaultSystems[0].id };
    }
    return { systemId: null };
  }

  if (allowedSystems.length === 0) {
    return {
      systemId: null,
      error: 'Категория не поддерживает размерные сетки',
    };
  }

  const matchingSystemIds: string[] = [];

  for (const sys of allowedSystems) {
    let values: SellerSizeValue[];
    try {
      values = await fetchSizeValues(sys.id);
    } catch (err: any) {
      return {
        systemId: null,
        error: `Data integrity error: failed to fetch canonical size values for allowed system "${sys.name || sys.id}": ${err?.message || 'unknown error'}`,
      };
    }

    const valueIdSet = new Set((values || []).map((v) => v.id));
    const allContained = sizeValueIds.every((id) => valueIdSet.has(id));
    if (allContained) {
      matchingSystemIds.push(sys.id);
    }
  }

  if (matchingSystemIds.length === 1) {
    return { systemId: matchingSystemIds[0] };
  }

  if (matchingSystemIds.length === 0) {
    return {
      systemId: null,
      error: `Data integrity error: product size values [${sizeValueIds.join(
        ', '
      )}] do not match any allowed size system for category`,
    };
  }

  return {
    systemId: null,
    error: 'Размерная система товара определяется неоднозначно',
  };
}
