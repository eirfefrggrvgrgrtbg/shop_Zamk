/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { SellerProducts } from './SellerProducts';
import { getSellerProductsPaginated, getSellerMe, getSellerCategories } from '@zamk/api-client/src/seller';
import { getProductAvailability } from '../lib/seller-products';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerProductsPaginated: vi.fn(),
  getSellerMe: vi.fn(),
  getSellerCategories: vi.fn(),
  submitSellerProductModeration: vi.fn(),
  deleteSellerProduct: vi.fn(),
  archiveSellerProduct: vi.fn(),
}));

// SELLER.PRODUCTS.UX.1: published is not purchasable. The page shows the backend's own verdict and reasons.

const raw = (over: Record<string, any> = {}): any => ({
  id: 'p-1',
  title: 'Худи',
  slug: 'hoodie',
  status: 'published',
  priceCents: 150000,
  actualVisibility: true,
  visibilityReasons: [],
  availableStock: 10,
  totalStock: 10,
  variants: [
    { id: 'v1', size: 'M', isActive: true, totalStock: 5, availableStock: 5 },
    { id: 'v2', size: 'L', isActive: true, totalStock: 5, availableStock: 5 },
  ],
  ...over,
});

const summary = { total: 7, published: 4, purchasable: 2, awaitingReview: 1 };

const renderPage = () =>
  render(
    <MemoryRouter initialEntries={['/products']}>
      <Routes>
        <Route path="/products" element={<SellerProducts />} />
        <Route path="/products/:id/edit" element={<div data-testid="edit-page">edit</div>} />
      </Routes>
    </MemoryRouter>
  );

