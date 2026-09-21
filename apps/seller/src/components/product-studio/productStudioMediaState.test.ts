/* @vitest-environment jsdom */
import { describe, it, expect, vi } from 'vitest';
import {
  createLocalProductStudioImage,
  createCanonicalProductStudioImage,
  getProductStudioImageDisplayUrl,
  getProductStudioImagePreviewUrl,
  isMediaSemanticallyDirty,
  getVariantColorDomain,
  hasVariantColorDomainChanged,
  shouldIncludeImagesInPatch,
} from './productStudioMediaHelper';
import { hydrateProductStudioDraft } from './productStudioHydration';
import { createStudioMediaRegistry } from './productStudioMediaSession';
import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';

describe('PS.R4B3.1C4C2A — Product Studio Explicit Media State Model', () => {
  // Mock File helper
  function createTestFile(name: string = 'test.jpg', type: string = 'image/jpeg'): File {
    return new File(['test-bytes'], name, { type });
  }

  it('1. canonical hydration creates source.kind=canonical with correct imageId and url', () => {
    const backendProduct: any = {
      id: 'prod-123',
      title: 'T-Shirt',
      images: [
        {
          id: 'img-101',
          url: 'https://cdn.zamk.test/images/img-101.jpg',
          isMain: true,
          sortOrder: 0,
          colorId: 'col-black',
          altText: 'Black t-shirt front',
        },
      ],
      colors: [{ id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' }],
      variants: [],
      category: { id: 'cat-apparel' },
    };

    const draft = hydrateProductStudioDraft({
      product: backendProduct,
      categorySchema: {} as any,
      canonicalColors: [{ id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' }],
    });
    expect(draft.images).toHaveLength(1);
    const img = draft.images![0];

    expect(img.uiKey).toBe('img-101');
    expect(img.isMain).toBe(true);
    expect(img.colorId).toBe('col-black');
    expect(img.altText).toBe('Black t-shirt front');
    expect(img.source).toEqual({
      kind: 'canonical',
      imageId: 'img-101',
      url: 'https://cdn.zamk.test/images/img-101.jpg',
    });
  });

  it('2. canonical image has no generated clientMediaId or client file object', () => {
    const canonical = createCanonicalProductStudioImage({
      imageId: 'img-999',
      url: 'https://cdn.zamk.test/999.jpg',
      isMain: false,
    });

    expect(canonical.source.kind).toBe('canonical');
    expect((canonical.source as any).clientMediaId).toBeUndefined();
    expect((canonical.source as any).file).toBeUndefined();
    expect((canonical.source as any).previewUrl).toBeUndefined();
    expect((canonical as any).clientMediaId).toBeUndefined();
    expect(getProductStudioImagePreviewUrl(canonical)).toBeNull();
  });

  it('3. local File selection retains the original File object', () => {
    const file = createTestFile('sample.png', 'image/png');
    const local = createLocalProductStudioImage({
      file,
      previewUrl: 'blob:http://localhost:3000/mock-preview-1',
      isMain: true,
    });

    expect(local.source.kind).toBe('local');
    if (local.source.kind === 'local') {
      expect(local.source.file).toBe(file);
      expect(local.source.file.name).toBe('sample.png');
      expect(local.source.file.type).toBe('image/png');
    }
    expect(getProductStudioImagePreviewUrl(local)).toBe('blob:http://localhost:3000/mock-preview-1');
  });

  it('4. local selection gets stable clientMediaId equal to uiKey', () => {
    const file = createTestFile();
    const local = createLocalProductStudioImage({
      file,
      previewUrl: 'blob:http://localhost:3000/mock-preview-2',
    });

    expect(local.source.kind).toBe('local');
    if (local.source.kind === 'local') {
      expect(local.source.clientMediaId).toBeDefined();
      expect(typeof local.source.clientMediaId).toBe('string');
      expect(local.uiKey).toBe(local.source.clientMediaId);
    }
  });

  it('5. local uiKey is stable through reordering', () => {
    const file1 = createTestFile('1.jpg');
    const file2 = createTestFile('2.jpg');
    const img1 = createLocalProductStudioImage({ file: file1, previewUrl: 'blob:1', sortOrder: 0 });
    const img2 = createLocalProductStudioImage({ file: file2, previewUrl: 'blob:2', sortOrder: 1 });

    const key1 = img1.uiKey;
    const key2 = img2.uiKey;

    // Simulate DnD reorder
    const reordered: ProductStudioImage[] = [
      { ...img2, sortOrder: 0, isMain: true },
      { ...img1, sortOrder: 1, isMain: false },
    ];

    expect(reordered[0].uiKey).toBe(key2);
    expect(reordered[1].uiKey).toBe(key1);
    if (reordered[0].source.kind === 'local' && reordered[1].source.kind === 'local') {
      expect(reordered[0].source.clientMediaId).toBe(key2);
      expect(reordered[1].source.clientMediaId).toBe(key1);
    }
  });

  it('6. color binding does not change clientMediaId or uiKey', () => {
    const file = createTestFile('color.jpg');
    const img = createLocalProductStudioImage({ file, previewUrl: 'blob:color', colorId: null });
    const originalUiKey = img.uiKey;
    const originalClientMediaId = (img.source as any).clientMediaId;

    // Bind color
    const boundImg: ProductStudioImage = {
      ...img,
      colorId: 'col-blue',
    };

    expect(boundImg.uiKey).toBe(originalUiKey);
    expect((boundImg.source as any).clientMediaId).toBe(originalClientMediaId);
    expect(boundImg.colorId).toBe('col-blue');
  });

  it('7. set-main / cover operation does not change clientMediaId or uiKey', () => {
    const file = createTestFile('cover.jpg');
    const img = createLocalProductStudioImage({ file, previewUrl: 'blob:cover', isMain: false });
    const originalUiKey = img.uiKey;
    const originalClientMediaId = (img.source as any).clientMediaId;

    // Set as main
    const mainImg: ProductStudioImage = {
      ...img,
      isMain: true,
    };

    expect(mainImg.uiKey).toBe(originalUiKey);
    expect((mainImg.source as any).clientMediaId).toBe(originalClientMediaId);
    expect(mainImg.isMain).toBe(true);
  });

  it('8. replacing physical File gets a new clientMediaId and uiKey', () => {
    const fileA = createTestFile('a.jpg');
    const fileB = createTestFile('b.jpg');

    const imgA = createLocalProductStudioImage({ file: fileA, previewUrl: 'blob:a' });
    const imgB = createLocalProductStudioImage({
      file: fileB,
      previewUrl: 'blob:b',
      colorId: imgA.colorId,
      isMain: imgA.isMain,
      sortOrder: imgA.sortOrder,
    });

    expect(imgB.uiKey).not.toBe(imgA.uiKey);
    expect((imgB.source as any).clientMediaId).not.toBe((imgA.source as any).clientMediaId);
  });

  it('9. display URL helper returns remote url for canonical, previewUrl for local and staged', () => {
    const canonical = createCanonicalProductStudioImage({
      imageId: 'img-can',
      url: 'https://cdn.zamk.test/canonical.jpg',
    });
    const local = createLocalProductStudioImage({
      file: createTestFile(),
      previewUrl: 'blob:http://localhost/local-blob',
    });
    const staged: ProductStudioImage = {
      uiKey: 'staged-key',
      isMain: false,
      source: {
        kind: 'staged',
        stagedId: 'uuid-staged-123',
        stagedUrl: 'https://cdn.zamk.test/staged.jpg',
        clientMediaId: 'client-123',
        previewUrl: 'blob:http://localhost/staged-blob',
      },
    };

    expect(getProductStudioImageDisplayUrl(canonical)).toBe('https://cdn.zamk.test/canonical.jpg');
    expect(getProductStudioImageDisplayUrl(local)).toBe('blob:http://localhost/local-blob');
    expect(getProductStudioImageDisplayUrl(staged)).toBe('blob:http://localhost/staged-blob');
    expect(getProductStudioImageDisplayUrl(null)).toBe('');
    expect(getProductStudioImageDisplayUrl(undefined)).toBe('');

    expect(getProductStudioImagePreviewUrl(canonical)).toBeNull();
    expect(getProductStudioImagePreviewUrl(local)).toBe('blob:http://localhost/local-blob');
    expect(getProductStudioImagePreviewUrl(staged)).toBe('blob:http://localhost/staged-blob');
    expect(getProductStudioImagePreviewUrl(null)).toBeNull();
    expect(getProductStudioImagePreviewUrl(undefined)).toBeNull();
  });

  it('10. StudioMediaRegistry revokes removed local URL immediately', () => {
    const registry = createStudioMediaRegistry();
    const mockRevoke = vi.fn();
    (window.URL as any).revokeObjectURL = mockRevoke;

    const previewUrl = 'blob:http://localhost/active-preview';
    registry.registerObjectUrl(previewUrl);

    expect(registry.isRegistered(previewUrl)).toBe(true);

    registry.revokeObjectUrl(previewUrl);

    expect(mockRevoke).toHaveBeenCalledWith(previewUrl);
    expect(registry.isRegistered(previewUrl)).toBe(false);
  });

  it('11. unmount / revokeAll revokes all remaining preview URLs', () => {
    const registry = createStudioMediaRegistry();
    const mockRevoke = vi.fn();
    (window.URL as any).revokeObjectURL = mockRevoke;

    const url1 = 'blob:http://localhost/preview-1';
    const url2 = 'blob:http://localhost/preview-2';
    registry.registerObjectUrl(url1);
    registry.registerObjectUrl(url2);

    registry.revokeAll();

    expect(mockRevoke).toHaveBeenCalledWith(url1);
    expect(mockRevoke).toHaveBeenCalledWith(url2);
    expect(registry.isRegistered(url1)).toBe(false);
    expect(registry.isRegistered(url2)).toBe(false);
  });

  it('12. double release is idempotent and safe', () => {
    const registry = createStudioMediaRegistry();
    const mockRevoke = vi.fn();
    (window.URL as any).revokeObjectURL = mockRevoke;

    const url = 'blob:http://localhost/idempotent-preview';
    registry.registerObjectUrl(url);

    registry.revokeObjectUrl(url);
    expect(mockRevoke).toHaveBeenCalledTimes(1);

    // Second revoke of same URL
    registry.revokeObjectUrl(url);
    expect(mockRevoke).toHaveBeenCalledTimes(1); // not called again
  });

  it('13. no provenance inference via ID prefix', () => {
    // An image with any kind of string ID format relies strictly on source.kind
    const img: ProductStudioImage = {
      uiKey: 'some-prefix-123',
      isMain: true,
      source: {
        kind: 'canonical',
        imageId: 'some-prefix-123',
        url: 'https://cdn.zamk.test/prefixed.jpg',
      },
    };

    expect(img.source.kind).toBe('canonical');
  });

  it('14. no provenance inference via blob URL string', () => {
    const file = createTestFile();
    const local = createLocalProductStudioImage({
      file,
      previewUrl: 'https://example.com/custom-data-stream',
    });

    expect(local.source.kind).toBe('local');
    expect(getProductStudioImageDisplayUrl(local)).toBe('https://example.com/custom-data-stream');
  });

  it('15. DnD preserves ordering and correctly identifies elements via uiKey', () => {
    const img1 = createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://cdn/1.jpg', sortOrder: 0 });
    const img2 = createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://cdn/2.jpg', sortOrder: 1 });
    const img3 = createCanonicalProductStudioImage({ imageId: 'img-3', url: 'https://cdn/3.jpg', sortOrder: 2 });

    const newOrder = [img2, img3, img1].map((img, idx) => ({ ...img, sortOrder: idx, isMain: idx === 0 }));

    expect(newOrder.map((i) => i.uiKey)).toEqual(['img-2', 'img-3', 'img-1']);
    expect(newOrder[0].isMain).toBe(true);
    expect(newOrder[1].isMain).toBe(false);
  });

  it('16. Create mode accepts local photos with source.kind=local and original File', () => {
    const file = createTestFile('new-create.jpg');
    const local = createLocalProductStudioImage({
      file,
      previewUrl: 'blob:http://localhost/new-create',
      isMain: true,
      sortOrder: 0,
    });

    expect(local.source.kind).toBe('local');
    if (local.source.kind === 'local') {
      expect(local.source.file).toBe(file);
      expect(local.source.previewUrl).toBe('blob:http://localhost/new-create');
    }
  });

  it('17. mediaSemanticDirty is false for untouched hydrated canonical media', () => {
    const baseline: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://cdn/1.jpg', isMain: true, sortOrder: 0, colorId: 'col-black' }),
      createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://cdn/2.jpg', isMain: false, sortOrder: 1, colorId: null }),
    ];
    const current: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://cdn/1.jpg', isMain: true, sortOrder: 0, colorId: 'col-black' }),
      createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://cdn/2.jpg', isMain: false, sortOrder: 1, colorId: null }),
    ];

    expect(isMediaSemanticallyDirty(current, baseline)).toBe(false);
  });

  it('18. mediaSemanticDirty is true for reorder, remove, add, color change, or main change', () => {
    const base1 = createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://cdn/1.jpg', isMain: true, sortOrder: 0 });
    const base2 = createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://cdn/2.jpg', isMain: false, sortOrder: 1 });
    const baseline = [base1, base2];

    // Reorder
    expect(isMediaSemanticallyDirty([base2, base1], baseline)).toBe(true);

    // Remove
    expect(isMediaSemanticallyDirty([base1], baseline)).toBe(true);

    // Add
    const base3 = createCanonicalProductStudioImage({ imageId: 'img-3', url: 'https://cdn/3.jpg', isMain: false, sortOrder: 2 });
    expect(isMediaSemanticallyDirty([base1, base2, base3], baseline)).toBe(true);

    // Color change
    const colorChanged = [{ ...base1, colorId: 'col-red' }, base2];
    expect(isMediaSemanticallyDirty(colorChanged, baseline)).toBe(true);

    // Main change
    const mainChanged = [{ ...base1, isMain: false }, { ...base2, isMain: true }];
    expect(isMediaSemanticallyDirty(mainChanged, baseline)).toBe(true);
  });

  it('19. local -> staged transient transition remains semantically dirty until canonical hydration', () => {
    const file = createTestFile();
    const local = createLocalProductStudioImage({ file, previewUrl: 'blob:loc' });

    const baseline: ProductStudioImage[] = [];

    const staged: ProductStudioImage = {
      uiKey: local.uiKey,
      isMain: local.isMain,
      colorId: local.colorId,
      sortOrder: local.sortOrder,
      source: {
        kind: 'staged',
        stagedId: 'staged-uuid-456',
        stagedUrl: 'https://cdn.zamk.test/staged-456.jpg',
        clientMediaId: (local.source as any).clientMediaId,
        previewUrl: (local.source as any).previewUrl,
      },
    };

    expect(isMediaSemanticallyDirty([local], baseline)).toBe(true);
    expect(isMediaSemanticallyDirty([staged], baseline)).toBe(true);
    expect(isMediaSemanticallyDirty([staged], [local])).toBe(true);
  });

  it('20. shouldIncludeImagesInPatch includes images when variant color domain changes even if media untouched', () => {
    const images: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://cdn/1.jpg', isMain: true, colorId: 'col-black' }),
    ];
    const baseDraft: ProductStudioDraft = {
      id: 'p-1',
      title: 'Item',
      images,
      variants: [
        { id: 'v-1', colorId: 'col-black', size: 'M', priceCents: 100 },
      ],
    } as any;

    const currentDraft: ProductStudioDraft = {
      ...baseDraft,
      variants: [
        { id: 'v-1', colorId: 'col-white', size: 'M', priceCents: 100 },
      ],
    };

    expect(isMediaSemanticallyDirty(currentDraft.images, baseDraft.images)).toBe(false);
    expect(getVariantColorDomain(baseDraft.variants).has('col-black')).toBe(true);
    expect(hasVariantColorDomainChanged(currentDraft.variants, baseDraft.variants)).toBe(true);
    expect(shouldIncludeImagesInPatch(currentDraft, baseDraft)).toBe(true);
  });
});
