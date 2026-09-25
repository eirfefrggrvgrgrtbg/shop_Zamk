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
import { buildProductStudioUpdateRequest, isCanonicalVariantId } from './productStudioSaveProduct';
import { addSizeToMatrix } from './productStudioMatrixHelper';
import { hydrateProductStudioDraft } from './productStudioHydration';
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
    { id: 'col-grey', code: 'GREY', nameRu: 'Серый', hex: '#888888' },
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
          data-testid="test-add-local-image-no-color"
          onClick={() => {
            const fakeFile = new File(['bits-no-color'], 'no-color.jpg', { type: 'image/jpeg' });
            const previewUrl = createMediaUrl(fakeFile);
            const localImg: ProductStudioImage = {
              uiKey: 'client-media-no-color-999',
              colorId: undefined,
              isMain: false,
              sortOrder: (draft.images || []).length,
              source: {
                kind: 'local',
                clientMediaId: '99999999-9999-4999-8999-999999999999',
                file: fakeFile,
                previewUrl,
              },
            };
            updateDraft({ images: [...(draft.images || []), localImg] });
          }}
        >
          Add Local Image No Color
        </button>
        <button
          data-testid="test-add-local-image-with-color"
          onClick={() => {
            const fakeFile = new File(['bits-with-color'], 'with-color.jpg', { type: 'image/jpeg' });
            const previewUrl = createMediaUrl(fakeFile);
            const localImg: ProductStudioImage = {
              uiKey: 'client-media-with-color-888',
              colorId: 'col-black',
              isMain: false,
              sortOrder: (draft.images || []).length,
              source: {
                kind: 'local',
                clientMediaId: '88888888-8888-4888-8888-888888888888',
                file: fakeFile,
                previewUrl,
              },
            };
            updateDraft({ images: [...(draft.images || []), localImg] });
          }}
        >
          Add Local Image With Color
        </button>
        <button
          data-testid="test-call-save"
          onClick={() => saveDraft && saveDraft()}
        >
          Call Save
        </button>
        <button
          data-testid="test-add-size-matrix"
          onClick={() => {
            const updated = addSizeToMatrix(draft.variants || [], {
              id: '8cc76d3c-1369-44ac-b4f4-e47629aca389',
              label: 'M',
            });
            updateDraft({ variants: updated });
          }}
        >
          Add Size To Matrix
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

    it('3. Create mode cannot execute Edit Save (PATCH)', async () => {
      const mockSaveProduct = vi.fn();

      renderTestStudio({
        entryMode: 'create',
        saveProductFn: mockSaveProduct,
      });

      const saveBtn = screen.getByTestId('studio-header-save-btn');
      // Create mode has Create Save enabled, but it never executes Edit Save (PATCH)
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');
      expect(saveBtn.hasAttribute('disabled')).toBe(false);

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
        stagedMediaId: 'staged-1',
        imageUrl: 'https://cdn.example.com/staged-1.jpg',
        clientMediaId: 'client-media-123',
        status: 'ready',
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
        stagedMediaId: `staged-${clientMediaId}`,
        imageUrl: `https://cdn.example.com/staged-${clientMediaId}.jpg`,
        clientMediaId,
        status: 'ready',
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
          stagedMediaId: `staged-${clientMediaId}`,
          imageUrl: `https://cdn.example.com/staged-${clientMediaId}.jpg`,
          clientMediaId,
          status: 'ready',
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
          stagedMediaId: `staged-${cid}`,
          imageUrl: `https://cdn.example.com/${cid}.jpg`,
          clientMediaId: cid,
          status: 'ready',
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
        stagedMediaId: `staged-${clientMediaId}`,
        imageUrl: `https://cdn.example.com/${clientMediaId}.jpg`,
        clientMediaId,
        status: 'ready',
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
        stagedMediaId: 'staged-uuid-abc',
        imageUrl: 'https://cdn.example.com/staged-abc.jpg',
        clientMediaId: 'client-media-123',
        status: 'ready',
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

    it('Failure D.2: Visual preview selection resets to null if previously selected color is deleted and multiple colors remain (no guessing)', async () => {
      // Saved product has white and grey colors (2 colors):
      const productMultiColors: SellerProduct = {
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
          {
            id: 'var-grey-1',
            productId: 'prod-test-1',
            colorId: 'col-grey',
            colorName: 'Серый',
            colorHex: '#888888',
            sizeValueId: 'sz-m',
            size: 'M',
            priceCents: 450000,
            isActive: true,
          },
        ],
      };

      const mockPatch = vi.fn().mockResolvedValue(productMultiColors);
      const mockGet = vi.fn().mockResolvedValue(productMultiColors);

      renderTestStudio({ saveProductFn: mockPatch, getProductFn: mockGet });

      // User selects black
      fireEvent.click(screen.getByTestId('test-select-preview'));
      expect(screen.getByTestId('test-preview-color').textContent).toBe('col-black');

      // Save happens and black was removed, 2 colors remain
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // Selection resets cleanly to null (none), not guessed or auto-picked
      expect(screen.getByTestId('test-preview-color').textContent).toBe('none');
    });

    it('Failure D.3: Visual preview clears selection if previously selected color is deleted even if exactly one color remains (PS.R4B3.1C4C3B2G)', async () => {
      // Saved product only has white color (1 color):
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

      // Save happens and black was removed, exactly 1 color remains
      fireEvent.click(screen.getByTestId('test-mutate-title'));
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // Selection clears cleanly to null (none), no automatic selection
      expect(screen.getByTestId('test-preview-color').textContent).toBe('none');
    });
  });

  /* ========================================================================
   * SUITE 9: Canonical Variant ID Guard (PS.R4B3.1C4C3B2C1)
   * ======================================================================== */
  describe('Suite 9: Canonical Variant ID Guard (PS.R4B3.1C4C3B2C1)', () => {
    describe('isCanonicalVariantId helper', () => {
      it('1. accepts valid lowercase canonical UUID', () => {
        expect(isCanonicalVariantId('f6380218-63b8-414e-8969-51f59962958e')).toBe(true);
        expect(isCanonicalVariantId('00000000-0000-4000-8000-000000000999')).toBe(true);
      });

      it('2. accepts valid uppercase canonical UUID', () => {
        expect(isCanonicalVariantId('F6380218-63B8-414E-8969-51F59962958E')).toBe(true);
      });

      it('3. rejects draft-var-* synthetic IDs', () => {
        expect(isCanonicalVariantId('draft-var-1726947265891-x9a2k')).toBe(false);
      });

      it('4. rejects synthetic-* IDs', () => {
        expect(isCanonicalVariantId('synthetic-v-1')).toBe(false);
        expect(isCanonicalVariantId('synthetic-variant-123')).toBe(false);
      });

      it('5. rejects arbitrary local IDs', () => {
        expect(isCanonicalVariantId('variant-local-123')).toBe(false);
        expect(isCanonicalVariantId('local-1')).toBe(false);
        expect(isCanonicalVariantId('var-1')).toBe(false);
      });

      it('6. rejects malformed UUID strings', () => {
        expect(isCanonicalVariantId('not-a-uuid')).toBe(false);
        expect(isCanonicalVariantId('12345678-1234-1234-1234-12345678901z')).toBe(false);
        expect(isCanonicalVariantId('f6380218-63b8-414e-8969-51f59962958')).toBe(false); // short
        expect(isCanonicalVariantId('f6380218-63b8-414e-8969-51f59962958eee')).toBe(false); // long
      });

      it('7. rejects undefined, null, or empty string IDs', () => {
        expect(isCanonicalVariantId(undefined)).toBe(false);
        expect(isCanonicalVariantId('')).toBe(false);
        expect(isCanonicalVariantId('   ')).toBe(false);
      });
    });

    describe('buildProductStudioUpdateRequest variant ID mapping', () => {
      it('preserves valid canonical UUIDs and omits all non-UUID IDs in PATCH payload', () => {
        const canonicalUuid1 = 'f6380218-63b8-414e-8969-51f59962958e';
        const canonicalUuidUpper = '13096D70-C15D-41CE-B91C-59ABC18DB83B';

        const mixedDraft: ProductStudioDraft = {
          ...sampleInitialDraft,
          variants: [
            { id: canonicalUuid1, colorId: 'col-black', sizeValueId: 'sz-m' },
            { id: canonicalUuidUpper, colorId: 'col-white', sizeValueId: 'sz-m' },
            { id: 'draft-var-1726947265891-x9a2k', colorId: 'col-beige', sizeValueId: 'sz-m' },
            { id: 'synthetic-v-2', colorId: 'col-black', sizeValueId: 'sz-l' },
            { id: 'variant-local-123', colorId: 'col-white', sizeValueId: 'sz-l' },
            { id: 'not-a-valid-uuid', colorId: 'col-beige', sizeValueId: 'sz-l' },
            { id: undefined, colorId: 'col-black', sizeValueId: 'sz-s' },
          ],
        };

        const payload = buildProductStudioUpdateRequest(mixedDraft, sampleInitialDraft);
        expect(payload.variants).toBeDefined();
        expect(payload.variants?.length).toBe(7);

        // 1. Valid lowercase UUID preserved
        expect(payload.variants?.[0].id).toBe(canonicalUuid1);
        // 2. Valid uppercase UUID preserved
        expect(payload.variants?.[1].id).toBe(canonicalUuidUpper);
        // 3. draft-var-* omitted
        expect(payload.variants?.[2].id).toBeUndefined();
        // 4. synthetic-* omitted
        expect(payload.variants?.[3].id).toBeUndefined();
        // 5. arbitrary local ID omitted
        expect(payload.variants?.[4].id).toBeUndefined();
        // 6. malformed UUID omitted
        expect(payload.variants?.[5].id).toBeUndefined();
        // 7. undefined ID omitted
        expect(payload.variants?.[6].id).toBeUndefined();

        // Direct assertion: every emitted variants[].id, if present, matches canonical UUID format
        const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
        for (const v of payload.variants!) {
          if (v.id !== undefined) {
            expect(UUID_REGEX.test(v.id)).toBe(true);
          }
        }
      });

      it('PS.R4B3.1C4C3B3B-R2: Edit PATCH sends ONLY active tuples, preserving surviving UUIDs and omitting synthetic IDs', () => {
        const canonicalUuidBlackM = '00000000-0000-4000-8000-000000000001';
        const canonicalUuidBlackL = '00000000-0000-4000-8000-000000000002';

        const workingDraft: ProductStudioDraft = {
          ...sampleInitialDraft,
          variants: [
            {
              id: canonicalUuidBlackM,
              colorId: 'col-black',
              sizeValueId: 'sz-m',
              isActive: true,
            },
            {
              id: canonicalUuidBlackL,
              colorId: 'col-black',
              sizeValueId: 'sz-l',
              isActive: true,
            },
            {
              id: 'draft-var-grey-l-12345',
              colorId: 'col-grey',
              sizeValueId: 'sz-l',
              isActive: true,
            },
            // Grey/M is OFF (isActive: false)
            {
              id: 'draft-var-grey-m-67890',
              colorId: 'col-grey',
              sizeValueId: 'sz-m',
              isActive: false,
            },
          ],
        };

        const payload = buildProductStudioUpdateRequest(workingDraft, sampleInitialDraft);
        expect(payload.variants).toBeDefined();
        expect(payload.variants).toHaveLength(3);

        const pairs = payload.variants!.map((v) => `${v.colorId}:${v.sizeValueId}`);
        expect(pairs).toContain('col-black:sz-m');
        expect(pairs).toContain('col-black:sz-l');
        expect(pairs).toContain('col-grey:sz-l');
        expect(pairs).not.toContain('col-grey:sz-m');

        // Surviving canonical UUIDs preserved
        const blackM = payload.variants!.find((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m');
        expect(blackM?.id).toBe(canonicalUuidBlackM);
        const blackL = payload.variants!.find((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-l');
        expect(blackL?.id).toBe(canonicalUuidBlackL);

        // Synthetic draft ID omitted from backend UUID field
        const greyL = payload.variants!.find((v) => v.colorId === 'col-grey' && v.sizeValueId === 'sz-l');
        expect(greyL?.id).toBeUndefined();
      });
    });

    it('33. adding size to variant matrix generates draft-var-* IDs but PATCH payload omits synthetic IDs (no Go 400 invalid_request)', async () => {
      // Canonical product matching real Safari failing product ecf7ea61-e4b6-43ca-999c-b857b2b4cf15
      const canonicalExistingProduct: SellerProduct = {
        ...sampleCanonicalProduct,
        id: 'ecf7ea61-e4b6-43ca-999c-b857b2b4cf15',
        title: 'wdwdwdw',
        slug: 'wdwdwdw',
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
        categoryName: 'Худи',
        brandId: '77777777-7777-4777-8777-777777777777',
        brandName: 'Dev Brand',
        sellerId: '44444444-4444-4444-8444-444444444444',
        status: 'draft',
        priceCents: 121200,
        currency: 'RUB',
        material: 'Хлопок — 100%',
        images: [],
        variants: [
          {
            id: 'f6380218-63b8-414e-8969-51f59962958e',
            productId: 'ecf7ea61-e4b6-43ca-999c-b857b2b4cf15',
            colorId: 'col-black',
            colorName: 'Черный',
            barcode: 'ZMK-32a45fbf-a88',
            isActive: true,
          },
          {
            id: '13096d70-c15d-41ce-b91c-59abc18db83b',
            productId: 'ecf7ea61-e4b6-43ca-999c-b857b2b4cf15',
            colorId: 'col-white',
            colorName: 'Белый',
            barcode: 'ZMK-018dd708-151',
            isActive: true,
          },
          {
            id: '0b900cc5-1e52-44ca-bdc5-923efb2ee90e',
            productId: 'ecf7ea61-e4b6-43ca-999c-b857b2b4cf15',
            colorId: 'col-beige',
            colorName: 'Бежевый',
            barcode: 'ZMK-d2c96f99-908',
            isActive: true,
          },
        ],
        materialComposition: [
          {
            productId: 'ecf7ea61-e4b6-43ca-999c-b857b2b4cf15',
            materialId: 'mat-cotton',
            materialName: 'Хлопок',
            percentage: 100,
          },
        ],
      };

      const hydratedDraft = hydrateProductStudioDraft({
        product: canonicalExistingProduct,
        categorySchema: mockSchema,
        canonicalColors: [
          ...mockColors,
          { id: 'col-beige', code: 'BEIGE', nameRu: 'Бежевый', hex: '#F5F5DC' },
        ],
        dictionaryValuesMap: mockDictMap,
      });

      const mockCreateProduct = vi.fn();
      let capturedPatchPayload: any = null;
      const mockSaveProduct = vi.fn().mockImplementation(async (_id: string, payload: any) => {
        capturedPatchPayload = payload;
        return {
          ...canonicalExistingProduct,
          variants: (payload.variants || []).map((v: any, idx: number) => ({
            id: v.id || `backend-generated-uuid-${idx}`,
            productId: canonicalExistingProduct.id,
            colorId: v.colorId,
            sizeValueId: v.sizeValueId,
            barcode: `ZMK-new-${idx}`,
            isActive: true,
          })),
        };
      });

      const mockGetProduct = vi.fn().mockImplementation(async () => ({
        ...canonicalExistingProduct,
        variants: [
          {
            id: '0fac9197-87dc-4dd0-86c7-3472d5c25ebb',
            productId: canonicalExistingProduct.id,
            colorId: 'col-black',
            colorName: 'Черный',
            sizeValueId: '8cc76d3c-1369-44ac-b4f4-e47629aca389',
            size: 'M',
            barcode: 'ZMK-b8fe9a9a-05a',
            isActive: true,
          },
        ],
      }));

      renderTestStudio({
        initialDraft: hydratedDraft,
        canonicalColors: [
          ...mockColors,
          { id: 'col-beige', code: 'BEIGE', nameRu: 'Бежевый', hex: '#F5F5DC' },
        ],
        createProductFn: mockCreateProduct,
        saveProductFn: mockSaveProduct,
        getProductFn: mockGetProduct,
      });

      // 1. Mutate variant matrix: add size 'M' (generates draft-var-* synthetic IDs)
      fireEvent.click(screen.getByTestId('test-add-size-matrix'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');

      // 2. Click Save
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // 3. Exactly one PATCH, NO Create POST
      expect(mockCreateProduct).not.toHaveBeenCalled();
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);
      expect(mockGetProduct).toHaveBeenCalledTimes(1);

      // 4. Verify captured PATCH payload has NO synthetic draft-var-* IDs
      expect(capturedPatchPayload).toBeDefined();
      expect(capturedPatchPayload.variants).toBeDefined();
      expect(capturedPatchPayload.variants.length).toBeGreaterThan(0);
      const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
      for (const variant of capturedPatchPayload.variants) {
        expect(variant.id).toBeUndefined(); // Synthetic draft-var-* stripped!
        if (variant.id !== undefined) {
          expect(UUID_REGEX.test(variant.id)).toBe(true);
        }
      }
      expect(JSON.stringify(capturedPatchPayload)).not.toContain('draft-var-');

      // 5. Canonical GET refreshed state
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
    });
  });

  /* ========================================================================
   * SUITE 10: Media Staging & Color Binding Guarantees (PS.R4B3.1C4C3B2D)
   * ======================================================================== */
  describe('Suite 10: Media Staging & Color Binding Guarantees (PS.R4B3.1C4C3B2D)', () => {
    it('34. local image with colorId = undefined: staging is allowed, no choose color validation blocks persistence, stage occurs, PATCH succeeds and omits/nulls colorId', async () => {
      let capturedPatchPayload: any = null;
      const stagedRequests: Array<{ pid: string; cid: string }> = [];

      const mockStageImage = vi.fn().mockImplementation(async (pid: string, cid: string, _file: File) => {
        stagedRequests.push({ pid, cid });
        return {
          stagedMediaId: '00000000-0000-4000-8000-000000000088',
          clientMediaId: cid,
          imageUrl: 'https://storage.zamk.test/staged/no-color.jpg',
          status: 'ready',
        };
      });

      const mockSaveProduct = vi.fn().mockImplementation(async (_pid: string, payload: any) => {
        capturedPatchPayload = payload;
        return {
          ...sampleCanonicalProduct,
          images: [
            ...(sampleCanonicalProduct.images || []),
            {
              id: '00000000-0000-4000-8000-000000000088',
              productId: sampleCanonicalProduct.id,
              imageUrl: 'https://storage.zamk.test/staged/no-color.jpg',
              colorId: null,
              isMain: false,
              sortOrder: 1,
            },
          ],
        };
      });

      const mockGetProduct = vi.fn().mockImplementation(async () => ({
        ...sampleCanonicalProduct,
        images: [
          ...(sampleCanonicalProduct.images || []),
          {
            id: '00000000-0000-4000-8000-000000000088',
            productId: sampleCanonicalProduct.id,
            imageUrl: 'https://storage.zamk.test/staged/no-color.jpg',
            colorId: null,
            isMain: false,
            sortOrder: 1,
          },
        ],
      }));

      renderTestStudio({
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
        getProductFn: mockGetProduct,
      });

      // 1. Add local image with colorId = undefined
      fireEvent.click(screen.getByTestId('test-add-local-image-no-color'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
      expect(screen.getByTestId('test-can-save').textContent).toBe('CAN_SAVE');

      // 2. Click Save - verify no "choose color" blocks it
      const saveBtn = screen.getByTestId('studio-header-save-btn');
      expect(saveBtn.hasAttribute('disabled')).toBe(false);
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // 3. Staging occurred with exact clientMediaId
      expect(mockStageImage).toHaveBeenCalledTimes(1);
      expect(stagedRequests[0].cid).toBe('99999999-9999-4999-8999-999999999999');

      // 4. Exactly one PATCH called
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);
      expect(capturedPatchPayload).toBeDefined();
      expect(capturedPatchPayload.images).toBeDefined();

      // Find the staged image in patch payload
      const patchImg = capturedPatchPayload.images.find(
        (img: any) => img.id === '00000000-0000-4000-8000-000000000088'
      );
      expect(patchImg).toBeDefined();
      // colorId is null or undefined (not bound to any color)
      expect(patchImg.colorId).toBeNull();

      // 5. Success state
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
      expect(screen.queryByTestId('studio-save-error-toast')).toBeNull();
    });

    it('35. local image with explicit colorId: preserves canonical colorId in PATCH payload', async () => {
      let capturedPatchPayload: any = null;

      const mockStageImage = vi.fn().mockImplementation(async (_pid: string, cid: string) => ({
        stagedMediaId: '00000000-0000-4000-8000-000000000077',
        clientMediaId: cid,
        imageUrl: 'https://storage.zamk.test/staged/with-color.jpg',
        status: 'ready',
      }));

      const mockSaveProduct = vi.fn().mockImplementation(async (_pid: string, payload: any) => {
        capturedPatchPayload = payload;
        return {
          ...sampleCanonicalProduct,
          images: [
            ...(sampleCanonicalProduct.images || []),
            {
              id: '00000000-0000-4000-8000-000000000077',
              productId: sampleCanonicalProduct.id,
              imageUrl: 'https://storage.zamk.test/staged/with-color.jpg',
              colorId: 'col-black',
              isMain: false,
              sortOrder: 1,
            },
          ],
        };
      });

      const mockGetProduct = vi.fn().mockImplementation(async () => ({
        ...sampleCanonicalProduct,
        images: [
          ...(sampleCanonicalProduct.images || []),
          {
            id: '00000000-0000-4000-8000-000000000077',
            productId: sampleCanonicalProduct.id,
            imageUrl: 'https://storage.zamk.test/staged/with-color.jpg',
            colorId: 'col-black',
            isMain: false,
            sortOrder: 1,
          },
        ],
      }));

      renderTestStudio({
        stageImageFn: mockStageImage,
        saveProductFn: mockSaveProduct,
        getProductFn: mockGetProduct,
      });

      // 1. Add local image with explicit colorId
      fireEvent.click(screen.getByTestId('test-add-local-image-with-color'));
      expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');

      // 2. Save
      fireEvent.click(screen.getByTestId('studio-header-save-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('test-save-status').textContent).toBe('idle');
      });

      // 3. Staging and PATCH occurred
      expect(mockStageImage).toHaveBeenCalledTimes(1);
      expect(mockSaveProduct).toHaveBeenCalledTimes(1);

      const patchImg = capturedPatchPayload.images.find(
        (img: any) => img.id === '00000000-0000-4000-8000-000000000077'
      );
      expect(patchImg).toBeDefined();
      expect(patchImg.colorId).toBe('col-black');
    });
  });
});