describe('SellerProducts — publication vs availability', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerMe).mockResolvedValue({ seller: { id: 's', status: 'active', brandName: 'B' } } as any);
    vi.mocked(getSellerCategories).mockResolvedValue([]);
  });

  it('A. a published product with stock says "Можно купить"; a published one without stock says why it cannot be bought', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [
        raw({ id: 'a', title: 'Доступное худи' }),
        raw({ id: 'b', title: 'Худи без остатка', actualVisibility: false, visibilityReasons: ['low_stock'], availableStock: 0, totalStock: 3,
          variants: [{ id: 'v', size: 'M', isActive: true, totalStock: 3, availableStock: 0 }] }),
      ],
      totalCount: 2,
      summary,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('Доступное худи')).toBeTruthy());

    const [rowA, rowB] = screen.getAllByTestId('product-availability');
    expect(rowA.textContent).toContain('Можно купить');
    expect(rowB.textContent).toContain('Нельзя купить');
    expect(rowB.textContent).toContain('Нет доступного остатка на складе ZAMK');
    expect(rowB.textContent).toContain('Склад ZAMK');
    // both are "Опубликован": the publication status is a separate fact
    expect(screen.getAllByText('Опубликован').filter((el) => el.tagName === 'SPAN' && el.closest('tbody'))).toHaveLength(2);
  });

  it('B. reasons are named by whose action is needed, and unknown data is not guessed', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [
        raw({ id: 'd', title: 'Черновик', status: 'draft', actualVisibility: false, visibilityReasons: ['moderation_required'] }),
        raw({ id: 'm', title: 'На проверке', status: 'pending_moderation', actualVisibility: false, visibilityReasons: ['moderation_required'] }),
        raw({ id: 'h', title: 'Скрытый', status: 'hidden', actualVisibility: false, visibilityReasons: ['product_hidden'] }),
        raw({ id: 'u', title: 'Без вердикта', actualVisibility: undefined, visibilityReasons: undefined }),
      ],
      totalCount: 4,
      summary,
    });
    renderPage();
    await waitFor(() => expect(screen.getAllByTestId('product-availability')).toHaveLength(4));
    const cells = screen.getAllByTestId('product-availability');
    expect(cells[0].textContent).toContain('Не отправлен на модерацию');
    expect(cells[0].textContent).toContain('Действие продавца');
    expect(cells[1].textContent).toContain('Ожидает проверки ZAMK');
    expect(cells[1].textContent).toContain('Решение модерации ZAMK');
    expect(cells[2].textContent).toContain('Товар скрыт из каталога');
    expect(cells[3].textContent).toContain('Нет данных');
    expect(cells[3].textContent).not.toContain('Можно купить');
    expect(cells[3].textContent).not.toContain('Нельзя купить');
  });

  it('C. a product with only some SKUs in stock stays purchasable and shows how many SKUs are sellable', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [raw({ id: 'p', title: 'Частичный', availableStock: 4,
        variants: [
          { id: 'v1', size: 'M', isActive: true, totalStock: 4, availableStock: 4 },
          { id: 'v2', size: 'L', isActive: true, totalStock: 2, availableStock: 0 },
          { id: 'v3', size: 'XL', isActive: false, totalStock: 9, availableStock: 9 },
        ] })],
      totalCount: 1,
      summary,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('Частичный')).toBeTruthy());
    expect(screen.getByTestId('product-availability').textContent).toContain('Можно купить');
    expect(screen.getByTestId('partial-sku').textContent).toContain('Доступны 1 из 2'); // the switched-off SKU is not counted

    fireEvent.click(screen.getByText('Частичный'));
    const drawer = await screen.findByTestId('seller-product-drawer');
    expect(within(drawer).getByText('Нет остатка')).toBeTruthy();
    expect(within(drawer).getByText('Выключен')).toBeTruthy();
    expect(within(drawer).getByText('Есть в наличии')).toBeTruthy();
  });

  it('D. the drawer lists every reason with its owner', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [raw({ id: 'x', title: 'Проблемный', actualVisibility: false, visibilityReasons: ['low_stock', 'invalid_price', 'no_active_variants'], availableStock: 0,
        variants: [{ id: 'v', size: 'M', isActive: true, totalStock: 2, availableStock: 0 }] })],
      totalCount: 1,
      summary,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('Проблемный')).toBeTruthy());
    fireEvent.click(screen.getByText('Проблемный'));
    const box = await screen.findByTestId('drawer-unavailable-reasons');
    expect(box.textContent).toContain('Нет активных вариантов (SKU)');
    expect(box.textContent).toContain('Не указана цена');
    expect(box.textContent).toContain('Нет доступного остатка на складе ZAMK');
    expect(screen.getByTestId('drawer-availability').textContent).toBe('Нельзя купить');
  });

  it('E. filters are sent to the server with a reset page; the summary comes from the server', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [raw()], totalCount: 1, summary });
    renderPage();
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenCalledWith({ q: undefined, status: undefined, availability: undefined, limit: 20, offset: 0 }));
    expect(screen.getAllByTestId('seller-kpi-label').map((e) => e.textContent)).toEqual(['Всего карточек', 'Опубликовано', 'Можно купить', 'На проверке']);
    expect(screen.getAllByTestId('seller-kpi-value').map((e) => e.textContent)).toEqual(['7', '4', '2', '1']);

    fireEvent.click(screen.getByTestId('products-availability-available'));
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenLastCalledWith({ q: undefined, status: undefined, availability: 'available', limit: 20, offset: 0 }));
    fireEvent.click(screen.getByTestId('products-availability-unavailable'));
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenLastCalledWith({ q: undefined, status: undefined, availability: 'unavailable', limit: 20, offset: 0 }));
    fireEvent.change(screen.getByLabelText('Статус товара'), { target: { value: 'moderation' } });
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenLastCalledWith({ q: undefined, status: 'moderation', availability: 'unavailable', limit: 20, offset: 0 }));
    fireEvent.change(screen.getByPlaceholderText(/Поиск по названию/), { target: { value: 'худи' } });
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenLastCalledWith({ q: 'худи', status: 'moderation', availability: 'unavailable', limit: 20, offset: 0 }), { timeout: 2000 });
    // the search box survives the reloads
    expect((screen.getByPlaceholderText(/Поиск по названию/) as HTMLInputElement).value).toBe('худи');
  });

  it('F. pagination uses the server total and moves by offset; a filter change returns to the first page', async () => {
    const page = (n: number) => Array.from({ length: 20 }, (_, i) => raw({ id: `p${n}-${i}`, title: `Товар ${n}-${i}` }));
    vi.mocked(getSellerProductsPaginated).mockImplementation(async (params?: any) => ({ items: page(params?.offset ? 2 : 1), totalCount: 45, summary }));
    renderPage();
    await waitFor(() => expect(screen.getByText('Товар 1-0')).toBeTruthy());
    expect(screen.getByTestId('products-pagination').textContent).toContain('1–20 из 45');
    fireEvent.click(screen.getByRole('button', { name: 'Вперёд' }));
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 20, limit: 20 })));
    await waitFor(() => expect(screen.getByText('Товар 2-0')).toBeTruthy());
    expect(screen.getByTestId('products-pagination').textContent).toContain('21–40 из 45');
    fireEvent.click(screen.getByTestId('products-availability-available'));
    await waitFor(() => expect(getSellerProductsPaginated).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 0, availability: 'available' })));
  });

  it('G. a filter with no result shows a filtered empty state, not "no products yet"', async () => {
    vi.mocked(getSellerProductsPaginated)
      .mockResolvedValueOnce({ items: [raw()], totalCount: 1, summary })
      .mockResolvedValue({ items: [], totalCount: 0, summary });
    renderPage();
    await waitFor(() => expect(screen.getByText('Худи')).toBeTruthy());
    fireEvent.click(screen.getByTestId('products-availability-unavailable'));
    await waitFor(() => expect(screen.getByTestId('products-empty-filtered')).toBeTruthy());
    expect(screen.queryByText('У вас пока нет товаров')).toBeNull();
  });

  it('H. a seller without products sees the add-product empty state', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [], totalCount: 0, summary: { total: 0, published: 0, purchasable: 0, awaitingReview: 0 } });
    renderPage();
    await waitFor(() => expect(screen.getByText('У вас пока нет товаров')).toBeTruthy());
  });

  it('I. an error shows a retry, and a later filter failure keeps the page and its filters', async () => {
    vi.mocked(getSellerProductsPaginated)
      .mockRejectedValueOnce(new Error('Ошибка загрузки товаров'))
      .mockResolvedValueOnce({ items: [raw()], totalCount: 1, summary })
      .mockRejectedValueOnce(new Error('Ошибка загрузки товаров'))
      .mockResolvedValue({ items: [raw()], totalCount: 1, summary });
    renderPage();
    expect((await screen.findByRole('alert')).textContent).toContain('Ошибка загрузки товаров');
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(screen.getByText('Худи')).toBeTruthy());

    fireEvent.click(screen.getByTestId('products-availability-available'));
    const alert = await screen.findByRole('alert');
    expect(screen.getByTestId('products-availability-available').getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(within(alert).getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
  });

  it('K. a slow answer to an old filter never overwrites the newest one', async () => {
    let releaseOld: (v: any) => void = () => {};
    vi.mocked(getSellerProductsPaginated)
      .mockResolvedValueOnce({ items: [raw({ id: 'first', title: 'Первая загрузка' })], totalCount: 1, summary })
      .mockImplementationOnce(() => new Promise((resolve) => { releaseOld = resolve; })) // availability=available: slow
      .mockResolvedValue({ items: [raw({ id: 'new', title: 'Свежий результат' })], totalCount: 1, summary }); // availability=unavailable: fast
    renderPage();
    await waitFor(() => expect(screen.getByText('Первая загрузка')).toBeTruthy());
    fireEvent.click(screen.getByTestId('products-availability-available'));
    fireEvent.click(screen.getByTestId('products-availability-unavailable'));
    await waitFor(() => expect(screen.getByText('Свежий результат')).toBeTruthy());
    releaseOld({ items: [raw({ id: 'old', title: 'Устаревший результат' })], totalCount: 1, summary });
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.queryByText('Устаревший результат')).toBeNull();
    expect(screen.getByText('Свежий результат')).toBeTruthy();
  });

  it('J. the existing edit route is reachable, and no stock or warehouse action is offered', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [raw({ title: 'Редактируемое' })], totalCount: 1, summary });
    renderPage();
    await waitFor(() => expect(screen.getByText('Редактируемое')).toBeTruthy());
    const buttons = screen.getAllByRole('button').map((b) => b.textContent || '');
    expect(buttons.some((t) => /принять|собрать|упаковать|отгрузить|списать|изменить остаток|одобрить|опубликовать/i.test(t))).toBe(false);
    expect(screen.queryByRole('spinbutton')).toBeNull(); // no stock input anywhere
    fireEvent.doubleClick(screen.getByText('Редактируемое'));
    await waitFor(() => expect(screen.getByTestId('edit-page')).toBeTruthy());
  });
});

