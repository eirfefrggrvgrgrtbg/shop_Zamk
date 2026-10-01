import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import {
  shouldIncludeImagesInPatch,
  deriveProductStudioMediaMode,
  MAX_PRODUCT_IMAGES,
  type ProductStudioMediaMode,
} from './productStudioMediaHelper';
import type { SellerProductPatchImageItem } from '@zamk/api-client';

export type StageImageFn = (
  productId: string,
  clientMediaId: string,
  file: File
) => Promise<{ stagedMediaId: string; imageUrl: string }>;

export interface StagePendingImagesOptions {
  productId: string;
  images: ProductStudioImage[];
  stageImage: StageImageFn;
  concurrencyLimit?: number;
}

export interface StageImageFailure {
  uiKey: string;
  clientMediaId: string;
  error: unknown;
}

export interface StagePendingImagesResult {
  images: ProductStudioImage[];
  failures: StageImageFailure[];
}

/**
 * Pure helper that transitions a successfully staged local image into a staged ProductStudioImage.
 * Preserves uiKey, clientMediaId, previewUrl, colorId, altText, isMain, and sortOrder.
 * Replaces source with source.kind = 'staged' and releases the File reference.
 * Does NOT revoke previewUrl.
 */
export function transitionLocalToStagedProductStudioImage(
  image: ProductStudioImage,
  stageResult: { stagedId: string; stagedUrl: string }
): ProductStudioImage {
  if (image.source.kind !== 'local') {
    return image;
  }
  return {
    ...image,
    source: {
      kind: 'staged',
      clientMediaId: image.source.clientMediaId,
      stagedId: stageResult.stagedId,
      stagedUrl: stageResult.stagedUrl,
      previewUrl: image.source.previewUrl,
    },
  };
}

/**
 * Framework-agnostic pure staging orchestrator.
 * Uploads all pending local images with bounded concurrency (default 3).
 * Skips canonical and already staged images (zero network calls).
 * Preserves successful uploads even on partial failure, keeping failed uploads local.
 */
export async function stagePendingProductStudioImages({
  productId,
  images,
  stageImage,
  concurrencyLimit = 3,
}: StagePendingImagesOptions): Promise<StagePendingImagesResult> {
  const resultImages = [...images];
  const localIndices: number[] = [];

  for (let i = 0; i < images.length; i++) {
    if (images[i].source.kind === 'local') {
      localIndices.push(i);
    }
  }

  if (localIndices.length === 0) {
    return {
      images: resultImages,
      failures: [],
    };
  }

  const effectiveLimit = Math.min(3, Math.max(1, concurrencyLimit ?? 3));
  const failures: StageImageFailure[] = [];
  let nextQueueIndex = 0;

  async function worker() {
    while (nextQueueIndex < localIndices.length) {
      const targetIdx = localIndices[nextQueueIndex++];
      const currentItem = resultImages[targetIdx];
      if (currentItem.source.kind !== 'local') continue;

      const clientMediaId = currentItem.source.clientMediaId;
      const file = currentItem.source.file;

      try {
        const resp = await stageImage(productId, clientMediaId, file);
        resultImages[targetIdx] = transitionLocalToStagedProductStudioImage(currentItem, {
          stagedId: resp.stagedMediaId,
          stagedUrl: resp.imageUrl,
        });
      } catch (err) {
        failures.push({
          uiKey: currentItem.uiKey,
          clientMediaId,
          error: err,
        });
      }
    }
  }

  const workerCount = Math.min(effectiveLimit, localIndices.length);
  const workers: Promise<void>[] = [];
  for (let w = 0; w < workerCount; w++) {
    workers.push(worker());
  }

  await Promise.all(workers);

  return {
    images: resultImages,
    failures,
  };
}

/**
 * Maps final ProductStudio media into Seller PATCH media DTO payload.
 * Allowed sources at PATCH-build time: canonical and staged.
 * Forbidden: local (throws descriptive error).
 * Array index order defines final product image sequence.
 *
 * Invariants:
 * - In LEGACY_MIXED: throws error, cannot serialize until resolved.
 * - If any image is UNASSIGNED: throws error, cannot serialize.
 * - In COLORWAY: all images must have a non-null colorId.
 * - In GENERAL: all images are serialized with colorId = null (no hybrid ever).
 */
export function mapProductStudioImagesToPatchPayload(
  images: ProductStudioImage[],
  mediaMode?: ProductStudioMediaMode,
  activeColorIds?: Set<string>
): SellerProductPatchImageItem[] {
  if (images.length > MAX_PRODUCT_IMAGES) {
    throw new Error(
      `Cannot build media PATCH payload: maximum ${MAX_PRODUCT_IMAGES} images allowed, got ${images.length}`
    );
  }
  const mode = mediaMode || deriveProductStudioMediaMode(images);
  if (mode === 'LEGACY_MIXED') {
    throw new Error('Cannot build media PATCH payload: product is in LEGACY_MIXED media mode');
  }

  return images.map((img) => {
    if (img.source.kind === 'local') {
      throw new Error(
        `Cannot build media PATCH payload: image with uiKey "${img.uiKey}" is still local and has not been staged`
      );
    }
    if (img.isUnassigned) {
      throw new Error(
        `Cannot build media PATCH payload: image with uiKey "${img.uiKey}" is unassigned`
      );
    }
    const id = img.source.kind === 'canonical' ? img.source.imageId : img.source.stagedId;

    if (mode === 'COLORWAY') {
      if (!img.colorId) {
        throw new Error(
          `Cannot build media PATCH payload: image with uiKey "${img.uiKey}" missing colorId in COLORWAY mode`
        );
      }
      if (activeColorIds && !activeColorIds.has(img.colorId)) {
        throw new Error(
          `Cannot build media PATCH payload: image with uiKey "${img.uiKey}" references invalid or stale colorId "${img.colorId}"`
        );
      }
      return {
        id,
        isMain: Boolean(img.isMain),
        colorId: img.colorId,
        altText: img.altText ?? null,
      };
    }

    // GENERAL mode: all images strictly colorId = null
    return {
      id,
      isMain: Boolean(img.isMain),
      colorId: null,
      altText: img.altText ?? null,
    };
  });
}

/**
 * Determines whether images should be included in Seller PATCH request,
 * and if so, constructs the mapped payload or empty array.
 * Semantics:
 * shouldIncludeImagesInPatch == false -> undefined (omitted)
 * true + images empty -> []
 * true + images nonempty -> mapped payload
 */
export function buildProductPatchMediaPayload(
  currentDraft: ProductStudioDraft,
  baselineDraft: ProductStudioDraft
): SellerProductPatchImageItem[] | undefined {
  if (!shouldIncludeImagesInPatch(currentDraft, baselineDraft)) {
    return undefined;
  }
  const images = currentDraft.images || [];
  if (images.length === 0) {
    return [];
  }
  const mode = currentDraft.mediaMode || deriveProductStudioMediaMode(images);
  return mapProductStudioImagesToPatchPayload(images, mode);
}
