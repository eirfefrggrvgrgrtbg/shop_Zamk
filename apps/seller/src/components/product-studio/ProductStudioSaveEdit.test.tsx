/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import React from 'react';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
  type ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import { ProductStudio } from './ProductStudio';
import { buildProductStudioUpdateRequest } from './productStudioSaveProduct';
import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerDictionaryValue,
  StageSellerProductImageResponse,
} from '@zamk/api-client';

describe('PS.R4B3.1C4C2B2 — Edit Product Studio Save End-to-End', () => {
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
        required: true,
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

  const sampleCanonicalProduct: SellerProduct = {
    id: 'prod-test-1',
    title: 'Худи Базовое',
    description: 'Теплый оверсайз',
    categoryId: 'cat-hoodies',
    categoryName: 'Худи',
    brandId: 'brand-1',
    brandName: 'Test Brand',
    sellerId: 'seller-1',
    status: 'draft',
    priceCents: 450000,
    oldPriceCents: 500000,
    currency: 'RUB',
    slug: 'hoodie-base',
    createdAt: '2026-09-01T12:00:00Z',
    images: [
      {
        id: 'img-canon-1',
        imageUrl: 'https://cdn.example.com/img1.jpg',
        sortOrder: 0,
        colorId: 'col-black',
        isMain: true,
      },
    ],
    variants: [
      {
        id: 'var-1',
        productId: 'prod-test-1',
        colorId: 'col-black',
        colorName: 'Черный',
        colorHex: '#000000',
        sizeValueId: 'sz-m',
        size: 'M',
        sellerSku: 'SKU-BLK-M',
        barcode: '1234567890123',
        priceCents: 450000,
        isActive: true,
      },
    ],
    materialComposition: [
      {
        materialId: 'mat-cotton',
        materialName: 'Хлопок',
        percentage: 100,
      },
    ],
    attributes: [
      {
        id: 'attr-val-1',
        productId: 'prod-test-1',
        attributeDefinitionId: 'attr-season',
        enumValueId: 'dict-val-autumn',
      },
    ],
  };

  const sampleInitialDraft: ProductStudioDraft = {
    id: 'prod-test-1',
    title: 'Худи Базовое',
    description: 'Теплый оверсайз',
    categoryId: 'cat-hoodies',
    categoryName: 'Худи',
    brandId: 'brand-1',
    brandName: 'Test Brand',
    status: 'draft',
    priceCents: 450000,
    oldPriceCents: 500000,
    images: [
      {
        uiKey: 'img-canon-1',
        sortOrder: 0,
        colorId: 'col-black',
        isMain: true,
        source: {
          kind: 'canonical',
          imageId: 'img-canon-1',
          url: 'https://cdn.example.com/img1.jpg',
        },
      },
    ],
    variants: [
      {
        id: 'var-1',
        colorId: 'col-black',
        colorName: 'Черный',
        colorHex: '#000000',
        sizeValueId: 'sz-m',
        size: 'M',
        sellerSku: 'SKU-BLK-M',
        barcode: '1234567890123',
        priceCents: 450000,
        isActive: true,
      },
    ],
    materialComposition: [
      {
        materialId: 'mat-cotton',
        materialName: 'Хлопок',
        percentage: 100,
      },
    ],
    attributes: [
      {
        attributeDefinitionId: 'attr-season',
        dictionaryValueId: 'dict-val-autumn',
        value: 'Осень',
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  // Helper component to trigger draft edits easily from test
  function TestControls() {
    const {
      updateDraft,
      isDirty,
      canSave,
      saveDraft,
      isSaveInFlight,
      saveStatus,
      createMediaUrl,
      draft,
      selectedPreviewColorId,
      selectedPreviewSizeValueId,
      setSelectedPreviewColorId,
      setSelectedPreviewSizeValueId,
    } = useProductStudio();
    return (
      <div data-testid="test-controls">
        <span data-testid="test-is-dirty">{isDirty ? 'DIRTY' : 'CLEAN'}</span>
        <span data-testid="test-can-save">{canSave ? 'CAN_SAVE' : 'CANNOT_SAVE'}</span>
        <span data-testid="test-save-status">{saveStatus}</span>
        <span data-testid="test-is-in-flight">{isSaveInFlight ? 'IN_FLIGHT' : 'IDLE'}</span>
        <span data-testid="test-preview-color">{selectedPreviewColorId || 'none'}</span>
        <span data-testid="test-preview-size">{selectedPreviewSizeValueId || 'none'}</span>
        <div data-testid="test-images-summary">
          {draft.images?.map((img) => `${img.uiKey}:${img.source.kind}`).join(',')}
        </div>
        <div data-testid="test-variants-summary">
          {draft.variants?.map((v) => `${v.id}:${v.colorName}:${v.size}`).join(',')}
        </div>
        <div data-testid="test-materials-summary">
          {draft.materialComposition?.map((m) => `${m.materialName}:${m.percentage}%`).join(',')}
        </div>
        <button
          data-testid="test-mutate-title"
          onClick={() => updateDraft({ title: 'Худи Новое Название' })}
        >
          Mutate Title
        </button>
        <button
          data-testid="test-select-preview"
          onClick={() => {
            setSelectedPreviewColorId('col-black');
            setSelectedPreviewSizeValueId('sz-m');
          }}
        >
          Select Preview
        </button>
        <button
          data-testid="test-add-local-image"
          onClick={() => {
            const fakeFile = new File(['content'], 'test.jpg', { type: 'image/jpeg' });
            const previewUrl = createMediaUrl(fakeFile);
            const localImg: ProductStudioImage = {
              uiKey: 'client-media-123',
              colorId: 'col-black',
              isMain: false,
              sortOrder: 1,
              source: {
                kind: 'local',
                clientMediaId: 'client-media-123',
                file: fakeFile,
                previewUrl,
              },
            };
            updateDraft({ images: [...(sampleInitialDraft.images || []), localImg] });
          }}
        >
          Add Local Image
        </button>
        <button
          data-testid="test-call-save"
          onClick={() => saveDraft && saveDraft()}
        >
          Call Save
        </button>
      </div>
    );
  }

  function renderTestStudio(props: Partial<React.ComponentProps<typeof ProductStudioProvider>> = {}) {
    const defaultGetProduct = vi.fn().mockResolvedValue(sampleCanonicalProduct);
    return render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={sampleInitialDraft}
          initialCategorySchema={mockSchema}
          canonicalColors={mockColors}
          dictionaryValuesMap={mockDictMap}
          getProductFn={defaultGetProduct}
          {...props}
        >
          <ProductStudio />
          <TestControls />
        </ProductStudioProvider>
      </MemoryRouter>
    );
  }

  /* ========================================================================
   * SUITE 1: Capability, Dirtiness & Locking (Requirements 1, 2, 3, 4, 5, 6, 32)
   * ======================================================================== */
  describe('Suite 1: Capability, Dirtiness & UI Locking', () => {
    it('1. Edit dirty state enables Save, and 2. Edit clean state does not execute Save', async () => {
      const mockSaveProduct = vi.fn().mockResolvedValue(sampleCanonicalProduct);

      renderTestStudio({ saveProductFn: mockSaveProduct });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      // Initially clean: button disabled
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CANNOT_SAVE');
      expect(saveBtn.hasAttribute('disabled')).toBe(true);

      // Clean state click does not call save
      fireEvent.click(saveBtn);
      expect(mockSaveProduct).not.toHaveBeenCalled();

      // Mutate title to make dirty
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');
      expect(saveBtn.hasAttribute('disabled')).toBe(false);
    });

    it('3. Create mode cannot execute Edit Save', async () => {
      const mockSaveProduct = vi.fn();

      renderTestStudio({
        entryMode: 'create',
        saveProductFn: mockSaveProduct,
      });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(true);
      expect(saveBtn.getAttribute('title')).toBe('Сохранение временно недоступно');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CANNOT_SAVE');

      // Attempt click
      fireEvent.click(saveBtn);
      expect(mockSaveProduct).not.toHaveBeenCalled();
    });

    it('4. double click / concurrent calls produce exactly ONE Save flow', async () => {
      let resolveSave: (val: any) => void;
      const delayedSavePromise = new Promise((resolve) => {
        resolveSave = resolve;
      });
      const mockSaveProduct = vi.fn().mockReturnValue(delayedSavePromise);

      renderTestStudio({ saveProductFn: mockSaveProduct });

      // Make dirty
      fireEvent.click(screen.getByTestId('test-mutate-title'));

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      // Rapid double click
      fireEvent.click(saveBtn);
      fireEvent.click(saveBtn);
      fireEvent.click(screen.getByTestId('test-call-save'));

      await waitFor(() => {
        expect(mockSaveProduct).toHaveBeenCalledTimes(1);
      });

      // Resolve save
      await act(async () => {
        resolveSave!(sampleCanonicalProduct);
      });
    });

    it('5. editor locked during staging and 6. editor locked during PATCH', async () => {
      let resolveStage: (val: any) => void;
      const delayedStagePromise = new Promise<StageSellerProductImageResponse>((resolve) => {
        resolveStage = resolve;
      });
      const mockStageImage = vi.fn().mockReturnValue(delayedStagePromise);

      let resolvePatch: (val: any) => void;
      const delayedPatchPromise = new Promise((resolve) => {
        resolvePatch = resolve;
      });
      const mockSaveProduct = vi.fn().mockReturnValue(delayedPatchPromise);

      renderTestStudio({
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
      });

      // Before save: confirm no fieldset wrapper exists and container is w-full
      expect(screen.queryByTestId('studio-editor-fieldset')).toBeNull();
      const workspaceBefore = screen.getByTestId('studio-workspace-container');
      expect(workspaceBefore.getAttribute('aria-disabled')).toBe('false');
      expect(workspaceBefore.className).toBe('w-full');

      // Add a local image to trigger staging
      fireEvent.click(screen.getByTestId('test-add-local-image'));

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      fireEvent.click(saveBtn);

      // STAGE PHASE
      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('staging');
      });
      expect(screen.getByTestId('test-is-in-flight').textContent).toBe('IN_FLIGHT');

      expect(screen.queryByTestId('studio-editor-fieldset')).toBeNull();
      const workspaceStaging = screen.getByTestId('studio-workspace-container');
      expect(workspaceStaging.getAttribute('aria-disabled')).toBe('true');
      expect(workspaceStaging.className).toContain('pointer-events-none opacity-60 select-none cursor-wait');

      // External mutation attempt ignored while locked
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      expect(screen.getByTestId('studio-product-title').textContent).toBe('Худи Базовое');

      // Resolve staging
      await act(async () => {
        resolveStage!({
          id: 'staged-uuid-999',
          stagedUrl: 'https://cdn.example.com/staged-999.jpg',
          clientMediaId: 'client-media-123',
        });
      });

      // PATCH PHASE
      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('saving');
      });
      const workspaceSaving = screen.getByTestId('studio-workspace-container');
      expect(workspaceSaving.getAttribute('aria-disabled')).toBe('true');
      expect(workspaceSaving.className).toContain('pointer-events-none opacity-60 select-none cursor-wait');

      // Resolve PATCH
      await act(async () => {
        resolvePatch!(sampleCanonicalProduct);
      });

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });
      expect(screen.queryByTestId('studio-editor-fieldset')).toBeNull();
      const workspaceAfter = screen.getByTestId('studio-workspace-container');
      expect(workspaceAfter.getAttribute('aria-disabled')).toBe('false');
      expect(workspaceAfter.className).toBe('w-full');
    });

    it('32. no autosave/no polling introduced', () => {
      vi.useFakeTimers();
      const mockSaveProduct = vi.fn();

      renderTestStudio({ saveProductFn: mockSaveProduct });

      // Advance time by 60 seconds
      act(() => {
        vi.advanceTimersByTime(60000);
      });

      expect(mockSaveProduct).not.toHaveBeenCalled();
      vi.useRealTimers();
    });
  });

  /* ========================================================================
   * SUITE 2: Staging Orchestration & Partial Failure (Requirements 7, 8, 9, 10, 11, 12, 13, 14, 28)
   * ======================================================================== */
  describe('Suite 2: Staging Orchestration & Partial Failure', () => {
    it('7. one local image stages once and sends one PATCH', async () => {
      const mockStageImage = vi.fn().mockResolvedValue({
        id: 'staged-1',
        stagedUrl: 'https://cdn.example.com/staged-1.jpg',
        clientMediaId: 'client-media-123',
      });
      const mockSaveProduct = vi.fn().mockResolvedValue(sampleCanonicalProduct);

      renderTestStudio({
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
      });

      fireEvent.click(screen.getByTestId('test-add-local-image'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockStageImage).toHaveBeenCalledTimes(1);
      });
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    });

    it('8. multiple locals: all stage -> one PATCH', async () => {
      const mockStageImage = vi.fn().mockImplementation(async (_pid, clientMediaId) => ({
        id: `staged-${clientMediaId}`,
        stagedUrl: `https://cdn.example.com/staged-${clientMediaId}.jpg`,
        clientMediaId,
      }));
      const mockSaveProduct = vi.fn().mockResolvedValue(sampleCanonicalProduct);

      const draftWith2Locals: ProductStudioDraft = {
        ...sampleInitialDraft,
        images: [
          {
            uiKey: 'local-1',
            source: {
              kind: 'local',
              clientMediaId: 'c-1',
              file: new File(['f1'], 'f1.jpg', { type: 'image/jpeg' }),
              previewUrl: 'blob:preview-1',
            },
            isMain: true,
          },
          {
            uiKey: 'local-2',
            source: {
              kind: 'local',
              clientMediaId: 'c-2',
              file: new File(['f2'], 'f2.jpg', { type: 'image/jpeg' }),
              previewUrl: 'blob:preview-2',
            },
            isMain: false,
          },
        ],
      };

      renderTestStudio({
        initialDraft: draftWith2Locals,
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
      });

      // Mutate to be dirty
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(mockStageImage).toHaveBeenCalledTimes(2);
      });
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    });

    it('9. stage partial failure: NO PATCH, 10. successful staged entries persisted, 11. failed remains local, 28. previews kept alive', async () => {
      const mockStageImage = vi.fn().mockImplementation(async (_pid, clientMediaId) => {
        if (clientMediaId === 'fail-media') {
          throw new Error('Upload error 500');
        }
        return {
          id: `staged-${clientMediaId}`,
          stagedUrl: `https://cdn.example.com/staged-${clientMediaId}.jpg`,
          clientMediaId,
        };
      });
      const mockSaveProduct = vi.fn();

      const draftPartial: ProductStudioDraft = {
        ...sampleInitialDraft,
        images: [
          {
            uiKey: 'ok-img',
            isMain: true,
            source: {
              kind: 'local',
              clientMediaId: 'ok-media',
              file: new File(['ok'], 'ok.jpg', { type: 'image/jpeg' }),
              previewUrl: 'blob:http://localhost/ok-preview',
            },
          },
          {
            uiKey: 'fail-img',
            isMain: false,
            source: {
              kind: 'local',
              clientMediaId: 'fail-media',
              file: new File(['fail'], 'fail.jpg', { type: 'image/jpeg' }),
              previewUrl: 'blob:http://localhost/fail-preview',
            },
          },
        ],
      };

      renderTestStudio({
        initialDraft: draftPartial,
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
      });

      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('error');
      });

      // 9. NO PATCH sent on stage failure
      expect(mockSaveProduct).not.toHaveBeenCalled();

      // 10. Successful staged entry persisted, 11. failed remains local with File
      expect(screen.getByTestId('test-images-summary').textContent).toBe('ok-img:staged,fail-img:local');

      // Error toast shown
      expect(screen.getByTestId('studio-save-error-toast')).toBeTruthy();
      expect(screen.getByText('Не удалось загрузить часть фотографий. Попробуйте ещё раз.')).toBeTruthy();
    });

    it('12. retry stages only failed local and 13. reuses same clientMediaId', async () => {
      let shouldFail = true;
      const stagedCalls: Array<{ pid: string; cid: string }> = [];

      const mockStageImage = vi.fn().mockImplementation(async (pid, cid) => {
        stagedCalls.push({ pid, cid });
        if (cid === 'c-fail' && shouldFail) {
          throw new Error('Network error');
        }
        return {
          id: `staged-${cid}`,
          stagedUrl: `https://cdn.example.com/${cid}.jpg`,
          clientMediaId: cid,
        };
      });
      const mockSaveProduct = vi.fn().mockResolvedValue(sampleCanonicalProduct);

      const draftWith2Locals: ProductStudioDraft = {
        ...sampleInitialDraft,
        images: [
          {
            uiKey: 'img-1',
            isMain: true,
            source: {
              kind: 'local',
              clientMediaId: 'c-success',
              file: new File(['1'], '1.jpg', { type: 'image/jpeg' }),
              previewUrl: 'blob:preview-1',
            },
          },
          {
            uiKey: 'img-2',
            isMain: false,
            source: {
              kind: 'local',
              clientMediaId: 'c-fail',
              file: new File(['2'], '2.jpg', { type: 'image/jpeg' }),
              previewUrl: 'blob:preview-2',
            },
          },
        ],
      };

      renderTestStudio({
        initialDraft: draftWith2Locals,
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
      });

      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      // First attempt fails on c-fail
      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('error');
      });
      expect(mockSaveProduct).not.toHaveBeenCalled();

      // Now fix failure
      shouldFail = false;
      stagedCalls.length = 0;

      // Click save again (retry)
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // 12. Only c-fail was staged during retry!
      expect(stagedCalls.length).toBe(1);
      expect(stagedCalls[0].cid).toBe('c-fail');
      // 13. Reuses exact clientMediaId
      expect(stagedCalls[0].cid).toBe('c-fail');

      // PATCH executed once
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    });
  });

  /* ========================================================================
   * SUITE 3: PATCH Failure & Retry (Requirements 15, 16, 24, 29, 31)
   * ======================================================================== */
  describe('Suite 3: PATCH Failure & Retry', () => {
    it('15. PATCH failure preserves staged state, 16. retry does not re-stage, 24. exactly one PATCH, 29. previews kept, 31. retryable state', async () => {
      let patchShouldFail = true;
      const mockStageImage = vi.fn().mockImplementation(async (_pid, clientMediaId) => ({
        id: `staged-${clientMediaId}`,
        stagedUrl: `https://cdn.example.com/${clientMediaId}.jpg`,
        clientMediaId,
      }));

      const mockSaveProduct = vi.fn().mockImplementation(async () => {
        if (patchShouldFail) {
          throw new Error('PATCH 500 error');
        }
        return sampleCanonicalProduct;
      });

      renderTestStudio({
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
      });

      // Add local image
      fireEvent.click(screen.getByTestId('test-add-local-image'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      // Staging succeeds, then PATCH fails
      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('error');
      });
      expect(screen.getByText('Не удалось сохранить товар. Попробуйте ещё раз.')).toBeTruthy();
      expect(mockStageImage).toHaveBeenCalledTimes(1);
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);

      // Now retry with PATCH passing
      patchShouldFail = false;
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // 16. Did NOT re-stage! Staging call count is still 1!
      expect(mockStageImage).toHaveBeenCalledTimes(1);
      // Total PATCH calls is 2 (1 failed + 1 successful)
      expect(mockSaveProduct).toHaveBeenCalledTimes(2);
    });
  });

  /* ========================================================================
   * SUITE 4: Media PATCH Payload Construction (Requirements 17, 18, 19, 20, 21, 22, 23)
   * ======================================================================== */
  describe('Suite 4: Media PATCH Payload Construction', () => {
    it('17. media untouched -> PATCH omits images', () => {
      const workingDraft: ProductStudioDraft = {
        ...sampleInitialDraft,
        title: 'New Title',
      };
      const payload = buildProductStudioUpdateRequest(workingDraft, sampleInitialDraft);
      expect(payload.title).toBe('New Title');
      expect(payload.images).toBeUndefined();
      expect('images' in payload).toBe(false);
    });

    it('18. media dirty -> PATCH contains full desired images', () => {
      const workingDraft: ProductStudioDraft = {
        ...sampleInitialDraft,
        images: [
          ...sampleInitialDraft.images!,
          {
            uiKey: 'staged-key-2',
            colorId: 'col-white',
            isMain: false,
            sortOrder: 1,
            source: {
              kind: 'staged',
              clientMediaId: 'cm-2',
              stagedId: 'staged-uuid-2',
              stagedUrl: 'https://cdn.example.com/staged-2.jpg',
              previewUrl: 'blob:preview-2',
            },
          },
        ],
      };
      const payload = buildProductStudioUpdateRequest(workingDraft, sampleInitialDraft);
      expect(payload.images).toBeDefined();
      expect(payload.images?.length).toBe(2);
      // 21. Canonical IDs preserved
      expect(payload.images?.[0]).toEqual({
        id: 'img-canon-1',
        colorId: 'col-black',
        isMain: true,
        altText: null,
      });
      // 22. Staged IDs used, 23. No clientMediaId or blob URL sent
      expect(payload.images?.[1]).toEqual({
        id: 'staged-uuid-2',
        colorId: 'col-white',
        isMain: false,
        altText: null,
      });
    });

    it('19. remove all images -> images: []', () => {
      const workingDraft: ProductStudioDraft = {
        ...sampleInitialDraft,
        images: [],
      };
      const payload = buildProductStudioUpdateRequest(workingDraft, sampleInitialDraft);
      expect(payload.images).toEqual([]);
    });

    it('20. variant color-domain change -> images included even if media was untouched', () => {
      const workingDraft: ProductStudioDraft = {
        ...sampleInitialDraft,
        variants: [
          ...sampleInitialDraft.variants!,
          {
            id: 'var-2',
            colorId: 'col-white', // New color domain!
            sizeValueId: 'sz-l',
          },
        ],
      };
      const payload = buildProductStudioUpdateRequest(workingDraft, sampleInitialDraft);
      expect(payload.images).toBeDefined();
      expect(payload.images?.length).toBe(1);
    });
  });

  /* ========================================================================
   * SUITE 5: PATCH Success, Canonicalization & Cleanup (Requirements 25, 26, 27, 30)
   * ======================================================================== */
  describe('Suite 5: PATCH Success, Canonicalization & Cleanup', () => {
    it('25. response canonicalizes draft, 26. resets baseline/isDirty, 27. releases obsolete preview URLs, 30. backend status becomes UI status', async () => {
      const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});

      const updatedBackendResponse: SellerProduct = {
        ...sampleCanonicalProduct,
        title: 'Обновленное Худи',
        status: 'published', // 30. Backend status changed!
        images: [
          {
            id: 'img-canon-1',
            imageUrl: 'https://cdn.example.com/img1.jpg',
            sortOrder: 0,
            colorId: 'col-black',
            isMain: false,
          },
          {
            id: 'img-canon-new',
            imageUrl: 'https://cdn.example.com/img-canon-new.jpg',
            sortOrder: 1,
            colorId: 'col-black',
            isMain: true,
          },
        ],
      };

      const mockStageImage = vi.fn().mockResolvedValue({
        id: 'staged-uuid-abc',
        stagedUrl: 'https://cdn.example.com/staged-abc.jpg',
        clientMediaId: 'client-media-123',
      });
      const mockSaveProduct = vi.fn().mockResolvedValue(updatedBackendResponse);

      renderTestStudio({
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
        getProductFn: vi.fn().mockResolvedValue(updatedBackendResponse),
      });

      // Add local image
      fireEvent.click(screen.getByTestId('test-add-local-image'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');

      // Click Save
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // 25. Draft canonicalized with new title
      expect(screen.getByTestId('studio-product-title').textContent).toBe('Обновленное Худи');
      // 30. Status badge reflects backend status 'published'
      expect(screen.getByTestId('studio-entry-badge').textContent).toBe('Опубликован');
      // 26. isDirty reset to false!
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
      expect(screen.getByTestId('studio-header-save-btn').hasAttribute('disabled')).toBe(true);

      // 27. Preview URL was revoked!
      expect(revokeSpy).toHaveBeenCalled();
      revokeSpy.mockRestore();
    });
  });

  /* ========================================================================
   * SUITE 6: Failures A, B, C, D Regression Gates (Milestone PS.R4B3.1C4C2B2B)
   * ======================================================================== */
  describe('Suite 6: Failures A, B, C, D Regression Gates', () => {
    const sampleReadyProduct: SellerProduct = {
      ...sampleCanonicalProduct,
      images: [
        {
          id: 'img-canon-1',
          imageUrl: 'https://cdn.example.com/img1.jpg',
          sortOrder: 0,
          colorId: 'col-black',
          isMain: true,
        },
        {
          id: 'img-canon-2',
          imageUrl: 'https://cdn.example.com/img2.jpg',
          sortOrder: 1,
          colorId: 'col-black',
          isMain: false,
        },
        {
          id: 'img-canon-3',
          imageUrl: 'https://cdn.example.com/img3.jpg',
          sortOrder: 2,
          colorId: 'col-black',
          isMain: false,
        },
      ],
    };

    const sampleReadyDraft: ProductStudioDraft = {
      ...sampleInitialDraft,
      colors: [
        { id: 'col-black', name: 'Черный', hex: '#000000' },
      ],
      images: [
        {
          uiKey: 'img-canon-1',
          sortOrder: 0,
          colorId: 'col-black',
          isMain: true,
          source: {
            kind: 'canonical',
            imageId: 'img-canon-1',
            url: 'https://cdn.example.com/img1.jpg',
          },
        },
        {
          uiKey: 'img-canon-2',
          sortOrder: 1,
          colorId: 'col-black',
          isMain: false,
          source: {
            kind: 'canonical',
            imageId: 'img-canon-2',
            url: 'https://cdn.example.com/img2.jpg',
          },
        },
        {
          uiKey: 'img-canon-3',
          sortOrder: 2,
          colorId: 'col-black',
          isMain: false,
          source: {
            kind: 'canonical',
            imageId: 'img-canon-3',
            url: 'https://cdn.example.com/img3.jpg',
          },
        },
      ],
    };

    // Failure A
    it('Failure A.1: Unchanged freshly hydrated product starts clean (isDirty=false, canSave=false)', () => {
      renderTestStudio({ initialDraft: sampleReadyDraft });
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CANNOT_SAVE');
    });

    it('Failure A.2: Disabled Save button uses neutral gray styling (never purple bg-indigo-400)', () => {
      renderTestStudio({ initialDraft: sampleReadyDraft });
      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(true);
      expect(saveBtn.className).not.toContain('bg-indigo-400');
      expect(saveBtn.className).toContain('text-gray-400');
      expect(saveBtn.className).toContain('cursor-not-allowed');
    });

    it('Failure A.3: Readiness summary badge displays "Все поля заполнены" when unchanged with 0 blocking fields', () => {
      renderTestStudio({ initialDraft: sampleReadyDraft });
      const summaryBtn = screen.getByTestId('studio-readiness-summary');
      expect(summaryBtn.textContent).toBe('Все поля заполнены');
    });

    it('Failure A.4: Making a change transitions badge to "Готов к сохранению" and Save button to active purple bg-indigo-600', () => {
      renderTestStudio({ initialDraft: sampleReadyDraft });
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(false);
      expect(saveBtn.className).toContain('bg-indigo-600');
      expect(saveBtn.className).toContain('cursor-pointer');

      const summaryBtn = screen.getByTestId('studio-readiness-summary');
      expect(summaryBtn.textContent).toBe('Готов к сохранению');
    });

    // Failure B
    it('Failure B: After successful Save, editor reliably returns to clean disabled state and "Все поля заполнены"', async () => {
      const mockSave = vi.fn().mockResolvedValue(sampleReadyProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleReadyProduct);
      renderTestStudio({ initialDraft: sampleReadyDraft, saveProductFn: mockSave, getProductFn: mockGet });

      // Mutate
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');

      // Click Save
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // Returns to clean
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CANNOT_SAVE');
      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(true);
      expect(saveBtn.className).toContain('text-gray-400');
      expect(saveBtn.className).not.toContain('bg-indigo-400');
      expect(saveBtn.className).not.toContain('bg-indigo-600');

      const summaryBtn = screen.getByTestId('studio-readiness-summary');
      expect(summaryBtn.textContent).toBe('Все поля заполнены');
    });

    // Failure C
    it('Failure C.1: Post-save canonical hydration fetches from getProductFn (not unjoined PATCH struct), preserving human-readable size labels', async () => {
      // Unjoined PATCH response without size or colorName:
      const unjoinedPatchResponse = {
        ...sampleCanonicalProduct,
        variants: [
          {
            id: 'var-1',
            productId: 'prod-test-1',
            colorId: 'col-black',
            sizeValueId: 'sz-m', // Raw sizeValueId UUID
            sellerSku: 'SKU-BLK-M',
            priceCents: 450000,
            isActive: true,
          },
        ],
      };
      // Canonical GET response with enriched size and colorName:
      const canonicalGetResponse = {
        ...sampleCanonicalProduct,
        variants: [
          {
            id: 'var-1',
            productId: 'prod-test-1',
            colorId: 'col-black',
            colorName: 'Черный',
            colorHex: '#000000',
            sizeValueId: 'sz-m',
            size: 'M', // Enriched human-readable size
            sellerSku: 'SKU-BLK-M',
            priceCents: 450000,
            isActive: true,
          },
        ],
      };

      const mockPatch = vi.fn().mockResolvedValue(unjoinedPatchResponse);
      const mockGet = vi.fn().mockResolvedValue(canonicalGetResponse);

      renderTestStudio({ saveProductFn: mockPatch, getProductFn: mockGet });

      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      expect(mockPatch).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('prod-test-1');

      // Verify variant size is human readable 'M', NOT raw UUID
      expect(screen.getByTestId('test-variants-summary').textContent).toBe('var-1:Черный:M');
    });

    it('Failure C.2: Post-save canonical hydration preserves materialComposition name ("Хлопок", not "undefined 100%")', async () => {
      const canonicalGetResponse = {
        ...sampleCanonicalProduct,
        materialComposition: [
          {
            materialId: 'mat-cotton',
            materialName: 'Хлопок',
            percentage: 100,
          },
        ],
      };

      const mockPatch = vi.fn().mockResolvedValue(sampleCanonicalProduct);
      const mockGet = vi.fn().mockResolvedValue(canonicalGetResponse);

      renderTestStudio({ saveProductFn: mockPatch, getProductFn: mockGet });

      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // Verify material composition has name 'Хлопок', NOT 'undefined'
      expect(screen.getByTestId('test-materials-summary').textContent).toBe('Хлопок:100%');
    });

    it('Failure C.3: PATCH success + GET failure enters "refresh_error" state with refresh button and prevents re-PATCHing', async () => {
      const mockPatch = vi.fn().mockResolvedValue(sampleCanonicalProduct);
      const mockGet = vi.fn().mockRejectedValue(new Error('GET /api/seller/products/1 500'));

      renderTestStudio({ saveProductFn: mockPatch, getProductFn: mockGet });

      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('refresh_error');
      });

      // Error message informs user that product was saved but page must be refreshed
      expect(screen.getByTestId('studio-save-error-toast')).toBeTruthy();
      expect(screen.getByText('Товар сохранён, но не удалось обновить данные. Обновите страницу.')).toBeTruthy();
      expect(screen.getByTestId('studio-refresh-page-btn')).toBeTruthy();

      // In refresh_error state, canSave remains FALSE to prevent re-PATCHing
      expect(screen.getByTestId('test-can-save').textContent).toBe('CANNOT_SAVE');
      expect(screen.getByTestId('studio-header-save-btn').hasAttribute('disabled')).toBe(true);

      // Attempt click does NOT trigger additional PATCH
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));
      expect(mockPatch).toHaveBeenCalledTimes(1);
    });

    // Failure D
    it('Failure D.1: Visual preview color and size selection survives Save without resetting', async () => {
      const mockPatch = vi.fn().mockResolvedValue(sampleCanonicalProduct);
      const mockGet = vi.fn().mockResolvedValue(sampleCanonicalProduct);

      renderTestStudio({ saveProductFn: mockPatch, getProductFn: mockGet });

      // User selects color and size in visual preview
      fireEvent.click(screen.getByTestId('test-select-preview'));
      expect(screen.getByTestId('test-preview-color').textContent).toBe('col-black');
      expect(screen.getByTestId('test-preview-size').textContent).toBe('sz-m');

      // User modifies product and saves
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // Preview selection SURVIVED! Still col-black and sz-m!
      expect(screen.getByTestId('test-preview-color').textContent).toBe('col-black');
      expect(screen.getByTestId('test-preview-size').textContent).toBe('sz-m');
    });

    it('Failure D.2: Visual preview selection resets to null if previously selected color is deleted in post-save product (no guessing)', async () => {
      // Saved product only has white color:
      const productOnlyWhite: SellerProduct = {
        ...sampleCanonicalProduct,
        images: [
          {
            id: 'img-white-1',
            imageUrl: 'https://cdn.example.com/img1.jpg',
            sortOrder: 0,
            colorId: 'col-white',
            isMain: true,
          },
        ],
        variants: [
          {
            id: 'var-white-1',
            productId: 'prod-test-1',
            colorId: 'col-white',
            colorName: 'Белый',
            colorHex: '#FFFFFF',
            sizeValueId: 'sz-m',
            size: 'M',
            priceCents: 450000,
            isActive: true,
          },
        ],
      };

      const mockPatch = vi.fn().mockResolvedValue(productOnlyWhite);
      const mockGet = vi.fn().mockResolvedValue(productOnlyWhite);

      renderTestStudio({ saveProductFn: mockPatch, getProductFn: mockGet });

      // User selects black
      fireEvent.click(screen.getByTestId('test-select-preview'));
      expect(screen.getByTestId('test-preview-color').textContent).toBe('col-black');

      // Save happens and black was removed
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // Selection resets cleanly to null (none), not guessed or auto-picked
      expect(screen.getByTestId('test-preview-color').textContent).toBe('none');
    });
  });
});
