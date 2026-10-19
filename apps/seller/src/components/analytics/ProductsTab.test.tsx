import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ProductsTab } from './ProductsTab';
import { SellerAnalyticsProduct } from '../../pages/SellerAnalyticsProduct';
import * as analyticsApi from '../../api/selleranalytics';
import type { ProductsResponse, ProductDetailResponse } from '../../api/selleranalytics';

vi.mock('../../api/selleranalytics', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/selleranalytics')>();
  return {
    ...actual,
    getAnalyticsProducts: vi.fn(),
    getAnalyticsProductDetail: vi.fn(),
  };
});

const mockProductsData: ProductsResponse = {
  items: [
    {
      productId: 'prod-111',
      title: 'Блузка Шелковая',
      grossSalesCents: 500000,
      ordersCount: 10,
      unitsSold: 12,
      returnedUnits: 1,
      returnRatePercent: 8.3,
      availableStock: 45,
      previousGrossSalesCents: 300000,
      grossSalesChangePercent: 66.7,
      comparisonState: 'positive',
    },
    {
      productId: 'prod-222',
      title: 'Платье Вечернее',
      grossSalesCents: 800000,
      ordersCount: 5,
      unitsSold: 5,
      returnedUnits: 0,
      returnRatePercent: 0.0,
      availableStock: 20,
      previousGrossSalesCents: 400000,
      grossSalesChangePercent: 100.0,
      comparisonState: 'positive',
    },
  ],
  totalCount: 2,
};

const mockProductDetailData: ProductDetailResponse = {
  productId: 'prod-111',
  title: 'Блузка Шелковая',
  grossSales: { currentCents: 500000, previousCents: 300000, changePercent: 66.7, comparisonState: 'positive' },
  unitsSold: { current: 12, previous: 8, changePercent: 50.0, comparisonState: 'positive' },
  orders: { current: 10, previous: 6, changePercent: 66.7, comparisonState: 'positive' },
  returnedUnits: { current: 1, previous: 0 },
  returnRate: { currentPercent: 8.3, previousPercent: 0.0 },
  currentAvailableStock: 45,
  timeseries: [],
  variants: [
    {
      variantId: 'var-1',
      sku: 'BLOUSE-S',
      displayName: 'S / Белый',
      unitsSold: 6,
      grossSalesCents: 250000,
      returnedUnits: 0,
      returnRatePercent: 0,
      availableStock: 20,
      salesVelocity: 0.2,
      daysOfStock: 100,
      stockCoverageState: 'normal',
    },
  ],
  insights: [],
};

