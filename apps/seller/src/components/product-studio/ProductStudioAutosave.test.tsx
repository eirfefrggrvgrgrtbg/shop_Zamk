/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import React from 'react';
import { render, screen, fireEvent, act, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { SellerProduct, SellerCategorySchema, SellerColor } from '@zamk/api-client';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';
import { ProductStudioHeader } from './ProductStudioHeader';
import { isProductStudioCreateAutosaveEligible } from './productStudioSaveProduct';

const mockSchema: SellerCategorySchema = {
  id: 'cat-hoodies',
  name: 'Толстовки и свитшоты',
  dimensionType: 'SINGLE_VARIANT',
  allowedAttributes: [],
} as any;

vi.mock('@zamk/api-client/src/seller', async (importOriginal) => {
  const actual = await importOriginal<any>();
  return {
    ...actual,
    getSellerCategorySchema: vi.fn().mockImplementation((id: string) => {
      if (id === 'cat-hoodies') {
        return Promise.resolve(mockSchema);
      }
      return Promise.resolve(null);
    }),
  };
});

const sampleInitialDraft: ProductStudioDraft = {
  id: 'prod-hoodie-1',
  title: 'Худи Базовое',
  description: 'Описание худи',
  categoryId: 'cat-hoodies',
  status: 'draft',
  priceCents: 450000,
  mediaMode: 'GENERAL',
  dimensionType: 'SINGLE_VARIANT',
  images: [
    {
      uiKey: 'img-1',
      sortOrder: 0,
      isMain: true,
      cropX: 0,
      cropY: 0,
      cropWidth: 800,
      cropHeight: 1000,
      renditionUrl: 'https://cdn.example.com/rendition1.jpg',
      renditionStatus: 'READY',
      source: {
        kind: 'canonical',
        imageId: 'img-1',
        url: 'https://cdn.example.com/source1.jpg',
        renditionUrl: 'https://cdn.example.com/rendition1.jpg',
      },
    },
    {
      uiKey: 'img-2',
      sortOrder: 1,
      isMain: false,
      cropX: 0,
      cropY: 0,
      cropWidth: 800,
      cropHeight: 1000,
      renditionUrl: 'https://cdn.example.com/rendition2.jpg',
      renditionStatus: 'READY',
      source: {
        kind: 'canonical',
        imageId: 'img-2',
        url: 'https://cdn.example.com/source2.jpg',
        renditionUrl: 'https://cdn.example.com/rendition2.jpg',
      },
    },
    {
      uiKey: 'img-3',
      sortOrder: 2,
      isMain: false,
      cropX: 0,
      cropY: 0,
      cropWidth: 800,
      cropHeight: 1000,
      renditionUrl: 'https://cdn.example.com/rendition3.jpg',
      renditionStatus: 'READY',
      source: {
        kind: 'canonical',
        imageId: 'img-3',
        url: 'https://cdn.example.com/source3.jpg',
        renditionUrl: 'https://cdn.example.com/rendition3.jpg',
      },
    },
  ],
  variants: [
    {
      id: '11111111-1111-4111-8111-111111111111',
      sellerSku: 'SKU-1',
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
};

const sampleCanonicalProduct: SellerProduct = {
  id: 'prod-hoodie-1',
  title: 'Худи Базовое',
  description: 'Описание худи',
  categoryId: 'cat-hoodies',
  status: 'draft',
  priceCents: 450000,
  mediaMode: 'GENERAL',
  dimensionType: 'SINGLE_VARIANT',
  images: [
    {
      id: 'img-1',
      sortOrder: 0,
      isMain: true,
      cropX: 0,
      cropY: 0,
      cropWidth: 800,
      cropHeight: 1000,
      renditionUrl: 'https://cdn.example.com/rendition1.jpg',
      url: 'https://cdn.example.com/source1.jpg',
    },
    {
      id: 'img-2',
      sortOrder: 1,
      isMain: false,
      cropX: 0,
      cropY: 0,
      cropWidth: 800,
      cropHeight: 1000,
      renditionUrl: 'https://cdn.example.com/rendition2.jpg',
      url: 'https://cdn.example.com/source2.jpg',
    },
    {
      id: 'img-3',
      sortOrder: 2,
      isMain: false,
      cropX: 0,
      cropY: 0,
      cropWidth: 800,
      cropHeight: 1000,
      renditionUrl: 'https://cdn.example.com/rendition3.jpg',
      url: 'https://cdn.example.com/source3.jpg',
    },
  ],
  variants: [
    {
      id: '11111111-1111-4111-8111-111111111111',
      sellerSku: 'SKU-1',
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
} as any;

const sampleCanonicalCreatedProduct: SellerProduct = {
  id: 'prod-created-101',
  title: 'Худи Базовое',
  description: '',
  categoryId: 'cat-hoodies',
  status: 'draft',
  priceCents: 450000,
  currency: 'RUB',
  mediaMode: 'GENERAL',
  dimensionType: 'SINGLE_VARIANT',
  images: [],
  variants: [
    {
      id: '11111111-1111-4111-8111-111111111111',
      productId: 'prod-created-101',
      colorId: 'col-black',
      sizeValueId: 'sz-m',
      priceCents: 450000,
      isActive: true,
    },
  ],
  materialComposition: [],
} as any;

function TestAutosaveControls() {
  const {
    draft,
    isDirty,
    updateDraft,
    flushSave,
    saveStatus,
    setViewMode,
    setActiveSection,
    selectCategory,
  } = useProductStudio();
  return (
    <div data-testid="test-autosave-controls">
      <span data-testid="test-is-dirty">{isDirty ? 'DIRTY' : 'CLEAN'}</span>
      <span data-testid="test-title">{draft.title}</span>
      <span data-testid="test-save-status">{saveStatus}</span>
      <button
        type="button"
        data-testid="btn-mutate-title"
        onClick={() => updateDraft({ title: 'Худи Обновленное' })}
      >
        Update Title
      </button>
      <button
        type="button"
        data-testid="btn-mutate-title-privet"
        onClick={() => updateDraft({ title: 'Привет' })}
      >
        Update Title Privet
      </button>
      <button
        type="button"
        data-testid="btn-select-category-hoodies"
        onClick={() => selectCategory?.({ id: 'cat-hoodies', name: 'Худи', slug: 'hoodies' } as any)}
      >
        Select Category Hoodies
      </button>
      <button
        type="button"
        data-testid="btn-mutate-title-custom"
        onClick={(e) => {
          const val = (e.target as any).dataset.val || 'Новый текст';
          updateDraft({ title: val });
        }}
      >
        Update Title Custom
      </button>
      <button
        type="button"
        data-testid="btn-mutate-description"
        onClick={() => updateDraft({ description: 'Новое описание' })}
      >
        Update Description
      </button>
      <button
        type="button"
        data-testid="btn-flush-save"
        onClick={() => flushSave()}
      >
        Flush Save
      </button>
      <button
        type="button"
        data-testid="btn-switch-view-form"
        onClick={() => setViewMode('form')}
      >
        Switch View Form
      </button>
      <button
        type="button"
        data-testid="btn-switch-section-media"
        onClick={() => setActiveSection('media')}
      >
        Switch Section Media
      </button>
    </div>
  );
}

function renderAutosaveHarness(props: Partial<React.ComponentProps<typeof ProductStudioProvider>> = {}) {
  const mockSaveProduct = vi.fn().mockImplementation((id: string, patch: any) => {
    return Promise.resolve({
      ...sampleCanonicalProduct,
      ...patch,
      id,
    });
  });
  const mockGetProduct = vi.fn().mockImplementation((id: string) => {
    const lastPatch = mockSaveProduct.mock.calls[mockSaveProduct.mock.calls.length - 1]?.[1] || {};
    return Promise.resolve({
      ...sampleCanonicalProduct,
      ...lastPatch,
      id,
    });
  });
  const mockSubmitModeration = vi.fn().mockResolvedValue(undefined);

  const view = render(
    <MemoryRouter>
      <ProductStudioProvider
        entryMode="edit"
        initialDraft={sampleInitialDraft}
        initialCategorySchema={mockSchema}
        saveProductFn={mockSaveProduct}
        getProductFn={mockGetProduct}
        submitModerationFn={mockSubmitModeration}
        {...props}
      >
        <ProductStudioHeader />
        <TestAutosaveControls />
      </ProductStudioProvider>
    </MemoryRouter>
  );

  return {
    ...view,
    mockSaveProduct,
    mockGetProduct,
    mockSubmitModeration,
  };
}

function renderCreateAutosaveHarness(props: Partial<React.ComponentProps<typeof ProductStudioProvider>> = {}) {
  const mockCreateProduct =
    props.createProductFn ||
    vi.fn().mockImplementation((payload: any) => {
      return Promise.resolve({
        ...sampleCanonicalCreatedProduct,
        ...payload,
        id: 'prod-created-101',
        status: 'draft',
      });
    });
  const mockSaveProduct =
    props.saveProductFn ||
    vi.fn().mockImplementation((id: string, patch: any) => {
      return Promise.resolve({
        ...sampleCanonicalCreatedProduct,
        ...patch,
        id,
      });
    });
  const mockGetProduct =
    props.getProductFn ||
    vi.fn().mockImplementation((id: string) => {
      const lastPatch = (mockSaveProduct as any).mock?.calls?.[(mockSaveProduct as any).mock?.calls?.length - 1]?.[1];
      const initialCreate = (mockCreateProduct as any).mock?.calls?.[0]?.[0] || {};
      return Promise.resolve({
        ...sampleCanonicalCreatedProduct,
        ...initialCreate,
        ...(lastPatch || {}),
        id,
      });
    });
  const mockSubmitModeration = props.submitModerationFn || vi.fn().mockResolvedValue(undefined);
  const mockNavigate = props.onNavigate || vi.fn();

  const emptyDraft: ProductStudioDraft = {
    title: '',
    description: '',
    categoryId: 'cat-hoodies',
    priceCents: 450000,
    variants: [
      {
        id: 'var-1',
        colorId: 'col-black',
        sizeValueId: 'sz-m',
        priceCents: 450000,
        isActive: true,
      },
    ],
    colors: [{ id: 'col-black', nameRu: 'Черный' }],
    images: [],
  };

  const mockCanonicalColors: SellerColor[] = [
    { id: 'col-black', nameRu: 'Черный', hex: '#000000', code: 'BLACK' } as any,
  ];

  const view = render(
    <MemoryRouter>
      <ProductStudioProvider
        entryMode="create"
        initialDraft={emptyDraft}
        initialCategorySchema={mockSchema}
        canonicalColors={mockCanonicalColors}
        {...props}
        createProductFn={mockCreateProduct}
        saveProductFn={mockSaveProduct}
        getProductFn={mockGetProduct}
        submitModerationFn={mockSubmitModeration}
        onNavigate={mockNavigate}
      >
        <ProductStudioHeader />
        <TestAutosaveControls />
      </ProductStudioProvider>
    </MemoryRouter>
  );

  return {
    ...view,
    mockCreateProduct,
    mockSaveProduct,
    mockGetProduct,
    mockSubmitModeration,
    mockNavigate,
  };
}

describe('ProductStudio Autosave Pipeline', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    act(() => {
      vi.runOnlyPendingTimers();
    });
    vi.useRealTimers();
    sessionStorage.clear();
    cleanup();
  });

  it('1. One field change triggers debounced autosave after 800ms', async () => {
    const { mockSaveProduct } = renderAutosaveHarness();

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');

    // Mutate title
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title'));
    });

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Есть изменения');
    expect(mockSaveProduct).not.toHaveBeenCalled();

    // Advance 500ms (less than 800ms)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500);
    });
    expect(mockSaveProduct).not.toHaveBeenCalled();

    // Advance remaining 300ms (total 800ms)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });

    expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    expect(mockSaveProduct).toHaveBeenCalledWith('prod-hoodie-1', expect.objectContaining({
      title: 'Худи Обновленное',
    }));

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
  });

  it('2. Rapid typing debounces correctly: only one save request after typing settles', async () => {
    const { mockSaveProduct } = renderAutosaveHarness();

    const mutateCustomBtn = screen.getByTestId('btn-mutate-title-custom');

    // Rapid keystroke 1
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Х');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });
    expect(mockSaveProduct).not.toHaveBeenCalled();

    // Rapid keystroke 2
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Ху');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });
    expect(mockSaveProduct).not.toHaveBeenCalled();

    // Rapid keystroke 3
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Худ');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });
    expect(mockSaveProduct).not.toHaveBeenCalled();

    // Rapid keystroke 4: settling
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Худи Зимнее');
      fireEvent.click(mutateCustomBtn);
    });

    // Advance 800ms after final keystroke
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    // Exactly 1 save call with final payload
    expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    expect(mockSaveProduct).toHaveBeenCalledWith('prod-hoodie-1', expect.objectContaining({
      title: 'Худи Зимнее',
    }));
  });

  it('3. User edits field while save is in-flight: queued save executes with latest edits and draft retains edits', async () => {
    let resolveSave1!: (val: any) => void;
    const savePromise1 = new Promise((resolve) => {
      resolveSave1 = resolve;
    });

    const mockSaveProduct = vi.fn()
      .mockImplementationOnce(() => savePromise1)
      .mockImplementationOnce((id, patch) => Promise.resolve({
        ...sampleCanonicalProduct,
        ...patch,
        id,
      }));

    const mockGetProduct = vi.fn().mockImplementation((id) => {
      const lastPatch = mockSaveProduct.mock.calls[mockSaveProduct.mock.calls.length - 1]?.[1] || {};
      return Promise.resolve({
        ...sampleCanonicalProduct,
        ...lastPatch,
        id,
      });
    });

    renderAutosaveHarness({
      saveProductFn: mockSaveProduct,
      getProductFn: mockGetProduct,
    });

    const mutateCustomBtn = screen.getByTestId('btn-mutate-title-custom');

    // 1. Initial edit to trigger first save
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Edit 1');
      fireEvent.click(mutateCustomBtn);
    });

    // Advance 800ms to fire save 1
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    expect(mockSaveProduct).toHaveBeenNthCalledWith(1, 'prod-hoodie-1', expect.objectContaining({
      title: 'Edit 1',
    }));
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохраняем…');

    // 2. While save 1 is in-flight, user makes newer edit 'Edit 2'
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Edit 2');
      fireEvent.click(mutateCustomBtn);
    });

    expect(screen.getByTestId('test-title').textContent).toBe('Edit 2');

    // 3. Resolve save 1 (server responds with 'Edit 1')
    await act(async () => {
      resolveSave1({
        ...sampleCanonicalProduct,
        title: 'Edit 1',
      });
      await vi.advanceTimersByTimeAsync(0);
    });

    // Crucial Invariant: Local draft MUST retain 'Edit 2' and not get wiped out by save 1 server response!
    expect(screen.getByTestId('test-title').textContent).toBe('Edit 2');

    // 4. Queued save fires immediately for 'Edit 2'
    expect(mockSaveProduct).toHaveBeenCalledTimes(2);
    expect(mockSaveProduct).toHaveBeenNthCalledWith(2, 'prod-hoodie-1', expect.objectContaining({
      title: 'Edit 2',
    }));

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
    expect(screen.getByTestId('test-title').textContent).toBe('Edit 2');
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
  });

  it('4. Autosave failure: displays error state in header and offers retry', async () => {
    let shouldFail = true;
    const mockSaveProduct = vi.fn().mockImplementation((id, patch) => {
      if (shouldFail) {
        return Promise.reject(new Error('Network connection dropped'));
      }
      return Promise.resolve({
        ...sampleCanonicalProduct,
        ...patch,
        id,
      });
    });

    renderAutosaveHarness({
      saveProductFn: mockSaveProduct,
    });

    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title'));
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    expect(mockSaveProduct).toHaveBeenCalledTimes(1);

    // Save error state in header
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Ошибка сохранения');
    expect(screen.getByTestId('studio-retry-save-btn')).toBeTruthy();

    // User draft is preserved and not corrupted
    expect(screen.getByTestId('test-title').textContent).toBe('Худи Обновленное');

    // Fix connection and retry
    shouldFail = false;
    await act(async () => {
      fireEvent.click(screen.getByTestId('studio-retry-save-btn'));
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
  });

  it('5. Submit with dirty changes: flushes pending save first, does not submit if save fails, submits after save succeeds', async () => {
    let failSave = true;
    const mockSaveProduct = vi.fn().mockImplementation((id, patch) => {
      if (failSave) {
        return Promise.reject(new Error('Save failed before submit'));
      }
      return Promise.resolve({
        ...sampleCanonicalProduct,
        ...patch,
        id,
      });
    });
    const mockSubmitModeration = vi.fn().mockResolvedValue(undefined);

    renderAutosaveHarness({
      saveProductFn: mockSaveProduct,
      submitModerationFn: mockSubmitModeration,
    });

    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');
    expect((submitBtn as HTMLButtonElement).disabled).toBe(false);

    // Mutate title -> dirty
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title'));
    });
    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');

    // Submit button is NOT disabled merely because dirty
    expect((submitBtn as HTMLButtonElement).disabled).toBe(false);

    // User clicks submit while save is pending
    await act(async () => {
      fireEvent.click(submitBtn);
      await vi.advanceTimersByTimeAsync(0);
    });

    // Save was attempted
    expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    // Since save failed, submitModeration was NOT called!
    expect(mockSubmitModeration).not.toHaveBeenCalled();
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Ошибка сохранения');

    // Now let save succeed on next attempt
    failSave = false;
    await act(async () => {
      fireEvent.click(submitBtn);
      await vi.advanceTimersByTimeAsync(0);
    });

    // Save was called again and succeeded
    expect(mockSaveProduct).toHaveBeenCalledTimes(2);
    // Moderation endpoint was called after successful save!
    expect(mockSubmitModeration).toHaveBeenCalledTimes(1);
    expect(mockSubmitModeration).toHaveBeenCalledWith('prod-hoodie-1');
  });

  it('6. Incomplete draft: submit button disabled with helpful tooltip, cannot submit incomplete draft even if dirty', async () => {
    const incompleteDraft: ProductStudioDraft = {
      ...sampleInitialDraft,
      title: '', // Missing title blocks readiness
    };

    renderAutosaveHarness({
      initialDraft: incompleteDraft,
    });

    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');
    // Disabled due to blocking fields
    expect((submitBtn as HTMLButtonElement).disabled).toBe(true);
    expect(submitBtn.getAttribute('title')).toContain('Заполните');

    // Mutate a non-blocking field (description) while title remains empty
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-description'));
    });

    // Still disabled
    expect((submitBtn as HTMLButtonElement).disabled).toBe(true);
  });

  it('7. Below-threshold in create mode: empty title keeps edits local, zero create POSTs after 800ms', async () => {
    const { mockCreateProduct } = renderCreateAutosaveHarness();

    // Mutate a non-identity field (description) while title remains empty
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-description'));
    });

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Advance past 800ms debounce
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    // Remains local, zero create POSTs
    expect(mockCreateProduct).not.toHaveBeenCalled();
    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
  });

  it('8. Reaching threshold triggers debounced create: typing valid title triggers single create POST after 800ms', async () => {
    const { mockCreateProduct } = renderCreateAutosaveHarness();

    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title'));
    });

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Advance 500ms (less than 800ms)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Advance remaining 300ms (total 800ms)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });

    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Худи Обновленное',
      }),
      expect.anything()
    );

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
  });

  it('9. Rapid typing before create debounces correctly: only ONE initial create POST with latest state', async () => {
    const { mockCreateProduct } = renderCreateAutosaveHarness();

    const mutateCustomBtn = screen.getByTestId('btn-mutate-title-custom');

    // Rapid keystroke 1
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Х');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Rapid keystroke 2
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Ху');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Rapid keystroke 3
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Худи Созданное');
      fireEvent.click(mutateCustomBtn);
    });

    // Advance 800ms after final keystroke
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    // Exactly 1 create call with final title
    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Худи Созданное',
      }),
      expect.anything()
    );
  });

  it('10. User edits field while initial create is in-flight: intermediate commit attaches ID, queued edit save executes via PATCH', async () => {
    let resolveCreate1!: (val: any) => void;
    const createPromise1 = new Promise((resolve) => {
      resolveCreate1 = resolve;
    });

    const mockCreateProduct = vi.fn().mockReturnValue(createPromise1);
    const mockSaveProduct = vi.fn().mockImplementation((id, patch) =>
      Promise.resolve({
        ...sampleCanonicalProduct,
        ...patch,
        id,
      })
    );

    renderCreateAutosaveHarness({
      createProductFn: mockCreateProduct,
      saveProductFn: mockSaveProduct,
    });

    const mutateCustomBtn = screen.getByTestId('btn-mutate-title-custom');

    // 1. Initial edit to trigger create
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Create 1');
      fireEvent.click(mutateCustomBtn);
    });

    // Advance 800ms to fire create
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Create 1',
      }),
      expect.anything()
    );
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохраняем…');

    // 2. While create 1 is in-flight, user makes newer edit 'Edit 2'
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Edit 2');
      fireEvent.click(mutateCustomBtn);
    });

    expect(screen.getByTestId('test-title').textContent).toBe('Edit 2');

    // 3. Resolve create 1 (server creates product with ID 'prod-created-101')
    await act(async () => {
      resolveCreate1({
        ...sampleCanonicalProduct,
        id: 'prod-created-101',
        title: 'Create 1',
      });
      await vi.advanceTimersByTimeAsync(0);
    });

    // Active draft MUST retain 'Edit 2' (not wiped out by create response)
    expect(screen.getByTestId('test-title').textContent).toBe('Edit 2');

    // 4. Queued save fires immediately via PATCH for 'Edit 2'
    expect(mockSaveProduct).toHaveBeenCalledTimes(1);
    expect(mockSaveProduct).toHaveBeenCalledWith(
      'prod-created-101',
      expect.objectContaining({
        title: 'Edit 2',
      })
    );

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
    expect(screen.getByTestId('test-title').textContent).toBe('Edit 2');
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
  });

  it('11. Create failure displays error in header and retry performs create', async () => {
    let shouldFail = true;
    const mockCreateProduct = vi.fn().mockImplementation((payload) => {
      if (shouldFail) {
        return Promise.reject(new Error('Network connection dropped on create'));
      }
      return Promise.resolve({
        ...sampleCanonicalProduct,
        ...payload,
        id: 'prod-created-101',
      });
    });

    renderCreateAutosaveHarness({
      createProductFn: mockCreateProduct,
    });

    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title'));
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    expect(mockCreateProduct).toHaveBeenCalledTimes(1);

    // Error state shown in header
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Ошибка сохранения');
    expect(screen.getByTestId('studio-retry-save-btn')).toBeTruthy();
    expect(screen.getByTestId('test-title').textContent).toBe('Худи Обновленное');

    // Retry
    shouldFail = false;
    await act(async () => {
      fireEvent.click(screen.getByTestId('studio-retry-save-btn'));
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(mockCreateProduct).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
  });

  it('12. Submit to moderation before initial create autosave flushes create then submits moderation', async () => {
    const fullDraft: ProductStudioDraft = {
      ...sampleInitialDraft,
      id: undefined, // Create mode, not yet persisted
    };

    const { mockCreateProduct, mockSubmitModeration } = renderCreateAutosaveHarness({
      initialDraft: fullDraft,
    });

    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');
    expect((submitBtn as HTMLButtonElement).disabled).toBe(false);

    // User clicks submit before debounce fires
    await act(async () => {
      fireEvent.click(submitBtn);
      await vi.advanceTimersByTimeAsync(0);
    });

    // Create was flushed
    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    // Moderation was submitted with newly created product ID
    expect(mockSubmitModeration).toHaveBeenCalledTimes(1);
    expect(mockSubmitModeration).toHaveBeenCalledWith('prod-created-101');
  });

  it('13. Tab or section switch in create mode: flushes create if threshold met, does not flush if below threshold', async () => {
    const { mockCreateProduct } = renderCreateAutosaveHarness();

    // Case A: Below threshold (no title) -> mutate description -> switch view
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-description'));
    });
    act(() => {
      fireEvent.click(screen.getByTestId('btn-switch-view-form'));
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Case B: Reaches threshold (title entered) -> switch section
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title'));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId('btn-switch-section-media'));
      await vi.advanceTimersByTimeAsync(0);
    });

    // Flushed immediately on section switch!
    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Худи Обновленное',
      }),
      expect.anything()
    );
  });

  // 14. A. TITLE ONLY: title = "Привет", category = none -> waits > 800ms -> 0 creates, no error, draft remains local
  it('14. A. TITLE ONLY: new draft with title="Привет" and no category keeps draft local, 0 creates, no error', async () => {
    const rawNewDraft: ProductStudioDraft = {
      title: '',
      description: '',
      categoryId: '',
      priceCents: 450000,
      variants: [],
      images: [],
    };

    const { mockCreateProduct } = renderCreateAutosaveHarness({
      initialDraft: rawNewDraft,
      initialCategorySchema: null,
    });

    // Owner types "Привет"
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title-privet'));
    });

    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
    expect(screen.getByTestId('test-title').textContent).toBe('Привет');
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Advance 1000ms (more than 800ms debounce)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    // Requirements:
    // create calls = 0
    expect(mockCreateProduct).not.toHaveBeenCalled();
    // save error = NONE
    expect(screen.queryByTestId('studio-save-error-toast')).toBeNull();
    expect(screen.getByTestId('studio-save-status').textContent).not.toContain('Ошибка сохранения');
    // toast = NONE
    expect(screen.queryByRole('alert')).toBeNull();
    // draft remains local
    expect(screen.getByTestId('test-title').textContent).toBe('Привет');
    expect(screen.getByTestId('test-is-dirty').textContent).toBe('DIRTY');
    // Header shows quiet unsaved state "Есть изменения"
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Есть изменения');
  });

  // 15. B. CATEGORY SELECTED BUT SCHEMA LOADING: draft with categoryId but schema pending -> 0 creates, no error
  it('15. B. CATEGORY SELECTED BUT SCHEMA LOADING: category chosen but schema still loading -> 0 creates, no error', async () => {
    const draftWithCategoryNoSchema: ProductStudioDraft = {
      title: 'Привет',
      description: '',
      categoryId: 'cat-loading',
      priceCents: 450000,
      variants: [],
      images: [],
    };

    const { mockCreateProduct } = renderCreateAutosaveHarness({
      initialDraft: draftWithCategoryNoSchema,
      initialCategorySchema: null, // Schema not yet loaded
    });

    // Mutate description to dirty it
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-description'));
    });

    // Advance past 800ms
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    // Expected: create calls = 0, save error = NONE
    expect(mockCreateProduct).not.toHaveBeenCalled();
    expect(screen.queryByTestId('studio-save-error-toast')).toBeNull();
    expect(screen.getByTestId('studio-save-status').textContent).not.toContain('Ошибка сохранения');
  });

  // 16. C. CATEGORY + SCHEMA READY: draft with title, categoryId, and matching categorySchema -> after debounce -> exactly 1 create -> status Saved
  it('16. C. CATEGORY + SCHEMA READY: draft with title, categoryId, and schema ready -> after debounce -> 1 create -> Saved', async () => {
    const draftWithCategoryAndSchema: ProductStudioDraft = {
      title: '',
      description: '',
      categoryId: 'cat-hoodies',
      priceCents: 450000,
      variants: [],
      images: [],
    };

    const { mockCreateProduct } = renderCreateAutosaveHarness({
      initialDraft: draftWithCategoryAndSchema,
      initialCategorySchema: mockSchema,
    });

    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title-privet'));
    });

    expect(mockCreateProduct).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Привет',
        categoryId: 'cat-hoodies',
      }),
      expect.anything()
    );
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
  });

  // 17. D. RAW BACKEND DTO WOULD ACCEPT BUT STUDIO CANNOT: proves threshold follows canonical orchestration
  it('17. D. RAW BACKEND DTO WOULD ACCEPT BUT STUDIO CANNOT: Product Studio threshold follows canonical orchestration', async () => {
    // Backend CreateProductRequest accepts { title: 'Raw DTO', priceCents: 10000 } without category.
    // But Product Studio cannot orchestrate without category & schema.
    const rawDtoDraft: ProductStudioDraft = {
      title: 'Valid Raw DTO Title',
      description: 'Raw description',
      categoryId: '', // No category
      priceCents: 10000,
      variants: [],
      images: [],
    };

    const { mockCreateProduct } = renderCreateAutosaveHarness({
      initialDraft: rawDtoDraft,
      initialCategorySchema: null,
    });

    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-description'));
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    // Zero create requests sent, despite raw backend DTO being structurally valid!
    expect(mockCreateProduct).not.toHaveBeenCalled();
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Есть изменения');
    expect(screen.queryByTestId('studio-save-error-toast')).toBeNull();
  });

  // 18. E. CATEGORY SELECTION OWNER CASE: title entered -> no network -> category "Худи" selected -> one create -> Saved
  it('18. E. CATEGORY SELECTION OWNER CASE: title "Привет" entered -> 0 network -> select "Худи" -> 1 create -> Saved', async () => {
    const freshDraft: ProductStudioDraft = {
      title: '',
      description: '',
      categoryId: '',
      priceCents: 450000,
      variants: [],
      images: [],
    };

    const { mockCreateProduct } = renderCreateAutosaveHarness({
      initialDraft: freshDraft,
      initialCategorySchema: null,
    });

    // Step 1: Owner enters title = "Привет"
    act(() => {
      fireEvent.click(screen.getByTestId('btn-mutate-title-privet'));
    });

    // Wait > 800ms
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    // NO network, NO error, quiet unsaved state
    expect(mockCreateProduct).not.toHaveBeenCalled();
    expect(screen.queryByTestId('studio-save-error-toast')).toBeNull();
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Есть изменения');

    // Step 2: Owner selects category "Худи"
    await act(async () => {
      fireEvent.click(screen.getByTestId('btn-select-category-hoodies'));
      // getSellerCategorySchema resolves in microtask
      await vi.advanceTimersByTimeAsync(0);
    });

    // 800ms debounce starts now
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Advance 800ms
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    // Exactly 1 create POST with both title and category!
    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Привет',
        categoryId: 'cat-hoodies',
      }),
      expect.anything()
    );

    // Transitions to Saved
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
    expect(screen.getByTestId('test-is-dirty').textContent).toBe('CLEAN');
  });

  // 19. F. RAPID CATEGORY/TITLE CHANGES: rapid typing and category changes debounce into single create
  it('19. F. RAPID CATEGORY/TITLE CHANGES: rapid typing and category changes debounce into single create', async () => {
    const freshDraft: ProductStudioDraft = {
      title: '',
      description: '',
      categoryId: '',
      priceCents: 450000,
      variants: [],
      images: [],
    };

    const { mockCreateProduct } = renderCreateAutosaveHarness({
      initialDraft: freshDraft,
      initialCategorySchema: null,
    });

    const mutateCustomBtn = screen.getByTestId('btn-mutate-title-custom');

    // Keystroke 1: "П"
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'П');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Keystroke 2: "При"
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'При');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Select category "Худи"
    await act(async () => {
      fireEvent.click(screen.getByTestId('btn-select-category-hoodies'));
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Keystroke 3: "Привет"
    act(() => {
      mutateCustomBtn.setAttribute('data-val', 'Привет');
      fireEvent.click(mutateCustomBtn);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mockCreateProduct).not.toHaveBeenCalled();

    // Let 800ms debounce settle
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    // Exactly 1 create POST with final settled state!
    expect(mockCreateProduct).toHaveBeenCalledTimes(1);
    expect(mockCreateProduct).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Привет',
        categoryId: 'cat-hoodies',
      }),
      expect.anything()
    );
    expect(screen.getByTestId('studio-save-status').textContent).toContain('Сохранено');
  });

  // 20. Unit: isProductStudioCreateAutosaveEligible boundary validation
  it('20. Unit: isProductStudioCreateAutosaveEligible boundary validation rules', () => {
    const validDraft: ProductStudioDraft = {
      title: 'Худи',
      description: 'Теплое базовое худи',
      categoryId: 'cat-hoodies',
      priceCents: 1000,
      variants: [],
      images: [],
    };

    // Valid case
    expect(isProductStudioCreateAutosaveEligible(validDraft, { categorySchema: mockSchema })).toBe(true);

    // Missing / empty title
    expect(isProductStudioCreateAutosaveEligible({ ...validDraft, title: '' }, { categorySchema: mockSchema })).toBe(false);
    expect(isProductStudioCreateAutosaveEligible({ ...validDraft, title: '   ' }, { categorySchema: mockSchema })).toBe(false);

    // Missing / empty categoryId
    expect(isProductStudioCreateAutosaveEligible({ ...validDraft, categoryId: '' }, { categorySchema: mockSchema })).toBe(false);
    expect(isProductStudioCreateAutosaveEligible({ ...validDraft, categoryId: undefined }, { categorySchema: mockSchema })).toBe(false);

    // Missing categorySchema
    expect(isProductStudioCreateAutosaveEligible(validDraft, null)).toBe(false);
    expect(isProductStudioCreateAutosaveEligible(validDraft, { categorySchema: null })).toBe(false);

    // Mismatched categorySchema
    expect(isProductStudioCreateAutosaveEligible(validDraft, { categorySchema: { ...mockSchema, id: 'cat-other' } })).toBe(false);

    // Negative price
    expect(isProductStudioCreateAutosaveEligible({ ...validDraft, priceCents: -1 }, { categorySchema: mockSchema })).toBe(false);

    // Negative variant price
    expect(isProductStudioCreateAutosaveEligible({
      ...validDraft,
      variants: [{ id: 'v1', priceCents: -50 } as any],
    }, { categorySchema: mockSchema })).toBe(false);

    // Invalid media (mediaMode = LEGACY_MIXED)
    expect(isProductStudioCreateAutosaveEligible({
      ...validDraft,
      mediaMode: 'LEGACY_MIXED',
    }, { categorySchema: mockSchema })).toBe(false);
  });
});
