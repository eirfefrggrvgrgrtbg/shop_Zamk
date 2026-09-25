/* @vitest-environment jsdom */
import { describe, it, expect, vi } from 'vitest';
import {
  transitionLocalToStagedProductStudioImage,
  stagePendingProductStudioImages,
  mapProductStudioImagesToPatchPayload,
  buildProductPatchMediaPayload,
  type StageImageFn,
} from './productStudioSaveMedia';
import {
  createLocalProductStudioImage,
  createCanonicalProductStudioImage,
} from './productStudioMediaHelper';
import { stageSellerProductImage } from '@zamk/api-client';
import type {
  ProductStudioDraft,
  ProductStudioImage,
} from '../../contexts/ProductStudioContext';

describe('PS.R4B3.1C4C2B1 — Stage Client + Pure Edit Media Save Orchestration', () => {
  function createTestFile(name = 'photo.jpg', content = 'dummy-bits'): File {
    return new File([content], name, { type: 'image/jpeg' });
  }

  // --------------------------------------------------------------------------
  // API CLIENT TESTS (1, 2, 3)
  // --------------------------------------------------------------------------
  describe('Stage API Client Contract (1, 2, 3)', () => {
    it('1, 2, 3. stageSellerProductImage sends POST to correct route with multipart clientMediaId and File', async () => {
      let capturedUrl = '';
      let capturedMethod = '';
      let capturedBody: FormData | null = null;

      const mockFetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
        capturedUrl = url;
        capturedMethod = init.method || 'GET';
        capturedBody = init.body as FormData;
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            stagedMediaId: 'staged-uuid-123',
            clientMediaId: 'client-uuid-456',
            imageUrl: 'https://storage.zamk.test/staged/staged-uuid-123.jpg',
            status: 'ready',
          }),
        };
      });

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mockFetch;

      try {
        const file = createTestFile('test.jpg', 'img-data');
        const res = await stageSellerProductImage('prod-abc', 'client-uuid-456', file);

        // 1. Correct route & method
        expect(capturedMethod).toBe('POST');
        expect(capturedMethod).not.toBe('PUT');
        expect(capturedMethod).not.toBe('GET');
        expect(capturedMethod).not.toBe('PATCH');
        expect(capturedUrl).toBe('http://127.0.0.1:8080/api/seller/products/prod-abc/images/stage');
        expect(capturedUrl).not.toContain('/media/staging');
        expect(capturedUrl).not.toContain('/seller/product/');
        expect(capturedUrl).not.toContain('/image/stage');

        // 2. Multipart includes exact clientMediaId
        expect(capturedBody).toBeInstanceOf(FormData);
        expect(capturedBody!.get('clientMediaId')).toBe('client-uuid-456');

        // 3. Multipart includes original File
        const sentFile = capturedBody!.get('image') as File;
        expect(sentFile).toBe(file);
        expect(sentFile.name).toBe('test.jpg');

        // Response matches backend contract
        expect(res).toEqual({
          stagedMediaId: 'staged-uuid-123',
          clientMediaId: 'client-uuid-456',
          imageUrl: 'https://storage.zamk.test/staged/staged-uuid-123.jpg',
          status: 'ready',
        });
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it('stageSellerProductImage strict contract guarantees: POST only, /api/seller/products/:id/images/stage only', async () => {
      let capturedMethod = '';
      let capturedUrl = '';

      const mockFetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
        capturedUrl = url;
        capturedMethod = init.method || '';
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            stagedMediaId: '00000000-0000-4000-8000-000000000001',
            clientMediaId: '00000000-0000-4000-8000-000000000002',
            imageUrl: 'https://storage.zamk.test/staged/img.jpg',
            status: 'ready',
          }),
        };
      });

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mockFetch;
      try {
        const file = createTestFile('check.png', 'binary-data');
        const pid = 'fb5ff610-8c77-422f-9289-b0988a90b3ac';
        const cid = '11111111-2222-4333-8444-555555555555';
        await stageSellerProductImage(pid, cid, file);

        expect(capturedMethod).toBe('POST');
        expect(capturedUrl).toBe(`http://127.0.0.1:8080/api/seller/products/${pid}/images/stage`);
        expect(capturedUrl.includes('/media/staging')).toBe(false);
        expect(capturedUrl.includes('/images/upload')).toBe(false);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });
  });

  // --------------------------------------------------------------------------
  // PURE LOCAL -> STAGED TRANSITION (4, 5, 6, 7, 8)
  // --------------------------------------------------------------------------
  describe('Pure Local -> Staged Transition (4, 5, 6, 7, 8)', () => {
    it('4-8. preserves uiKey, clientMediaId, previewUrl, metadata, and transforms source to staged', () => {
      const file = createTestFile('photo.jpg');
      const local = createLocalProductStudioImage({
        file,
        previewUrl: 'blob:http://localhost/local-prev',
        colorId: 'col-black',
        isMain: true,
        sortOrder: 2,
        altText: 'Alt text',
      });

      const staged = transitionLocalToStagedProductStudioImage(local, {
        stagedId: 'staged-id-777',
        stagedUrl: 'https://storage.zamk.test/777.jpg',
      });

      // 4. uiKey unchanged
      expect(staged.uiKey).toBe(local.uiKey);
      // 5. clientMediaId unchanged
      expect(staged.source.kind).toBe('staged');
      if (staged.source.kind === 'staged' && local.source.kind === 'local') {
        expect(staged.source.clientMediaId).toBe(local.source.clientMediaId);
        // 6. previewUrl unchanged
        expect(staged.source.previewUrl).toBe('blob:http://localhost/local-prev');
        // Staged identifiers
        expect(staged.source.stagedId).toBe('staged-id-777');
        expect(staged.source.stagedUrl).toBe('https://storage.zamk.test/777.jpg');
      }
      // 7. color/main/altText/sortOrder unchanged
      expect(staged.colorId).toBe('col-black');
      expect(staged.isMain).toBe(true);
      expect(staged.sortOrder).toBe(2);
      expect(staged.altText).toBe('Alt text');
      // 8. source.kind is staged
      expect(staged.source.kind).toBe('staged');
      // File reference is released from source
      expect((staged.source as any).file).toBeUndefined();
    });

    it('returns non-local image untouched', () => {
      const canonical = createCanonicalProductStudioImage({
        imageId: 'img-can',
        url: 'https://cdn/can.jpg',
      });
      const result = transitionLocalToStagedProductStudioImage(canonical, {
        stagedId: 'staged-1',
        stagedUrl: 'https://cdn/1.jpg',
      });
      expect(result).toBe(canonical);
    });
  });

  // --------------------------------------------------------------------------
  // BOUNDED STAGING ORCHESTRATOR (9, 10, 11, 12, 13, 14, 15, 16, 17, 18)
  // --------------------------------------------------------------------------
  describe('Bounded Staging Orchestrator (9 - 18)', () => {
    it('9, 10. skips canonical and already staged images with zero stage calls', async () => {
      const stageMock = vi.fn();
      const canonical = createCanonicalProductStudioImage({
        imageId: 'can-1',
        url: 'https://cdn/can-1.jpg',
      });
      const staged: ProductStudioImage = {
        uiKey: 'staged-1',
        isMain: false,
        source: {
          kind: 'staged',
          clientMediaId: 'client-1',
          stagedId: 'staged-id-1',
          stagedUrl: 'https://cdn/staged-1.jpg',
          previewUrl: 'blob:staged-1',
        },
      };

      const res = await stagePendingProductStudioImages({
        productId: 'prod-1',
        images: [canonical, staged],
        stageImage: stageMock,
      });

      expect(stageMock).not.toHaveBeenCalled();
      expect(res.images).toEqual([canonical, staged]);
      expect(res.failures).toHaveLength(0);
    });

    it('11. stages one local image once', async () => {
      const file = createTestFile('one.jpg');
      const local = createLocalProductStudioImage({ file, previewUrl: 'blob:one' });

      const stageMock: StageImageFn = vi.fn().mockResolvedValue({
        stagedMediaId: 'staged-one-id',
        imageUrl: 'https://cdn/staged-one.jpg',
      });

      const res = await stagePendingProductStudioImages({
        productId: 'prod-1',
        images: [local],
        stageImage: stageMock,
      });

      expect(stageMock).toHaveBeenCalledTimes(1);
      expect(stageMock).toHaveBeenCalledWith('prod-1', (local.source as any).clientMediaId, file);
      expect(res.failures).toHaveLength(0);
      expect(res.images[0].source.kind).toBe('staged');
    });

    it('12, 13, 18. stages multiple local images with bounded concurrency never exceeding 3 and 0 failures on all-success', async () => {
      let activeCalls = 0;
      let maxActiveCalls = 0;

      const stageMock: StageImageFn = vi.fn().mockImplementation(async (_pid, cid, _file) => {
        activeCalls++;
        maxActiveCalls = Math.max(maxActiveCalls, activeCalls);
        // Simulate network delay
        await new Promise((resolve) => setTimeout(resolve, 20));
        activeCalls--;
        return {
          stagedMediaId: `staged-${cid}`,
          imageUrl: `https://cdn/staged-${cid}.jpg`,
        };
      });

      const locals = Array.from({ length: 7 }, (_, i) =>
        createLocalProductStudioImage({
          file: createTestFile(`file-${i}.jpg`),
          previewUrl: `blob:${i}`,
        })
      );

      const res = await stagePendingProductStudioImages({
        productId: 'prod-multi',
        images: locals,
        stageImage: stageMock,
        concurrencyLimit: 3,
      });

      // 12. Multiple local images stage
      expect(stageMock).toHaveBeenCalledTimes(7);
      // 13. Concurrency never exceeds 3
      expect(maxActiveCalls).toBeLessThanOrEqual(3);
      expect(maxActiveCalls).toBeGreaterThanOrEqual(1);
      // 18. All-success result contains zero failures
      expect(res.failures).toHaveLength(0);
      expect(res.images).toHaveLength(7);
      for (const img of res.images) {
        expect(img.source.kind).toBe('staged');
      }
    });

    it('PS.R4B3.1C4C2B1A — capping concurrencyLimit = 10 to maximum 3 active uploads', async () => {
      let activeCalls = 0;
      let maxActiveCalls = 0;

      const stageMock: StageImageFn = vi.fn().mockImplementation(async (_pid, cid, _file) => {
        activeCalls++;
        maxActiveCalls = Math.max(maxActiveCalls, activeCalls);
        await new Promise((resolve) => setTimeout(resolve, 20));
        activeCalls--;
        return {
          stagedMediaId: `staged-${cid}`,
          imageUrl: `https://cdn/staged-${cid}.jpg`,
        };
      });

      const locals = Array.from({ length: 7 }, (_, i) =>
        createLocalProductStudioImage({
          file: createTestFile(`file-${i}.jpg`),
          previewUrl: `blob:${i}`,
        })
      );

      const res = await stagePendingProductStudioImages({
        productId: 'prod-capped-10',
        images: locals,
        stageImage: stageMock,
        concurrencyLimit: 10,
      });

      expect(stageMock).toHaveBeenCalledTimes(7);
      expect(maxActiveCalls).toBeLessThanOrEqual(3);
      expect(maxActiveCalls).toBe(3);
      expect(res.failures).toHaveLength(0);
    });

    it('14, 15. partial failure preserves successful staged results and retains local File on failed entries', async () => {
      const fileA = createTestFile('a.jpg');
      const fileB = createTestFile('b.jpg');
      const fileC = createTestFile('c.jpg');

      const imgA = createLocalProductStudioImage({ file: fileA, previewUrl: 'blob:a' });
      const imgB = createLocalProductStudioImage({ file: fileB, previewUrl: 'blob:b' });
      const imgC = createLocalProductStudioImage({ file: fileC, previewUrl: 'blob:c' });

      const clientBId = (imgB.source as any).clientMediaId;

      const stageMock: StageImageFn = vi.fn().mockImplementation(async (_pid, cid) => {
        if (cid === clientBId) {
          throw new Error('Upload of B failed');
        }
        return {
          stagedMediaId: `staged-${cid}`,
          imageUrl: `https://cdn/staged-${cid}.jpg`,
        };
      });

      const res = await stagePendingProductStudioImages({
        productId: 'prod-partial',
        images: [imgA, imgB, imgC],
        stageImage: stageMock,
      });

      // 14. One failure does NOT erase successful stage results
      expect(res.images[0].source.kind).toBe('staged');
      expect(res.images[2].source.kind).toBe('staged');

      // 15. Failed entry stays local and retains original File
      expect(res.images[1].source.kind).toBe('local');
      if (res.images[1].source.kind === 'local') {
        expect(res.images[1].source.file).toBe(fileB);
        expect(res.images[1].source.clientMediaId).toBe(clientBId);
      }

      // Failures array exposes failure details
      expect(res.failures).toHaveLength(1);
      expect(res.failures[0].uiKey).toBe(imgB.uiKey);
      expect(res.failures[0].clientMediaId).toBe(clientBId);
    });

    it('16, 17. retry stages only failed/local entries using the exact same clientMediaId', async () => {
      const fileA = createTestFile('a.jpg');
      const fileB = createTestFile('b.jpg');

      const imgA = createLocalProductStudioImage({ file: fileA, previewUrl: 'blob:a' });
      const imgB = createLocalProductStudioImage({ file: fileB, previewUrl: 'blob:b' });
      const clientBId = (imgB.source as any).clientMediaId;

      // First run: B fails
      const stageMock1: StageImageFn = vi.fn().mockImplementation(async (_pid, cid) => {
        if (cid === clientBId) {
          throw new Error('500 Server Error');
        }
        return {
          stagedMediaId: `staged-${cid}`,
          imageUrl: `https://cdn/staged-${cid}.jpg`,
        };
      });

      const firstRun = await stagePendingProductStudioImages({
        productId: 'prod-retry',
        images: [imgA, imgB],
        stageImage: stageMock1,
      });

      expect(firstRun.images[0].source.kind).toBe('staged');
      expect(firstRun.images[1].source.kind).toBe('local');
      expect(firstRun.failures).toHaveLength(1);

      // Second run (retry): B now succeeds
      const stageMock2: StageImageFn = vi.fn().mockImplementation(async (_pid, cid) => {
        return {
          stagedMediaId: `staged-${cid}`,
          imageUrl: `https://cdn/staged-${cid}.jpg`,
        };
      });

      const secondRun = await stagePendingProductStudioImages({
        productId: 'prod-retry',
        images: firstRun.images,
        stageImage: stageMock2,
      });

      // 16. Retry stages only failed/local entries (A was skipped!)
      expect(stageMock2).toHaveBeenCalledTimes(1);
      // 17. Retry uses same clientMediaId
      expect(stageMock2).toHaveBeenCalledWith('prod-retry', clientBId, fileB);

      expect(secondRun.failures).toHaveLength(0);
      expect(secondRun.images[0].source.kind).toBe('staged');
      expect(secondRun.images[1].source.kind).toBe('staged');
    });
  });

  // --------------------------------------------------------------------------
  // MEDIA PATCH PAYLOAD BUILDER (19, 20, 21, 22, 23, 24, 28)
  // --------------------------------------------------------------------------
  describe('Media PATCH Payload Builder (19 - 24, 28)', () => {
    it('19, 20, 21, 22, 23, 28. maps canonical and staged images to ID-only payload with metadata and order', () => {
      const canonical = createCanonicalProductStudioImage({
        imageId: 'img-can-999',
        url: 'https://cdn/canonical.jpg',
        colorId: 'col-black',
        isMain: false,
        altText: 'Front',
      });
      const staged: ProductStudioImage = {
        uiKey: 'client-staged-1',
        isMain: true,
        colorId: null,
        altText: 'Side',
        source: {
          kind: 'staged',
          clientMediaId: 'client-staged-1',
          stagedId: 'staged-uuid-888',
          stagedUrl: 'https://cdn/staged.jpg',
          previewUrl: 'blob:preview-1',
        },
      };

      // Order: staged first, canonical second
      const payload = mapProductStudioImagesToPatchPayload([staged, canonical]);

      // 21. Array ordering matches input
      expect(payload).toHaveLength(2);

      // 20. Staged uses stagedId
      expect(payload[0]).toEqual({
        id: 'staged-uuid-888',
        isMain: true,
        colorId: null,
        altText: 'Side',
      });

      // 19. Canonical uses imageId
      expect(payload[1]).toEqual({
        id: 'img-can-999',
        isMain: false,
        colorId: 'col-black',
        altText: 'Front',
      });

      // 23, 28. No blob/preview/clientMediaId/uiKey/imageUrl identity leak
      for (const item of payload) {
        expect((item as any).uiKey).toBeUndefined();
        expect((item as any).clientMediaId).toBeUndefined();
        expect((item as any).previewUrl).toBeUndefined();
        expect((item as any).stagedUrl).toBeUndefined();
        expect((item as any).imageUrl).toBeUndefined();
      }
    });

    it('24. rejects remaining local image with clear orchestration error', () => {
      const file = createTestFile('local.jpg');
      const local = createLocalProductStudioImage({ file, previewUrl: 'blob:loc' });
      const canonical = createCanonicalProductStudioImage({
        imageId: 'can-1',
        url: 'https://cdn/can.jpg',
      });

      expect(() => {
        mapProductStudioImagesToPatchPayload([canonical, local]);
      }).toThrowError(/Cannot build media PATCH payload: image with uiKey ".*" is still local/);
    });
  });

  // --------------------------------------------------------------------------
  // OMITTED VS EMPTY BUILDER SEMANTICS (25, 26, 27)
  // --------------------------------------------------------------------------
  describe('Omitted vs Empty Builder Semantics (25, 26, 27)', () => {
    it('25. media untouched and variant colors unchanged -> images property omitted (undefined)', () => {
      const canonical = createCanonicalProductStudioImage({
        imageId: 'img-1',
        url: 'https://cdn/1.jpg',
        isMain: true,
      });

      const baseDraft: ProductStudioDraft = {
        id: 'prod-1',
        title: 'Title',
        images: [canonical],
        variants: [{ id: 'v-1', colorId: 'col-1', size: 'M', priceCents: 100 }],
      } as any;

      const currentDraft: ProductStudioDraft = {
        ...baseDraft,
        title: 'New Title', // text change only
      };

      const payload = buildProductPatchMediaPayload(currentDraft, baseDraft);
      expect(payload).toBeUndefined();
    });

    it('26. remove-all dirty -> images: []', () => {
      const canonical = createCanonicalProductStudioImage({
        imageId: 'img-1',
        url: 'https://cdn/1.jpg',
        isMain: true,
      });

      const baseDraft: ProductStudioDraft = {
        id: 'prod-1',
        images: [canonical],
        variants: [],
      } as any;

      const currentDraft: ProductStudioDraft = {
        ...baseDraft,
        images: [], // seller removed all photos
      };

      const payload = buildProductPatchMediaPayload(currentDraft, baseDraft);
      expect(payload).toEqual([]);
    });

    it('27. variant color-domain change -> images included even if media unchanged', () => {
      const canonical = createCanonicalProductStudioImage({
        imageId: 'img-1',
        url: 'https://cdn/1.jpg',
        colorId: 'col-1',
        isMain: true,
      });

      const baseDraft: ProductStudioDraft = {
        id: 'prod-1',
        images: [canonical],
        variants: [{ id: 'v-1', colorId: 'col-1', size: 'M', priceCents: 100 }],
      } as any;

      // Media array unchanged, but variant changed color to col-2
      const currentDraft: ProductStudioDraft = {
        ...baseDraft,
        variants: [{ id: 'v-1', colorId: 'col-2', size: 'M', priceCents: 100 }],
      };

      const payload = buildProductPatchMediaPayload(currentDraft, baseDraft);
      expect(payload).toBeDefined();
      expect(payload).toEqual([
        {
          id: 'img-1',
          isMain: true,
          colorId: 'col-1',
          altText: null,
        },
      ]);
    });
  });
});
