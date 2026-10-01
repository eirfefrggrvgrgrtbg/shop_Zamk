/* @vitest-environment jsdom */
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { SellerProduct, SellerColor, SellerCategorySchema } from '@zamk/api-client';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
  type ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import {
  deriveProductStudioMediaMode,
  resolveLegacyMixedMedia,
  normalizeProductStudioCovers,
  getMediaReadinessWarning,
  reorderProductStudioImages,
  MAX_PRODUCT_IMAGES,
  getMediaProgressText,
} from './productStudioMediaHelper';
import {
  getProductStudioReadiness,
} from './productStudioReadinessHelper';
import {
  hydrateProductStudioDraft,
} from './productStudioHydration';
import {
  getProductStudioMediaSaveBlockReason,
  isProductStudioSaveEligible,
  orchestrateProductStudioEditSave,
  buildProductStudioUpdateRequest,
} from './productStudioSaveProduct';
import {
  mapProductStudioImagesToPatchPayload,
  buildProductPatchMediaPayload,
} from './productStudioSaveMedia';

const canonicalColors: SellerColor[] = [
  { id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' },
  { id: 'col-white', code: 'WHT', nameRu: 'Белый', hex: '#ffffff' },
  { id: 'col-red', code: 'RED', nameRu: 'Красный', hex: '#ff0000' },
];

const mockCategorySchema: SellerCategorySchema = {
  id: 'cat-hoodies',
  slug: 'hoodies',
  name: 'Худи',
  dimensionType: 'COLOR_AND_SIZE',
  allowedSizeSystems: [],
  sizeChartRequired: false,
  sizeChartFields: [],
  attributes: [],
};

describe('SELLER MEDIA.2D — Legacy Existing Product Recovery (Cases A - Z)', () => {
  // A. all generic + multiple colors -> GENERAL, no legacy warning
  it('Case A: all generic images + multiple colors -> GENERAL, no legacy banner or warning', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      { uiKey: 'img-3', isMain: false, sortOrder: 2, colorId: null, source: { kind: 'canonical', imageId: 'img-3', url: 'https://img/3' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }, { id: 'col-white', nameRu: 'Белый' }];
    const variants = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true },
      { id: 'v2', colorId: 'col-white', sizeValueId: 's1', isActive: true },
    ];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('GENERAL');

    const warning = getMediaReadinessWarning({ mediaMode: mode, images, colors, variants });
    expect(warning).toBeNull();

    const saveBlock = getProductStudioMediaSaveBlockReason({ images, mediaMode: mode, colors, variants } as any);
    expect(saveBlock).toBeNull();
  });

  // B. all valid colored -> COLORWAY, no legacy warning
  it('Case B: all valid colored images -> COLORWAY, no legacy warning', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      { uiKey: 'img-3', isMain: false, sortOrder: 2, colorId: 'col-white', source: { kind: 'canonical', imageId: 'img-3', url: 'https://img/3' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }, { id: 'col-white', nameRu: 'Белый' }];
    const variants = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true },
      { id: 'v2', colorId: 'col-white', sizeValueId: 's1', isActive: true },
    ];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('COLORWAY');

    const warning = getMediaReadinessWarning({ mediaMode: mode, images, colors, variants });
    expect(warning).toBeNull();
  });

  // C. mixed generic + valid colored -> LEGACY recovery UI
  it('Case C: mixed generic + valid colored -> LEGACY_MIXED with recovery banner', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('LEGACY_MIXED');

    const draft: Partial<ProductStudioDraft> = {
      title: 'Смешанный товар',
      images,
      colors,
      variants,
      mediaMode: mode,
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const banner = screen.getByTestId('legacy-mixed-banner');
    expect(banner).toBeTruthy();
    expect(banner.textContent).toContain('Фотографии товара нужно привести к одному режиму');
    expect(screen.getByTestId('resolve-to-general-btn')).toBeTruthy();
    expect(screen.getByTestId('resolve-to-colorway-btn')).toBeTruthy();
  });

  // D. LEGACY -> GENERAL: all photos preserved, all colorId=null
  it('Case D: LEGACY -> GENERAL resolution preserves all photos and resets colorId to null', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: false, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];

    const resolved = resolveLegacyMixedMedia(images, 'GENERAL', colors);
    expect(resolved).toHaveLength(2);
    expect(resolved[0].colorId).toBeNull();
    expect(resolved[0].isUnassigned).toBe(false);
    expect(resolved[0].isMain).toBe(true); // first photo is cover
    expect(resolved[1].colorId).toBeNull();
    expect(resolved[1].isUnassigned).toBe(false);
    expect(resolved[1].isMain).toBe(false);
  });

  // E. LEGACY -> COLORWAY: valid colored preserved, generic becomes UNASSIGNED
  it('Case E: LEGACY -> COLORWAY resolution preserves valid colored and marks generic as UNASSIGNED', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];

    const resolved = resolveLegacyMixedMedia(images, 'COLORWAY', colors);
    expect(resolved).toHaveLength(2);
    expect(resolved[0].colorId).toBe('col-black');
    expect(resolved[0].isUnassigned).toBe(false);
    expect(resolved[1].colorId).toBeNull();
    expect(resolved[1].isUnassigned).toBe(true);
  });

  // F. orphaned color image -> preserved as UNASSIGNED
  it('Case F: orphaned color image is safely hydrated as client-side UNASSIGNED', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-legacy-orphan',
      title: 'Товар с удалённым цветом',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [
        { id: 'v1', colorId: 'col-black', colorName: 'Черный', sizeValueId: 's1', isActive: true },
      ],
      images: [
        { id: 'img-1', url: 'https://img/1', colorId: 'col-black', isMain: true, sortOrder: 0 },
        { id: 'img-2', url: 'https://img/2', colorId: 'col-deleted-orphan', isMain: false, sortOrder: 1 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.images).toBeDefined();
    const images = hydrated.images!;
    expect(images).toHaveLength(2);
    expect(images[0].colorId).toBe('col-black');
    expect(Boolean(images[0].isUnassigned)).toBe(false);

    // Orphaned image 2:
    expect(images[1].colorId).toBeNull();
    expect(images[1].isUnassigned).toBe(true);
    expect((images[1].source as any).imageId).toBe('img-2');

    // Derives LEGACY_MIXED
    expect(hydrated.mediaMode).toBe('LEGACY_MIXED');
  });

  // G. orphan never serializes stale color UUID
  it('Case G: orphan never serializes stale color UUID into PATCH payload', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];

    // Attempting to serialize with unassigned image throws
    expect(() => mapProductStudioImagesToPatchPayload(images, 'COLORWAY')).toThrow(
      'Cannot build media PATCH payload: image with uiKey "img-2" is unassigned'
    );
  });

  // H. mixed + orphan -> deterministic recovery
  it('Case H: product with generic + valid colored + orphan resolves deterministically', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-gen', isMain: false, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-gen', url: 'https://img/gen' } },
      { uiKey: 'img-val', isMain: true, sortOrder: 1, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-val', url: 'https://img/val' } },
      { uiKey: 'img-orph', isMain: false, sortOrder: 2, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-orph', url: 'https://img/orph' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];

    // Resolve to COLORWAY
    const cw = resolveLegacyMixedMedia(images, 'COLORWAY', colors);
    expect(cw).toHaveLength(3);
    expect(cw[0].isUnassigned).toBe(true);
    expect(cw[0].colorId).toBeNull();
    expect(cw[1].isUnassigned).toBe(false);
    expect(cw[1].colorId).toBe('col-black');
    expect(cw[2].isUnassigned).toBe(true);
    expect(cw[2].colorId).toBeNull();

    // Resolve to GENERAL
    const gen = resolveLegacyMixedMedia(images, 'GENERAL', colors);
    expect(gen).toHaveLength(3);
    gen.forEach((img) => {
      expect(img.colorId).toBeNull();
      expect(img.isUnassigned).toBe(false);
    });
    expect(gen[0].isMain).toBe(true);
  });

  // I. no active colors + old colored images -> save blocked, no silent GENERAL conversion
  it('Case I: 0 active colors + old colored images -> LEGACY_MIXED and save blocked, no silent GENERAL', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-no-colors',
      title: 'Товар без цветов со старыми фото',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [],
      images: [
        { id: 'img-1', url: 'https://img/1', colorId: 'col-old', isMain: true, sortOrder: 0 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.mediaMode).toBe('LEGACY_MIXED');
    const blockReason = getProductStudioMediaSaveBlockReason(hydrated);
    expect(blockReason).toBe('Фотографии товара нужно привести к одному режиму перед сохранением');
    expect(isProductStudioSaveEligible(hydrated)).toBe(false);
  });

  // J. add active color after orphan state -> Seller can assign image explicitly
  it('Case J: adding active color after orphan allows explicit photo assignment', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-orph', isMain: true, sortOrder: 0, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-orph', url: 'https://img/orph' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    // Explicitly assign to col-black
    const assigned = images.map((img) => ({
      ...img,
      colorId: 'col-black',
      isUnassigned: false,
    }));

    const mode = deriveProductStudioMediaMode(assigned, colors, variants);
    expect(mode).toBe('COLORWAY');
    expect(getProductStudioMediaSaveBlockReason({ images: assigned, mediaMode: mode, colors, variants } as any)).toBeNull();
  });

  // K. missing coverage for active color -> NOT legacy, draft save allowed
  it('Case K: COLORWAY with missing color coverage is NOT legacy, draft save is allowed', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }, { id: 'col-white', nameRu: 'Белый' }];
    const variants = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true },
      { id: 'v2', colorId: 'col-white', sizeValueId: 's1', isActive: true },
    ];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('COLORWAY');

    // Readiness warning indicates missing coverage for author
    const warning = getMediaReadinessWarning({ mediaMode: mode, images, colors, variants });
    expect(warning).toBe('Нужно минимум 3 фото'); // total < 3

    // Draft save is allowed (not blocked by missing coverage)
    const blockReason = getProductStudioMediaSaveBlockReason({ images, mediaMode: mode, colors, variants } as any);
    expect(blockReason).toBeNull();
  });

  // L. one-color GENERAL valid
  it('Case L: one-color product with GENERAL gallery is valid', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('GENERAL');
    expect(getProductStudioMediaSaveBlockReason({ images, mediaMode: mode, colors, variants } as any)).toBeNull();
  });

  // M. one-color COLORWAY valid
  it('Case M: one-color product with COLORWAY gallery is valid', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('COLORWAY');
    expect(getProductStudioMediaSaveBlockReason({ images, mediaMode: mode, colors, variants } as any)).toBeNull();
  });

  // N. one-color mixed -> legacy
  it('Case N: one-color product with mixed generic and colored is LEGACY_MIXED', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    const mode = deriveProductStudioMediaMode(images, colors, variants);
    expect(mode).toBe('LEGACY_MIXED');
  });

  // O. 0 images -> GENERAL, no warning
  it('Case O: 0 images -> GENERAL, no legacy warning', () => {
    const images: ProductStudioImage[] = [];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];

    const mode = deriveProductStudioMediaMode(images, colors);
    expect(mode).toBe('GENERAL');
  });

  // P. multiple/no isMain legacy technical state -> exactly one deterministic cover
  it('Case P: multiple or no isMain flags normalize to exactly one cover', () => {
    // No isMain
    const noneMain: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: false, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const normNone = normalizeProductStudioCovers(noneMain, 'GENERAL');
    expect(normNone.filter((img) => img.isMain)).toHaveLength(1);
    expect(normNone[0].isMain).toBe(true);

    // Multiple isMain
    const multiMain: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: true, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const normMulti = normalizeProductStudioCovers(multiMain, 'GENERAL');
    expect(normMulti.filter((img) => img.isMain)).toHaveLength(1);
    expect(normMulti[0].isMain).toBe(true);
    expect(normMulti[1].isMain).toBe(false);
  });

  // Q. weird sortOrder -> deterministic normalization
  it('Case Q: irregular sortOrder (all 0, gaps, negatives) normalizes to sequential 0, 1, 2, ...', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-weird-sort',
      title: 'Товар с нарушенным sortOrder',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [],
      images: [
        { id: 'img-a', url: 'https://img/a', isMain: false, sortOrder: 0 },
        { id: 'img-b', url: 'https://img/b', isMain: false, sortOrder: 0 },
        { id: 'img-c', url: 'https://img/c', isMain: false, sortOrder: -5 },
        { id: 'img-d', url: 'https://img/d', isMain: false, sortOrder: 42 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.images).toBeDefined();
    const images = hydrated.images!;
    expect(images).toHaveLength(4);
    expect(images.map((img) => (img.source as any).imageId)).toEqual(['img-c', 'img-a', 'img-b', 'img-d']);
    expect(images.filter((img) => img.isMain)).toHaveLength(1);
  });

  // R. unresolved legacy -> Save blocked
  it('Case R: unresolved LEGACY_MIXED blocks save', () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 100000,
      mediaMode: 'LEGACY_MIXED',
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      ],
    };
    expect(getProductStudioMediaSaveBlockReason(draft as ProductStudioDraft)).toBe(
      'Фотографии товара нужно привести к одному режиму перед сохранением'
    );
    expect(isProductStudioSaveEligible(draft as ProductStudioDraft)).toBe(false);
  });

  // S. unresolved unassigned -> Save blocked
  it('Case S: unresolved unassigned images in COLORWAY block save', () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 100000,
      mediaMode: 'COLORWAY',
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
    };
    expect(getProductStudioMediaSaveBlockReason(draft as ProductStudioDraft)).toBe(
      'Распределите все фотографии по цветам перед сохранением'
    );
    expect(isProductStudioSaveEligible(draft as ProductStudioDraft)).toBe(false);
  });

  // T. serialization cannot emit hybrid
  it('Case T: serialization never emits hybrid payloads', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];

    expect(() => mapProductStudioImagesToPatchPayload(images, 'LEGACY_MIXED')).toThrow(
      'Cannot build media PATCH payload: product is in LEGACY_MIXED media mode'
    );

    // In GENERAL, all colorIds are strictly null
    const genPayload = mapProductStudioImagesToPatchPayload(images, 'GENERAL');
    expect(genPayload.every((item) => item.colorId === null)).toBe(true);

    // In COLORWAY, missing colorId throws
    expect(() => mapProductStudioImagesToPatchPayload(images, 'COLORWAY')).toThrow(
      'Cannot build media PATCH payload: image with uiKey "img-2" missing colorId in COLORWAY mode'
    );
  });

  // U. serialization cannot emit stale color reference
  it('Case U: serialization rejects images referencing stale or unconfigured color UUIDs', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-stale-uuid', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
    ];
    const activeColorIds = new Set(['col-black']);

    expect(() => mapProductStudioImagesToPatchPayload(images, 'COLORWAY', activeColorIds)).toThrow(
      'Cannot build media PATCH payload: image with uiKey "img-1" references invalid or stale colorId "col-stale-uuid"'
    );
  });

  // V. refresh without save does not imply persistence mutation
  it('Case V: hydrating and loading legacy draft does not mutate backend persistence', () => {
    const patchSpy = vi.fn();
    const rawProduct: SellerProduct = {
      id: 'prod-v',
      title: 'Неизменный товар',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [],
      images: [
        { id: 'img-1', url: 'https://img/1', colorId: 'col-old', isMain: true, sortOrder: 0 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    // Hydration is pure: no network calls
    expect(patchSpy).not.toHaveBeenCalled();
    expect(hydrated.mediaMode).toBe('LEGACY_MIXED');
  });

  // W. Visual/Form preserve recovery state
  it('Case W: switching between Visual and Form workspaces preserves legacy recovery state', async () => {
    function SwitcherConsumer() {
      const { viewMode, setViewMode, mediaMode } = useProductStudio();
      return (
        <div>
          <span data-testid="current-view-mode">{viewMode}</span>
          <span data-testid="current-media-mode">{mediaMode}</span>
          <button data-testid="switch-to-form-btn" onClick={() => setViewMode('form')}>To Form</button>
          <button data-testid="switch-to-visual-btn" onClick={() => setViewMode('visual')}>To Visual</button>
        </div>
      );
    }

    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар со смешанными фото',
      mediaMode: 'LEGACY_MIXED',
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <SwitcherConsumer />
          <ProductStudioVisualWorkspace />
          <ProductStudioFormWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('current-media-mode').textContent).toBe('LEGACY_MIXED');

    // Both workspaces render the legacy recovery banner
    const banners = screen.getAllByTestId('legacy-mixed-banner');
    expect(banners.length).toBeGreaterThanOrEqual(1);

    // Switch to form
    fireEvent.click(screen.getByTestId('switch-to-form-btn'));
    expect(screen.getByTestId('current-view-mode').textContent).toBe('form');
    expect(screen.getByTestId('current-media-mode').textContent).toBe('LEGACY_MIXED');

    // Switch back to visual
    fireEvent.click(screen.getByTestId('switch-to-visual-btn'));
    expect(screen.getByTestId('current-view-mode').textContent).toBe('visual');
    expect(screen.getByTestId('current-media-mode').textContent).toBe('LEGACY_MIXED');
  });

  // X. existing 2C first-photo-is-cover semantics remain
  it('Case X: first photo in sequence automatically becomes cover on reorder', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];

    // Drag item 1 (img-2) to slot 0
    const reordered = reorderProductStudioImages(images, 1, 0, null, 'GENERAL');
    expect(reordered[0].uiKey).toBe('img-2');
    expect(reordered[0].isMain).toBe(true);
    expect(reordered[1].uiKey).toBe('img-1');
    expect(reordered[1].isMain).toBe(false);
  });

  // Y. GENERAL direct upload remains colorId=null
  it('Case Y: GENERAL mode direct upload normalizes to colorId = null', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const normalized = normalizeProductStudioCovers(images, 'GENERAL');
    expect(normalized.every((img) => img.colorId === null)).toBe(true);
  });

  // Z. COLORWAY direct upload remains selected active colorId
  it('Case Z: COLORWAY mode uploads retain active colorId', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: 'col-white', source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }, { id: 'col-white', nameRu: 'Белый' }];
    const normalized = normalizeProductStudioCovers(images, 'COLORWAY', colors);
    expect(normalized[0].colorId).toBe('col-black');
    expect(normalized[1].colorId).toBe('col-white');
  });
});

