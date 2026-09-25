/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import React from 'react';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
  type ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import { ProductStudio } from './ProductStudio';
import {
  mergeRetainedLocalMedia,
} from './productStudioSaveCreate';
import {
  PRODUCT_STUDIO_CREATE_SESSION_KEY,
  loadProductStudioCreateSession,
  saveProductStudioCreateSession,
  prepareAddProductNavigation,
} from './productStudioCreateSession';
import { SellerProductStudioNew } from '../../pages/SellerProductStudioNew';
import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerDictionaryValue,
} from '@zamk/api-client';

describe('PS.R4B3.1C4C3B2 — Product Studio Create Save + Durable Recovery', () => {
  const mockColors: SellerColor[] = [
    { id: 'col-black', code: 'BLACK', nameRu: 'Черный', hex: '#000000' },
    { id: 'col-white', code: 'WHITE', nameRu: 'Белый', hex: '#FFFFFF' },
  ];

  const mockSchema: SellerCategorySchema = {
    id: 'cat-hoodies',
    name: 'Худи',
    slug: 'hoodies',
    dimensionType: 'COLOR_AND_SIZE',
    sizeChartRequired: false,
    allowedSizeSystems: [],
    sizeChartFields: [],
    attributes: [
      {
        id: 'attr-season',
        code: 'SEASON',
        nameRu: 'Сезон',
        valueType: 'ENUM',
        valueSource: 'DICTIONARY',
        scope: 'PRODUCT',
        required: false,
        filterable: true,
        variantAxis: false,
        sortOrder: 1,
        dictionaryId: 'dict-season',
      },
    ],
  };

  const mockDictMap: Record<string, SellerDictionaryValue[]> = {
    'dict-season': [
      { id: 'dict-val-autumn', dictionaryId: 'dict-season', code: 'AUTUMN', nameRu: 'Осень' },
    ],
  };

  const sampleInitialCreateDraft: ProductStudioDraft = {
    title: 'Худи Новое Оверсайз',
    description: 'Теплый футер',
    categoryId: 'cat-hoodies',
    categoryName: 'Худи',
    brandId: 'brand-1',
    priceCents: 450000,
    currency: 'RUB',
    variants: [
      {
        id: 'synthetic-v-1', // Pre-create synthetic ID! Must NOT reach backend!
        colorId: 'col-black',
        sizeValueId: 'sz-m',
        sellerSku: 'SKU-HOODIE-M',
        priceCents: 450000,
      },
    ],
    materialComposition: [
      { materialId: 'mat-cotton', percentage: 100 },
    ],
  };

  const sampleCanonicalCreatedProduct: SellerProduct = {
    id: 'prod-canonical-created-123',
    title: 'Худи Новое Оверсайз',
    description: 'Теплый футер',
    categoryId: 'cat-hoodies',
    categoryName: 'Худи',
    brandId: 'brand-1',
    brandName: 'Brand 1',
    sellerId: 'seller-1',
    status: 'draft',
    priceCents: 450000,
    currency: 'RUB',
    slug: 'hoodie-new',
    createdAt: '2026-09-21T12:00:00Z',
    images: [],
    variants: [
      {
        id: '00000000-0000-4000-8000-000000000999', // Canonical backend UUID
        productId: 'prod-canonical-created-123',
        colorId: 'col-black',
        colorName: 'Черный',
        colorHex: '#000000',
        sizeValueId: 'sz-m',
        size: 'M',
        sellerSku: 'SKU-HOODIE-M',
        barcode: '200000000001',
        priceCents: 450000,
        isActive: true,
      },
    ],
  };

  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  function TestControls() {
    const { draft, updateDraft, isDirty, canSave, saveStatus, saveDraft } = useProductStudio();
    return (
      <div data-testid="test-controls" className="hidden">
        <span data-testid="test-is-dirty">{isDirty ? 'DIRTY' : 'CLEAN'}</span>
        <span data-testid="test-can-save">{canSave ? 'CAN_SAVE' : 'CANNOT_SAVE'}</span>
        <span data-testid="test-save-status">{saveStatus}</span>
        <span data-testid="test-draft-title">{draft.title}</span>
        <div data-testid="test-images-summary">
          {draft.images?.map((img) => `${img.uiKey}:${img.source.kind}`).join(',')}
        </div>
        <button
          type="button"
          data-testid="test-mutate-title"
          onClick={() => updateDraft({ title: 'Mutated Title' })}
        >
          Mutate Title
        </button>
        <button
          type="button"
          data-testid="test-call-save"
          onClick={() => saveDraft && saveDraft()}
        >
          Call Save
        </button>
      </div>
    );
  }

  function renderTestCreateStudio(props: Partial<React.ComponentProps<typeof ProductStudioProvider>> = {}) {
    const defaultGetProduct = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
    const defaultCreateProduct = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
    const defaultUpdateProduct = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
    const defaultNavigate = vi.fn();

    const view = render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="create"
          initialDraft={sampleInitialCreateDraft}
          initialCategorySchema={mockSchema}
          canonicalColors={mockColors}
          dictionaryValuesMap={mockDictMap}
          getProductFn={defaultGetProduct}
          createProductFn={defaultCreateProduct}
          saveProductFn={defaultUpdateProduct}
          onNavigate={defaultNavigate}
          {...props}
        >
          <ProductStudio />
          <TestControls />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    return {
      ...view,
      defaultGetProduct,
      defaultCreateProduct,
      defaultUpdateProduct,
      defaultNavigate,
    };
  }

  /* ========================================================================
   * A. Basic Create
   * ======================================================================== */
  describe('A. Basic Create', () => {
    it('1. fresh /products/new does NOT POST automatically', async () => {
      const mockCreate = vi.fn();
      renderTestCreateStudio({ createProductFn: mockCreate });

      expect(mockCreate).not.toHaveBeenCalled();
      expect(sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY)).toBeNull();
    });

    it('2. Create Save disabled when minimum identity fields invalid', () => {
      // Empty title is invalid
      renderTestCreateStudio({
        initialDraft: { ...sampleInitialCreateDraft, title: '' },
      });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(true);
      expect(screen.getByTestId('test-can-save').textContent).toBe('CANNOT_SAVE');
    });

    it('3. valid Create Save enabled', () => {
      renderTestCreateStudio();

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(false);
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');
    });

    it('3a. sessionStorage write failure fails closed: zero Create POST', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const setItemSpy = vi.spyOn(Object.getPrototypeOf(sessionStorage), 'setItem').mockImplementation(() => {
        throw new Error('QuotaExceededError');
      });

      renderTestCreateStudio({ createProductFn: mockCreate });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('error');
      });

      // Crucial: zero createProductFn calls!
      expect(mockCreate).toHaveBeenCalledTimes(0);

      setItemSpy.mockRestore();
    });

    it('3b. Create Save eligibility != moderation readiness: minimal draft can save as draft while moderation is disabled', () => {
      // Draft has title and price, but zero photos, incomplete composition, no size chart
      const minimalDraft: ProductStudioDraft = {
        title: 'Минимальный товар для черновика',
        description: '',
        priceCents: 100000,
        currency: 'RUB',
        categoryId: 'cat-hoodies',
        images: [],
        variants: [],
      };

      renderTestCreateStudio({ initialDraft: minimalDraft });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      const modBtn = screen.getByText('Отправить на модерацию').closest('button');

      // Save is enabled because minimum identity / persistence rules are met
      expect(saveBtn.hasAttribute('disabled')).toBe(false);
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');

      // Moderation is blocked because full catalogue readiness is not met
      expect(modBtn?.hasAttribute('disabled')).toBe(true);
    });

    it('4. Save sends exactly one POST with Idempotency-Key', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      renderTestCreateStudio({ createProductFn: mockCreate });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledTimes(1);
      });

      const callArgs = mockCreate.mock.calls[0];
      expect(callArgs[1]?.idempotencyKey).toBeTruthy();
      expect(typeof callArgs[1].idempotencyKey).toBe('string');
      expect(callArgs[1].idempotencyKey).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
      );
    });

    it('5 & 6. POST body contains non-media Product Studio state and NO local/blob media', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);

      const fakeFile = new File(['fake content'], 'photo.jpg', { type: 'image/jpeg' });
      const draftWithMedia: ProductStudioDraft = {
        ...sampleInitialCreateDraft,
        images: [
          {
            uiKey: 'local-1',
            isMain: true,
            sortOrder: 0,
            colorId: 'col-black',
            source: {
              kind: 'local',
              clientMediaId: 'client-media-1',
              file: fakeFile,
              previewUrl: 'blob:http://localhost/fake-preview',
            },
          },
        ],
      };

      renderTestCreateStudio({
        initialDraft: draftWithMedia,
        createProductFn: mockCreate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledTimes(1);
      });

      const body = mockCreate.mock.calls[0][0];
      expect(body.title).toBe('Худи Новое Оверсайз');
      expect(body.priceCents).toBe(450000);
      expect(body.currency).toBe('RUB');
      expect(body.categoryId).toBe('cat-hoodies');

      // Crucial: NO media in Create request!
      expect(body.images).toBeUndefined();
      expect(body.mainImageUrl).toBeUndefined();
      expect(JSON.stringify(body)).not.toContain('blob:');
      expect(JSON.stringify(body)).not.toContain('client-media-1');
    });

    it('PS.R4B3.1C4C3B3B-R2: Create request sends ONLY active tuples in sparse matrix', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const sparseDraft: ProductStudioDraft = {
        ...sampleInitialCreateDraft,
        colors: [
          { id: 'col-black', name: 'Черный', hex: '#000000' },
          { id: 'col-grey', name: 'Серый', hex: '#808080' },
        ],
        variants: [
          { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-m', priceCents: 450000, isActive: true },
          { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-l', priceCents: 450000, isActive: true },
          { id: 'v3', colorId: 'col-grey', sizeValueId: 'sz-l', priceCents: 450000, isActive: true },
          // Grey/M is OFF (isActive: false)
          { id: 'v4', colorId: 'col-grey', sizeValueId: 'sz-m', priceCents: 450000, isActive: false },
        ],
      };

      renderTestCreateStudio({ initialDraft: sparseDraft, createProductFn: mockCreate });
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledTimes(1);
      });

      const body = mockCreate.mock.calls[0][0];
      expect(body.variants).toHaveLength(3);
      const variantPairs = body.variants.map((v: any) => `${v.colorId}:${v.sizeValueId}`);
      expect(variantPairs).toContain('col-black:sz-m');
      expect(variantPairs).toContain('col-black:sz-l');
      expect(variantPairs).toContain('col-grey:sz-l');
      expect(variantPairs).not.toContain('col-grey:sz-m');
    });

    it('7. snapshot written before POST', async () => {
      let snapshotAtCallTime: any = null;
      const mockCreate = vi.fn().mockImplementation(async () => {
        // Inspect sessionStorage exactly when createProductFn is executed
        snapshotAtCallTime = loadProductStudioCreateSession();
        return sampleCanonicalCreatedProduct;
      });

      renderTestCreateStudio({ createProductFn: mockCreate });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledTimes(1);
      });

      expect(snapshotAtCallTime).not.toBeNull();
      expect(snapshotAtCallTime.phase).toBe('identity_pending');
      expect(snapshotAtCallTime.createRequestSnapshot.title).toBe('Худи Новое Оверсайз');
    });
  });

  /* ========================================================================
   * B. Canonicalization
   * ======================================================================== */
  describe('B. Canonicalization', () => {
    it('8. POST success stores productId into session', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      let sessionDuringGet: any = null;
      const mockGet = vi.fn().mockImplementation(async (_id: string) => {
        sessionDuringGet = loadProductStudioCreateSession();
        return sampleCanonicalCreatedProduct;
      });

      renderTestCreateStudio({
        createProductFn: mockCreate,
        getProductFn: mockGet,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockGet).toHaveBeenCalledWith('prod-canonical-created-123');
      });

      expect(sessionDuringGet).not.toBeNull();
      expect(sessionDuringGet.productId).toBe('prod-canonical-created-123');
      expect(sessionDuringGet.phase).toBe('identity_established');
    });

    it('9 & 10. canonical GET happens after POST and Product Studio hydrates from GET', async () => {
      const callOrder: string[] = [];
      const mockCreate = vi.fn().mockImplementation(async () => {
        callOrder.push('CREATE_POST');
        return { id: 'prod-canonical-created-123' } as any; // Deliberately incomplete POST response
      });
      const mockGet = vi.fn().mockImplementation(async (_id: string) => {
        callOrder.push('CANONICAL_GET');
        return sampleCanonicalCreatedProduct;
      });

      renderTestCreateStudio({
        createProductFn: mockCreate,
        getProductFn: mockGet,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(callOrder).toEqual(['CREATE_POST', 'CANONICAL_GET']);
      });
    });

    it('11. pre-create synthetic variant IDs do not reach final PATCH', async () => {
      const fakeFile = new File(['fake content'], 'photo.jpg', { type: 'image/jpeg' });
      const draftWithMedia: ProductStudioDraft = {
        ...sampleInitialCreateDraft,
        images: [
          {
            uiKey: 'local-1',
            isMain: true,
            source: {
              kind: 'local',
              clientMediaId: 'media-1',
              file: fakeFile,
              previewUrl: 'blob:test',
            },
          },
        ],
      };

      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockStage = vi.fn().mockResolvedValue({
        id: 'staged-1',
        imageUrl: 'https://cdn.example.com/staged-1.jpg',
      });
      const mockUpdate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);

      renderTestCreateStudio({
        initialDraft: draftWithMedia,
        createProductFn: mockCreate,
        getProductFn: mockGet,
        stageImageFn: mockStage,
        saveProductFn: mockUpdate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockUpdate).toHaveBeenCalledTimes(1);
      });

      const patchPayload = mockUpdate.mock.calls[0][1];
      expect(patchPayload.variants).toBeTruthy();
      // Must contain backend canonical variant ID, NEVER synthetic-v-1
      expect(patchPayload.variants[0].id).toBe('00000000-0000-4000-8000-000000000999');
      expect(JSON.stringify(patchPayload)).not.toContain('synthetic-v-1');
    });

    it('12. local media survives canonical GET merge', () => {
      const fakeFile = new File(['fake content'], 'photo.jpg', { type: 'image/jpeg' });
      const retainedImages: ProductStudioImage[] = [
        {
          uiKey: 'local-1',
          isMain: true,
          sortOrder: 0,
          colorId: 'col-black',
          source: {
            kind: 'local',
            clientMediaId: 'client-m-1',
            file: fakeFile,
            previewUrl: 'blob:test-preview',
          },
        },
      ];

      const canonicalHydratedDraft: ProductStudioDraft = {
        id: 'prod-123',
        title: 'Title',
        description: 'Desc',
        variants: [{ id: 'v1', colorId: 'col-black' }],
        colors: [{ id: 'col-black', nameRu: 'Черный' }],
        images: [],
      };

      const merged = mergeRetainedLocalMedia(retainedImages, canonicalHydratedDraft);
      expect(merged).toHaveLength(1);
      expect(merged[0].uiKey).toBe('local-1');
      expect(merged[0].colorId).toBe('col-black');
      expect(merged[0].source.kind).toBe('local');
      if (merged[0].source.kind === 'local') {
        expect(merged[0].source.file).toBe(fakeFile);
        expect(merged[0].source.previewUrl).toBe('blob:test-preview');
      }
    });

    it('13. invalid media color binding clears without label guessing', () => {
      const fakeFile = new File(['fake content'], 'photo.jpg', { type: 'image/jpeg' });
      const retainedImages: ProductStudioImage[] = [
        {
          uiKey: 'local-invalid-color',
          isMain: false,
          colorId: 'col-non-existent', // No such color in canonical product
          source: {
            kind: 'local',
            clientMediaId: 'client-m-2',
            file: fakeFile,
            previewUrl: 'blob:test-preview-2',
          },
        },
      ];

      const canonicalHydratedDraft: ProductStudioDraft = {
        id: 'prod-123',
        title: 'Title',
        description: 'Desc',
        variants: [{ id: 'v1', colorId: 'col-black' }],
        colors: [{ id: 'col-black', nameRu: 'Черный' }],
        images: [],
      };

      const merged = mergeRetainedLocalMedia(retainedImages, canonicalHydratedDraft);
      expect(merged).toHaveLength(1);
      expect(merged[0].colorId).toBeUndefined(); // Cleared explicitly, no label guessing!
    });
  });

  /* ========================================================================
   * C. Happy Path
   * ======================================================================== */
  describe('C. Happy path', () => {
    it('14. no local media: POST=1, GET=1, PATCH=0, navigate replace to /products/{id}/edit', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockUpdate = vi.fn();
      const mockNavigate = vi.fn();

      renderTestCreateStudio({
        initialDraft: { ...sampleInitialCreateDraft, images: [] },
        createProductFn: mockCreate,
        getProductFn: mockGet,
        saveProductFn: mockUpdate,
        onNavigate: mockNavigate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/products/prod-canonical-created-123/edit',
          { replace: true }
        );
      });

      expect(mockCreate).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockUpdate).toHaveBeenCalledTimes(0); // Zero PATCH!
      expect(loadProductStudioCreateSession()).toBeNull(); // Session cleared on success
    });

    it('15. local media: POST=1, GET=1, stage only local media, PATCH=1, final GET=1, navigate to Edit', async () => {
      const fakeFile = new File(['fake content'], 'photo.jpg', { type: 'image/jpeg' });
      const draftWithMedia: ProductStudioDraft = {
        ...sampleInitialCreateDraft,
        images: [
          {
            uiKey: 'local-1',
            isMain: true,
            sortOrder: 0,
            colorId: 'col-black',
            source: {
              kind: 'local',
              clientMediaId: 'media-1',
              file: fakeFile,
              previewUrl: 'blob:test',
            },
          },
        ],
      };

      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockStage = vi.fn().mockResolvedValue({
        id: 'staged-1',
        imageUrl: 'https://cdn.example.com/staged-1.jpg',
      });
      const mockUpdate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockNavigate = vi.fn();

      renderTestCreateStudio({
        initialDraft: draftWithMedia,
        createProductFn: mockCreate,
        getProductFn: mockGet,
        stageImageFn: mockStage,
        saveProductFn: mockUpdate,
        onNavigate: mockNavigate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/products/prod-canonical-created-123/edit',
          { replace: true }
        );
      });

      expect(mockCreate).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledTimes(2); // 1 after Create, 1 after PATCH
      expect(mockStage).toHaveBeenCalledTimes(1);
      expect(mockUpdate).toHaveBeenCalledTimes(1);
      expect(loadProductStudioCreateSession()).toBeNull();
    });
  });

  /* ========================================================================
   * D. Failure / Retry
   * ======================================================================== */
  describe('D. Failure/retry', () => {
    it('16 & 17. partial stage failure retains successful staged items + failed File, retry performs NO second POST', async () => {
      const file1 = new File(['content 1'], 'photo1.jpg', { type: 'image/jpeg' });
      const file2 = new File(['content 2'], 'photo2.jpg', { type: 'image/jpeg' });

      const draftWith2Media: ProductStudioDraft = {
        ...sampleInitialCreateDraft,
        images: [
          {
            uiKey: 'local-1',
            isMain: true,
            source: { kind: 'local', clientMediaId: 'm1', file: file1, previewUrl: 'blob:1' },
          },
          {
            uiKey: 'local-2',
            isMain: false,
            source: { kind: 'local', clientMediaId: 'm2', file: file2, previewUrl: 'blob:2' },
          },
        ],
      };

      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockUpdate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);

      // First call: photo 1 succeeds, photo 2 fails
      let attempt = 1;
      const mockStage = vi.fn().mockImplementation(async (_pid: string, mediaId: string) => {
        if (attempt === 1 && mediaId === 'm2') {
          throw new Error('Network error uploading m2');
        }
        return { id: `staged-${mediaId}`, imageUrl: `https://cdn.example.com/${mediaId}.jpg` };
      });

      renderTestCreateStudio({
        initialDraft: draftWith2Media,
        createProductFn: mockCreate,
        getProductFn: mockGet,
        stageImageFn: mockStage,
        saveProductFn: mockUpdate,
      });

      // 1st attempt
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('studio-save-error-toast')).toBeTruthy();
      });

      expect(mockCreate).toHaveBeenCalledTimes(1);
      expect(mockUpdate).not.toHaveBeenCalled(); // No PATCH when staging failed!

      // Verify session kept productId
      const session = loadProductStudioCreateSession();
      expect(session?.productId).toBe('prod-canonical-created-123');

      // 2nd attempt (Retry)
      attempt = 2;
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockUpdate).toHaveBeenCalledTimes(1);
      });

      // ZERO second POST!
      expect(mockCreate).toHaveBeenCalledTimes(1);
    });

    it('18 & 19. PATCH failure preserves staged media in context, retry executes PATCH without restaging (stage=1, PATCH=2)', async () => {
      const file1 = new File(['content 1'], 'photo1.jpg', { type: 'image/jpeg' });
      const draftWithMedia: ProductStudioDraft = {
        ...sampleInitialCreateDraft,
        images: [
          {
            uiKey: 'local-1',
            isMain: true,
            sortOrder: 0,
            colorId: 'col-black',
            source: { kind: 'local', clientMediaId: 'm1', file: file1, previewUrl: 'blob:1' },
          },
        ],
      };

      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockStage = vi.fn().mockResolvedValue({
        id: 'staged-media-id-123',
        stagedMediaId: 'staged-media-id-123',
        objectKey: 'obj-key-123.jpg',
        imageUrl: 'https://cdn.example.com/1.jpg',
      });
      const mockNavigate = vi.fn();

      let patchAttempt = 1;
      const mockUpdate = vi.fn().mockImplementation(async () => {
        if (patchAttempt === 1) {
          throw new Error('PATCH 500 server error');
        }
        return {
          ...sampleCanonicalCreatedProduct,
          images: [
            { id: 'canon-img-1', url: 'https://cdn.example.com/1.jpg', isMain: true, sortOrder: 0 },
          ],
        };
      });

      renderTestCreateStudio({
        initialDraft: draftWithMedia,
        createProductFn: mockCreate,
        getProductFn: mockGet,
        stageImageFn: mockStage,
        saveProductFn: mockUpdate,
        onNavigate: mockNavigate,
      });

      // Initially: context has local image
      expect(screen.getByTestId('test-images-summary').textContent).toBe('local-1:local');

      // Attempt 1 -> click Save
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('studio-save-error-toast')).toBeTruthy();
      });

      // Verify Attempt 1 counts
      expect(mockCreate).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledTimes(1); // Canonical GET after POST
      expect(mockStage).toHaveBeenCalledTimes(1);
      expect(mockUpdate).toHaveBeenCalledTimes(1);
      expect(mockNavigate).not.toHaveBeenCalled(); // NO navigate before successful PATCH

      // After failure: context/draft contains STAGED media, not local media!
      expect(screen.getByTestId('test-images-summary').textContent).toBe('local-1:staged');

      // Verify session kept established productId
      const session = loadProductStudioCreateSession();
      expect(session?.productId).toBe('prod-canonical-created-123');

      // Attempt 2: Retry Save
      patchAttempt = 2;
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/products/prod-canonical-created-123/edit',
          { replace: true }
        );
      });

      // Verify Attempt 2 counts:
      // Create POST remains = 1
      expect(mockCreate).toHaveBeenCalledTimes(1);
      // Stage remains = 1 total (NO restaging!)
      expect(mockStage).toHaveBeenCalledTimes(1);
      // PATCH becomes = 2
      expect(mockUpdate).toHaveBeenCalledTimes(2);

      // Verify PATCH 2 payload: same productId, same stagedMediaId, no restaging
      const patchPayload = mockUpdate.mock.calls[1][1];
      expect(mockUpdate.mock.calls[1][0]).toBe('prod-canonical-created-123');
      expect(patchPayload.images).toBeDefined();
      expect(patchPayload.images[0].id).toBe('staged-media-id-123');

      // Successful completion clears session
      expect(loadProductStudioCreateSession()).toBeNull();
    });

    it('20. first canonical GET failure performs NO staging/PATCH/Create retry', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockRejectedValue(new Error('Network error on GET'));
      const mockStage = vi.fn();
      const mockUpdate = vi.fn();

      renderTestCreateStudio({
        createProductFn: mockCreate,
        getProductFn: mockGet,
        stageImageFn: mockStage,
        saveProductFn: mockUpdate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('studio-save-error-toast')).toBeTruthy();
      });

      expect(mockCreate).toHaveBeenCalledTimes(1);
      expect(mockStage).not.toHaveBeenCalled();
      expect(mockUpdate).not.toHaveBeenCalled();
      expect(loadProductStudioCreateSession()?.productId).toBe('prod-canonical-created-123');
    });
  });

  /* ========================================================================
   * E. Durable Recovery & Lifecycle Guard (PS.R4B3.1C4C3B2B)
   * ======================================================================== */
  describe('E. Durable recovery & Lifecycle Guard', () => {
    it('21. valid identity_established still canonical-GET recovers normally', async () => {
      saveProductStudioCreateSession({
        version: 1,
        clientCreateId: '11111111-1111-4111-a111-111111111111',
        createRequestSnapshot: { title: 'Recovered', priceCents: 100, currency: 'RUB' },
        productId: 'prod-authoritative-999',
        phase: 'identity_established',
        createdAt: Date.now(),
      });

      const mockGetProduct = vi.fn().mockResolvedValue({
        id: 'prod-authoritative-999',
        title: 'Recovered',
      });
      const mockCreateProduct = vi.fn();

      vi.spyOn(await import('@zamk/api-client/src/seller'), 'getSellerProduct').mockImplementation(mockGetProduct);
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreateProduct);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <Routes>
            <Route path="/products/new" element={<SellerProductStudioNew />} />
            <Route path="/products/:id/edit" element={<div data-testid="edit-page-routed" />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('edit-page-routed')).toBeTruthy();
      });

      expect(mockGetProduct).toHaveBeenCalledWith('prod-authoritative-999');
      expect(mockCreateProduct).not.toHaveBeenCalled();
      expect(loadProductStudioCreateSession()).toBeNull();
    });

    it('22. valid identity_pending still replays: SAME snapshot, SAME Idempotency-Key', async () => {
      const storedSnapshot = { title: 'Pending Crash Product', priceCents: 150000, currency: 'RUB' };
      saveProductStudioCreateSession({
        version: 1,
        clientCreateId: '22222222-2222-4222-a222-222222222222',
        createRequestSnapshot: storedSnapshot,
        phase: 'identity_pending',
        createdAt: Date.now(),
      });

      const mockCreateProduct = vi.fn().mockResolvedValue({ id: 'prod-recovered-888' });
      const mockGetProduct = vi.fn().mockResolvedValue({ id: 'prod-recovered-888', title: 'Pending Crash Product' });

      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreateProduct);
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'getSellerProduct').mockImplementation(mockGetProduct);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <Routes>
            <Route path="/products/new" element={<SellerProductStudioNew />} />
            <Route path="/products/:id/edit" element={<div data-testid="edit-page-routed" />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('edit-page-routed')).toBeTruthy();
      });

      expect(mockCreateProduct).toHaveBeenCalledWith(storedSnapshot, {
        idempotencyKey: '22222222-2222-4222-a222-222222222222',
      });
      expect(loadProductStudioCreateSession()).toBeNull();
    });

    it('23 & 24. response-loss retry uses immutable stored snapshot, does not generate new UUID', async () => {
      const mockCreate = vi.fn().mockRejectedValueOnce(new Error('Network transport loss'))
        .mockResolvedValueOnce(sampleCanonicalCreatedProduct);

      renderTestCreateStudio({ createProductFn: mockCreate });

      // 1st attempt: fails network transport
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('studio-save-error-toast')).toBeTruthy();
      });

      const session1 = loadProductStudioCreateSession();
      expect(session1?.phase).toBe('identity_pending');
      const originalKey = session1?.clientCreateId;

      // 2nd attempt: retry
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledTimes(2);
      });

      // Second call must have the EXACT SAME key and payload
      expect(mockCreate.mock.calls[1][1].idempotencyKey).toBe(originalKey);
      expect(mockCreate.mock.calls[1][0]).toEqual(session1?.createRequestSnapshot);
    });

    it('25. successful final Save writes completed before attempting clear', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockNavigate = vi.fn();

      const savedPhases: string[] = [];
      const originalSetItem = sessionStorage.setItem.bind(sessionStorage);
      const setItemSpy = vi.spyOn(Object.getPrototypeOf(sessionStorage), 'setItem').mockImplementation((...args: any[]) => {
        const [key, val] = args;
        if (key === PRODUCT_STUDIO_CREATE_SESSION_KEY && val) {
          try {
            const parsed = JSON.parse(val);
            if (parsed.phase) savedPhases.push(parsed.phase);
          } catch {}
        }
        originalSetItem(key, val);
      });

      renderTestCreateStudio({
        createProductFn: mockCreate,
        getProductFn: mockGet,
        onNavigate: mockNavigate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalled();
      });

      // Must have written identity_pending, identity_established, AND completed
      expect(savedPhases).toContain('identity_pending');
      expect(savedPhases).toContain('identity_established');
      expect(savedPhases).toContain('completed');
      expect(savedPhases[savedPhases.length - 1]).toBe('completed');

      setItemSpy.mockRestore();
    });

    it('25a. removeItem failure after completed marker: no second Create, stale record is completed', async () => {
      const mockCreate = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalCreatedProduct);
      const mockNavigate = vi.fn();

      // Mock removeItem and setItem('') to throw
      const removeItemSpy = vi.spyOn(Object.getPrototypeOf(sessionStorage), 'removeItem').mockImplementation(() => {
        throw new Error('StorageAccessDenied');
      });

      renderTestCreateStudio({
        createProductFn: mockCreate,
        getProductFn: mockGet,
        onNavigate: mockNavigate,
      });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalled();
      });

      // Stale record remains in storage, but its phase MUST be completed!
      const rawSession = sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY);
      expect(rawSession).toBeTruthy();
      const parsed = JSON.parse(rawSession!);
      expect(parsed.phase).toBe('completed');
      expect(parsed.productId).toBe('prod-canonical-created-123');

      // Crucial: mockCreate called exactly 1 time
      expect(mockCreate).toHaveBeenCalledTimes(1);

      removeItemSpy.mockRestore();
    });

    it('25b. mount /products/new with completed stale session: NO POST, attempts cleanup, fresh Create if cleared', async () => {
      saveProductStudioCreateSession({
        version: 1,
        clientCreateId: '33333333-3333-4333-a333-333333333333',
        createRequestSnapshot: { title: 'Completed Item', priceCents: 100, currency: 'RUB' },
        productId: 'prod-already-completed-1',
        phase: 'completed',
        createdAt: Date.now(),
      });

      const mockCreate = vi.fn();
      const mockGet = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'getSellerProduct').mockImplementation(mockGet);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      // Must NOT call create or get on mount
      expect(mockCreate).not.toHaveBeenCalled();
      expect(mockGet).not.toHaveBeenCalled();

      // Session must be cleaned up
      expect(sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY)).toBeFalsy();
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });

    it('25c. completed session + Add Product: safely clears and opens new Create without old recovery', () => {
      saveProductStudioCreateSession({
        version: 1,
        clientCreateId: '44444444-4444-4444-a444-444444444444',
        createRequestSnapshot: { title: 'Old Done', priceCents: 100, currency: 'RUB' },
        productId: 'prod-old-done',
        phase: 'completed',
        createdAt: Date.now(),
      });

      const clearResult = prepareAddProductNavigation();
      expect(clearResult).toBe(true);

      // Session is cleared
      expect(loadProductStudioCreateSession()).toBeNull();
      expect(sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY)).toBeFalsy();
    });

    it('25d. prepareAddProductNavigation preserves active unresolved session', () => {
      saveProductStudioCreateSession({
        version: 1,
        clientCreateId: '55555555-5555-4555-a555-555555555555',
        createRequestSnapshot: { title: 'Active', priceCents: 100, currency: 'RUB' },
        phase: 'identity_pending',
        createdAt: Date.now(),
      });

      const res = prepareAddProductNavigation();
      expect(res).toBe(true);

      expect(loadProductStudioCreateSession()?.clientCreateId).toBe('55555555-5555-4555-a555-555555555555');
    });

    it('25e. malformed JSON: NO POST and shows safe recoverable session error', async () => {
      sessionStorage.setItem(PRODUCT_STUDIO_CREATE_SESSION_KEY, '{malformed-invalid-json');

      const mockCreate = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      // Zero POST calls
      expect(mockCreate).not.toHaveBeenCalled();
      expect(screen.getByTestId('studio-new-recovery-error')).toBeTruthy();

      // Click "Очистить и создать новый"
      fireEvent.click(screen.getByTestId('studio-new-clear-malformed-session'));

      await waitFor(() => {
        expect(screen.getByTestId('product-studio-root')).toBeTruthy();
      });
      expect(sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY)).toBeFalsy();
    });

    it('25f. unknown phase: NO POST and rejected by loadProductStudioCreateSession', async () => {
      sessionStorage.setItem(
        PRODUCT_STUDIO_CREATE_SESSION_KEY,
        JSON.stringify({
          version: 1,
          clientCreateId: '66666666-6666-4666-a666-666666666666',
          createRequestSnapshot: { title: 'Unknown', priceCents: 100, currency: 'RUB' },
          phase: 'bogus_phase',
          createdAt: Date.now(),
        })
      );

      expect(loadProductStudioCreateSession()).toBeNull();

      const mockCreate = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      expect(mockCreate).not.toHaveBeenCalled();
    });

    it('25g. invalid clientCreateId (non-UUID): NO POST and rejected by loader', async () => {
      sessionStorage.setItem(
        PRODUCT_STUDIO_CREATE_SESSION_KEY,
        JSON.stringify({
          version: 1,
          clientCreateId: 'invalid-not-a-uuid',
          createRequestSnapshot: { title: 'Invalid UUID', priceCents: 100, currency: 'RUB' },
          phase: 'identity_pending',
          createdAt: Date.now(),
        })
      );

      expect(loadProductStudioCreateSession()).toBeNull();

      const mockCreate = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      expect(mockCreate).not.toHaveBeenCalled();
    });

    it('25h. identity_pending without valid snapshot: NO POST and rejected by loader', async () => {
      sessionStorage.setItem(
        PRODUCT_STUDIO_CREATE_SESSION_KEY,
        JSON.stringify({
          version: 1,
          clientCreateId: '77777777-7777-4777-a777-777777777777',
          createRequestSnapshot: null,
          phase: 'identity_pending',
          createdAt: Date.now(),
        })
      );

      expect(loadProductStudioCreateSession()).toBeNull();

      const mockCreate = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      expect(mockCreate).not.toHaveBeenCalled();
    });

    it('25i. identity_established without productId: NO POST and rejected by loader', async () => {
      sessionStorage.setItem(
        PRODUCT_STUDIO_CREATE_SESSION_KEY,
        JSON.stringify({
          version: 1,
          clientCreateId: '88888888-8888-4888-a888-888888888888',
          createRequestSnapshot: { title: 'Valid', priceCents: 100, currency: 'RUB' },
          productId: '',
          phase: 'identity_established',
          createdAt: Date.now(),
        })
      );

      expect(loadProductStudioCreateSession()).toBeNull();

      const mockCreate = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      expect(mockCreate).not.toHaveBeenCalled();
    });

    it('25j. unsupported version: NO POST and rejected by loader', async () => {
      sessionStorage.setItem(
        PRODUCT_STUDIO_CREATE_SESSION_KEY,
        JSON.stringify({
          version: 99,
          clientCreateId: '99999999-9999-4999-a999-999999999999',
          createRequestSnapshot: { title: 'V99', priceCents: 100, currency: 'RUB' },
          phase: 'identity_pending',
          createdAt: Date.now(),
        })
      );

      expect(loadProductStudioCreateSession()).toBeNull();

      const mockCreate = vi.fn();
      vi.spyOn(await import('@zamk/api-client/src/seller'), 'createSellerProduct').mockImplementation(mockCreate);

      render(
        <MemoryRouter initialEntries={['/products/new']}>
          <SellerProductStudioNew />
        </MemoryRouter>
      );

      expect(mockCreate).not.toHaveBeenCalled();
    });
  });

  /* ========================================================================
   * F. Save Guard
   * ======================================================================== */
  describe('F. Save guard', () => {
    it('26. synchronous double Save => one logical POST', async () => {
      let resolveCreate: (val: any) => void;
      const delayedCreate = new Promise((resolve) => {
        resolveCreate = resolve;
      });
      const mockCreate = vi.fn().mockReturnValue(delayedCreate);

      renderTestCreateStudio({ createProductFn: mockCreate });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      // Rapid double click
      fireEvent.click(saveBtn);
      fireEvent.click(saveBtn);

      expect(mockCreate).toHaveBeenCalledTimes(1);

      act(() => {
        resolveCreate(sampleCanonicalCreatedProduct);
      });
    });

    it('27. Create workspace cannot mutate payload during uncertain identity state', async () => {
      let resolveCreate: (val: any) => void;
      const delayedCreate = new Promise((resolve) => {
        resolveCreate = resolve;
      });
      const mockCreate = vi.fn().mockReturnValue(delayedCreate);

      renderTestCreateStudio({ createProductFn: mockCreate });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      // While create is in flight, workspace has aria-disabled and pointer-events-none
      const workspace = screen.getByTestId('studio-workspace-container');
      expect(workspace.getAttribute('aria-disabled')).toBe('true');
      expect(workspace.className).toContain('pointer-events-none');
      expect(screen.getByTestId('studio-header-category-btn').hasAttribute('disabled')).toBe(true);

      act(() => {
        resolveCreate(sampleCanonicalCreatedProduct);
      });
    });

    it('28. identity_recovery_required freezes draft payload from mutation', async () => {
      const mockCreate = vi.fn().mockRejectedValue(new Error('Network transport error'));
      renderTestCreateStudio({ createProductFn: mockCreate });

      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('identity_recovery_required');
      });

      // Verify initial title
      expect(screen.getByTestId('test-draft-title').textContent).toBe('Худи Новое Оверсайз');

      // Attempt to mutate title
      fireEvent.click(screen.getByTestId('test-mutate-title'));

      // Must remain frozen!
      expect(screen.getByTestId('test-draft-title').textContent).toBe('Худи Новое Оверсайз');

      // Save button shows retry action
      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.textContent).toContain('Повторить попытку');
    });
  });
});
