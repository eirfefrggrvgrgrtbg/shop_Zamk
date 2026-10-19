import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { TimeseriesChart, getTimeseriesDateContext } from './TimeseriesChart';
import { SellerAnalyticsProduct } from '../../pages/SellerAnalyticsProduct';
import * as analyticsApi from '../../api/selleranalytics';
import type { TimeseriesBucketDTO, ProductDetailResponse } from '../../api/selleranalytics';

vi.mock('../../api/selleranalytics', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/selleranalytics')>();
  return {
    ...actual,
    getAnalyticsProductDetail: vi.fn(),
  };
});

const sampleTimeseries: TimeseriesBucketDTO[] = [
  {
    date: '2026-10-10',
    grossSalesCents: 150000,
    ordersCount: 1,
    unitsSold: 2,
    commissionCents: 0,
    sellerEarningCents: 0,
    returnDeductionsCents: 0,
    netCommercialEarningCents: 0,
    returnedUnits: 0,
  },
  {
    date: '2026-10-11',
    grossSalesCents: 300000,
    ordersCount: 2,
    unitsSold: 3,
    commissionCents: 0,
    sellerEarningCents: 0,
    returnDeductionsCents: 0,
    netCommercialEarningCents: 0,
    returnedUnits: 1,
  },
  {
    date: '2026-10-12',
    grossSalesCents: 0,
    ordersCount: 0,
    unitsSold: 0,
    commissionCents: 0,
    sellerEarningCents: 0,
    returnDeductionsCents: 0,
    netCommercialEarningCents: 0,
    returnedUnits: 0,
  },
];

const mockProductDetailWithTimeseries: ProductDetailResponse = {
  productId: 'prod-ts-123',
  title: 'Куртка Демисезонная',
  grossSales: { currentCents: 450000, previousCents: 0, changePercent: 0, comparisonState: 'positive' },
  unitsSold: { current: 5, previous: 0, changePercent: 0, comparisonState: 'positive' },
  orders: { current: 3, previous: 0, changePercent: 0, comparisonState: 'positive' },
  returnedUnits: { current: 1, previous: 0 },
  returnRate: { currentPercent: 20.0, previousPercent: 0 },
  currentAvailableStock: 15,
  timeseries: sampleTimeseries,
  variants: [],
  insights: [],
};

describe('TimeseriesChart & Product Analytics Timeseries (SA.ANALYTICS.3C)', () => {
  // 1. Overview mode metrics
  it('1. Overview mode renders all four standard tabs including Доход', () => {
    render(<TimeseriesChart data={sampleTimeseries} mode="overview" />);
    expect(screen.getByRole('button', { name: 'Выручка' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Заказы' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Продано' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Доход' })).toBeTruthy();
  });

  // 2. Product mode metrics
  it('2. Product mode renders only delivered metrics: Выручка, Продано товаров, Заказы с доставкой', () => {
    render(<TimeseriesChart data={sampleTimeseries} mode="product" />);
    expect(screen.getByRole('button', { name: 'Выручка' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Продано товаров' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Заказы с доставкой' })).toBeTruthy();
  });

  // 3. Product mode does not have Доход tab
  it('3. Product mode strictly excludes Доход button', () => {
    render(<TimeseriesChart data={sampleTimeseries} mode="product" />);
    expect(screen.queryByRole('button', { name: 'Доход' })).toBeNull();
  });

  // 4. Product mode date context and grouping hint
  it('4. Product mode provides delivery date context for all metrics', () => {
    const salesCtx = getTimeseriesDateContext('sales', 'product');
    const unitsCtx = getTimeseriesDateContext('units', 'product');
    const ordersCtx = getTimeseriesDateContext('orders', 'product');

    expect(salesCtx.label).toBe('дата доставки');
    expect(salesCtx.groupingHint).toBe('Группировка по дате доставки');

    expect(unitsCtx.label).toBe('дата доставки');
    expect(unitsCtx.groupingHint).toBe('Группировка по дате доставки');

    expect(ordersCtx.label).toBe('дата доставки');
    expect(ordersCtx.groupingHint).toBe('Группировка по дате доставки');
  });

  // 5. Product mode displays delivery grouping hint in UI
  it('5. Product mode displays "Группировка по дате доставки" in chart header', () => {
    render(<TimeseriesChart data={sampleTimeseries} mode="product" />);
    expect(screen.getByText('Группировка по дате доставки')).toBeTruthy();
  });

  // 6. Interactive switching between metric tabs in product mode
  it('6. Allows toggling between Выручка, Продано товаров and Заказы с доставкой', () => {
    render(<TimeseriesChart data={sampleTimeseries} mode="product" />);

    const salesBtn = screen.getByRole('button', { name: 'Выручка' });
    const unitsBtn = screen.getByRole('button', { name: 'Продано товаров' });
    const ordersBtn = screen.getByRole('button', { name: 'Заказы с доставкой' });

    // Initial state: Выручка active
    expect(salesBtn.className).toContain('bg-blue-50');

    // Switch to Продано товаров
    fireEvent.click(unitsBtn);
    expect(unitsBtn.className).toContain('bg-blue-50');
    expect(salesBtn.className).not.toContain('bg-blue-50');

    // Switch to Заказы с доставкой
    fireEvent.click(ordersBtn);
    expect(ordersBtn.className).toContain('bg-blue-50');
    expect(unitsBtn.className).not.toContain('bg-blue-50');
  });

  // 7. Product detail page renders chart when timeseries is provided
  it('7. SellerAnalyticsProduct renders product timeseries chart when data has timeseries', async () => {
    vi.mocked(analyticsApi.getAnalyticsProductDetail).mockResolvedValue(mockProductDetailWithTimeseries);

    render(
      <MemoryRouter initialEntries={['/analytics/products/prod-ts-123']}>
        <Routes>
          <Route path="/analytics/products/:id" element={<SellerAnalyticsProduct />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Куртка Демисезонная')).toBeTruthy();
    });

    // In product detail, the chart must render with product mode tabs
    expect(screen.getByRole('button', { name: 'Выручка' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Продано товаров' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Заказы с доставкой' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Доход' })).toBeNull();
    expect(screen.getByText('Группировка по дате доставки')).toBeTruthy();
  });

  // 8. Product detail page omits chart when timeseries is empty
  it('8. SellerAnalyticsProduct hides chart when timeseries is empty', async () => {
    vi.mocked(analyticsApi.getAnalyticsProductDetail).mockResolvedValue({
      ...mockProductDetailWithTimeseries,
      timeseries: [],
    });

    render(
      <MemoryRouter initialEntries={['/analytics/products/prod-ts-123']}>
        <Routes>
          <Route path="/analytics/products/:id" element={<SellerAnalyticsProduct />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Куртка Демисезонная')).toBeTruthy();
    });

    expect(screen.queryByText('Группировка по дате доставки')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Продано товаров' })).toBeNull();
  });

  // 9. Zero-sales timeseries renders safely without crash or NaN
  it('9. Zero-sales timeseries renders cleanly without errors', () => {
    const zeroSeries: TimeseriesBucketDTO[] = [
      {
        date: '2026-10-10',
        grossSalesCents: 0,
        ordersCount: 0,
        unitsSold: 0,
        commissionCents: 0,
        sellerEarningCents: 0,
        returnDeductionsCents: 0,
        netCommercialEarningCents: 0,
        returnedUnits: 0,
      },
    ];

    const { container } = render(<TimeseriesChart data={zeroSeries} mode="product" />);
    expect(container).toBeTruthy();
    expect(screen.getByText('Группировка по дате доставки')).toBeTruthy();
  });
});
