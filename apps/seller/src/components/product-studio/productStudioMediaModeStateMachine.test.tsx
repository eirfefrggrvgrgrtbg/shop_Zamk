/* @vitest-environment jsdom */
import { describe, it, expect } from 'vitest';
import {
  deriveProductStudioMediaMode,
  transitionMediaToColorway,
  transitionMediaToGeneral,
  resolveLegacyMixedMedia,
  reconcileMediaOnColorRemoval,
  createCanonicalProductStudioImage,
} from './productStudioMediaHelper';
import { hydrateProductStudioDraft } from './productStudioHydration';
import {
  mapProductStudioImagesToPatchPayload,
  buildProductPatchMediaPayload,
} from './productStudioSaveMedia';
import {
  getProductStudioMediaSaveBlockReason,
  isProductStudioSaveEligible,
} from './productStudioSaveProduct';
import {
  getProductStudioReadiness,
} from './productStudioReadinessHelper';
import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';

describe('SELLER MEDIA.2B — Product Studio Media Mode State Machine (Matrix A-Z)', () => {
  const sampleColors = [
    { id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' },
    { id: 'col-white', code: 'WHT', nameRu: 'Белый', hex: '#ffffff' },
  ];

  // A. New product (no images) => GENERAL mode
  it('A. New product (no images) => GENERAL mode', () => {
    const mode = deriveProductStudioMediaMode([]);
    expect(mode).toBe('GENERAL');

    const draft = hydrateProductStudioDraft({
      product: { id: 'new-p', title: 'New', images: [] } as any,
      categorySchema: {} as any,
      canonicalColors: [],
    });
    expect(draft.mediaMode).toBe('GENERAL');
  });

  // B. Existing product (all generic images) => GENERAL mode
  it('B. Existing product (all generic images) => GENERAL mode', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
    ];
    expect(deriveProductStudioMediaMode(images)).toBe('GENERAL');

    const draft = hydrateProductStudioDraft({
      product: {
        id: 'p-1',
        title: 'Generic',
        images: [
          { id: '1', url: 'https://img/1', colorId: null, isMain: true, sortOrder: 0 },
          { id: '2', url: 'https://img/2', colorId: null, isMain: false, sortOrder: 1 },
        ],
      } as any,
      categorySchema: {} as any,
      canonicalColors: [],
    });
    expect(draft.mediaMode).toBe('GENERAL');
  });

  // C. Existing product (all colored images) => COLORWAY mode
  it('C. Existing product (all colored images) => COLORWAY mode', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-white' }),
    ];
    expect(deriveProductStudioMediaMode(images)).toBe('COLORWAY');

    const draft = hydrateProductStudioDraft({
      product: {
        id: 'p-2',
        title: 'Colored',
        images: [
          { id: '1', url: 'https://img/1', colorId: 'col-black', isMain: true, sortOrder: 0 },
          { id: '2', url: 'https://img/2', colorId: 'col-white', isMain: false, sortOrder: 1 },
        ],
      } as any,
      categorySchema: {} as any,
      canonicalColors: [],
    });
    expect(draft.mediaMode).toBe('COLORWAY');
  });

  // D. Existing product (mixed images) => LEGACY_MIXED mode
  it('D. Existing product (mixed images) => LEGACY_MIXED mode', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
    ];
    expect(deriveProductStudioMediaMode(images)).toBe('LEGACY_MIXED');

    const draft = hydrateProductStudioDraft({
      product: {
        id: 'p-3',
        title: 'Mixed',
        images: [
          { id: '1', url: 'https://img/1', colorId: 'col-black', isMain: true, sortOrder: 0 },
          { id: '2', url: 'https://img/2', colorId: null, isMain: false, sortOrder: 1 },
        ],
      } as any,
      categorySchema: {} as any,
      canonicalColors: [],
    });
    expect(draft.mediaMode).toBe('LEGACY_MIXED');
  });

  // E. GENERAL: product may have 1 or more active colors
  it('E. GENERAL: product may have 1 or more active colors', () => {
    const draft: ProductStudioDraft = {
      id: 'p-general-with-colors',
      title: 'General with colors',
      description: 'Описание',
      mediaMode: 'GENERAL',
      colors: sampleColors,
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
        createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
        createCanonicalProductStudioImage({ imageId: '3', url: 'https://img/3', colorId: null }),
      ],
      variants: [
        { id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' },
        { id: 'v2', colorId: 'col-white', sizeValueId: 's1', sellerSku: 'SKU2' },
      ],
    };

    expect(getProductStudioMediaSaveBlockReason(draft)).toBeNull();
    expect(isProductStudioSaveEligible(draft)).toBe(true);
  });

  // F. GENERAL -> COLORWAY transition: all generic images become UNASSIGNED
  it('F. GENERAL -> COLORWAY transition: all generic images become UNASSIGNED', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
    ];

    const transitioned = transitionMediaToColorway(initialImages);
    expect(transitioned).toHaveLength(2);
    expect(transitioned[0].colorId).toBeNull();
    expect(transitioned[0].isUnassigned).toBe(true);
    expect(transitioned[1].colorId).toBeNull();
    expect(transitioned[1].isUnassigned).toBe(true);
  });

  // G. GENERAL -> COLORWAY: no silent assignment to first active color
  it('G. GENERAL -> COLORWAY: no silent assignment to first active color', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
    ];
    const transitioned = transitionMediaToColorway(initialImages);
    expect(transitioned[0].colorId).toBeNull();
    expect(transitioned[0].isUnassigned).toBe(true);
    expect(transitioned[0].colorId).not.toBe('col-black');
  });

  // H. COLORWAY -> GENERAL transition: all colorIds cleared to null
  it('H. COLORWAY -> GENERAL transition: all colorIds cleared to null', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-white' }),
    ];

    const transitioned = transitionMediaToGeneral(initialImages);
    expect(transitioned[0].colorId).toBeNull();
    expect(transitioned[0].isUnassigned).toBe(false);
    expect(transitioned[1].colorId).toBeNull();
    expect(transitioned[1].isUnassigned).toBe(false);
  });

  // I. COLORWAY -> GENERAL: deterministic order preserved
  it('I. COLORWAY -> GENERAL: deterministic order preserved', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: 'c1', url: 'https://img/c1', colorId: 'col-black', sortOrder: 0 }),
      createCanonicalProductStudioImage({ imageId: 'c2', url: 'https://img/c2', colorId: 'col-white', sortOrder: 1 }),
      createCanonicalProductStudioImage({ imageId: 'c3', url: 'https://img/c3', colorId: 'col-black', sortOrder: 2 }),
    ];

    const transitioned = transitionMediaToGeneral(initialImages);
    expect(transitioned.map((img) => img.uiKey)).toEqual(['c1', 'c2', 'c3']);
    expect(transitioned.map((img) => img.sortOrder)).toEqual([0, 1, 2]);
  });

  // J. COLORWAY -> GENERAL: exactly one global isMain normalized to first photo
  it('J. COLORWAY -> GENERAL: exactly one global isMain normalized to first photo', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: 'c1', url: 'https://img/c1', colorId: 'col-black', isMain: false }),
      createCanonicalProductStudioImage({ imageId: 'c2', url: 'https://img/c2', colorId: 'col-white', isMain: true }),
      createCanonicalProductStudioImage({ imageId: 'c3', url: 'https://img/c3', colorId: 'col-black', isMain: false }),
    ];

    const transitioned = transitionMediaToGeneral(initialImages);
    const mainImages = transitioned.filter((img) => img.isMain);
    expect(mainImages).toHaveLength(1);
    expect(mainImages[0].uiKey).toBe('c1');
  });

  // K. LEGACY_MIXED: save button blocked with clear reason
  it('K. LEGACY_MIXED: save button blocked with clear reason', () => {
    const draft: ProductStudioDraft = {
      id: 'p-mixed',
      title: 'Mixed Product',
      description: 'Описание',
      mediaMode: 'LEGACY_MIXED',
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
        createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    const reason = getProductStudioMediaSaveBlockReason(draft);
    expect(reason).toBe('Фотографии товара нужно привести к одному режиму перед сохранением');
    expect(isProductStudioSaveEligible(draft)).toBe(false);
  });

  // L. LEGACY_MIXED -> GENERAL: all photos flattened to colorId = null, save allowed
  it('L. LEGACY_MIXED -> GENERAL: all photos flattened to colorId = null, save allowed', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
    ];

    const resolved = resolveLegacyMixedMedia(initialImages, 'GENERAL');
    expect(resolved.every((img) => img.colorId === null)).toBe(true);
    expect(resolved.every((img) => !img.isUnassigned)).toBe(true);

    const draft: ProductStudioDraft = {
      id: 'p-resolved-gen',
      title: 'Resolved General',
      description: 'Описание',
      mediaMode: 'GENERAL',
      images: resolved,
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    expect(getProductStudioMediaSaveBlockReason(draft)).toBeNull();
    expect(isProductStudioSaveEligible(draft)).toBe(true);
  });

  // M. LEGACY_MIXED -> COLORWAY: generic photos become UNASSIGNED, colored retain colorId
  it('M. LEGACY_MIXED -> COLORWAY: generic photos become UNASSIGNED, colored retain colorId', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
    ];

    const resolved = resolveLegacyMixedMedia(initialImages, 'COLORWAY');
    expect(resolved[0].colorId).toBe('col-black');
    expect(resolved[0].isUnassigned).toBe(false);
    expect(resolved[1].colorId).toBeNull();
    expect(resolved[1].isUnassigned).toBe(true);
  });

  // N. COLORWAY: removing an active color makes its photos UNASSIGNED
  it('N. COLORWAY: removing an active color makes its photos UNASSIGNED', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-white' }),
    ];

    // Seller deletes white color, only black remains active
    const activeColorIds = new Set(['col-black']);
    const reconciled = reconcileMediaOnColorRemoval(images, activeColorIds, 'COLORWAY');

    expect(reconciled[0].colorId).toBe('col-black');
    expect(reconciled[0].isUnassigned).toBe(false);

    expect(reconciled[1].colorId).toBeNull();
    expect(reconciled[1].isUnassigned).toBe(true);
  });

  // O. GENERAL: removing an active color DOES NOT affect images
  it('O. GENERAL: removing an active color DOES NOT affect images', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
    ];

    const activeColorIds = new Set(['col-black']);
    const reconciled = reconcileMediaOnColorRemoval(images, activeColorIds, 'GENERAL');

    expect(reconciled[0].colorId).toBeNull();
    expect(reconciled[0].isUnassigned).toBe(false);
    expect(reconciled[1].colorId).toBeNull();
    expect(reconciled[1].isUnassigned).toBe(false);
  });

  // P. COLORWAY: adding new color allows draft save without photos for this color
  it('P. COLORWAY: adding new color allows draft save without photos for this color', () => {
    const draft: ProductStudioDraft = {
      id: 'p-colorway-added-color',
      title: 'Colorway Product',
      description: 'Описание',
      mediaMode: 'COLORWAY',
      colors: [
        { id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' },
        { id: 'col-red', code: 'RED', nameRu: 'Красный', hex: '#ff0000' }, // added, has no photos
      ],
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    // Draft save is allowed!
    expect(getProductStudioMediaSaveBlockReason(draft)).toBeNull();
    expect(isProductStudioSaveEligible(draft)).toBe(true);
  });

  // Q. COLORWAY: unassigned photos exist => save blocked with clear reason
  it('Q. COLORWAY: unassigned photos exist => save blocked with clear reason', () => {
    const draft: ProductStudioDraft = {
      id: 'p-unassigned',
      title: 'Unassigned Photo Product',
      description: 'Описание',
      mediaMode: 'COLORWAY',
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
        {
          ...createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null }),
          isUnassigned: true,
        },
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    const reason = getProductStudioMediaSaveBlockReason(draft);
    expect(reason).toBe('Распределите все фотографии по цветам перед сохранением');
    expect(isProductStudioSaveEligible(draft)).toBe(false);
  });

  // R. COLORWAY: all photos assigned to active colors => PATCH payload contains only non-null colorId
  it('R. COLORWAY: all photos assigned to active colors => PATCH payload contains only non-null colorId', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-white', isMain: false }),
    ];

    const payload = mapProductStudioImagesToPatchPayload(images, 'COLORWAY');
    expect(payload).toHaveLength(2);
    expect(payload[0].colorId).toBe('col-black');
    expect(payload[1].colorId).toBe('col-white');
    expect(payload.every((p) => typeof p.colorId === 'string' && p.colorId.length > 0)).toBe(true);
  });

  // S. GENERAL: PATCH payload contains only null colorId
  it('S. GENERAL: PATCH payload contains only null colorId', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null, isMain: false }),
    ];

    const payload = mapProductStudioImagesToPatchPayload(images, 'GENERAL');
    expect(payload).toHaveLength(2);
    expect(payload[0].colorId).toBeNull();
    expect(payload[1].colorId).toBeNull();
  });

  // T. HYBRID payload: frontend never produces mixed { colorId: null } and { colorId: uuid }
  it('T. HYBRID payload: frontend never produces mixed { colorId: null } and { colorId: uuid }', () => {
    const mixedImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null, isMain: false }),
    ];

    // In LEGACY_MIXED: mapProductStudioImagesToPatchPayload throws error
    expect(() => mapProductStudioImagesToPatchPayload(mixedImages, 'LEGACY_MIXED')).toThrow(
      /LEGACY_MIXED/
    );

    // In GENERAL: forces all to null, no hybrid produced
    const generalPayload = mapProductStudioImagesToPatchPayload(mixedImages, 'GENERAL');
    expect(generalPayload.every((item) => item.colorId === null)).toBe(true);

    // In COLORWAY: throws if any unassigned or null
    expect(() => mapProductStudioImagesToPatchPayload(mixedImages, 'COLORWAY')).toThrow(
      /missing colorId in COLORWAY mode/
    );
  });

  // U. Draft save allowed with <3 photos in GENERAL
  it('U. Draft save allowed with <3 photos in GENERAL', () => {
    const draft: ProductStudioDraft = {
      id: 'p-few-photos',
      title: 'Few Photos',
      description: 'Описание',
      mediaMode: 'GENERAL',
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true }),
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    expect(getProductStudioMediaSaveBlockReason(draft)).toBeNull();
    expect(isProductStudioSaveEligible(draft)).toBe(true);
  });

  // V. Draft save allowed with <3 photos in COLORWAY (if assigned)
  it('V. Draft save allowed with <3 photos in COLORWAY (if assigned)', () => {
    const draft: ProductStudioDraft = {
      id: 'p-few-photos-colorway',
      title: 'Few Photos Colorway',
      description: 'Описание',
      mediaMode: 'COLORWAY',
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    expect(getProductStudioMediaSaveBlockReason(draft)).toBeNull();
    expect(isProductStudioSaveEligible(draft)).toBe(true);
  });

  // W. Moderation submit: GENERAL allows >=3 photos even with color variants
  it('W. Moderation submit: GENERAL allows >=3 photos even with color variants', () => {
    const draft: ProductStudioDraft = {
      id: 'p-mod-gen',
      title: 'General Moderation Product',
      description: 'Desc',
      categoryId: 'c1',
      brandId: 'b1',
      priceCents: 100000,
      mediaMode: 'GENERAL',
      colors: sampleColors,
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true, cropWidth: 1, cropHeight: 1 }),
        createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: null, cropWidth: 1, cropHeight: 1 }),
        createCanonicalProductStudioImage({ imageId: '3', url: 'https://img/3', colorId: null, cropWidth: 1, cropHeight: 1 }),
      ],
      variants: [
        { id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' },
        { id: 'v2', colorId: 'col-white', sizeValueId: 's1', sellerSku: 'SKU2' },
      ],
      materialComposition: [{ materialName: 'Хлопок', percentage: 100 }],
    };

    const readiness = getProductStudioReadiness(draft);
    expect(readiness.blockingFields).not.toContain('media');
  });

  // X. Moderation submit: COLORWAY requires all active colors covered by >=1 photo
  it('X. Moderation submit: COLORWAY requires all active colors covered by >=1 photo', () => {
    // 3 photos total, but all 3 are black; white has 0 photos
    const draft: ProductStudioDraft = {
      id: 'p-mod-cw',
      title: 'Colorway Moderation Product',
      description: 'Desc',
      mediaMode: 'COLORWAY',
      colors: sampleColors,
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true, cropWidth: 1, cropHeight: 1 }),
        createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-black', cropWidth: 1, cropHeight: 1 }),
        createCanonicalProductStudioImage({ imageId: '3', url: 'https://img/3', colorId: 'col-black', cropWidth: 1, cropHeight: 1 }),
      ],
      variants: [
        { id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' },
        { id: 'v2', colorId: 'col-white', sizeValueId: 's1', sellerSku: 'SKU2' },
      ],
    };

    const readiness = getProductStudioReadiness(draft);
    expect(readiness.blockingFields).toContain('media');
    expect(readiness.warnings.some((w) => w.includes('Белый'))).toBe(true);

    // Now cover white:
    const fixedDraft: ProductStudioDraft = {
      ...draft,
      images: [
        createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: true, cropWidth: 1, cropHeight: 1 }),
        createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-black', cropWidth: 1, cropHeight: 1 }),
        createCanonicalProductStudioImage({ imageId: '3', url: 'https://img/3', colorId: 'col-white', cropWidth: 1, cropHeight: 1 }),
      ],
    };

    const fixedReadiness = getProductStudioReadiness(fixedDraft);
    expect(fixedReadiness.blockingFields).not.toContain('media');
  });

  // Y. Preserving single global isMain across mode switches (normalized to first photo)
  it('Y. Preserving single global isMain across mode switches', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: 'col-black', isMain: false }),
      createCanonicalProductStudioImage({ imageId: '2', url: 'https://img/2', colorId: 'col-white', isMain: true }),
      createCanonicalProductStudioImage({ imageId: '3', url: 'https://img/3', colorId: 'col-black', isMain: false }),
    ];

    // COLORWAY -> GENERAL: first image becomes cover
    const toGeneral = transitionMediaToGeneral(images);
    expect(toGeneral.filter((img) => img.isMain)).toHaveLength(1);
    expect(toGeneral[0].isMain).toBe(true);

    // GENERAL -> COLORWAY: first unassigned image becomes cover
    const toColorway = transitionMediaToColorway(toGeneral);
    expect(toColorway.filter((img) => img.isMain)).toHaveLength(1);
    expect(toColorway[0].isMain).toBe(true);
  });

  // Z. No regressions in existing Product Studio behaviors
  it('Z. No regressions in existing Product Studio behaviors', () => {
    // Unchanged baseline vs current draft dirty calculation
    const img1 = createCanonicalProductStudioImage({ imageId: '1', url: 'https://img/1', colorId: null, isMain: true });
    const baselineDraft: ProductStudioDraft = {
      id: 'prod-base',
      title: 'Original Title',
      description: 'Описание',
      mediaMode: 'GENERAL',
      images: [img1],
      variants: [],
    };

    const textOnlyChangeDraft: ProductStudioDraft = {
      ...baselineDraft,
      title: 'Edited Title',
    };

    // Images payload omitted when media untouched
    const patchMedia = buildProductPatchMediaPayload(textOnlyChangeDraft, baselineDraft);
    expect(patchMedia).toBeUndefined();
  });
});
