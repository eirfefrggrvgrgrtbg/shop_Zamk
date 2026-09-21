import type { ProductStudioDraft } from '../../contexts/ProductStudioContext';
import type { SellerCategorySchema } from '@zamk/api-client/src/seller';
import { MIN_PRODUCT_IMAGES } from './productStudioMediaHelper';

export type ProductStudioBlockerField =
  | 'title'
  | 'category'
  | 'price'
  | 'media'
  | 'color'
  | 'size'
  | 'description'
  | 'composition'
  | 'characteristics'
  | 'sizeChart';

export interface FieldReadinessStatus {
  required: boolean;
  isSatisfied: boolean;
}

export interface ProductStudioReadiness {
  blockingFields: ProductStudioBlockerField[];
  warnings: string[];
  isReadyForSave: boolean;
  isReadyForModeration: boolean;
  fieldStatus: Record<ProductStudioBlockerField, FieldReadinessStatus>;
}

/**
 * Determines whether the given schema or draft indicates color is required.
 */
export function isColorRequired(
  draft: ProductStudioDraft,
  schema?: SellerCategorySchema | null
): boolean {
  if (draft.dimensionType === 'ONLY_SIZE' || draft.dimensionType === 'SIZE_ONLY' || draft.dimensionType === 'SINGLE_VARIANT') {
    return false;
  }
  if (draft.dimensionType === 'COLOR_AND_SIZE' || draft.dimensionType === 'COLOR_ONLY' || draft.dimensionType === 'ONLY_COLOR') {
    return true;
  }
  if (schema) {
    if (schema.dimensionType === 'ONLY_SIZE' || schema.dimensionType === 'SIZE_ONLY' || schema.dimensionType === 'SINGLE_VARIANT') {
      return false;
    }
    if (schema.dimensionType === 'COLOR_AND_SIZE' || schema.dimensionType === 'COLOR_ONLY' || schema.dimensionType === 'ONLY_COLOR') {
      return true;
    }
    const hasColorAttr = schema.attributes?.some(
      (a) => a.valueSource === 'VARIANT_COLOR' && a.required
    );
    if (hasColorAttr !== undefined) {
      return hasColorAttr;
    }
    // If schema is present without explicit dimensionType, default to true
    return true;
  }
  // Before category/schema is selected: we do not yet know whether color is required
  return false;
}

/**
 * Determines whether the given schema or draft indicates size is required.
 */
export function isSizeRequired(
  draft: ProductStudioDraft,
  schema?: SellerCategorySchema | null
): boolean {
  if (draft.dimensionType === 'COLOR_ONLY' || draft.dimensionType === 'ONLY_COLOR' || draft.dimensionType === 'SINGLE_VARIANT') {
    return false;
  }
  if (draft.dimensionType === 'COLOR_AND_SIZE' || draft.dimensionType === 'ONLY_SIZE' || draft.dimensionType === 'SIZE_ONLY') {
    return true;
  }
  if (schema) {
    if (schema.dimensionType === 'COLOR_ONLY' || schema.dimensionType === 'ONLY_COLOR' || schema.dimensionType === 'SINGLE_VARIANT') {
      return false;
    }
    if (schema.dimensionType === 'COLOR_AND_SIZE' || schema.dimensionType === 'ONLY_SIZE' || schema.dimensionType === 'SIZE_ONLY') {
      return true;
    }
    const hasSizeAttr = schema.attributes?.some(
      (a) => a.valueSource === 'VARIANT_SIZE' && a.required
    );
    if (hasSizeAttr !== undefined) {
      return hasSizeAttr;
    }
    // If schema is present without explicit dimensionType, default to true
    return true;
  }
  // Before category/schema is selected: we do not yet know whether size is required
  return false;
}

export interface OfferedSize {
  sizeValueId: string;
  sizeValueName: string;
}

export function getOfferedSizes(draft: ProductStudioDraft): OfferedSize[] {
  const map = new Map<string, string>();
  for (const v of draft.variants || []) {
    const id = v.sizeValueId || v.size;
    if (id && !map.has(id)) {
      map.set(id, v.size || id);
    }
  }
  if (map.size === 0 && Array.isArray((draft as any).sizes)) {
    for (const s of (draft as any).sizes) {
      const id = s.id || s.sizeValueId || s.name || s.label;
      if (id && !map.has(id)) {
        map.set(id, s.label || s.name || id);
      }
    }
  }
  return Array.from(map.entries()).map(([sizeValueId, sizeValueName]) => ({
    sizeValueId,
    sizeValueName,
  }));
}

export interface SizeChartMissingCell {
  sizeValueId: string;
  sizeValueName: string;
  fieldCode: string;
  fieldName: string;
}

export interface SizeChartCompleteness {
  isNeeded: boolean;
  requiredCellCount: number;
  filledRequiredCellCount: number;
  isComplete: boolean;
  missingCells: SizeChartMissingCell[];
}

