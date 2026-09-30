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

/**
 * CATALOG VARIANTS.3C-R1: Presentation Media Modes
 *
 * Exactly TWO presentation modes:
 * - GENERAL_GALLERY: All usable product images are generic (colorId == null).
 *   Color selection does not change gallery.
 * - COLORWAY_GALLERIES: Product has at least one color-specific image (colorId != null).
 *   Each colorway has its own isolated gallery. Generic images are legacy/non-canonical
 *   and NEVER shown in customer presentation.
 */
export type PresentationMediaMode = 'GENERAL_GALLERY' | 'COLORWAY_GALLERIES';

export function derivePresentationMediaMode(
  images?: GalleryMediaItem[] | null
): PresentationMediaMode {
  if (!images || images.length === 0) {
    return 'GENERAL_GALLERY';
  }
  const hasColorSpecific = images.some((img) => Boolean(img.colorId));
  return hasColorSpecific ? 'COLORWAY_GALLERIES' : 'GENERAL_GALLERY';
}

/**
 * Determines the deterministic default colorway to preview in COLORWAY_GALLERIES
 * mode when no color has been explicitly selected yet:
 * 1. colorId of canonical/main image if it has a colorId
 * 2. first colorway containing a canonical/primary image
 * 3. first colorway by existing stable media order
 */
export function getDeterministicDefaultColorId(
  images?: GalleryMediaItem[] | null,
  canonicalMainImageUrl?: string | null
): string | null {
  if (!images || images.length === 0) return null;

  // 1. colorId of canonical/main image if it has a colorId
  if (canonicalMainImageUrl) {
    const mainImg = images.find((img) => img.url === canonicalMainImageUrl && Boolean(img.colorId));
    if (mainImg?.colorId) {
      return mainImg.colorId;
    }
  }

  // 2. First colorway containing a canonical/primary image or by stable media order
  const firstColorImage = images.find((img) => Boolean(img.colorId));
  return firstColorImage?.colorId || null;
}

/**
 * CATALOG VARIANTS.3C-R1: Stable Fashion Gallery Filtering
 *
 * Mode A (GENERAL_GALLERY):
 *   Returns all generic product images (image.colorId == null).
 *   Color selection does NOT alter the gallery.
 *
 * Mode B (COLORWAY_GALLERIES):
 *   No hybrid presentation: generic images are strictly excluded.
 *   - If selectedColorId is set: returns ONLY images tagged with selectedColorId.
 *   - If selectedColorId is null: returns ONLY images of the deterministic default colorway.
 *   - If the active colorway has 0 own images: returns neutral placeholder.
 *     NEVER falls back to generic images or another colorway's images.
 */
export function getVisibleGalleryImages(
  allImages: GalleryMediaItem[] | undefined | null,
  selectedColorId: string | null | undefined,
  defaultFallbackImage: GalleryMediaItem = { url: 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image' },
  canonicalMainImageUrl?: string | null
): GalleryMediaItem[] {
  const images = allImages && allImages.length > 0 ? allImages : [defaultFallbackImage];
  const mode = derivePresentationMediaMode(images);

  if (mode === 'GENERAL_GALLERY') {
    const genericImages = images.filter((img) => !img.colorId);
    return genericImages.length > 0 ? genericImages : [defaultFallbackImage];
  }

  // Mode B: COLORWAY_GALLERIES
  const activeColorId = selectedColorId || getDeterministicDefaultColorId(images, canonicalMainImageUrl);

  if (activeColorId) {
    const colorImages = images.filter((img) => img.colorId === activeColorId);
    if (colorImages.length > 0) {
      return colorImages;
    }
  }

  // If active colorway has 0 own images, show neutral placeholder.
  // NEVER show generic images or another colorway's images.
  return [defaultFallbackImage];
}
