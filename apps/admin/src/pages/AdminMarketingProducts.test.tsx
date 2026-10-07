import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingProducts } from './AdminMarketingProducts';
import * as marketingApi from '../api/marketing';
import * as opsApi from '../api/adminOperations';

vi.mock('../api/marketing');
vi.mock('../api/adminOperations');

const sampleCoverageAvailable: marketingApi.ProductAnalyticsCoverage = {
  views: { status: 'available' },
  favorites: { status: 'available' },
  addToCart: { status: 'available' },
};

const sampleCoveragePartial: marketingApi.ProductAnalyticsCoverage = {
  views: { status: 'partial', trackedFrom: '2026-10-01T00:00:00Z' },
  favorites: { status: 'partial', trackedFrom: '2026-10-01T00:00:00Z' },
  addToCart: { status: 'partial', trackedFrom: '2026-10-01T00:00:00Z' },
};

const sampleCoverageUnavailable: marketingApi.ProductAnalyticsCoverage = {
  views: { status: 'unavailable' },
  favorites: { status: 'unavailable' },
  addToCart: { status: 'unavailable' },
};

const sampleProducts: marketingApi.ProductAnalyticsRow[] = [
  {
    productId: 'prod-1',
    productName: 'Trench Coat Classic',
    primaryImage: 'https://example.com/coat.jpg',
    designerName: 'Acme Studio',
    categoryName: 'Coats',
    views: 120,
    favorites: 15,
    addToCart: 8,
    purchases: 6,
    soldUnits: 8,
    revenueCents: 8000000,
    conversionRate: 0.05,
    returns: 1,
    returnedUnits: 1,
  },
  {
    productId: 'prod-2',
    productName: 'Silk Dress',
    primaryImage: null,
    designerName: 'e4d8a1b2-3c4d-5e6f-7a8b-9c0d1e2f3a4b',
    categoryName: null,
    views: 0,
    favorites: 0,
    addToCart: 0,
    purchases: 0,
    soldUnits: 0,
    revenueCents: 0,
    conversionRate: 0,
    returns: 0,
    returnedUnits: 0,
  },
];

const mockCategories = [
  { id: 'cat-1', name: 'Coats' },
  { id: 'cat-2', name: 'Dresses' },
];

const mockDesigners = [
  { id: 'sel-1', brandName: 'Acme Studio' },
  { id: 'sel-2', brandName: '', name: '' },
];

const mount = () =>
  render(
    <MemoryRouter>
      <AdminMarketingProducts />
    </MemoryRouter>
  );

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(opsApi.getAdminCategories).mockResolvedValue(mockCategories as any);
  vi.mocked(opsApi.getAdminSellers).mockResolvedValue({ items: mockDesigners } as any);
  vi.mocked(marketingApi.getMarketingProducts).mockResolvedValue({
    coverage: sampleCoverageAvailable,
    products: sampleProducts,
  });
});

afterEach(cleanup);