describe('getProductAvailability', () => {
  const sizes = [{ size: 'M', stock: 3, active: true }, { size: 'L', stock: 0, active: true }, { size: 'XL', stock: 5, active: false }];
  it('follows the backend verdict and never invents one', () => {
    expect(getProductAvailability({ status: 'published', sizes, actualVisibility: true, visibilityReasons: [] }).state).toBe('available');
    expect(getProductAvailability({ status: 'published', sizes, actualVisibility: false, visibilityReasons: ['low_stock'] }).state).toBe('unavailable');
    expect(getProductAvailability({ status: 'published', sizes }).state).toBe('unknown');
  });
  it('counts only active SKUs and flags a partial range', () => {
    const av = getProductAvailability({ status: 'published', sizes, actualVisibility: true, visibilityReasons: [] });
    expect(av.skuTotal).toBe(2);
    expect(av.skuSellable).toBe(1);
    expect(av.partial).toBe(true);
    expect(getProductAvailability({ status: 'published', sizes: [{ size: 'M', stock: 2, active: true }], actualVisibility: true }).partial).toBe(false);
  });
  it('orders reasons by importance', () => {
    const av = getProductAvailability({ status: 'blocked', sizes, actualVisibility: false, visibilityReasons: ['low_stock', 'product_blocked'] });
    expect(av.reasons.map((r) => r.code)).toEqual(['product_blocked', 'low_stock']);
    expect(av.reasons[0].owner).toBe('moderation');
    expect(av.reasons[1].owner).toBe('warehouse');
  });
});
