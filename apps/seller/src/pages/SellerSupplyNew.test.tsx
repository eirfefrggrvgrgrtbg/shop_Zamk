/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { SellerSupplyNew } from './SellerSupplyNew';
import * as sellerApi from '@zamk/api-client/src/seller';
import type { SellerProduct } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerProducts: vi.fn(),
  createSellerSupply: vi.fn(),
  getSellerCategories: vi.fn(),
}));

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<any>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('SellerSupplyNew Component (SUP.2B3.1 - Category Name & Filter Chips)', () => {
  const CLOTHING_CAT_ID = 'c741aa40-4f5f-4b58-8581-5cfae5e77c16';
  const ELECTRONICS_CAT_ID = 'e1111111-2222-3333-4444-555555555555';
  const ACCESSORIES_CAT_ID = 'a9999999-8888-7777-6666-555555555555';
  const UNKNOWN_UUID_CAT_ID = '66666666-6666-4666-8666-666666666666';

  const mockCategories: sellerApi.SellerCategory[] = [
    { id: CLOTHING_CAT_ID, name: 'Одежда', slug: 'clothing', sortOrder: 1 },
    { id: ELECTRONICS_CAT_ID, name: 'Электроника', slug: 'electronics', sortOrder: 2 },
    { id: ACCESSORIES_CAT_ID, name: 'Аксессуары', slug: 'accessories', sortOrder: 3 },
  ];

  const mockSuitProduct: SellerProduct = {
    id: 'prod-suit',
    sellerId: 'seller-1',
    title: 'Костюм классический',
    slug: 'costume-classic',
    description: 'Костюм двойка',
    categoryId: CLOTHING_CAT_ID,
    priceCents: 1500000,
    status: 'approved',
    images: [{ id: 'img-1', url: 'https://example.com/suit.jpg', isMain: true, sortOrder: 0 }],
    variants: [
      {
        id: 'var-suit-gry-l',
        productId: 'prod-suit',
        sku: 'SUIT-GRY-L',
        sellerSku: 'SKU-SUIT-GRY-L',
        barcode: '460000001001',
        color: 'Серый',
        size: 'L',
        priceCents: 1500000,
        isActive: true,
      },
      {
        id: 'var-suit-gry-xl',
        productId: 'prod-suit',
        sku: 'SUIT-GRY-XL',
        sellerSku: 'SKU-SUIT-GRY-XL',
        barcode: '460000001002',
        color: 'Серый',
        size: 'XL',
        priceCents: 1500000,
        isActive: true,
      },
      {
        id: 'var-suit-pnk-l',
        productId: 'prod-suit',
        sku: 'SUIT-PNK-L',
        sellerSku: 'SKU-SUIT-PNK-L',
        barcode: '460000001003',
        color: 'Розовый',
        size: 'L',
        priceCents: 1500000,
        isActive: true,
      },
    ],
    createdAt: '2026-01-01',
  };

  const mockPantsProduct: SellerProduct = {
    id: 'prod-pants',
    sellerId: 'seller-1',
    title: 'Брюки хлопковые',
    slug: 'pants-cotton',
    description: 'Легкие брюки',
    categoryId: CLOTHING_CAT_ID,
    priceCents: 500000,
    status: 'approved',
    images: [],
    variants: [
      {
        id: 'var-pants-blu-m',
        productId: 'prod-pants',
        sku: 'PANTS-BLU-M',
        sellerSku: 'SKU-PANTS-BLU-M',
        barcode: '460000002001',
        color: 'Синий',
        size: 'M',
        priceCents: 500000,
        isActive: true,
      },
      {
        id: 'var-pants-blu-l',
        productId: 'prod-pants',
        sku: 'PANTS-BLU-L',
        sellerSku: 'SKU-PANTS-BLU-L',
        barcode: '460000002002',
        color: 'Синий',
        size: 'L',
        priceCents: 500000,
        isActive: true,
      },
    ],
    createdAt: '2026-01-01',
  };

  const mockPhoneProduct: SellerProduct = {
    id: 'prod-phone',
    sellerId: 'seller-1',
    title: 'Смартфон Pro',
    slug: 'smartphone-pro',
    description: 'Флагманский телефон',
    categoryId: ELECTRONICS_CAT_ID,
    priceCents: 8000000,
    status: 'approved',
    images: [],
    variants: [
      {
        id: 'var-phn-128',
        productId: 'prod-phone',
        sku: 'PHN-128',
        sellerSku: 'SKU-PHN-128',
        barcode: '460000003001',
        optionValues: { 'Память': '128 GB' },
        priceCents: 8000000,
        isActive: true,
      },
      {
        id: 'var-phn-256',
        productId: 'prod-phone',
        sku: 'PHN-256',
        sellerSku: 'SKU-PHN-256',
        barcode: '460000003002',
        optionValues: { 'Память': '256 GB' },
        priceCents: 9000000,
        isActive: true,
      },
    ],
    createdAt: '2026-01-01',
  };

  const mockBagProduct: SellerProduct = {
    id: 'prod-bag',
    sellerId: 'seller-1',
    title: 'Сумка тоут',
    slug: 'bag-tote',
    description: 'Кожаная сумка',
    categoryId: ACCESSORIES_CAT_ID,
    priceCents: 600000,
    status: 'approved',
    images: [],
    variants: [
      {
        id: 'var-bag-1',
        productId: 'prod-bag',
        sku: 'BAG-ONE',
        sellerSku: 'SKU-BAG-ONE',
        barcode: '460000004001',
        priceCents: 600000,
        isActive: true,
      },
    ],
    createdAt: '2026-01-01',
  };

  const mockUnknownCatProduct: SellerProduct = {
    id: 'prod-unknown',
    sellerId: 'seller-1',
    title: 'Неизвестный товар',
    slug: 'unknown-item',
    description: 'Товар с неразрешенной категорией',
    categoryId: UNKNOWN_UUID_CAT_ID,
    priceCents: 100000,
    status: 'approved',
    images: [],
    variants: [
      {
        id: 'var-unk-1',
        productId: 'prod-unknown',
        sku: 'UNK-ONE',
        sellerSku: 'SKU-UNK-ONE',
        barcode: '460000005001',
        priceCents: 100000,
        isActive: true,
      },
    ],
    createdAt: '2026-01-01',
  };

  const mockCatalog = [mockSuitProduct, mockPantsProduct, mockPhoneProduct, mockBagProduct, mockUnknownCatProduct];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(sellerApi.getSellerCategories).mockResolvedValue(mockCategories);
  });

  const getFilterButton = () => screen.getByRole('button', { name: /^Фильтры/i });

  // --------------------------------------------------------------------------
  // TESTS — CATEGORY NAME & FILTER DISPLAY (A, B, C, D)
  // --------------------------------------------------------------------------

  it('A & B & C: category select options NEVER show raw UUID, value is canonical ID, label is name, and unmapped UUID is omitted', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Open filter popover
    fireEvent.click(getFilterButton());

    const catSelect = screen.getByLabelText('Категория') as HTMLSelectElement;
    const options = Array.from(catSelect.options);

    // Option texts should be human-readable, none should be UUID
    const optionTexts = options.map(o => o.text);
    const optionValues = options.map(o => o.value);

    // Default option
    expect(optionTexts).toContain('Все категории');

    // Mapped categories
    expect(optionTexts).toContain('Одежда');
    expect(optionTexts).toContain('Электроника');
    expect(optionTexts).toContain('Аксессуары');

    // Requirement A: No option text is a raw UUID
    options.forEach(opt => {
      expect(opt.text).not.toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
    });

    // Requirement B: Option value is canonical ID, option text is name
    const clothingOption = options.find(o => o.text === 'Одежда');
    expect(clothingOption?.value).toBe(CLOTHING_CAT_ID);

    const electronicsOption = options.find(o => o.text === 'Электроника');
    expect(electronicsOption?.value).toBe(ELECTRONICS_CAT_ID);

    // Requirement C: Unmapped UUID does NOT cause raw UUID to appear in dropdown options
    expect(optionTexts).not.toContain(UNKNOWN_UUID_CAT_ID);
    expect(optionValues).not.toContain(UNKNOWN_UUID_CAT_ID);
  });

  it('D: category filter actually filters products correctly by their canonical categoryId', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Open filter popover
    fireEvent.click(getFilterButton());

    // Filter by Clothing canonical UUID
    fireEvent.change(screen.getByLabelText('Категория'), { target: { value: CLOTHING_CAT_ID } });
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    // Clothing items visible
    expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    expect(screen.getByRole('heading', { level: 4, name: 'Брюки хлопковые' })).toBeDefined();

    // Others hidden
    expect(screen.queryByRole('heading', { level: 4, name: 'Смартфон Pro' })).toBeNull();
    expect(screen.queryByRole('heading', { level: 4, name: 'Сумка тоут' })).toBeNull();
    expect(screen.queryByRole('heading', { level: 4, name: 'Неизвестный товар' })).toBeNull();
  });

  // --------------------------------------------------------------------------
  // TESTS — ACTIVE FILTER CHIPS & INDIVIDUAL REMOVAL (E, F, G, H, I, J, K, L, M)
  // --------------------------------------------------------------------------

  it('E, F, G, H, M: applying filters renders active chips with correct labels and filter badge count matches chips count', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Open filter popover
    fireEvent.click(getFilterButton());

    // Select category, color, size, and only added
    fireEvent.change(screen.getByLabelText('Категория'), { target: { value: CLOTHING_CAT_ID } });
    fireEvent.change(screen.getByLabelText('Цвет'), { target: { value: 'Серый' } });
    fireEvent.change(screen.getByLabelText('Размер'), { target: { value: 'L' } });
    fireEvent.click(screen.getByLabelText('Только добавленные'));

    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    // E: Category chip
    expect(screen.getByText('Категория: Одежда')).toBeDefined();

    // F: Color chip
    expect(screen.getByText('Серый')).toBeDefined();

    // G: Size chip
    expect(screen.getByText('Размер: L')).toBeDefined();

    // H: Only added chip
    expect(screen.getByText('Только добавленные')).toBeDefined();

    // M: Filter button active count badge equals 4
    expect(getFilterButton().textContent).toContain('4');
  });

  it('I, J, K, L: clicking "x" on individual chips removes only that filter and updates count', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Apply 4 filters
    fireEvent.click(getFilterButton());
    fireEvent.change(screen.getByLabelText('Категория'), { target: { value: CLOTHING_CAT_ID } });
    fireEvent.change(screen.getByLabelText('Цвет'), { target: { value: 'Серый' } });
    fireEvent.change(screen.getByLabelText('Размер'), { target: { value: 'L' } });
    fireEvent.click(screen.getByLabelText('Только добавленные'));
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    expect(getFilterButton().textContent).toContain('4');

    // I: Remove Category chip
    fireEvent.click(screen.getByRole('button', { name: 'Удалить фильтр категории' }));
    expect(screen.queryByText('Категория: Одежда')).toBeNull();
    expect(screen.getByText('Серый')).toBeDefined();
    expect(screen.getByText('Размер: L')).toBeDefined();
    expect(screen.getByText('Только добавленные')).toBeDefined();
    expect(getFilterButton().textContent).toContain('3');

    // J: Remove Color chip
    fireEvent.click(screen.getByRole('button', { name: 'Удалить фильтр цвета' }));
    expect(screen.queryByText('Серый')).toBeNull();
    expect(screen.getByText('Размер: L')).toBeDefined();
    expect(screen.getByText('Только добавленные')).toBeDefined();
    expect(getFilterButton().textContent).toContain('2');

    // K: Remove Size chip
    fireEvent.click(screen.getByRole('button', { name: 'Удалить фильтр размера' }));
    expect(screen.queryByText('Размер: L')).toBeNull();
    expect(screen.getByText('Только добавленные')).toBeDefined();
    expect(getFilterButton().textContent).toContain('1');

    // L: Remove Only Added chip
    fireEvent.click(screen.getByRole('button', { name: 'Удалить фильтр только добавленные' }));
    expect(screen.queryByText('Только добавленные')).toBeNull();
    expect(getFilterButton().textContent).not.toMatch(/Фильтры\s+\d+/);
  });

  // --------------------------------------------------------------------------
  // TESTS — RESET ALL & STATE SYNCHRONIZATION (N, O, P, Q, R, S)
  // --------------------------------------------------------------------------

  it('N & O: "Сбросить фильтры" removes all chips and filters without resetting search query or sort mode', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Enter search query
    const searchInput = screen.getByPlaceholderText('Поиск по названию, артикулу или штрихкоду') as HTMLInputElement;
    fireEvent.change(searchInput, { target: { value: 'Костюм' } });

    // Set sort mode
    fireEvent.click(screen.getByRole('button', { name: /Сортировка/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /По названию Я–А/i }));

    // Apply color filter
    fireEvent.click(getFilterButton());
    fireEvent.change(screen.getByLabelText('Цвет'), { target: { value: 'Серый' } });
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    expect(screen.getByText('Серый')).toBeDefined();
    const resetFiltersBtn = screen.getByRole('button', { name: 'Сбросить фильтры' });
    expect(resetFiltersBtn).toBeDefined();

    // Click "Сбросить фильтры"
    fireEvent.click(resetFiltersBtn);

    // Chips are removed
    expect(screen.queryByText('Серый')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Сбросить фильтры' })).toBeNull();

    // Requirement O: Search query is preserved
    expect(searchInput.value).toBe('Костюм');

    // Requirement O: Sort mode is preserved
    fireEvent.click(screen.getByRole('button', { name: /Сортировка/i }));
    const sortMenuItems = screen.getAllByRole('menuitem');
    const yaOption = sortMenuItems.find(m => m.textContent?.includes('По названию Я–А'));
    expect(yaOption?.className).toContain('bg-black/5');
  });

  it('P: reopening filter popover after removing chips reflects updated state in drafts', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Apply Category and Color
    fireEvent.click(getFilterButton());
    fireEvent.change(screen.getByLabelText('Категория'), { target: { value: CLOTHING_CAT_ID } });
    fireEvent.change(screen.getByLabelText('Цвет'), { target: { value: 'Серый' } });
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    // Remove Category chip
    fireEvent.click(screen.getByRole('button', { name: 'Удалить фильтр категории' }));

    // Reopen filter popover
    fireEvent.click(getFilterButton());

    // Category should be reset to empty draft, Color should still be "Серый"
    expect((screen.getByLabelText('Категория') as HTMLSelectElement).value).toBe('');
    expect((screen.getByLabelText('Цвет') as HTMLSelectElement).value).toBe('Серый');
  });

  it('Q & R: draft selections do NOT create chips until "Применить", and closing popover discards drafts', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Open filter popover
    fireEvent.click(getFilterButton());

    // Select color in draft
    fireEvent.change(screen.getByLabelText('Цвет'), { target: { value: 'Розовый' } });

    // Q: No chip created before Apply
    expect(screen.queryByRole('button', { name: 'Удалить фильтр цвета' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Сбросить фильтры' })).toBeNull();

    // R: Close popover without Apply (via X button)
    fireEvent.click(screen.getByLabelText('Закрыть фильтры'));

    // Popover closed, still no chip created
    expect(screen.queryByRole('button', { name: 'Удалить фильтр цвета' })).toBeNull();
    expect(screen.queryByText('Розовый')).toBeNull();

    // Reopen popover: draft should be discarded back to empty
    fireEvent.click(getFilterButton());
    expect((screen.getByLabelText('Цвет') as HTMLSelectElement).value).toBe('');
  });

  it('S: removing chips or resetting filters preserves entered variant quantities in matrix and single editors', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Expand Suit and set 7 in matrix
    fireEvent.click(screen.getAllByRole('button', { name: /Развернуть/i })[0]);
    const matrixInput = screen.getByLabelText('Серый L') as HTMLInputElement;
    fireEvent.change(matrixInput, { target: { value: '7' } });
    expect(matrixInput.value).toBe('7');

    // Apply color filter "Серый"
    fireEvent.click(getFilterButton());
    fireEvent.change(screen.getByLabelText('Цвет'), { target: { value: 'Серый' } });
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    // Remove color chip
    fireEvent.click(screen.getByRole('button', { name: 'Удалить фильтр цвета' }));

    // Suit matrix quantity 7 is preserved
    expect((screen.getByLabelText('Серый L') as HTMLInputElement).value).toBe('7');

    // Apply only added filter, then reset filters
    fireEvent.click(getFilterButton());
    fireEvent.click(screen.getByLabelText('Только добавленные'));
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));

    fireEvent.click(screen.getByRole('button', { name: 'Сбросить фильтры' }));

    // Quantity 7 is still preserved
    expect((screen.getByLabelText('Серый L') as HTMLInputElement).value).toBe('7');
  });

  // --------------------------------------------------------------------------
  // TESTS — MATRIX, STEPPER, SPINNER, SORT & LIFECYCLE (T, U)
  // --------------------------------------------------------------------------

  it('T: quantity inputs suppress native spinners via CSS classes and single-variant stepper buttons work', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue([mockSuitProduct, mockBagProduct]);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Expand matrix product (Suit)
    fireEvent.click(screen.getAllByRole('button', { name: /Развернуть/i })[0]);

    // Check matrix input class contains spinner suppression
    const matrixInput = screen.getByLabelText('Серый L') as HTMLInputElement;
    expect(matrixInput.className).toContain('[appearance:textfield]');
    expect(matrixInput.className).toContain('[&::-webkit-outer-spin-button]:appearance-none');

    // Expand single-variant product (Bag)
    fireEvent.click(screen.getAllByRole('button', { name: /Развернуть/i })[0]);

    const singleInput = screen.getByLabelText('Количество') as HTMLInputElement;
    expect(singleInput.className).toContain('[appearance:textfield]');

    // Test +/- buttons on single-variant editor
    const plusBtn = screen.getByRole('button', { name: '+' });
    fireEvent.click(plusBtn);
    expect(singleInput.value).toBe('1');

    const minusBtn = screen.getByRole('button', { name: '-' });
    fireEvent.click(minusBtn);
    expect(singleInput.value).toBe('');
  });

  it('U: complete supply creation submission lifecycle functions seamlessly with compact toolbar', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);
    vi.mocked(sellerApi.createSellerSupply).mockResolvedValue({
      id: 'sup-new-2b3-1',
      supplyNumber: 'SUP-2026-000888',
      status: 'draft',
      createdAt: '2026-01-01',
      updatedAt: '2026-01-01',
    } as any);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // Expand Suit
    fireEvent.click(screen.getAllByRole('button', { name: /Развернуть/i })[0]);

    // Enter quantities
    fireEvent.change(screen.getByLabelText('Серый L'), { target: { value: '3' } });
    fireEvent.change(screen.getByLabelText('Серый XL'), { target: { value: '4' } });

    // Step 1 continue button enabled
    const continueBtn = screen.getByRole('button', { name: /Продолжить/i });
    expect((continueBtn as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(continueBtn);

    // Step 2: Handoff
    expect(screen.getByText('Доставка на склад ZAMK')).toBeDefined();
    fireEvent.change(screen.getByPlaceholderText('Например: 121212123241'), {
      target: { value: 'TRK-2B3-1-TEST' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Продолжить/i }));

    // Step 3: Review
    expect(screen.getByText('Проверьте поставку')).toBeDefined();
    expect(screen.getByText(/2 SKU ·/)).toBeDefined();
    expect(screen.getAllByText(/7 единиц/).length).toBeGreaterThanOrEqual(1);

    // Submit supply
    fireEvent.click(screen.getByRole('button', { name: /Создать поставку/i }));

    await waitFor(() => {
      expect(sellerApi.createSellerSupply).toHaveBeenCalledTimes(1);
      expect(sellerApi.createSellerSupply).toHaveBeenCalledWith({
        handoffMethod: 'carrier_delivery',
        carrierName: 'СДЭК',
        trackingNumber: 'TRK-2B3-1-TEST',
        items: [
          { variantId: 'var-suit-gry-l', expectedQuantity: 3 },
          { variantId: 'var-suit-gry-xl', expectedQuantity: 4 },
        ],
      });
      expect(mockNavigate).toHaveBeenCalledWith('/supplies/sup-new-2b3-1');
    });
  });

  // --------------------------------------------------------------------------
  // TESTS — SORTING
  // --------------------------------------------------------------------------

  it('clicking Sort opens sort popover menu with all 5 required sort modes', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue(mockCatalog);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: /Сортировка/i }));

    expect(screen.getByRole('menuitem', { name: /По умолчанию/i })).toBeDefined();
    expect(screen.getByRole('menuitem', { name: /По названию А–Я/i })).toBeDefined();
    expect(screen.getByRole('menuitem', { name: /По названию Я–А/i })).toBeDefined();
    expect(screen.getByRole('menuitem', { name: /Сначала добавленные/i })).toBeDefined();
    expect(screen.getByRole('menuitem', { name: /По количеству: больше → меньше/i })).toBeDefined();
  });

  it('sorting by name (А–Я and Я–А) orders products correctly', async () => {
    vi.mocked(sellerApi.getSellerProducts).mockResolvedValue([
      mockSuitProduct,
      mockPantsProduct,
      mockPhoneProduct,
      mockBagProduct,
    ]);

    render(
      <MemoryRouter initialEntries={['/supplies/new']}>
        <Routes>
          <Route path="/supplies/new" element={<SellerSupplyNew />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 4, name: 'Костюм классический' })).toBeDefined();
    });

    // 1. Sort A-Z: Брюки хлопковые -> Костюм классический -> Смартфон Pro -> Сумка тоут
    fireEvent.click(screen.getByRole('button', { name: /Сортировка/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /По названию А–Я/i }));

    let headings = screen.getAllByRole('heading', { level: 4 }).map(h => h.textContent?.trim());
    expect(headings).toEqual(['Брюки хлопковые', 'Костюм классический', 'Смартфон Pro', 'Сумка тоут']);

    // 2. Sort Z-A: Сумка тоут -> Смартфон Pro -> Костюм классический -> Брюки хлопковые
    fireEvent.click(screen.getByRole('button', { name: /Сортировка/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /По названию Я–А/i }));

    headings = screen.getAllByRole('heading', { level: 4 }).map(h => h.textContent?.trim());
    expect(headings).toEqual(['Сумка тоут', 'Смартфон Pro', 'Костюм классический', 'Брюки хлопковые']);
  });
});
