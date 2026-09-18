export interface GalleryMediaItem {
  url: string;
  colorId?: string;
}

/**
 * Deduplicates product images by URL while preserving deterministic order
 * and retaining color metadata if an image was first registered without colorId
 * but a subsequent duplicate entry provides a colorId.
 */
export function deduplicateGalleryImages(
  images?: { url: string; colorId?: string }[] | null,
  singleImage?: string | null,
  defaultImage: GalleryMediaItem = { url: 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image' }
): GalleryMediaItem[] {
  if (images && images.length > 0) {
    const map = new Map<string, GalleryMediaItem>();
    const result: GalleryMediaItem[] = [];

    for (const img of images) {
      if (!img?.url) continue;
      const existing = map.get(img.url);
      if (!existing) {
        const item: GalleryMediaItem = {
          url: img.url,
          colorId: img.colorId || undefined,
        };
        map.set(img.url, item);
        result.push(item);
      } else if (!existing.colorId && img.colorId) {
        // Preserve color metadata if a duplicate occurrence provides it
        existing.colorId = img.colorId;
      }
    }

    if (result.length > 0) {
      return result;
    }
  }

  if (singleImage) {
    return [{ url: singleImage }];
  }

  return [defaultImage];
}

/**
 * Resolves the target gallery index to focus when a customer selects a color:
 * 1. Find the FIRST image matching the requested colorId.
 * 2. Preferred fallback: find the first GENERAL product image (without colorId).
 * 3. Last-resort fallback: first image in the gallery (canonical main product image, index 0).
 */
export function findMediaIndexForColor(
  images: GalleryMediaItem[] | undefined | null,
  colorId: string | null | undefined
): number {
  if (!images || images.length === 0) {
    return 0;
  }

  if (colorId) {
    const matchingIndex = images.findIndex((img) => img.colorId === colorId);
    if (matchingIndex !== -1) {
      return matchingIndex;
    }
  }

  // Preferred fallback: first general image without colorId
  const generalIndex = images.findIndex((img) => !img.colorId);
  if (generalIndex !== -1) {
    return generalIndex;
  }

  // Last-resort fallback: first image in gallery
  return 0;
}