export function getSizeChartCompleteness(
  draft: ProductStudioDraft,
  schema?: SellerCategorySchema | null
): SizeChartCompleteness {
  const categoryUsesSizes =
    schema?.dimensionType === 'COLOR_AND_SIZE' ||
    schema?.dimensionType === 'ONLY_SIZE' ||
    schema?.dimensionType === 'SIZE_ONLY' ||
    Boolean(schema?.sizeChartRequired) ||
    Boolean(schema?.attributes?.some((a) => a.valueSource === 'VARIANT_SIZE' && a.required));

  if (!schema || !categoryUsesSizes) {
    return {
      isNeeded: false,
      requiredCellCount: 0,
      filledRequiredCellCount: 0,
      isComplete: true,
      missingCells: [],
    };
  }

  const offeredSizes = getOfferedSizes(draft);
  if (offeredSizes.length === 0) {
    return {
      isNeeded: false,
      requiredCellCount: 0,
      filledRequiredCellCount: 0,
      isComplete: true,
      missingCells: [],
    };
  }

  const chartFields = schema.sizeChartFields || [];
  const requiredFields = chartFields.filter((f) => f.isRequired);

  if (requiredFields.length === 0) {
    return {
      isNeeded: true,
      requiredCellCount: 0,
      filledRequiredCellCount: 0,
      isComplete: true,
      missingCells: [],
    };
  }

  const totalRequiredCells = offeredSizes.length * requiredFields.length;
  let filledRequiredCells = 0;
  const missingCells: SizeChartMissingCell[] = [];

  const rows = draft.sizeChart?.rows || [];

  for (const size of offeredSizes) {
    const row = rows.find(
      (r: any) =>
        (r.sizeValueId && r.sizeValueId === size.sizeValueId) ||
        (r.sizeValueName && r.sizeValueName === size.sizeValueName) ||
        (r.size && (r.size === size.sizeValueName || r.size === size.sizeValueId))
    );

    for (const field of requiredFields) {
      const val = row?.measurements?.[field.code];
      let isValid = false;
      if (typeof val === 'number') {
        isValid = !isNaN(val) && val > 0;
      } else if (typeof val === 'string') {
        const parsed = parseFloat(val.trim().replace(',', '.'));
        isValid = !isNaN(parsed) && parsed > 0;
      }

      if (isValid) {
        filledRequiredCells++;
      } else {
        missingCells.push({
          sizeValueId: size.sizeValueId,
          sizeValueName: size.sizeValueName,
          fieldCode: field.code,
          fieldName: field.name,
        });
      }
    }
  }

  return {
    isNeeded: true,
    requiredCellCount: totalRequiredCells,
    filledRequiredCellCount: filledRequiredCells,
    isComplete: filledRequiredCells === totalRequiredCells,
    missingCells,
  };
}

export interface CompositionCompleteness {
  isComplete: boolean;
  totalPercentage: number;
  rowCount: number;
  allRowsValid: boolean;
}

export function getCompositionCompleteness(draft: ProductStudioDraft): CompositionCompleteness {
  const rows = draft.materialComposition || [];
  if (rows.length === 0) {
    // Legacy backwards compatibility: ONLY if existing product in edit mode (draft.id exists) and draft.material exists
    const hasLegacyOnly = Boolean(draft.id && draft.material?.trim());
    return {
      isComplete: hasLegacyOnly,
      totalPercentage: hasLegacyOnly ? 100 : 0,
      rowCount: 0,
      allRowsValid: hasLegacyOnly,
    };
  }

  let total = 0;
  let allRowsValid = true;

  for (const r of rows) {
    const hasMat = Boolean(
      (r.materialId && r.materialId.trim()) ||
      (r.materialName && r.materialName.trim()) ||
      (r.material && r.material.trim())
    );
    const pct = typeof r.percentage === 'number' ? r.percentage : parseFloat(String(r.percentage));
    const validPct = !isNaN(pct) && pct > 0 && pct <= 100;
    if (!hasMat || !validPct) {
      allRowsValid = false;
    }
    if (!isNaN(pct)) {
      total += pct;
    }
  }

  const isComplete = rows.length > 0 && allRowsValid && Math.abs(total - 100) < 0.001;

  return {
    isComplete,
    totalPercentage: total,
    rowCount: rows.length,
    allRowsValid,
  };
}

/**
 * Returns canonical product-level attributes for a category schema,
 * excluding variant-axis and special material composition attributes.
 */
export function getCanonicalProductAttributes(schema?: SellerCategorySchema | null) {
  return (schema?.attributes || []).filter(
    (a) => a.scope === 'PRODUCT' && a.valueSource !== 'MATERIAL_COMPOSITION'
  );
}

export function getCanonicalRequiredProductAttributes(schema?: SellerCategorySchema | null) {
  return getCanonicalProductAttributes(schema).filter((a) => a.required);
}