describe('ProductsTab & Product Detail Analytics (SA.ANALYTICS.3B.2)', () => {
  const from = '2026-10-01T00:00:00+03:00';
  const to = '2026-10-31T23:59:59+03:00';

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(analyticsApi.getAnalyticsProducts).mockResolvedValue(mockProductsData);
    vi.mocked(analyticsApi.getAnalyticsProductDetail).mockResolvedValue(mockProductDetailData);
  });

  // Assertion A: Clicking the revenue sort header sends correct backend sort key 'gross_sales'
  it('A. Clicking revenue sort header toggles order and sends gross_sales', async () => {
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Блузка Шелковая')).toBeTruthy();
    });

    // Default call is gross_sales, desc
    expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'gross_sales', 'desc', 50, 0);

    const revenueHeader = screen.getByText('Выручка');
    fireEvent.click(revenueHeader);

    await waitFor(() => {
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'gross_sales', 'asc', 50, 0);
    });
  });

  // Assertion B: Sorting by delivered orders sends 'orders'
  it('B. Sorting by delivered orders header sends "orders" key to backend', async () => {
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Блузка Шелковая')).toBeTruthy();
    });

    const ordersHeader = screen.getByText('Заказы с доставкой');
    fireEvent.click(ordersHeader);

    await waitFor(() => {
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'orders', 'desc', 50, 0);
    });
  });

  // Assertion C: Sorting by units sends 'units'
  it('C. Sorting by units header sends "units" key to backend', async () => {
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Блузка Шелковая')).toBeTruthy();
    });

    const unitsHeader = screen.getByText('Штук');
    fireEvent.click(unitsHeader);

    await waitFor(() => {
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'units', 'desc', 50, 0);
    });
  });

  // Assertion D: Sorting by return percentage sends 'returns'
  it('D. Sorting by return percentage header sends "returns" key to backend', async () => {
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Блузка Шелковая')).toBeTruthy();
    });

    const returnsHeader = screen.getByText('Доля возвратов');
    fireEvent.click(returnsHeader);

    await waitFor(() => {
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'returns', 'desc', 50, 0);
    });
  });

  // Assertion E: Sorting by available stock sends 'stock'
  it('E. Sorting by available stock header sends "stock" key to backend', async () => {
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Блузка Шелковая')).toBeTruthy();
    });

    const stockHeader = screen.getByText('Остаток');
    fireEvent.click(stockHeader);

    await waitFor(() => {
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'stock', 'desc', 50, 0);
    });
  });

  // Assertion F: Current page resets to 0 when sorting changes
  it('F. Changing sorting resets pagination page to 0', async () => {
    // Return larger dataset to allow pagination
    vi.mocked(analyticsApi.getAnalyticsProducts).mockResolvedValue({
      items: mockProductsData.items,
      totalCount: 120,
    });

    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Вперед')).toBeTruthy();
    });

    // Go to page 1
    fireEvent.click(screen.getByText('Вперед'));

    await waitFor(() => {
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'gross_sales', 'desc', 50, 50);
    });

    // Now change sort field to orders
    fireEvent.click(screen.getByText('Заказы с доставкой'));

    await waitFor(() => {
      // Must reset offset to 0!
      expect(analyticsApi.getAnalyticsProducts).toHaveBeenCalledWith(from, to, 'orders', 'desc', 50, 0);
    });
  });

  // Assertion G: "Заказы с доставкой" accurately describes the underlying metric
  it('G. Renders "Заказы с доставкой" with accurate delivered-orders title tooltip', async () => {
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Заказы с доставкой')).toBeTruthy();
    });

    const header = screen.getByText('Заказы с доставкой').closest('th');
    expect(header?.getAttribute('title')).toBe('Количество заказов, в которых товары были доставлены покупателям за выбранный период.');
  });

  // Assertion H: Product detail preserves the delivered-order meaning
  it('H. Product detail displays "Заказы с доставкой" with explanatory tooltip', async () => {
    render(
      <MemoryRouter initialEntries={['/analytics/products/prod-111']}>
        <Routes>
          <Route path="/analytics/products/:id" element={<SellerAnalyticsProduct />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Блузка Шелковая')).toBeTruthy();
    });

    expect(screen.getByRole('heading', { level: 3, name: 'Заказы с доставкой' })).toBeTruthy();
    expect(screen.getByText('10 шт.')).toBeTruthy();
    expect(screen.getByLabelText('Справка: Заказы с доставкой')?.getAttribute('title')).toBe(
      'Количество заказов, в которых товары были доставлены покупателям за выбранный период.'
    );
  });

  // Assertion I: Loading, empty and error states remain intact
  it('I. Loading, empty and error states render correctly', async () => {
    // 1. Loading state
    vi.mocked(analyticsApi.getAnalyticsProducts).mockReturnValue(new Promise(() => {}));
    const { unmount } = render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );
    expect(document.querySelector('.animate-pulse')).toBeTruthy();
    unmount();

    // 2. Empty state
    vi.mocked(analyticsApi.getAnalyticsProducts).mockResolvedValue({ items: [], totalCount: 0 });
    const { unmount: unmountEmpty } = render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );
    await waitFor(() => {
      expect(screen.getByText('За выбранный период продаж не было.')).toBeTruthy();
    });
    unmountEmpty();

    // 3. Error state with retry
    vi.mocked(analyticsApi.getAnalyticsProducts).mockRejectedValueOnce(new Error('Network error'));
    render(
      <MemoryRouter>
        <ProductsTab from={from} to={to} />
      </MemoryRouter>
    );
    await waitFor(() => {
      expect(screen.getByText('Ошибка загрузки товаров')).toBeTruthy();
      expect(screen.getByText('Network error')).toBeTruthy();
      expect(screen.getByText('Повторить попытку')).toBeTruthy();
    });
  });

  // Assertion J: No foreign product information is displayed from unauthorized responses
  it('J. Product detail displays safe error message when product is unauthorized (404/error)', async () => {
    vi.mocked(analyticsApi.getAnalyticsProductDetail).mockRejectedValue(new Error('Product not found'));

    render(
      <MemoryRouter initialEntries={['/analytics/products/foreign-prod-uuid']}>
        <Routes>
          <Route path="/analytics/products/:id" element={<SellerAnalyticsProduct />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Не удалось загрузить аналитику товара')).toBeTruthy();
      expect(screen.getByText('Product not found')).toBeTruthy();
    });

    // Ensure no secret variant or stock info is leaked
    expect(screen.queryByText('SECRET-SKU')).toBeNull();
  });
});
