import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import { shouldIncludeImagesInPatch } from './productStudioMediaHelper';
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
 */
export function mapProductStudioImagesToPatchPayload(
  images: ProductStudioImage[]
): SellerProductPatchImageItem[] {
  return images.map((img) => {
    if (img.source.kind === 'local') {
      throw new Error(
        `Cannot build media PATCH payload: image with uiKey "${img.uiKey}" is still local and has not been staged`
      );
    }
    const id = img.source.kind === 'canonical' ? img.source.imageId : img.source.stagedId;
    return {
      id,
      isMain: Boolean(img.isMain),
      colorId: img.colorId ?? null,
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
  return mapProductStudioImagesToPatchPayload(images);
}
