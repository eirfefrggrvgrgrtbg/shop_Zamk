/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { SellerProducts } from './SellerProducts';
import { adaptProductList } from '../api/adapter';
import { getSellerProducts, getSellerMe, getSellerCategories, submitSellerProductModeration } from '@zamk/api-client/src/seller';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerProducts: vi.fn(),
  getSellerMe: vi.fn(),
  getSellerCategories: vi.fn(),
  submitSellerProductModeration: vi.fn(),
}));

describe('SellerProducts - Quick View Truth + Safe Existing Actions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerMe).mockResolvedValue({
      seller: { id: 'seller-1', status: 'active', brandName: 'Test Brand' },
    } as any);
    vi.mocked(getSellerCategories).mockResolvedValue([]);
  });

  const baseProductRaw: any = {
    id: 'prod-draft-21',
    title: 'Худи оверсайз',
    slug: 'hoodie-oversize',
    status: 'draft',
    priceCents: 150000,
    totalStock: 21,
    availableStock: 21,
    description: 'Теплый оверсайз худи из плотного хлопка.',
    variants: [
      { id: 'v1', size: 'L', colorName: 'Черный', totalStock: 5, availableStock: 5 },
      { id: 'v2', size: 'XL', colorName: 'Черный', totalStock: 6, availableStock: 6 },
      { id: 'v3', size: 'L', colorName: 'Белый', totalStock: 5, availableStock: 5 },
      { id: 'v4', size: 'XL', colorName: 'Белый', totalStock: 5, availableStock: 5 },
    ],
  };

  it('A: renders Color × Size variants canonically in the drawer', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    // Click row to open drawer
    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      expect(screen.getByText('Черный · L')).toBeTruthy();
      expect(screen.getByText('Черный · XL')).toBeTruthy();
      expect(screen.getByText('Белый · L')).toBeTruthy();
      expect(screen.getByText('Белый · XL')).toBeTruthy();
    });
  });

  it('B, C, D: variant formats for single-dimension and empty variants', async () => {
    const multiFormatProduct = {
      ...baseProductRaw,
      id: 'prod-multi-fmt',
      variants: [
        { id: 'v-no-color', size: 'M', totalStock: 2, availableStock: 2 },
        { id: 'v-no-size', colorName: 'Красный', totalStock: 3, availableStock: 3 },
        { id: 'v-neither', totalStock: 1, availableStock: 1 },
      ],
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([multiFormatProduct]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByText('M')).toBeTruthy();
      expect(screen.getByText('Красный')).toBeTruthy();
      expect(screen.getByText('Единый вариант')).toBeTruthy();
    });
  });

  it('E: description has "Описание" heading with plain text', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByText('Описание')).toBeTruthy();
      expect(screen.getByText('Теплый оверсайз худи из плотного хлопка.')).toBeTruthy();
    });
  });

  it('F: empty description section is hidden entirely', async () => {
    const productNoDesc = {
      ...baseProductRaw,
      id: 'prod-no-desc',
      description: '',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([productNoDesc]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      expect(screen.queryByText('Описание')).toBeNull();
    });
  });

  it('G: draft physical stock displays as "21 шт. на складе" and does NOT imply published availability', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      // Physical stock card
      expect(screen.getByText('21 шт. на складе')).toBeTruthy();
      // Storefront availability card
      expect(screen.getByText('Не опубликован')).toBeTruthy();
      // In drawer header, no "В наличии (21 шт.)" badge
      expect(screen.queryByText('В наличии (21 шт.)')).toBeNull();
    });
  });

  it('H: draft primary CTA opens Product Studio (/products/:id/edit)', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      const editBtn = screen.getByRole('link', { name: /Продолжить заполнение/i });
      expect(editBtn).toBeTruthy();
      expect(editBtn.getAttribute('href')).toBe('/products/prod-draft-21/edit');
    });
  });

  it('I, J: submit moderation uses canonical endpoint and updates drawer status on success', async () => {
    let mockProducts = [baseProductRaw];
    vi.mocked(getSellerProducts).mockImplementation(async () => mockProducts);
    vi.mocked(submitSellerProductModeration).mockImplementation(async () => {
      mockProducts = [{ ...baseProductRaw, status: 'pending_moderation' }];
    });

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Отправить на модерацию/i })).toBeTruthy();
    });

    // Click submit
    fireEvent.click(screen.getByRole('button', { name: /Отправить на модерацию/i }));

    await waitFor(() => {
      expect(submitSellerProductModeration).toHaveBeenCalledWith('prod-draft-21');
    });

    // Submit button should disappear once submitted
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /Отправить на модерацию/i })).toBeNull();
    });

    await waitFor(() => {
      // Both the select option and the product badge in drawer now show "На модерации"
      expect(screen.getAllByText('На модерации').length).toBeGreaterThanOrEqual(2);
      expect(screen.getByRole('link', { name: /Открыть карточку/i })).toBeTruthy();
    });
  });

  it('K: submit validation failure is visible to Seller', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);
    vi.mocked(submitSellerProductModeration).mockRejectedValueOnce(
      new Error('Загрузите не менее 3 фотографий товара в разделе «Фото».')
    );

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Отправить на модерацию/i })).toBeTruthy();
    });

    fireEvent.click(screen.getByRole('button', { name: /Отправить на модерацию/i }));

    await waitFor(() => {
      expect(screen.getByTestId('submit-error-message')).toBeTruthy();
      expect(screen.getByText('Загрузите не менее 3 фотографий товара в разделе «Фото».')).toBeTruthy();
    });
  });

  it('L: published does NOT show fake Archive action', async () => {
    const publishedProduct = {
      ...baseProductRaw,
      id: 'prod-pub-1',
      status: 'published',
      actualVisibility: true,
      storefrontUrl: 'http://127.0.0.1:3000/product/hoodie-oversize',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([publishedProduct]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.queryByText(/Архивировать/i)).toBeNull();
      expect(screen.getByRole('link', { name: /Открыть в магазине/i })).toBeTruthy();
      expect(screen.getByText('В продаже')).toBeTruthy();
    });
  });

  it('M: delete action is absent for all products', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.queryByText(/Удалить/i)).toBeNull();
      expect(screen.queryByRole('button', { name: /Удалить/i })).toBeNull();
    });
  });

  it('N: existing Product Studio routes still resolve properly in router', async () => {
    render(
      <MemoryRouter initialEntries={['/products/prod-123/edit']}>
        <Routes>
          <Route path="/products/:id/edit" element={<div data-testid="studio-edit">Studio Edit</div>} />
          <Route path="/products/new" element={<div data-testid="studio-new">Studio New</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByTestId('studio-edit').textContent).toBe('Studio Edit');
  });

  // ==========================================
  // 1B1 Hardened Semantic Requirements A - I
  // ==========================================

  it('1B1-A: published + storefrontUrl renders "Открыть в магазине" linking to storefront URL', async () => {
    const pubWithUrl = {
      ...baseProductRaw,
      id: 'prod-pub-url',
      status: 'published',
      actualVisibility: true,
      storefrontUrl: 'http://127.0.0.1:3000/product/hoodie-oversize',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([pubWithUrl]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      const storeBtn = screen.getByRole('link', { name: /Открыть в магазине/i });
      expect(storeBtn).toBeTruthy();
      expect(storeBtn.getAttribute('href')).toBe('http://127.0.0.1:3000/product/hoodie-oversize');
      expect(storeBtn.getAttribute('target')).toBe('_blank');
      // Also renders secondary edit button
      const editBtn = screen.getByRole('link', { name: /Редактировать карточку/i });
      expect(editBtn.getAttribute('href')).toBe('/products/prod-pub-url/edit');
    });
  });

  it('1B1-B: published without storefrontUrl renders "Редактировать карточку" linking to editor', async () => {
    const pubWithoutUrl = {
      ...baseProductRaw,
      id: 'prod-pub-nourl',
      status: 'published',
      actualVisibility: false,
      storefrontUrl: null,
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([pubWithoutUrl]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: /Открыть в магазине/i })).toBeNull();
      const editBtn = screen.getByRole('link', { name: /Редактировать карточку/i });
      expect(editBtn).toBeTruthy();
      expect(editBtn.getAttribute('href')).toBe('/products/prod-pub-nourl/edit');
    });
  });

  it('1B1-C: No published state renders "Открыть товар" when opening editor', async () => {
    const pubWithoutUrl = {
      ...baseProductRaw,
      id: 'prod-pub-check',
      status: 'published',
      actualVisibility: false,
      storefrontUrl: null,
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([pubWithoutUrl]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.queryByText(/Открыть товар/i)).toBeNull();
    });
  });

  it('1B1-D: hidden storefront semantics truthful ("Скрыт из каталога")', async () => {
    const hiddenProduct = {
      ...baseProductRaw,
      id: 'prod-hidden',
      status: 'hidden',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([hiddenProduct]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByText('Скрыт из каталога')).toBeTruthy();
    });
  });

  it('1B1-E: blocked storefront semantics truthful ("Заблокирован")', async () => {
    const blockedProduct = {
      ...baseProductRaw,
      id: 'prod-blocked',
      status: 'blocked',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([blockedProduct]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      // Both the status badge and the Продажа card show "Заблокирован"
      expect(screen.getAllByText('Заблокирован').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('1B1-F: out_of_stock storefront semantics truthful ("Нет в наличии")', async () => {
    const outOfStockProduct = {
      ...baseProductRaw,
      id: 'prod-oos',
      status: 'out_of_stock',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([outOfStockProduct]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getAllByText('Нет в наличии').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('1B1-G: draft remains "Не опубликован"', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByText('Не опубликован')).toBeTruthy();
    });
  });

  it('1B1-H: published visible remains "В продаже"', async () => {
    const pubVisible = {
      ...baseProductRaw,
      id: 'prod-pub-vis',
      status: 'published',
      actualVisibility: true,
      storefrontUrl: 'http://127.0.0.1:3000/product/test',
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([pubVisible]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    fireEvent.click(screen.getByText('Худи оверсайз'));

    await waitFor(() => {
      expect(screen.getByText('В продаже')).toBeTruthy();
    });
  });

  it('1B1-I: no _raw remains in adapted product contract', async () => {
    const adapted = adaptProductList([baseProductRaw]);
    expect((adapted[0] as any)._raw).toBeUndefined();
  });

  // ==========================================
  // 1B2 Status Column Purity & Warehouse Hints
  // ==========================================

  it('1B2-A, B: approved + zero stock has pure STATUS column ("Одобрен") and warehouse hint in СКЛАД ZAMK', async () => {
    const approvedZeroStock = {
      ...baseProductRaw,
      id: 'prod-app-0',
      status: 'approved',
      totalStock: 0,
      availableStock: 0,
      variants: [],
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([approvedZeroStock]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    // Find table row
    const rows = screen.getAllByRole('row');
    // Row 0 is header, Row 1 is the product row
    const productRow = rows[1];
    const cells = productRow.querySelectorAll('td');
    // Column index 4: Статус
    const statusCell = cells[4];
    // Column index 5: Склад ZAMK
    const warehouseCell = cells[5];

    // Status column contains ONLY "Одобрен"
    expect(statusCell.textContent).toContain('Одобрен');
    expect(statusCell.textContent).not.toContain('Требуется поставка');

    // Warehouse column contains "Нет на складе" and "Требуется поставка"
    expect(warehouseCell.textContent).toContain('Нет на складе');
    expect(warehouseCell.textContent).toContain('Требуется поставка');
  });

  it('1B2-C: published product table row status column contains only "Опубликован"', async () => {
    const publishedProd = {
      ...baseProductRaw,
      id: 'prod-pub-0',
      status: 'published',
      totalStock: 0,
      availableStock: 0,
      variants: [],
    };
    vi.mocked(getSellerProducts).mockResolvedValueOnce([publishedProd]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    const rows = screen.getAllByRole('row');
    const productRow = rows[1];
    const cells = productRow.querySelectorAll('td');
    const statusCell = cells[4];
    const warehouseCell = cells[5];

    expect(statusCell.textContent).toContain('Опубликован');
    expect(statusCell.textContent).not.toContain('Требуется поставка');
    expect(warehouseCell.textContent).toContain('Требуется поставка');
  });

  it('1B2-D: draft lifecycle in table row status column contains only "Черновик"', async () => {
    vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

    render(
      <MemoryRouter initialEntries={['/products']}>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();
    });

    const rows = screen.getAllByRole('row');
    const productRow = rows[1];
    const cells = productRow.querySelectorAll('td');
    const statusCell = cells[4];

    expect(statusCell.textContent).toContain('Черновик');
    expect(statusCell.textContent).not.toContain('Требуется поставка');
  });

  describe('Category Display Purity (No UUID Leaks & Canonical Taxonomy Resolution)', () => {
    it('E: renders human-readable categoryName ("Худи") from product response', async () => {
      const hoodieProduct = {
        ...baseProductRaw,
        id: 'prod-hoodie-long',
        title: 'Лонг',
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
        categoryName: 'Худи',
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([hoodieProduct]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Лонг')).toBeTruthy();
      });

      expect(screen.getByText('Худи')).toBeTruthy();
      expect(screen.queryByText('c741aa40-4f5f-4b58-8581-5cfae5e77c16')).toBeNull();
    });

    it('F: resolves categoryId to "Худи" via canonical taxonomy when categoryName is omitted in product', async () => {
      const hoodieProduct = {
        ...baseProductRaw,
        id: 'prod-hoodie-long',
        title: 'Лонг',
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
        categoryName: undefined,
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([hoodieProduct]);
      vi.mocked(getSellerCategories).mockResolvedValueOnce([
        { id: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16', name: 'Худи' } as any,
        { id: 'cat-shoes-id', name: 'Обувь' } as any,
      ]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Лонг')).toBeTruthy();
      });

      expect(screen.getByText('Худи')).toBeTruthy();
      expect(screen.queryByText('c741aa40-4f5f-4b58-8581-5cfae5e77c16')).toBeNull();
      expect(screen.queryByText('Категория не определена')).toBeNull();
    });

    it('G: resolves multiple products sharing the same categoryId to the same label ("Худи") and other categories to their respective labels', async () => {
      const prod1 = {
        ...baseProductRaw,
        id: 'prod-1',
        title: 'Лонг Черный',
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
      };
      const prod2 = {
        ...baseProductRaw,
        id: 'prod-2',
        title: 'Лонг Белый',
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
      };
      const prod3 = {
        ...baseProductRaw,
        id: 'prod-3',
        title: 'Кеды Canvas',
        categoryId: 'cat-shoes-id',
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([prod1, prod2, prod3]);
      vi.mocked(getSellerCategories).mockResolvedValueOnce([
        { id: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16', name: 'Худи' } as any,
        { id: 'cat-shoes-id', name: 'Обувь' } as any,
      ]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Лонг Черный')).toBeTruthy();
        expect(screen.getByText('Лонг Белый')).toBeTruthy();
        expect(screen.getByText('Кеды Canvas')).toBeTruthy();
      });

      const hoodieLabels = screen.getAllByText('Худи');
      expect(hoodieLabels.length).toBe(2);
      expect(screen.getByText('Обувь')).toBeTruthy();
      expect(screen.queryByText('c741aa40-4f5f-4b58-8581-5cfae5e77c16')).toBeNull();
      expect(screen.queryByText('cat-shoes-id')).toBeNull();
    });

    it('H: renders safe fallback "Категория не определена" only when category is genuinely unresolved / deleted', async () => {
      const unresolvedProduct = {
        ...baseProductRaw,
        id: 'prod-unresolved',
        title: 'Неизвестный товар',
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
        categoryName: undefined,
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([unresolvedProduct]);
      vi.mocked(getSellerCategories).mockResolvedValueOnce([]); // empty taxonomy

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Неизвестный товар')).toBeTruthy();
      });

      expect(screen.getByText('Категория не определена')).toBeTruthy();
      expect(screen.queryByText('c741aa40-4f5f-4b58-8581-5cfae5e77c16')).toBeNull();
    });
  });
});