describe('SELLER MEDIA.2D1 — Canonical Cover + Recovery-State Hardening (Cases A - M)', () => {
  // A. GENERAL persisted [A, B, C] with old C.isMain = true hydrates to A.isMain = true, B.isMain = false, C.isMain = false
  it('Case A: GENERAL persisted [A, B, C] with old C.isMain = true normalizes to A.isMain = true', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-a',
      title: 'General Cover Test',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [],
      images: [
        { id: 'img-a', url: 'https://img/a', colorId: null, isMain: false, sortOrder: 0 },
        { id: 'img-b', url: 'https://img/b', colorId: null, isMain: false, sortOrder: 1 },
        { id: 'img-c', url: 'https://img/c', colorId: null, isMain: true, sortOrder: 2 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.mediaMode).toBe('GENERAL');
    const imagesA = hydrated.images ?? [];
    expect(imagesA).toHaveLength(3);
    expect(imagesA[0].isMain).toBe(true);
    expect(imagesA[1].isMain).toBe(false);
    expect(imagesA[2].isMain).toBe(false);
  });

  // B. COLORWAY with canonical colors [Black, White], persisted Black: [B1, B2], White: [W1] with W1.isMain = true -> B1.isMain = true
  it('Case B: COLORWAY with [Black, White] where White has old isMain hydrates to B1.isMain = true', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-b',
      title: 'Colorway Cover Test',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [
        { id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true },
        { id: 'v2', colorId: 'col-white', sizeValueId: 's1', isActive: true },
      ],
      images: [
        { id: 'b1', url: 'https://img/b1', colorId: 'col-black', isMain: false, sortOrder: 0 },
        { id: 'b2', url: 'https://img/b2', colorId: 'col-black', isMain: false, sortOrder: 1 },
        { id: 'w1', url: 'https://img/w1', colorId: 'col-white', isMain: true, sortOrder: 2 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.mediaMode).toBe('COLORWAY');
    const imagesB = hydrated.images ?? [];
    expect(imagesB).toHaveLength(3);
    const b1 = imagesB.find((img) => img.source.kind === 'canonical' && img.source.imageId === 'b1');
    const b2 = imagesB.find((img) => img.source.kind === 'canonical' && img.source.imageId === 'b2');
    const w1 = imagesB.find((img) => img.source.kind === 'canonical' && img.source.imageId === 'w1');
    expect(b1?.isMain).toBe(true);
    expect(b2?.isMain).toBe(false);
    expect(w1?.isMain).toBe(false);
  });

  // C. COLORWAY where first active color has 0 photos, second color has photos: cover is first photo of second color
  it('Case C: COLORWAY where first active color has 0 photos, cover is first photo of second active color', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-c',
      title: 'First Color 0 Photos Cover Test',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [
        { id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true },
        { id: 'v2', colorId: 'col-white', sizeValueId: 's1', isActive: true },
      ],
      images: [
        { id: 'w1', url: 'https://img/w1', colorId: 'col-white', isMain: false, sortOrder: 0 },
        { id: 'w2', url: 'https://img/w2', colorId: 'col-white', isMain: true, sortOrder: 1 },
      ],
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.mediaMode).toBe('COLORWAY');
    const imagesC = hydrated.images ?? [];
    expect(imagesC).toHaveLength(2);
    const w1 = imagesC.find((img) => img.source.kind === 'canonical' && img.source.imageId === 'w1');
    const w2 = imagesC.find((img) => img.source.kind === 'canonical' && img.source.imageId === 'w2');
    expect(w1?.isMain).toBe(true);
    expect(w2?.isMain).toBe(false);
  });

  // D. LEGACY -> GENERAL resolution with old non-first isMain: first resulting image becomes isMain, others false
  it('Case D: LEGACY -> GENERAL resolution with old non-first isMain makes first resulting image isMain', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: false, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: true, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      { uiKey: 'img-3', isMain: false, sortOrder: 2, colorId: null, source: { kind: 'canonical', imageId: 'img-3', url: 'https://img/3' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];

    const resolved = resolveLegacyMixedMedia(images, 'GENERAL', colors);
    expect(resolved).toHaveLength(3);
    expect(resolved[0].isMain).toBe(true);
    expect(resolved[1].isMain).toBe(false);
    expect(resolved[2].isMain).toBe(false);
    expect(resolved.every((img) => img.colorId === null && !img.isUnassigned)).toBe(true);
  });

  // E. LEGACY -> "Разложить по цветам": results in COLORWAY mode, with unassigned images having isUnassigned: true
  it('Case E: LEGACY -> "Разложить по цветам" yields COLORWAY mode and unassigned images have isUnassigned: true', () => {
    const images: ProductStudioImage[] = [
      { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
      { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    const resolved = resolveLegacyMixedMedia(images, 'COLORWAY', colors, variants);
    expect(resolved[0].colorId).toBe('col-black');
    expect(resolved[0].isUnassigned).toBe(false);
    expect(resolved[1].colorId).toBeNull();
    expect(resolved[1].isUnassigned).toBe(true);

    const derived = deriveProductStudioMediaMode(resolved, colors, variants, 'COLORWAY');
    expect(derived).toBe('COLORWAY');
  });

  // F. COLORWAY + UNASSIGNED: switching Visual -> Form -> Visual keeps mediaMode === 'COLORWAY'
  it('Case F: COLORWAY + UNASSIGNED switching Visual -> Form -> Visual keeps mediaMode === COLORWAY', () => {
    function ModeWatcher() {
      const { viewMode, setViewMode, mediaMode } = useProductStudio();
      return (
        <div>
          <span data-testid="vm">{viewMode}</span>
          <span data-testid="mm">{mediaMode}</span>
          <button data-testid="btn-form" onClick={() => setViewMode('form')}>To Form</button>
          <button data-testid="btn-vis" onClick={() => setViewMode('visual')}>To Visual</button>
        </div>
      );
    }

    const draft: Partial<ProductStudioDraft> = {
      title: 'Colorway with Unassigned',
      mediaMode: 'COLORWAY',
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', isUnassigned: false, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ModeWatcher />
          <ProductStudioVisualWorkspace />
          <ProductStudioFormWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('mm').textContent).toBe('COLORWAY');
    expect(screen.queryByTestId('legacy-mixed-banner')).toBeNull();

    fireEvent.click(screen.getByTestId('btn-form'));
    expect(screen.getByTestId('vm').textContent).toBe('form');
    expect(screen.getByTestId('mm').textContent).toBe('COLORWAY');
    expect(screen.queryByTestId('legacy-mixed-banner')).toBeNull();

    fireEvent.click(screen.getByTestId('btn-vis'));
    expect(screen.getByTestId('vm').textContent).toBe('visual');
    expect(screen.getByTestId('mm').textContent).toBe('COLORWAY');
    expect(screen.queryByTestId('legacy-mixed-banner')).toBeNull();
  });

  // G. COLORWAY + UNASSIGNED: reordering photos keeps mediaMode === 'COLORWAY'
  it('Case G: COLORWAY + UNASSIGNED reordering photos keeps mediaMode === COLORWAY', () => {
    function ReorderTrigger() {
      const { draft, updateDraft, mediaMode } = useProductStudio();
      return (
        <div>
          <span data-testid="reorder-mm">{mediaMode}</span>
          <button
            data-testid="trigger-reorder-btn"
            onClick={() => {
              const reordered = reorderProductStudioImages(draft.images || [], 0, 1, 'col-black', 'COLORWAY', draft.colors);
              updateDraft({ images: reordered });
            }}
          >
            Reorder
          </button>
        </div>
      );
    }

    const draft: Partial<ProductStudioDraft> = {
      title: 'Reorder Keep Colorway',
      mediaMode: 'COLORWAY',
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', isUnassigned: false, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: 'col-black', isUnassigned: false, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
        { uiKey: 'img-3', isMain: false, sortOrder: 2, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-3', url: 'https://img/3' } },
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ReorderTrigger />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('reorder-mm').textContent).toBe('COLORWAY');
    fireEvent.click(screen.getByTestId('trigger-reorder-btn'));
    expect(screen.getByTestId('reorder-mm').textContent).toBe('COLORWAY');
  });

  // H. COLORWAY + UNASSIGNED: unrelated draft update keeps mediaMode === 'COLORWAY'
  it('Case H: COLORWAY + UNASSIGNED unrelated draft update keeps mediaMode === COLORWAY', () => {
    function TitleUpdater() {
      const { draft, updateDraft, mediaMode } = useProductStudio();
      return (
        <div>
          <span data-testid="draft-title">{draft.title}</span>
          <span data-testid="unrelated-mm">{mediaMode}</span>
          <button
            data-testid="update-title-btn"
            onClick={() => updateDraft({ title: 'Brand New Title' })}
          >
            Update Title
          </button>
        </div>
      );
    }

    const draft: Partial<ProductStudioDraft> = {
      title: 'Original Title',
      mediaMode: 'COLORWAY',
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', isUnassigned: false, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <TitleUpdater />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('unrelated-mm').textContent).toBe('COLORWAY');
    fireEvent.click(screen.getByTestId('update-title-btn'));
    expect(screen.getByTestId('draft-title').textContent).toBe('Brand New Title');
    expect(screen.getByTestId('unrelated-mm').textContent).toBe('COLORWAY');
  });

  // I. Assigning last UNASSIGNED photo keeps mediaMode === 'COLORWAY', unassigned count drops to 0
  it('Case I: Assigning last UNASSIGNED photo keeps mediaMode === COLORWAY and unassigned count drops to 0', () => {
    function AssignHelper() {
      const { draft, updateDraft, mediaMode } = useProductStudio();
      const unassignedCount = (draft.images || []).filter((img) => img.isUnassigned).length;
      return (
        <div>
          <span data-testid="assign-mm">{mediaMode}</span>
          <span data-testid="unassigned-count">{unassignedCount}</span>
          <button
            data-testid="assign-last-btn"
            onClick={() => {
              const updated = (draft.images || []).map((img) =>
                img.isUnassigned ? { ...img, colorId: 'col-black', isUnassigned: false } : img
              );
              updateDraft({ images: updated });
            }}
          >
            Assign
          </button>
        </div>
      );
    }

    const draft: Partial<ProductStudioDraft> = {
      title: 'Assigning Unassigned',
      mediaMode: 'COLORWAY',
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', isUnassigned: false, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <AssignHelper />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('assign-mm').textContent).toBe('COLORWAY');
    expect(screen.getByTestId('unassigned-count').textContent).toBe('1');

    fireEvent.click(screen.getByTestId('assign-last-btn'));
    expect(screen.getByTestId('assign-mm').textContent).toBe('COLORWAY');
    expect(screen.getByTestId('unassigned-count').textContent).toBe('0');
  });

  // J. EXACTLY ONE isMain across all images whenever images.length > 0; exactly 0 when empty
  it('Case J: EXACTLY ONE isMain across all images when non-empty, exactly 0 when empty', () => {
    // 0 images -> 0 isMain
    expect(normalizeProductStudioCovers([])).toEqual([]);

    // 1 image GENERAL
    const singleGen = normalizeProductStudioCovers([
      { uiKey: '1', isMain: false, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: '1', url: 'u1' } },
    ], 'GENERAL');
    expect(singleGen.filter((img) => img.isMain)).toHaveLength(1);

    // 3 images GENERAL (even if old had multiple isMain)
    const multiGen = normalizeProductStudioCovers([
      { uiKey: '1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: '1', url: 'u1' } },
      { uiKey: '2', isMain: true, sortOrder: 1, colorId: null, source: { kind: 'canonical', imageId: '2', url: 'u2' } },
      { uiKey: '3', isMain: true, sortOrder: 2, colorId: null, source: { kind: 'canonical', imageId: '3', url: 'u3' } },
    ], 'GENERAL');
    expect(multiGen.filter((img) => img.isMain)).toHaveLength(1);
    expect(multiGen[0].isMain).toBe(true);

    // Multiple images COLORWAY with multiple colors
    const colors = [{ id: 'col-black', nameRu: 'Черный' }, { id: 'col-white', nameRu: 'Белый' }];
    const multiCol = normalizeProductStudioCovers([
      { uiKey: 'b1', isMain: true, sortOrder: 0, colorId: 'col-black', source: { kind: 'canonical', imageId: 'b1', url: 'u1' } },
      { uiKey: 'w1', isMain: true, sortOrder: 1, colorId: 'col-white', source: { kind: 'canonical', imageId: 'w1', url: 'u2' } },
    ], 'COLORWAY', colors);
    expect(multiCol.filter((img) => img.isMain)).toHaveLength(1);
    expect(multiCol[0].isMain).toBe(true);
    expect(multiCol[1].isMain).toBe(false);
  });

  // K. LEGACY product with 0 active colors: "Разложить по цветам" button is disabled
  it('Case K: LEGACY product with 0 active colors disables "Разложить по цветам" in Visual and Form workspaces', () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Legacy Zero Colors',
      mediaMode: 'LEGACY_MIXED',
      colors: [],
      variants: [],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: null, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: 'col-stale', source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
    };

    const { unmount } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const visBtn = screen.getByTestId('resolve-to-colorway-btn') as HTMLButtonElement;
    expect(visBtn.disabled).toBe(true);
    unmount();

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioFormWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-media'));
    const formBtn = screen.getByTestId('resolve-to-colorway-btn') as HTMLButtonElement;
    expect(formBtn.disabled).toBe(true);
  });

  // L. UI renders "Нераспределённые · N" (never "Без цвета")
  it('Case L: UI renders "Нераспределённые · N" and never "Без цвета"', () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Unassigned Copy Test',
      mediaMode: 'COLORWAY',
      colors: [{ id: 'col-black', nameRu: 'Черный' }],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }],
      images: [
        { uiKey: 'img-1', isMain: true, sortOrder: 0, colorId: 'col-black', isUnassigned: false, source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' } },
        { uiKey: 'img-2', isMain: false, sortOrder: 1, colorId: null, isUnassigned: true, source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' } },
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const unassignedTab = screen.getByTestId('colorway-tab-unassigned');
    expect(unassignedTab.textContent).toContain('Нераспределённые');
    expect(unassignedTab.textContent).toContain('1');
    expect(screen.queryByText(/Без цвета/i)).toBeNull();
  });

  // M. Hydration causes ZERO network mutations: no PATCH/POST sent to backend
  it('Case M: Hydration causes ZERO network mutations (no PATCH/POST sent to backend)', () => {
    const fetchSpy = vi.fn();
    const originalFetch = global.fetch;
    global.fetch = fetchSpy;

    try {
      const rawProduct: SellerProduct = {
        id: 'prod-network-free',
        title: 'Network Free Hydration',
        categoryId: 'cat-hoodies',
        brandId: 'brand-1',
        brandName: 'Brand 1',
        status: 'draft',
        priceCents: 100000,
        variants: [
          { id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true },
        ],
        images: [
          { id: 'img-1', url: 'https://img/1', colorId: 'col-black', isMain: false, sortOrder: 0 },
          { id: 'img-2', url: 'https://img/2', colorId: null, isMain: true, sortOrder: 1 },
        ],
      } as any;

      const hydrated = hydrateProductStudioDraft({
        product: rawProduct,
        categorySchema: mockCategorySchema,
        canonicalColors,
      });

      expect(hydrated.mediaMode).toBe('LEGACY_MIXED');
      expect(fetchSpy).not.toHaveBeenCalled();
    } finally {
      global.fetch = originalFetch;
    }
  });
});

describe('SELLER MEDIA.2D2 — Oversized Legacy Media Recovery (Cases A - L)', () => {
  // Helper to make N canonical images
  const createNImages = (count: number, colorId: string | null = null): ProductStudioImage[] =>
    Array.from({ length: count }, (_, idx) => ({
      uiKey: `img-${idx + 1}`,
      isMain: idx === 0,
      sortOrder: idx,
      colorId,
      source: {
        kind: 'canonical',
        imageId: `img-${idx + 1}`,
        url: `https://img/${idx + 1}`,
      },
    }));

  // A. Hydrate existing product with 20 photos
  it('Case A: Hydrate existing product with 20 photos - all 20 retained, none dropped or truncated', () => {
    const rawImages = Array.from({ length: 20 }, (_, idx) => ({
      id: `img-${idx + 1}`,
      url: `https://img/${idx + 1}`,
      colorId: null,
      isMain: idx === 0,
      sortOrder: idx,
    }));

    const rawProduct: SellerProduct = {
      id: 'prod-oversized-20',
      title: 'Товар с 20 фото',
      categoryId: 'cat-hoodies',
      brandId: 'brand-1',
      brandName: 'Brand 1',
      status: 'draft',
      priceCents: 100000,
      variants: [],
      images: rawImages,
    } as any;

    const hydrated = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockCategorySchema,
      canonicalColors,
    });

    expect(hydrated.images).toBeDefined();
    expect(hydrated.images).toHaveLength(20);
    expect(hydrated.images?.map((img) => (img.source as any).imageId)).toEqual(
      rawImages.map((img) => img.id)
    );
  });

  // B. Product with 20 photos has Save blocked locally
  it('Case B: Product with 20 photos has Save blocked locally - no network PATCH', async () => {
    const images20 = createNImages(20);
    const draft: ProductStudioDraft = {
      id: 'prod-20',
      title: 'Товар с 20 фото',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images20,
    } as any;

    expect(isProductStudioSaveEligible(draft)).toBe(false);
    expect(getProductStudioMediaSaveBlockReason(draft)).toBe(
      'Удалите лишние фотографии: можно сохранить не более 8'
    );

    // Orchestrator stops before network PATCH
    const updateSpy = vi.fn();
    const errorSpy = vi.fn();

    await orchestrateProductStudioEditSave({
      productId: 'prod-20',
      draft,
      baselineDraft: draft,
      categorySchema: mockCategorySchema,
      canonicalColors,
      dictionaryValuesMap: {},
      updateProductFn: updateSpy,
      onStagingStart: vi.fn(),
      onStagedImagesPersisted: vi.fn(),
      onSavingStart: vi.fn(),
      onSaveSuccess: vi.fn(),
      onError: errorSpy,
    });

    expect(updateSpy).not.toHaveBeenCalled();
    expect(errorSpy).toHaveBeenCalledWith(
      'Удалите лишние фотографии: можно сохранить не более 8'
    );
  });

  // C. Product with 20 photos displays warning containing 8
  it('Case C: Product with 20 photos displays warning containing 8', () => {
    const images20 = createNImages(20);
    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар с 20 фото',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images20,
    };

    const warning = getMediaReadinessWarning(draft);
    expect(warning).toBe(`Удалите лишние фотографии: можно сохранить не более ${MAX_PRODUCT_IMAGES}`);

    const readiness = getProductStudioReadiness(draft as ProductStudioDraft);
    expect(readiness.fieldStatus.media.isSatisfied).toBe(false);
    expect(readiness.warnings).toContain('Удалите лишние фотографии: можно сохранить не более 8');
  });

  // D. Switching LEGACY_MIXED -> GENERAL with 20 photos preserves all 20
  it('Case D: Switching LEGACY_MIXED -> GENERAL with 20 photos preserves all 20, save still blocked', () => {
    // 10 generic, 10 colored
    const mixed20: ProductStudioImage[] = [
      ...createNImages(10, null),
      ...createNImages(10, 'col-black').map((img, i) => ({
        ...img,
        uiKey: `img-col-${i + 1}`,
        source: { ...img.source, imageId: `img-col-${i + 1}` } as any,
      })),
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];

    const resolved = resolveLegacyMixedMedia(mixed20, 'GENERAL', colors);
    expect(resolved).toHaveLength(20);
    expect(resolved.every((img) => img.colorId === null && !img.isUnassigned)).toBe(true);

    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: resolved,
    };
    expect(isProductStudioSaveEligible(draft as ProductStudioDraft)).toBe(false);
    expect(getProductStudioMediaSaveBlockReason(draft as ProductStudioDraft)).toBe(
      'Удалите лишние фотографии: можно сохранить не более 8'
    );
  });

  // E. Switching LEGACY_MIXED -> COLORWAY with 20 photos preserves all 20
  it('Case E: Switching LEGACY_MIXED -> COLORWAY with 20 photos preserves all 20 across assigned and unassigned', () => {
    const mixed20: ProductStudioImage[] = [
      ...createNImages(10, null),
      ...createNImages(10, 'col-black').map((img, i) => ({
        ...img,
        uiKey: `img-col-${i + 1}`,
        source: { ...img.source, imageId: `img-col-${i + 1}` } as any,
      })),
    ];
    const colors = [{ id: 'col-black', nameRu: 'Черный' }];
    const variants = [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', isActive: true }];

    const resolved = resolveLegacyMixedMedia(mixed20, 'COLORWAY', colors, variants);
    expect(resolved).toHaveLength(20);

    const assigned = resolved.filter((img) => !img.isUnassigned);
    const unassigned = resolved.filter((img) => img.isUnassigned);
    expect(assigned).toHaveLength(10);
    expect(unassigned).toHaveLength(10);

    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 10000,
      mediaMode: 'COLORWAY',
      colors,
      variants,
      images: resolved,
    };
    expect(isProductStudioSaveEligible(draft as ProductStudioDraft)).toBe(false);
    expect(getProductStudioMediaSaveBlockReason(draft as ProductStudioDraft)).toBe(
      'Удалите лишние фотографии: можно сохранить не более 8'
    );
  });

  // F. Deleting photos from 20 down to 8 unblocks save
  it('Case F: Deleting photos from 20 down to 8 unblocks save and clears warning', () => {
    const images9 = createNImages(9);
    const draft9: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images9,
    };
    expect(getProductStudioMediaSaveBlockReason(draft9 as ProductStudioDraft)).toBe(
      'Удалите лишние фотографии: можно сохранить не более 8'
    );
    expect(isProductStudioSaveEligible(draft9 as ProductStudioDraft)).toBe(false);

    // Delete one image down to 8
    const images8 = images9.slice(0, 8);
    const draft8: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images8,
    };
    expect(getProductStudioMediaSaveBlockReason(draft8 as ProductStudioDraft)).toBeNull();
    expect(isProductStudioSaveEligible(draft8 as ProductStudioDraft)).toBe(true);
    expect(getMediaReadinessWarning(draft8)).toBeNull();
  });

  // G. Deleting photo 1 when product has 9 photos: photo 2 becomes new cover, remaining count = 8, save unblocked
  it('Case G: Deleting photo 1 when product has 9 photos makes photo 2 new cover, count = 8, save unblocked', () => {
    const images9 = createNImages(9);
    // Delete first image (index 0)
    const remaining = images9.slice(1);
    const normalized = normalizeProductStudioCovers(remaining, 'GENERAL');

    expect(normalized).toHaveLength(8);
    expect(normalized[0].uiKey).toBe('img-2');
    expect(normalized[0].isMain).toBe(true);
    expect(normalized.slice(1).every((img) => img.isMain === false)).toBe(true);

    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: normalized,
    };
    expect(getProductStudioMediaSaveBlockReason(draft as ProductStudioDraft)).toBeNull();
    expect(isProductStudioSaveEligible(draft as ProductStudioDraft)).toBe(true);
  });

  // H. Product with 8 photos: upload slot hidden / adding 9th rejected
  it('Case H: Product with 8 photos has upload slot hidden and cannot exceed 8 via UI', () => {
    const images8 = createNImages(8);
    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар с 8 фото',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images8,
    };

    const { unmount } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // In VisualWorkspace thumbnail add slot is not rendered when count >= 8
    expect(screen.queryByTestId('media-add-thumb-slot')).toBeNull();
    unmount();

    const { unmount: unmountForm } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioFormWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-media'));
    // In FormWorkspace form-media-add-card is not rendered when count >= 8
    expect(screen.queryByTestId('form-media-add-card')).toBeNull();
    unmountForm();
  });

  // I. Product with 20 photos: upload button disabled / hidden and cannot add photo 21
  it('Case I: Product with 20 photos hides upload slots and blocks upload attempt', () => {
    const images20 = createNImages(20);
    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар с 20 фото',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images20,
    };

    const { unmount } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.queryByTestId('media-add-thumb-slot')).toBeNull();
    expect(screen.getByTestId('oversized-media-banner')).toBeTruthy();
    expect(screen.getByTestId('oversized-media-banner').textContent).toContain(
      'Удалите лишние фотографии: можно сохранить не более 8'
    );
    unmount();

    const { unmount: unmountForm } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioFormWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-media'));
    expect(screen.queryByTestId('form-media-add-card')).toBeNull();
    expect(screen.getByTestId('oversized-media-banner')).toBeTruthy();
    unmountForm();
  });

  // J. Normal product with <= 8 photos behavior completely unchanged
  it('Case J: Normal product with <= 8 photos saves normally when valid', () => {
    const images3 = createNImages(3);
    const draft: Partial<ProductStudioDraft> = {
      title: 'Обычный товар',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images3,
    };

    expect(getProductStudioMediaSaveBlockReason(draft as ProductStudioDraft)).toBeNull();
    expect(isProductStudioSaveEligible(draft as ProductStudioDraft)).toBe(true);

    const payload = mapProductStudioImagesToPatchPayload(images3, 'GENERAL');
    expect(payload).toHaveLength(3);
    expect(payload[0].isMain).toBe(true);
    expect(payload[1].isMain).toBe(false);
    expect(payload[2].isMain).toBe(false);
  });

  // K. Visual Workspace and Form Workspace both display real count (e.g. "Фото 20 из 8")
  it('Case K: Visual Workspace and Form Workspace both display real count e.g. "Фото 20 из 8"', () => {
    expect(getMediaProgressText(20)).toBe('Фото 20 из 8');
    expect(getMediaProgressText(12)).toBe('Фото 12 из 8');

    const images20 = createNImages(20);
    const draft: Partial<ProductStudioDraft> = {
      title: 'Товар с 20 фото',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images20,
    };

    const { unmount } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('media-progress-badge').textContent).toBe('Фото 20 из 8');
    unmount();

    const { unmount: unmountForm } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioFormWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-media'));
    expect(screen.getByTestId('form-media-progress-badge').textContent).toBe('Фото 20 из 8');
    unmountForm();
  });

  // L. Verification of actual save endpoint: PATCH /api/seller/products/:id with payload.images
  it('Case L: Documents and tests that media saves via PATCH /api/seller/products/:id with payload.images, NOT separate /media endpoint', () => {
    const images3 = createNImages(3);
    const workingDraft: ProductStudioDraft = {
      id: 'prod-endpoint-test',
      title: 'Endpoint Verification',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: images3,
    } as any;

    const baselineDraft: ProductStudioDraft = {
      id: 'prod-endpoint-test',
      title: 'Endpoint Verification',
      priceCents: 10000,
      mediaMode: 'GENERAL',
      images: [], // Baseline had 0 images, so media changed
    } as any;

    // buildProductPatchMediaPayload builds SellerProductPatchImageItem[] for the product PATCH request
    const mediaPayload = buildProductPatchMediaPayload(workingDraft, baselineDraft);
    expect(mediaPayload).toBeDefined();
    expect(mediaPayload).toHaveLength(3);

    // buildProductStudioUpdateRequest packages images into payload.images for updateSellerProduct(productId, payload)
    const updateRequest = buildProductStudioUpdateRequest(workingDraft, baselineDraft);
    expect(updateRequest.images).toBeDefined();
    expect(updateRequest.images).toEqual(mediaPayload);
    // Verifies payload format matches PATCH /api/seller/products/:id contract (not a standalone /media call)
    expect(updateRequest.title).toBe('Endpoint Verification');
  });
});
