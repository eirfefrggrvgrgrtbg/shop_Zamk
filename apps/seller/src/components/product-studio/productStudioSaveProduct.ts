import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerDictionaryValue,
  StageSellerProductImageResponse,
  SellerProductPatchImageItem,
} from '@zamk/api-client';
import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import {
  stagePendingProductStudioImages,
  buildProductPatchMediaPayload,
} from './productStudioSaveMedia';
import { hydrateProductStudioDraft } from './productStudioHydration';
import { updateSellerProduct, stageSellerProductImage, getSellerProduct } from '@zamk/api-client/src/seller';

export interface ProductStudioUpdateRequestPayload {
  title?: string;
  slug?: string;
  description?: string;
  categoryId?: string;
  brandId?: string;
  gender?: string;
  color?: string;
  material?: string;
  careInstructions?: string;
  priceCents?: number;
  oldPriceCents?: number;
  variants?: Array<{
    id?: string;
    colorId?: string;
    sizeValueId?: string;
    sellerSku?: string;
    barcode?: string;
    priceCents?: number;
  }>;
  materialComposition?: Array<{
    materialId: string;
    percentage: number;
  }>;
  sizeChartRows?: Array<{
    sizeValueId: string;
    measurements: Record<string, any>;
  }>;
  attributes?: Array<{
    attributeDefinitionId: string;
    enumValueId?: string;
    textValue?: string;
    numberValue?: number;
    boolValue?: boolean;
  }>;
  images?: SellerProductPatchImageItem[];
}

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Validates whether a variant ID represents a canonical backend UUID.
 * Rule:
 * - absent/empty => false
 * - valid RFC-compatible UUID string (case-insensitive) => true
 * - anything else (e.g. draft-var-*, synthetic-*, random strings) => false
 */
export function isCanonicalVariantId(id?: string): boolean {
  if (!id || typeof id !== 'string') return false;
  return UUID_REGEX.test(id.trim());
}

export function buildProductStudioUpdateRequest(
  workingDraft: ProductStudioDraft,
  baselineDraft: ProductStudioDraft
): ProductStudioUpdateRequestPayload {
  const payload: ProductStudioUpdateRequestPayload = {};

  if (workingDraft.title !== undefined) {
    payload.title = workingDraft.title.trim();
  }

  if (workingDraft.slug) {
    payload.slug = workingDraft.slug.trim();
  }

  if (workingDraft.description !== undefined) {
    payload.description = workingDraft.description;
  }

  if (workingDraft.categoryId) {
    payload.categoryId = workingDraft.categoryId;
  }

  if (workingDraft.brandId) {
    payload.brandId = workingDraft.brandId;
  }

  if (workingDraft.gender) {
    payload.gender = workingDraft.gender;
  }

  if (workingDraft.color) {
    payload.color = workingDraft.color;
  }

  if (workingDraft.material) {
    payload.material = workingDraft.material;
  }

  if (workingDraft.careInstructions !== undefined) {
    payload.careInstructions = workingDraft.careInstructions;
  }

  if (typeof workingDraft.priceCents === 'number') {
    payload.priceCents = workingDraft.priceCents;
  }

  if (typeof workingDraft.oldPriceCents === 'number') {
    payload.oldPriceCents = workingDraft.oldPriceCents;
  }

  if (workingDraft.variants && workingDraft.variants.length > 0) {
    payload.variants = workingDraft.variants
      .filter((v) => v.isActive !== false)
      .map((v) => ({
        id: isCanonicalVariantId(v.id) ? v.id : undefined,
        colorId: v.colorId || undefined,
        sizeValueId: v.sizeValueId || undefined,
        sellerSku: v.sellerSku ? v.sellerSku.trim() : undefined,
        barcode: v.barcode ? v.barcode.trim() : undefined,
        priceCents: typeof v.priceCents === 'number' ? v.priceCents : undefined,
      }));
  }

  if (workingDraft.materialComposition && workingDraft.materialComposition.length > 0) {
    const validComps = workingDraft.materialComposition
      .filter((r) => r.materialId && typeof r.percentage === 'number' && r.percentage > 0)
      .map((r) => ({
        materialId: r.materialId!,
        percentage: Number(r.percentage),
      }));
    if (validComps.length > 0) {
      payload.materialComposition = validComps;
    }
  }

  if (workingDraft.sizeChart?.rows && workingDraft.sizeChart.rows.length > 0) {
    const validRows = workingDraft.sizeChart.rows
      .filter((r: any) => r.sizeValueId)
      .map((r: any) => ({
        sizeValueId: r.sizeValueId!,
        measurements: r.measurements || {},
      }));
    if (validRows.length > 0) {
      payload.sizeChartRows = validRows;
    }
  }

  if (workingDraft.attributes && workingDraft.attributes.length > 0) {
    const validAttrs = workingDraft.attributes
      .filter((a) => a.attributeDefinitionId && (a.dictionaryValueId || a.value !== undefined))
      .map((a) => {
        const item: {
          attributeDefinitionId: string;
          enumValueId?: string;
          textValue?: string;
          numberValue?: number;
          boolValue?: boolean;
        } = {
          attributeDefinitionId: a.attributeDefinitionId!,
        };
        if (a.dictionaryValueId) {
          item.enumValueId = a.dictionaryValueId;
        } else if (typeof a.value === 'string') {
          item.textValue = a.value;
        } else if (typeof a.value === 'number') {
          item.numberValue = a.value;
        } else if (typeof a.value === 'boolean') {
          item.boolValue = a.value;
        }
        return item;
      });
    if (validAttrs.length > 0) {
      payload.attributes = validAttrs;
    }
  }

  // Media payload: buildProductPatchMediaPayload returns undefined if media is unchanged
  const patchMedia = buildProductPatchMediaPayload(workingDraft, baselineDraft);
  if (patchMedia !== undefined) {
    payload.images = patchMedia;
  }

  return payload;
}

