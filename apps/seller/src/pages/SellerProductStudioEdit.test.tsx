/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, act, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import SellerProductStudioEdit from './SellerProductStudioEdit';
import * as sellerApi from '@zamk/api-client/src/seller';
import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerSizeValue,
} from '@zamk/api-client';

vi.mock('@zamk/api-client/src/seller', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@zamk/api-client/src/seller')>();
  return {
    ...actual,
    getSellerProduct: vi.fn(),
    getSellerCategorySchema: vi.fn(),
    getSellerColors: vi.fn(),
    getSellerSizeValues: vi.fn(),
    getSellerDictionaryValues: vi.fn(),
    updateSellerProduct: vi.fn(),
    createSellerProduct: vi.fn(),
    submitSellerProductModeration: vi.fn(),
  };
});

vi.mock('@zamk/api-client', async (importOriginal) => {
  const actual = await importOriginal<any>();
  const seller = await import('@zamk/api-client/src/seller');
  return {
    ...actual,
    ...seller,
  };
});

describe('SellerProductStudioEdit Page', () => {
  const mockColors: SellerColor[] = [
    { id: 'col-red', code: 'RED', nameRu: 'Красный', hex: '#FF0000' },
    { id: 'col-white', code: 'WHITE', nameRu: 'Белый', hex: '#FFFFFF' },
  ];

  const mockSchema: SellerCategorySchema = {
    id: 'cat-hoodies',
    name: 'Худи',
    slug: 'hoodies',
    dimensionType: 'COLOR_AND_SIZE',
    sizeChartRequired: true,
    allowedSizeSystems: [
      { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
    ],
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
    sizeChartFields: [
      { code: 'CHEST', name: 'Обхват груди', unit: 'cm', isRequired: true, sortOrder: 1 },
    ],
  };

  const mockSizeValues: SellerSizeValue[] = [
    { id: 'sz-l', sizeSystemId: 'sys-int', value: 'L', sortOrder: 1 },
    { id: 'sz-xl', sizeSystemId: 'sys-int', value: 'XL', sortOrder: 2 },
  ];

  const sampleProduct: SellerProduct = {
    id: 'prod-hoodie-1',
    title: 'худи',
    description: 'Комфортное худи',
    categoryId: 'cat-hoodies',
    categoryName: 'Худи',
    brandId: 'brand-1',
    brandName: 'Dev Brand',
    sellerId: 'seller-1',
    createdAt: '2026-08-25T17:00:00Z',
    status: 'pending_moderation',
    priceCents: 122200,
    oldPriceCents: 150000,
    currency: 'RUB',
    slug: 'hoodie',
    images: [
      {
        id: 'img-1',
        imageUrl: 'https://cdn.example.com/hoodie-red.jpg',
        sortOrder: 0,
        colorId: 'col-red',
        isMain: true,
      },
    ],
    variants: [
      {
        id: 'var-red-l',
        productId: 'prod-hoodie-1',
        colorId: 'col-red',
        colorName: 'Красный',
        colorHex: '#FF0000',
        sizeValueId: 'sz-l',
        size: 'L',
        sellerSku: 'SKU-RED-L',
        barcode: 'ZMK-001',
        priceCents: 122200,
        isActive: true,
      },
      {
        id: 'var-white-xl',
        productId: 'prod-hoodie-1',
        colorId: 'col-white',
        colorName: 'Белый',
        colorHex: '#FFFFFF',
        sizeValueId: 'sz-xl',
        size: 'XL',
        sellerSku: 'SKU-WHT-XL',
        barcode: 'ZMK-002',
        priceCents: 122200,
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
    sizeChart: {
      id: 'sc-1',
      productId: 'prod-hoodie-1',
      categoryId: 'cat-hoodies',
      rows: [
        {
          sizeChartId: 'sc-1',
          sizeValueId: 'sz-l',
          sizeValueName: 'L',
          measurements: { CHEST: 120 },
        },
      ],
    },
    attributes: [
      {
        id: 'attr-val-1',
        productId: 'prod-hoodie-1',
        attributeDefinitionId: 'attr-season',
        enumValueId: 'dict-val-winter',
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(sellerApi.getSellerProduct).mockResolvedValue(sampleProduct);
    vi.mocked(sellerApi.getSellerCategorySchema).mockResolvedValue(mockSchema);
    vi.mocked(sellerApi.getSellerColors).mockResolvedValue(mockColors);
    vi.mocked(sellerApi.getSellerSizeValues).mockResolvedValue(mockSizeValues);
    vi.mocked(sellerApi.getSellerDictionaryValues).mockResolvedValue([
      { id: 'dict-val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
    ]);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('1. shows loading state initially before data arrives', async () => {
    vi.mocked(sellerApi.getSellerProduct).mockReturnValue(new Promise(() => {}));

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByTestId('studio-edit-loading')).toBeTruthy();
    expect(screen.getByText('Загрузка данных товара...')).toBeTruthy();
  });

  it('2. handles 404 error with "Товар не найден" and "Вернуться в ассортимент"', async () => {
    vi.mocked(sellerApi.getSellerProduct).mockRejectedValue({
      status: 404,
      code: 'not_found',
      message: 'Product not found',
    });

    render(
      <MemoryRouter initialEntries={['/products/prod-missing/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-404')).toBeTruthy();
    });

    expect(screen.getByText('Товар не найден')).toBeTruthy();
    const backBtn = screen.getByTestId('studio-edit-back-to-products');
    expect(backBtn.getAttribute('href')).toBe('/products');
    expect(backBtn.textContent).toContain('Вернуться в ассортимент');
  });

  it('3. handles 403 error with "Нет доступа к этому товару" and "Вернуться в ассортимент"', async () => {
    vi.mocked(sellerApi.getSellerProduct).mockRejectedValue({
      status: 403,
      code: 'forbidden',
      message: 'Forbidden',
    });

    render(
      <MemoryRouter initialEntries={['/products/prod-forbidden/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-403')).toBeTruthy();
    });

    expect(screen.getByText('Нет доступа к этому товару')).toBeTruthy();
    const backBtn = screen.getByTestId('studio-edit-back-to-products');
    expect(backBtn.getAttribute('href')).toBe('/products');
  });

  it('4. handles 500 / network error with "Не удалось загрузить товар" and "Повторить" button', async () => {
    vi.mocked(sellerApi.getSellerProduct).mockRejectedValueOnce(new Error('Network offline'));

    render(
      <MemoryRouter initialEntries={['/products/prod-500/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-500')).toBeTruthy();
    });

    expect(screen.getByRole('heading', { name: 'Не удалось загрузить товар' })).toBeTruthy();
    const retryBtn = screen.getByTestId('studio-edit-retry');
    expect(retryBtn).toBeTruthy();

    // Clicking retry refetches
    vi.mocked(sellerApi.getSellerProduct).mockResolvedValueOnce(sampleProduct);
    fireEvent.click(retryBtn);

    await waitFor(() => {
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });
  });

  it('5. successfully hydrates existing product into ProductStudio in Edit mode', async () => {
    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });

    // Header checks
    expect(screen.getByTestId('studio-product-title').textContent).toBe('худи');
    expect(screen.getByTestId('studio-entry-badge').textContent).toBe('На модерации');

    // Save and Moderation remain disabled
    const saveBtn = screen.getByText('Сохранить').closest('button');
    expect((saveBtn as HTMLButtonElement).disabled).toBe(true);
    const modBtn = screen.getByText('Отправить на модерацию').closest('button');
    expect((modBtn as HTMLButtonElement).disabled).toBe(true);

    // No mutation endpoints called on mount
    expect(sellerApi.createSellerProduct).not.toHaveBeenCalled();
    expect(sellerApi.updateSellerProduct).not.toHaveBeenCalled();
    expect(sellerApi.submitSellerProductModeration).not.toHaveBeenCalled();
  });

  it('6. PROVES NO POLLING: product is fetched once on mount and timer advance does NOT refetch', async () => {
    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });

    expect(sellerApi.getSellerProduct).toHaveBeenCalledTimes(1);

    vi.useFakeTimers();

    // Advance timers by 4 seconds (legacy polling interval)
    await act(async () => {
      vi.advanceTimersByTime(4000);
    });
    expect(sellerApi.getSellerProduct).toHaveBeenCalledTimes(1);

    // Advance timers by another 8 seconds
    await act(async () => {
      vi.advanceTimersByTime(8000);
    });
    expect(sellerApi.getSellerProduct).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('7. LOCAL EDIT SURVIVES TIME: local changes are never overwritten by background timer', async () => {
    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-product-title')).toBeTruthy();
    });

    expect(screen.getByTestId('studio-product-title').textContent).toBe('худи');

    // Switch to Form mode to edit title
    const formTab = screen.getByTestId('studio-view-toggle-form');
    fireEvent.click(formTab);

    const titleInput = screen.getByTestId('form-product-title-input') as HTMLInputElement;
    fireEvent.change(titleInput, { target: { value: 'худи оверсайз черное' } });

    expect(titleInput.value).toBe('худи оверсайз черное');
    expect(screen.getByTestId('studio-product-title').textContent).toBe('худи оверсайз черное');

    vi.useFakeTimers();

    // Advance timers > 8 seconds
    await act(async () => {
      vi.advanceTimersByTime(12000);
    });

    // Local title survives untouched
    expect(screen.getByTestId('studio-product-title').textContent).toBe('худи оверсайз черное');
    expect(sellerApi.getSellerProduct).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('8. REMOTE MEDIA LIFECYCLE: switching modes never revokes remote URL', async () => {
    const revokeSpy = vi.fn();
    globalThis.URL.revokeObjectURL = revokeSpy;

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });

    // Visual -> Form
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    expect(screen.getByTestId('studio-form-workspace')).toBeTruthy();

    // Form -> Visual
    fireEvent.click(screen.getByTestId('studio-view-toggle-visual'));
    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();

    // URL.revokeObjectURL was NEVER called with the remote image URL
    expect(revokeSpy).not.toHaveBeenCalledWith('https://cdn.example.com/hoodie-red.jpg');
  });

  it('9. EDIT SIZE PICKER PROOF: INT default + existing RU IDs => active size system is RU and shows RU values', async () => {
    const dualSchema: SellerCategorySchema = {
      ...mockSchema,
      allowedSizeSystems: [
        { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
        { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: false },
      ],
    };

    const ruProduct: SellerProduct = {
      ...sampleProduct,
      variants: [
        {
          ...sampleProduct.variants![0],
          sizeValueId: 'sz-48',
          size: '48',
        },
      ],
    };

    vi.mocked(sellerApi.getSellerProduct).mockResolvedValue(ruProduct);
    vi.mocked(sellerApi.getSellerCategorySchema).mockResolvedValue(dualSchema);
    vi.mocked(sellerApi.getSellerSizeValues).mockImplementation(async (sysId: string) => {
      if (sysId === 'sys-int') {
        return [
          { id: 'sz-l', sizeSystemId: 'sys-int', value: 'L', sortOrder: 1 },
          { id: 'sz-xl', sizeSystemId: 'sys-int', value: 'XL', sortOrder: 2 },
        ];
      }
      if (sysId === 'sys-ru') {
        return [
          { id: 'sz-46', sizeSystemId: 'sys-ru', value: '46', sortOrder: 1 },
          { id: 'sz-48', sizeSystemId: 'sys-ru', value: '48', sortOrder: 2 },
        ];
      }
      return [];
    });

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });

    // Open size popover via size-manage-btn
    const sizeManageBtn = screen.getByTestId('size-manage-btn');
    fireEvent.click(sizeManageBtn);

    // Popover is open
    const popover = screen.getByTestId('size-popover');
    expect(popover).toBeTruthy();

    // Verify RU is active and displays 46, 48 values
    expect(within(popover).getByText('46')).toBeTruthy();
    expect(within(popover).getByText('48')).toBeTruthy();
    // Must NOT have switched to default INT merely because INT is default
    expect(within(popover).queryByText('XL')).toBeNull();
  });

  it('10. AMBIGUOUS SYSTEM TEST: fails with explicit ambiguity error when multiple systems contain existing IDs', async () => {
    const dualSchema: SellerCategorySchema = {
      ...mockSchema,
      allowedSizeSystems: [
        { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
        { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: false },
      ],
    };

    const universalProduct: SellerProduct = {
      ...sampleProduct,
      variants: [
        {
          ...sampleProduct.variants![0],
          sizeValueId: 'sz-universal',
          size: 'ONE-SIZE',
        },
      ],
    };

    vi.mocked(sellerApi.getSellerProduct).mockResolvedValue(universalProduct);
    vi.mocked(sellerApi.getSellerCategorySchema).mockResolvedValue(dualSchema);
    vi.mocked(sellerApi.getSellerSizeValues).mockImplementation(async (sysId: string) => {
      return [
        { id: 'sz-universal', sizeSystemId: sysId, value: 'ONE-SIZE', sortOrder: 1 },
      ];
    });

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-500')).toBeTruthy();
    });

    expect(screen.getByText('Размерная система товара определяется неоднозначно')).toBeTruthy();
    expect(screen.queryByTestId('product-studio-root')).toBeNull();
  });

  it('11. SIZE REFERENCE FETCH FAILURE: hydration fails safely when allowed-system size fetch fails', async () => {
    const dualSchema: SellerCategorySchema = {
      ...mockSchema,
      allowedSizeSystems: [
        { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
        { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: false },
      ],
    };

    vi.mocked(sellerApi.getSellerCategorySchema).mockResolvedValue(dualSchema);
    vi.mocked(sellerApi.getSellerSizeValues).mockImplementation(async (sysId: string) => {
      if (sysId === 'sys-ru') {
        throw new Error('Size service unavailable');
      }
      return mockSizeValues;
    });

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-500')).toBeTruthy();
    });

    expect(screen.getByText(/Data integrity error: failed to fetch canonical size values/)).toBeTruthy();
    expect(screen.queryByTestId('product-studio-root')).toBeNull();
  });

  it('12. DICTIONARY FETCH FAILURE: fails with explicit loading error if canonical dictionary fails', async () => {
    vi.mocked(sellerApi.getSellerDictionaryValues).mockRejectedValue(new Error('Dictionary 503 error'));

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-500')).toBeTruthy();
    });

    expect(screen.getByText(/Data integrity error: failed to fetch canonical dictionary/)).toBeTruthy();
    expect(screen.queryByTestId('product-studio-root')).toBeNull();
  });

  it('13. UNKNOWN ENUM VALUE: fails with explicit integrity error when enumValueId is not found', async () => {
    const productUnknownEnum: SellerProduct = {
      ...sampleProduct,
      attributes: [
        {
          id: 'attr-val-1',
          productId: 'prod-hoodie-1',
          attributeDefinitionId: 'attr-season',
          enumValueId: 'dict-val-nonexistent',
        },
      ],
    };
    vi.mocked(sellerApi.getSellerProduct).mockResolvedValue(productUnknownEnum);

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-500')).toBeTruthy();
    });

    expect(screen.getByText(/references unknown enumValueId "dict-val-nonexistent"/)).toBeTruthy();
    expect(screen.queryByTestId('product-studio-root')).toBeNull();
  });

  it('14. UNKNOWN ATTRIBUTE DEFINITION: fails with explicit integrity error when definition not in schema', async () => {
    const productUnknownDef: SellerProduct = {
      ...sampleProduct,
      attributes: [
        {
          id: 'attr-val-1',
          productId: 'prod-hoodie-1',
          attributeDefinitionId: 'attr-deleted-from-category',
          textValue: 'Winter Edition',
        },
      ],
    };
    vi.mocked(sellerApi.getSellerProduct).mockResolvedValue(productUnknownDef);

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('studio-edit-error-500')).toBeTruthy();
    });

    expect(screen.getByText(/references attributeDefinitionId "attr-deleted-from-category" not found in category schema/)).toBeTruthy();
    expect(screen.queryByTestId('product-studio-root')).toBeNull();
  });

  it('15. Form characteristics subsection displays resolved category name and never "undefined" when schema has no name property', async () => {
    // Mimic real production API response where category schema endpoint omits name/id
    const prodSchemaWithoutName: SellerCategorySchema = {
      ...mockSchema,
      name: undefined as any,
    };
    vi.mocked(sellerApi.getSellerCategorySchema).mockResolvedValue(prodSchemaWithoutName);
    const prodWithoutAttrs: SellerProduct = {
      ...sampleProduct,
      attributes: [],
    };
    vi.mocked(sellerApi.getSellerProduct).mockResolvedValue(prodWithoutAttrs);

    render(
      <MemoryRouter initialEntries={['/products/prod-hoodie-1/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<SellerProductStudioEdit />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    });

    // Switch view mode to Form
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));

    // Switch section to Characteristics
    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    expect(
      screen.getByText('Категория: Худи. Заполнено 0 из 1 обязательных.')
    ).toBeTruthy();

    // Ensure "undefined" is never present in text content
    expect(screen.queryByText(/undefined/)).toBeNull();
  });
});