/**
 * Unified canonical readiness evaluator for Product Studio (Visual and Form).
 */
export function getProductStudioReadiness(
  draft: ProductStudioDraft,
  schema?: SellerCategorySchema | null
): ProductStudioReadiness {
  const blockingFields: ProductStudioBlockerField[] = [];
  const warnings: string[] = [];

  // 1. Title
  const titleSatisfied = Boolean(draft.title?.trim());
  if (!titleSatisfied) {
    blockingFields.push('title');
  }

  // 2. Category
  const categorySatisfied = Boolean(draft.categoryId);
  if (!categorySatisfied) {
    blockingFields.push('category');
  }

  // 3. Price
  const priceVal = draft.priceCents ?? 0;
  const variants = draft.variants || [];
  const allVariantsHavePrice =
    variants.length > 0
      ? variants.every((v) => (v.priceCents ?? priceVal) > 0)
      : priceVal > 0;
  const priceSatisfied = priceVal > 0 || (variants.length > 0 && allVariantsHavePrice);
  if (!priceSatisfied) {
    blockingFields.push('price');
  }

  // 4. Media
  const mediaCount = draft.images?.length ?? 0;
  const mediaSatisfied = mediaCount >= MIN_PRODUCT_IMAGES;
  if (!mediaSatisfied) {
    blockingFields.push('media');
  }

  // 5. Color
  const colorNeeded = isColorRequired(draft, schema);
  const colorSatisfied = !colorNeeded || (draft.colors?.length ?? 0) > 0;
  if (!colorSatisfied) {
    blockingFields.push('color');
  }

  // 6. Size
  const sizeNeeded = isSizeRequired(draft, schema);
  const hasSelectedSizes = variants.some((v) => Boolean(v.sizeValueId || v.size));
  const sizeSatisfied = !sizeNeeded || hasSelectedSizes;
  if (!sizeSatisfied) {
    blockingFields.push('size');
  }

  // 7. Description (REQUIRED)
  const descriptionSatisfied = Boolean(draft.description?.trim());
  if (!descriptionSatisfied) {
    blockingFields.push('description');
  }

  // 8. Composition (REQUIRED - structured rows summing to 100%)
  const compositionCompleteness = getCompositionCompleteness(draft);
  const compositionSatisfied = compositionCompleteness.isComplete;
  if (!compositionSatisfied) {
    blockingFields.push('composition');
  }

  // 9. Characteristics (Schema category-driven required product attributes)
  const requiredProductAttrs = getCanonicalRequiredProductAttributes(schema);
  const characteristicsNeeded = requiredProductAttrs.length > 0;
  const characteristicsSatisfied =
    !characteristicsNeeded ||
    requiredProductAttrs.every((reqAttr) => {
      const found = draft.attributes?.find(
        (a) =>
          a.attributeDefinitionId === reqAttr.id ||
          (reqAttr.nameRu && a.name === reqAttr.nameRu)
      );
      if (!found) return false;
      if (found.dictionaryValueId) return true;
      if (typeof found.value === 'string') return found.value.trim().length > 0;
      if (typeof found.value === 'number') return !isNaN(found.value);
      if (typeof found.value === 'boolean') return true;
      return false;
    });
  if (characteristicsNeeded && !characteristicsSatisfied) {
    blockingFields.push('characteristics');
  }

  // 10. Size Chart (REQUIRED when category uses sizes AND offered sizes exist)
  const sizeChartCompleteness = getSizeChartCompleteness(draft, schema);
  const sizeChartNeeded = sizeChartCompleteness.isNeeded;
  const sizeChartSatisfied = sizeChartCompleteness.isComplete;
  if (sizeChartNeeded && !sizeChartSatisfied) {
    blockingFields.push('sizeChart');
  }

  const fieldStatus: Record<ProductStudioBlockerField, FieldReadinessStatus> = {
    title: { required: true, isSatisfied: titleSatisfied },
    category: { required: true, isSatisfied: categorySatisfied },
    price: { required: true, isSatisfied: priceSatisfied },
    media: { required: true, isSatisfied: mediaSatisfied },
    color: { required: colorNeeded, isSatisfied: colorSatisfied },
    size: { required: sizeNeeded, isSatisfied: sizeSatisfied },
    description: { required: true, isSatisfied: descriptionSatisfied },
    composition: { required: true, isSatisfied: compositionSatisfied },
    characteristics: { required: characteristicsNeeded, isSatisfied: characteristicsSatisfied },
    sizeChart: { required: sizeChartNeeded, isSatisfied: sizeChartSatisfied },
  };

  return {
    blockingFields,
    warnings,
    // In PS.R4A persistence and submission are strictly not enabled:
    isReadyForSave: false,
    isReadyForModeration: false,
    fieldStatus,
  };
}