/**
 * Checks whether a draft is structurally eligible for draft persistence.
 * Note: Moderation readiness (3+ photos, 100% composition, etc.) is separate.
 */
export function isProductStudioSaveEligible(draft: ProductStudioDraft): boolean {
  if (!draft.title || !draft.title.trim()) {
    return false;
  }
  if (draft.priceCents !== undefined && draft.priceCents < 0) {
    return false;
  }
  if (draft.variants && draft.variants.length > 0) {
    const hasNegativePrice = draft.variants.some(
      (v) => v.priceCents !== undefined && v.priceCents < 0
    );
    if (hasNegativePrice) {
      return false;
    }
  }
  return true;
}

export interface OrchestrateEditSaveParams {
  productId: string;
  draft: ProductStudioDraft;
  baselineDraft: ProductStudioDraft;
  categorySchema: SellerCategorySchema | null;
  canonicalColors: SellerColor[];
  dictionaryValuesMap: Record<string, SellerDictionaryValue[]>;
  stageImageFn?: (productId: string, clientMediaId: string, file: File) => Promise<StageSellerProductImageResponse>;
  updateProductFn?: (productId: string, input: any) => Promise<SellerProduct>;
  getProductFn?: (productId: string) => Promise<SellerProduct>;
  onStagingStart: () => void;
  onStagedImagesPersisted: (images: ProductStudioImage[]) => void;
  onSavingStart: () => void;
  onSaveSuccess: (hydratedProduct: ProductStudioDraft, stagedImages: ProductStudioImage[]) => void;
  onError: (errorMessage: string) => void;
  onRefreshError?: (errorMessage: string) => void;
}

/**
 * Pure Save Orchestrator for Product Studio Edit mode.
 */
export async function orchestrateProductStudioEditSave({
  productId,
  draft,
  baselineDraft,
  categorySchema,
  canonicalColors,
  dictionaryValuesMap,
  stageImageFn = stageSellerProductImage,
  updateProductFn = updateSellerProduct,
  getProductFn = getSellerProduct,
  onStagingStart,
  onStagedImagesPersisted,
  onSavingStart,
  onSaveSuccess,
  onError,
  onRefreshError,
}: OrchestrateEditSaveParams): Promise<void> {
  // 1. Staging phase
  onStagingStart();

  const stageResult = await stagePendingProductStudioImages({
    productId,
    images: draft.images || [],
    stageImage: stageImageFn,
    concurrencyLimit: 3,
  });

  // 2. CRITICAL: Always persist returned staged images before evaluating failures or PATCHing
  onStagedImagesPersisted(stageResult.images);

  // 3. Partial or total stage failure: STOP, no PATCH, show error, keep retryable state
  if (stageResult.failures.length > 0) {
    onError('Не удалось загрузить часть фотографий. Попробуйте ещё раз.');
    return;
  }

  // 4. Staging succeeded completely: enter saving phase
  onSavingStart();

  const workingDraft: ProductStudioDraft = {
    ...draft,
    images: stageResult.images,
  };

  let patchPayload: ProductStudioUpdateRequestPayload;
  try {
    patchPayload = buildProductStudioUpdateRequest(workingDraft, baselineDraft);
  } catch (err: any) {
    onError(err?.message || 'Не удалось подготовить данные товара для сохранения.');
    return;
  }

  // 5. Send exactly ONE PATCH request
  try {
    await updateProductFn(productId, patchPayload);
  } catch (_err: any) {
    onError('Не удалось сохранить товар. Попробуйте ещё раз.');
    return;
  }

  // 6. Fetch canonical product state via single canonical GET (joins size_values, colors, materials)
  let canonicalProduct: SellerProduct;
  try {
    canonicalProduct = await getProductFn(productId);
  } catch (_refreshErr: any) {
    const msg = 'Товар сохранён, но не удалось обновить данные. Обновите страницу.';
    if (onRefreshError) {
      onRefreshError(msg);
    } else {
      onError(msg);
    }
    return;
  }

  // 7. Canonicalize from backend GET response truth
  if (!categorySchema) {
    onError('Не удалось завершить сохранение: отсутствует схема категории.');
    return;
  }

  try {
    const hydrated = hydrateProductStudioDraft({
      product: canonicalProduct,
      categorySchema,
      canonicalColors,
      dictionaryValuesMap,
    });
    onSaveSuccess(hydrated, stageResult.images);
  } catch (err: any) {
    onError(`Ошибка обработки ответа сервера: ${err?.message || 'некорректные данные'}`);
  }
}