describe('AdminMarketingProducts Component Tests', () => {
  it('1. loading state is rendered while request is in flight', async () => {
    let finish: any;
    vi.mocked(marketingApi.getMarketingProducts).mockReturnValue(
      new Promise(resolve => {
        finish = resolve;
      })
    );

    mount();
    expect(screen.getByText('Загрузка товаров')).toBeTruthy();

    finish({ coverage: sampleCoverageAvailable, products: [] });
    await waitFor(() => expect(screen.queryByText('Загрузка товаров')).toBeNull());
  });

  it('2. populated rows are rendered with tabular metrics and revenue formatting', async () => {
    mount();
    await screen.findByText('Trench Coat Classic');

    const table = screen.getByRole('table');
    expect(within(table).getByText('Acme Studio')).toBeTruthy(); // designer
    expect(within(table).getByText('Coats')).toBeTruthy();       // category
    expect(within(table).getByText('120')).toBeTruthy();         // views
    expect(within(table).getByText('15')).toBeTruthy();          // favorites
    expect(within(table).getByText('8')).toBeTruthy();           // add to cart
    expect(within(table).getByText('6')).toBeTruthy();           // purchases
    expect(within(table).getByText('8 шт.')).toBeTruthy();       // sold units
    expect(within(table).getByText(/80\s*000/)).toBeTruthy();    // revenue formatting
    expect(within(table).getByText('5.00%')).toBeTruthy();       // conversion
    expect(within(table).getByText('1')).toBeTruthy();           // returns

    // Known zero under available coverage remains truthful '0'
    const zeroCells = within(table).getAllByText('0');
    expect(zeroCells.length).toBeGreaterThanOrEqual(3);
  });

  it('3. empty state is rendered with reset filters button', async () => {
    vi.mocked(marketingApi.getMarketingProducts).mockResolvedValue({
      coverage: sampleCoverageAvailable,
      products: [],
    });
    mount();
    await screen.findByText('Нет товаров');
    expect(screen.getByText('Сбросить фильтры')).toBeTruthy();
  });

  it('4. error state and retry button refetches data upon click', async () => {
    vi.mocked(marketingApi.getMarketingProducts).mockRejectedValueOnce(new Error('Network error'));

    mount();
    await screen.findByText('Не удалось загрузить аналитику товаров');
    expect(marketingApi.getMarketingProducts).toHaveBeenCalledTimes(1);

    const retryBtn = screen.getByRole('button', { name: 'Повторить' });
    expect(retryBtn).toBeTruthy();

    // Setup success on retry
    vi.mocked(marketingApi.getMarketingProducts).mockResolvedValueOnce({
      coverage: sampleCoverageAvailable,
      products: sampleProducts,
    });

    fireEvent.click(retryBtn);

    await screen.findByText('Trench Coat Classic');
    expect(screen.queryByText('Не удалось загрузить аналитику товаров')).toBeNull();
    expect(marketingApi.getMarketingProducts).toHaveBeenCalledTimes(2);
  });

  it('5. no raw UUID is rendered', async () => {
    mount();
    await screen.findByText('Trench Coat Classic');
    expect(screen.queryByText('e4d8a1b2-3c4d-5e6f-7a8b-9c0d1e2f3a4b')).toBeNull();
    expect(screen.getByText('Без названия')).toBeTruthy();
  });

  it('6. period change updates requested date range', async () => {
    mount();
    await screen.findByText('Trench Coat Classic');
    const btn7d = screen.getByRole('button', { name: '7 дней' });
    fireEvent.click(btn7d);
    await waitFor(() => {
      const calls = vi.mocked(marketingApi.getMarketingProducts).mock.calls;
      expect(calls.length).toBeGreaterThan(1);
    });
  });

  it('7. category and seller filter changes trigger query updates', async () => {
    mount();
    await screen.findByText('Trench Coat Classic');

    // Select category Coats
    const catButton = screen.getByTestId('select-Все категории');
    fireEvent.click(catButton);
    const coatsOption = await screen.findByRole('option', { name: 'Coats' });
    fireEvent.click(coatsOption);

    await waitFor(() => {
      expect(marketingApi.getMarketingProducts).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        expect.any(String),
        undefined,
        '',
        'cat-1',
        ''
      );
    });

    // Select designer Acme Studio
    const desButton = screen.getByTestId('select-Все дизайнеры');
    fireEvent.click(desButton);
    const acmeOption = await screen.findByRole('option', { name: 'Acme Studio' });
    fireEvent.click(acmeOption);

    await waitFor(() => {
      expect(marketingApi.getMarketingProducts).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        expect.any(String),
        undefined,
        '',
        'cat-1',
        'sel-1'
      );
    });
  });

  it('8. search input debounces and triggers search query', async () => {
    mount();
    await screen.findByText('Trench Coat Classic');

    const searchInput = screen.getByPlaceholderText('ПОИСК ПО НАЗВАНИЮ...');
    fireEvent.change(searchInput, { target: { value: 'Trench' } });

    await waitFor(
      () => {
        expect(marketingApi.getMarketingProducts).toHaveBeenCalledWith(
          expect.any(String),
          expect.any(String),
          expect.any(String),
          undefined,
          'Trench',
          '',
          ''
        );
      },
      { timeout: 1000 }
    );
  });

  it('9. sorting dropdown triggers sort update and disables favorites when coverage is not available', async () => {
    vi.mocked(marketingApi.getMarketingProducts).mockResolvedValue({
      coverage: sampleCoveragePartial,
      products: sampleProducts,
    });

    mount();
    await screen.findByText('Trench Coat Classic');

    const sortButton = screen.getByTestId('select-Сортировка');
    fireEvent.click(sortButton);
    const salesOption = await screen.findByRole('option', { name: 'По продажам' });
    fireEvent.click(salesOption);

    await waitFor(() => {
      expect(marketingApi.getMarketingProducts).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        'sales',
        undefined,
        expect.any(String),
        expect.any(String),
        expect.any(String)
      );
    });

    fireEvent.click(screen.getByTestId('select-Сортировка'));

    const favOption = await screen.findByRole('option', { name: /По избранному/ });
    expect(favOption.getAttribute('aria-disabled')).toBe('true');
  });

  it('10. coverage unavailable != 0: renders "—" badge with insufficient data tooltip, never fake 0', async () => {
    vi.mocked(marketingApi.getMarketingProducts).mockResolvedValue({
      coverage: sampleCoverageUnavailable,
      products: sampleProducts,
    });
    mount();
    await screen.findByText('Trench Coat Classic');
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThanOrEqual(6);

    const tooltips = screen.getAllByTitle('Недостаточно данных');
    expect(tooltips.length).toBeGreaterThanOrEqual(6);
  });

  it('11. partial coverage treatment: positive value shows with partial marker, tooltip explains minimum count, legend explains partial period', async () => {
    vi.mocked(marketingApi.getMarketingProducts).mockResolvedValue({
      coverage: sampleCoveragePartial,
      products: sampleProducts,
    });
    mount();
    await screen.findByText('Trench Coat Classic');

    // 1. Legend is rendered with human-readable copy
    expect(screen.getByText('Поведенческие данные собраны не за весь период')).toBeTruthy();

    // 2. Partial positive value has dynamic tooltip
    const partialViews = screen.getAllByTitle('Зафиксировано минимум 120. Трекинг покрывает не весь выбранный период.');
    expect(partialViews.length).toBeGreaterThan(0);

    // 3. Partial zero value renders "—" with insufficient data tooltip
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThan(0);
  });

  it('12. conversion unavailable when views coverage is not trustworthy', async () => {
    vi.mocked(marketingApi.getMarketingProducts).mockResolvedValue({
      coverage: sampleCoveragePartial,
      products: sampleProducts,
    });
    mount();
    await screen.findByText('Trench Coat Classic');
    expect(screen.queryByText('5.00%')).toBeNull();
  });

  it('13. returns rendering shows dash when 0 returns', async () => {
    mount();
    await screen.findByText('Trench Coat Classic');

    // Silk dress has 0 returns -> renders "—"
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThan(0);
  });
});
