import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerDictionaryValue,
  StageSellerProductImageResponse,
} from '@zamk/api-client';
import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import {
  stagePendingProductStudioImages,
} from './productStudioSaveMedia';
import { shouldIncludeImagesInPatch } from './productStudioMediaHelper';
import { hydrateProductStudioDraft } from './productStudioHydration';
import {
  buildProductStudioUpdateRequest,
  type ProductStudioUpdateRequestPayload,
} from './productStudioSaveProduct';
import {
  loadProductStudioCreateSession,
  saveProductStudioCreateSession,
  clearProductStudioCreateSession,
  generateClientCreateId,
  type ProductStudioCreateRequestPayload,
} from './productStudioCreateSession';
import {
  createSellerProduct,
  stageSellerProductImage,
  updateSellerProduct,
  getSellerProduct,
} from '@zamk/api-client/src/seller';

export type { ProductStudioCreateRequestPayload };

/**
 * Pure builder: constructs ONE non-media CreateProductRequest from ProductStudioDraft.
 * Adheres strictly to backend contract:
 * - NO media fields (images, mainImageUrl, etc.)
 * - Canonical IDs only
 * - Synthetic/local variant IDs are NOT included
 * - No label matching, no default guessing, no fake SKU/barcode
 */
export function buildProductStudioCreateRequest(
  draft: ProductStudioDraft
): ProductStudioCreateRequestPayload {
  const payload: ProductStudioCreateRequestPayload = {
    title: draft.title ? draft.title.trim() : '',
    priceCents: typeof draft.priceCents === 'number' ? draft.priceCents : 0,
    currency: draft.currency || 'RUB',
  };

  if (draft.slug && draft.slug.trim()) {
    payload.slug = draft.slug.trim();
  }

  if (draft.description !== undefined && draft.description !== '') {
    payload.description = draft.description;
  }

  if (draft.categoryId) {
    payload.categoryId = draft.categoryId;
  }

  if (draft.brandId) {
    payload.brandId = draft.brandId;
  }

  if (draft.gender) {
    payload.gender = draft.gender;
  }

  if (draft.color) {
    payload.color = draft.color;
  }

  if (draft.material) {
    payload.material = draft.material;
  }

  if (draft.careInstructions !== undefined && draft.careInstructions !== '') {
    payload.careInstructions = draft.careInstructions;
  }

  if (typeof draft.oldPriceCents === 'number') {
    payload.oldPriceCents = draft.oldPriceCents;
  }

  if (draft.variants && draft.variants.length > 0) {
    const validVariants = draft.variants
      .filter((v) => v.isActive !== false)
      .map((v) => ({
        colorId: v.colorId || undefined,
        sizeValueId: v.sizeValueId || undefined,
        sellerSku: v.sellerSku ? v.sellerSku.trim() : undefined,
        priceCents: typeof v.priceCents === 'number' ? v.priceCents : undefined,
      }));
    if (validVariants.length > 0) {
      payload.variants = validVariants;
    }
  }

  if (draft.materialComposition && draft.materialComposition.length > 0) {
    const validComps = draft.materialComposition
      .filter((r) => r.materialId && typeof r.percentage === 'number' && r.percentage > 0)
      .map((r) => ({
        materialId: r.materialId!,
        percentage: Number(r.percentage),
      }));
    if (validComps.length > 0) {
      payload.materialComposition = validComps;
    }
  }

  if (draft.sizeChart?.rows && draft.sizeChart.rows.length > 0) {
    const validRows = draft.sizeChart.rows
      .filter((r: any) => r.sizeValueId)
      .map((r: any) => ({
        sizeValueId: r.sizeValueId!,
        measurements: r.measurements || {},
      }));
    if (validRows.length > 0) {
      payload.sizeChartRows = validRows;
    }
  }

  if (draft.attributes && draft.attributes.length > 0) {
    const validAttrs = draft.attributes
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

  return payload;
}

/**
 * Pure merger: merges pre-create local media into canonical hydrated draft.
 * Rules:
 * - Local File objects, previewUrl, clientMediaId, uiKey, isMain, altText, sortOrder are preserved.
 * - Color binding (colorId) is preserved ONLY if that exact colorId exists in canonical product colors/variants.
 * - If colorId is invalid, it is cleared (undefined). ZERO label matching.
 */
export function mergeRetainedLocalMedia(
  retainedImages: ProductStudioImage[],
  canonicalDraft: ProductStudioDraft
): ProductStudioImage[] {
  const validColorIds = new Set<string>();
  if (canonicalDraft.variants) {
    for (const v of canonicalDraft.variants) {
      if (v.colorId) validColorIds.add(v.colorId);
    }
  }
  if (canonicalDraft.colors) {
    for (const c of canonicalDraft.colors) {
      if (c.colorId) validColorIds.add(c.colorId);
      if (c.id) validColorIds.add(c.id);
    }
  }

  return retainedImages.map((img) => {
    const isValidColor = img.colorId ? validColorIds.has(img.colorId) : false;
    return {
      ...img,
      colorId: isValidColor ? img.colorId : undefined,
    };
  });
}

export interface OrchestrateCreateSaveParams {
  draft: ProductStudioDraft;
  categorySchema: SellerCategorySchema | null;
  canonicalColors: SellerColor[];
  dictionaryValuesMap: Record<string, SellerDictionaryValue[]>;
  createProductFn?: (input: any, options?: { idempotencyKey?: string }) => Promise<SellerProduct>;
  stageImageFn?: (productId: string, clientMediaId: string, file: File) => Promise<StageSellerProductImageResponse>;
  updateProductFn?: (productId: string, input: any) => Promise<SellerProduct>;
  getProductFn?: (productId: string) => Promise<SellerProduct>;
  onSavingStart: () => void;
  onStagingStart: () => void;
  onStagedImagesPersisted: (images: ProductStudioImage[]) => void;
  onIdentityEstablished?: (productId: string) => void;
  onSaveSuccess: (productId: string, hydratedProduct: ProductStudioDraft, stagedImages: ProductStudioImage[]) => void;
  onError: (errorMessage: string) => void;
  onIdentityRecoveryRequired?: (errorMessage: string) => void;
  onRefreshError?: (errorMessage: string) => void;
  navigate: (to: string, options?: { replace?: boolean }) => void;
}

/**
 * Pure Save Orchestrator for Product Studio Create mode.
 * Integrates durable session storage, non-media POST, canonical GET, media merge, and Edit PATCH.
 */
export async function orchestrateProductStudioCreateSave({
  draft,
  categorySchema,
  canonicalColors,
  dictionaryValuesMap,
  createProductFn = createSellerProduct,
  stageImageFn = stageSellerProductImage,
  updateProductFn = updateSellerProduct,
  getProductFn = getSellerProduct,
  onSavingStart,
  onStagingStart,
  onStagedImagesPersisted,
  onIdentityEstablished,
  onSaveSuccess,
  onError,
  onIdentityRecoveryRequired,
  onRefreshError,
  navigate,
}: OrchestrateCreateSaveParams): Promise<void> {
  // 1. Session check & Identity establishment
  const existingSession = loadProductStudioCreateSession();
  let productId = existingSession?.productId;
  let clientCreateId = existingSession?.clientCreateId || generateClientCreateId();
  let snapshot = existingSession?.createRequestSnapshot;

  if (!snapshot) {
    try {
      snapshot = buildProductStudioCreateRequest(draft);
    } catch (err: any) {
      onError(err?.message || 'Не удалось подготовить данные товара для создания.');
      return;
    }
  }

  if (!productId) {
    if (!existingSession || existingSession.phase !== 'identity_pending') {
      // CRITICAL: Persist snapshot to sessionStorage BEFORE network call
      // Requirement 1: Fails closed. If write fails, ZERO Create POST.
      try {
        saveProductStudioCreateSession({
          clientCreateId,
          createRequestSnapshot: snapshot,
          phase: 'identity_pending',
          createdAt: Date.now(),
        });
      } catch (err: any) {
        onError(err?.message || 'Не удалось сохранить сессию создания товара. Попробуйте ещё раз.');
        return;
      }
    }

    onSavingStart();

    let createdProduct: SellerProduct;
    try {
      createdProduct = await createProductFn(snapshot, { idempotencyKey: clientCreateId });
    } catch (err: any) {
      // Requirement 5: Definite pre-identity validation rejection (400)
      const isDefiniteValidationRejection =
        (err?.status === 400 || err?.code === 'validation_error' || err?.code === 'bad_request') &&
        err?.status !== 409 &&
        err?.code !== 'idempotency_conflict' &&
        err?.code !== 'duplicate_product';

      if (isDefiniteValidationRejection) {
        clearProductStudioCreateSession();
        onError(err?.message || 'Ошибка валидации данных товара. Проверьте форму и попробуйте снова.');
        return;
      }

      // Requirement 4: 409, 5xx, network, timeout -> identity outcome unknown!
      // Must NOT clear session. Freeze draft mutation and require retry of the same operation.
      const msg =
        err?.status === 409 || err?.code === 'idempotency_conflict' || err?.code === 'duplicate_product'
          ? 'Конфликт операции создания товара. Нажмите "Повторить попытку".'
          : (err?.message || 'Связь с сервером прервана. Результат создания товара не подтвержден. Повторите попытку.');

      if (onIdentityRecoveryRequired) {
        onIdentityRecoveryRequired(msg);
      } else {
        onError(msg);
      }
      return;
    }

    productId = createdProduct.id;
    // CRITICAL: Immediately persist authoritative productId
    try {
      saveProductStudioCreateSession({
        clientCreateId,
        createRequestSnapshot: snapshot,
        productId,
        phase: 'identity_established',
        createdAt: Date.now(),
      });
    } catch (persistErr) {
      console.warn('Failed to update create session to identity_established:', persistErr);
    }
  }

  onIdentityEstablished?.(productId);

  // 2. Canonical GET (Mandatory: Product Studio does NOT hydrate from POST response)
  let canonicalProduct: SellerProduct;
  try {
    canonicalProduct = await getProductFn(productId);
  } catch (_err: any) {
    const msg = 'Товар создан, но не удалось загрузить данные с сервера. Обновите страницу.';
    if (onRefreshError) {
      onRefreshError(msg);
    } else {
      onError(msg);
    }
    return;
  }

  if (!categorySchema) {
    onError('Не удалось завершить сохранение: отсутствует схема категории.');
    return;
  }

  let baselineDraft: ProductStudioDraft;
  try {
    baselineDraft = hydrateProductStudioDraft({
      product: canonicalProduct,
      categorySchema,
      canonicalColors,
      dictionaryValuesMap,
    });
  } catch (err: any) {
    onError(`Ошибка обработки ответа сервера: ${err?.message || 'некорректные данные'}`);
    return;
  }

  // 3. Merge retained local media
  const retainedImages = draft.images || [];
  const mergedImages = mergeRetainedLocalMedia(retainedImages, baselineDraft);

  const workingDraft: ProductStudioDraft = {
    ...baselineDraft,
    images: mergedImages,
  };

  // 4. Staging phase for local media
  const hasLocalImages = mergedImages.some((img) => img.source.kind === 'local');
  let stagedImages = mergedImages;

  if (hasLocalImages) {
    onStagingStart();

    const stageResult = await stagePendingProductStudioImages({
      productId,
      images: mergedImages,
      stageImage: stageImageFn,
      concurrencyLimit: 3,
    });

    // Always persist returned staged images before evaluating failures or PATCHing
    onStagedImagesPersisted(stageResult.images);

    if (stageResult.failures.length > 0) {
      onError('Не удалось загрузить часть фотографий. Попробуйте ещё раз.');
      return;
    }

    stagedImages = stageResult.images;
  }

  const draftAfterStaging: ProductStudioDraft = {
    ...workingDraft,
    images: stagedImages,
  };

  // 5. Semantic Media Check: Is PATCH needed?
  // Fast path is allowed ONLY when media is clean against canonical baseline.
  // Handles:
  // Case A: baseline images=[], working images=[] -> isMediaDirty=false (PATCH 0)
  // Case B: baseline images=[], working had local images, now staged -> isMediaDirty=true (PATCH 1)
  // Case C: baseline images=[], working has staged images from previous failed PATCH -> isMediaDirty=true (stage 0, PATCH 1)
  // Case D: baseline images equals working canonical media -> isMediaDirty=false (PATCH 0)
  const isMediaDirty = shouldIncludeImagesInPatch(draftAfterStaging, baselineDraft);

  if (!isMediaDirty) {
    try {
      saveProductStudioCreateSession({
        version: 1,
        clientCreateId,
        createRequestSnapshot: snapshot,
        productId,
        phase: 'completed',
        createdAt: Date.now(),
      });
    } catch (persistErr) {
      console.warn('Failed to persist completed create session marker in fast path:', persistErr);
    }
    onSaveSuccess(productId, baselineDraft, []);
    clearProductStudioCreateSession();
    navigate(`/products/${productId}/edit`, { replace: true });
    return;
  }

  // 6. Enter saving phase for PATCH
  onSavingStart();

  let patchPayload: ProductStudioUpdateRequestPayload;
  try {
    patchPayload = buildProductStudioUpdateRequest(draftAfterStaging, baselineDraft);
  } catch (err: any) {
    onError(err?.message || 'Не удалось подготовить данные товара для сохранения.');
    return;
  }

  try {
    await updateProductFn(productId, patchPayload);
  } catch (_err: any) {
    onError('Не удалось сохранить товар. Попробуйте ещё раз.');
    return;
  }

  // 7. Final canonical GET
  let finalCanonicalProduct: SellerProduct;
  try {
    finalCanonicalProduct = await getProductFn(productId);
  } catch (_refreshErr: any) {
    const msg = 'Товар сохранён, но не удалось обновить данные. Обновите страницу.';
    if (onRefreshError) {
      onRefreshError(msg);
    } else {
      onError(msg);
    }
    return;
  }

  let finalHydrated: ProductStudioDraft;
  try {
    finalHydrated = hydrateProductStudioDraft({
      product: finalCanonicalProduct,
      categorySchema,
      canonicalColors,
      dictionaryValuesMap,
    });
  } catch (err: any) {
    onError(`Ошибка обработки ответа сервера: ${err?.message || 'некорректные данные'}`);
    return;
  }

  try {
    saveProductStudioCreateSession({
      version: 1,
      clientCreateId,
      createRequestSnapshot: snapshot,
      productId,
      phase: 'completed',
      createdAt: Date.now(),
    });
  } catch (persistErr) {
    console.warn('Failed to persist completed create session marker:', persistErr);
  }

  onSaveSuccess(productId, finalHydrated, stagedImages);
  clearProductStudioCreateSession();
  navigate(`/products/${productId}/edit`, { replace: true });
}
