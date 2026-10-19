import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { OverviewTab } from './OverviewTab';
import { formatTimeseriesYAxisTick, getTimeseriesDateContext } from './TimeseriesChart';
import * as analyticsApi from '../../api/selleranalytics';
import type { OverviewResponse, ProductsResponse } from '../../api/selleranalytics';

vi.mock('../../api/selleranalytics', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/selleranalytics')>();
  return {
    ...actual,
    getAnalyticsOverview: vi.fn(),
    getAnalyticsProducts: vi.fn(),
  };
});

const mockOverviewData: OverviewResponse = {
  period: {
    from: '2026-10-01T00:00:00+03:00',
    to: '2026-10-15T00:00:00+03:00',
    timezone: 'Europe/Moscow',
  },
  hasHistoricalSales: true,
  grossSales: {
    currentCents: 450000,
    previousCents: 300000,
    changePercent: 50.0,
    comparisonState: 'positive',
  },
  orders: {
    current: 3,
    previous: 2,
    changePercent: 50.0,
    comparisonState: 'positive',
  },
  deliveredOrders: {
    current: 2,
    previous: 2,
  },
  unitsSold: {
    current: 4,
    previous: 3,
    changePercent: 33.3,
    comparisonState: 'positive',
  },
  averageOrderValue: {
    currentCents: 225000,
    previousCents: 150000,
    changePercent: 50.0,
    comparisonState: 'positive',
  },
  commission: {
    currentCents: 36000,
    previousCents: 24000,
  },
  sellerEarningBeforeReturns: {
    currentCents: 414000,
    previousCents: 276000,
  },
  returnDeductions: {
    currentCents: -50000,
    previousCents: 0,
  },
  otherAdjustments: {
    currentCents: 0,
    previousCents: 0,
  },
  netCommercialEarning: {
    currentCents: 364000,
    previousCents: 276000,
    changePercent: 31.9,
    comparisonState: 'positive',
  },
  returnedUnits: {
    current: 1,
    previous: 0,
  },
  returnRate: {
    currentPercent: 25.0,
    previousPercent: 0.0,
  },
  timeseries: [
    {
      date: '2026-10-10',
      grossSalesCents: 200000,
      ordersCount: 1,
      unitsSold: 2,
      commissionCents: 16000,
      sellerEarningCents: 184000,
      returnDeductionsCents: 0,
      netCommercialEarningCents: 184000,
      returnedUnits: 0,
    },
    {
      date: '2026-10-12',
      grossSalesCents: 250000,
      ordersCount: 2,
      unitsSold: 2,
      commissionCents: 20000,
      sellerEarningCents: 230000,
      returnDeductionsCents: -50000,
      netCommercialEarningCents: 180000,
      returnedUnits: 1,
    },
  ],
  insights: [],
};

const mockProductsData: ProductsResponse = {
  items: [],
  totalCount: 0,
};

describe('OverviewTab - Option A Alignment', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(analyticsApi.getAnalyticsProducts).mockResolvedValue(mockProductsData);
  });

  it('renders Option A KPI card titles: Выручка, Заказы, Продано товаров, Средний чек', async () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(mockOverviewData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 3, name: 'Выручка' })).toBeTruthy();
      expect(screen.getByRole('heading', { level: 3, name: 'Заказы' })).toBeTruthy();
      expect(screen.getByRole('heading', { level: 3, name: 'Продано товаров' })).toBeTruthy();
      expect(screen.getByRole('heading', { level: 3, name: 'Средний чек' })).toBeTruthy();
    });

    // Check values
    expect(screen.getByText(/4\s*500\s*₽/)).toBeTruthy(); // grossSales
    expect(screen.getByText('3 шт.')).toBeTruthy();       // paid orders
    expect(screen.getByText('4 шт.')).toBeTruthy();       // units sold
    expect(screen.getByText(/2\s*250\s*₽/)).toBeTruthy(); // delivered AOV
  });

  it('renders explanations for all 4 KPI cards via accessible help buttons', async () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(mockOverviewData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Справка: Выручка/i })).toBeTruthy();
    });

    const revenueBtn = screen.getByRole('button', { name: /Справка: Выручка/i });
    expect(revenueBtn.getAttribute('title')).toBe(
      'Стоимость доставленных товаров до вычета комиссии и удержаний за возвраты.'
    );

    const ordersBtn = screen.getByRole('button', { name: /Справка: Заказы/i });
    expect(ordersBtn.getAttribute('title')).toBe(
      'Оплаченные покупателями заказы за выбранный период. Включая ещё не доставленные. Отменённые после оплаты сохраняются в истории.'
    );

    const unitsBtn = screen.getByRole('button', { name: /Справка: Продано товаров/i });
    expect(unitsBtn.getAttribute('title')).toBe(
      'Количество фактически доставленных покупателям единиц товара.'
    );

    const aovBtn = screen.getByRole('button', { name: /Справка: Средний чек/i });
    expect(aovBtn.getAttribute('title')).toBe(
      'Выручка по доставленным товарам, разделённая на количество заказов с доставкой.'
    );
  });

  it('renders delivered orders supporting text under Средний чек when deliveredOrders is present', async () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(mockOverviewData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Доставлено заказов: 2 шт.')).toBeTruthy();
    });
  });

  it('omits delivered orders supporting text when deliveredOrders is undefined/null in DTO', async () => {
    const dataWithoutDeliveredOrders = { ...mockOverviewData, deliveredOrders: undefined };
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(dataWithoutDeliveredOrders);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Средний чек')).toBeTruthy();
    });

    expect(screen.queryByText(/Доставлено заказов:/)).toBeNull();
  });

  it('renders TimeseriesChart with Выручка label and date context indication', async () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(mockOverviewData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      // In chart button group
      expect(screen.getByRole('button', { name: 'Выручка' })).toBeTruthy();
      expect(screen.getByRole('button', { name: 'Заказы' })).toBeTruthy();
      expect(screen.getByRole('button', { name: 'Продано' })).toBeTruthy();
      expect(screen.getByRole('button', { name: 'Доход' })).toBeTruthy();
    });

    // Default active metric is 'sales' (Выручка) -> shows delivery grouping hint
    expect(screen.getByText('Группировка по дате доставки')).toBeTruthy();
  });

  it('renders separate secondary blocks for Удержания за возвраты and Возвраты товаров', async () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(mockOverviewData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Удержания за возвраты')).toBeTruthy();
      expect(screen.getByText('Возвраты товаров')).toBeTruthy();
    });

    // Monetary deduction is shown as formatted rubles without conflicting physical units inline
    expect(screen.getByText(/-500\s*₽/)).toBeTruthy();
    // Physical return units and percentage are in the distinct return volume block
    expect(screen.getByText('1 шт.')).toBeTruthy();
    expect(screen.getByText('(25.0%)')).toBeTruthy();
  });

  it('renders zero period state when all activity metrics are 0', async () => {
    const zeroData = {
      ...mockOverviewData,
      grossSales: { currentCents: 0, previousCents: 0, changePercent: null, comparisonState: 'unchanged' as const },
      orders: { current: 0, previous: 0, changePercent: null, comparisonState: 'unchanged' as const },
      unitsSold: { current: 0, previous: 0, changePercent: null, comparisonState: 'unchanged' as const },
      returnedUnits: { current: 0, previous: 0 },
      returnRate: { currentPercent: 0, previousPercent: 0 },
      returnDeductions: { currentCents: 0, previousCents: 0 },
      netCommercialEarning: { currentCents: 0, previousCents: 0, changePercent: null, comparisonState: 'unchanged' as const },
    };
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(zeroData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('За выбранный период не было заказов, продаж и возвратов.')).toBeTruthy();
    });
  });

  it('does not trigger empty state when sales and orders are 0 but return deductions exist', async () => {
    const returnOnlyData = {
      ...mockOverviewData,
      grossSales: { currentCents: 0, previousCents: 0, changePercent: null, comparisonState: 'unchanged' as const },
      orders: { current: 0, previous: 0, changePercent: null, comparisonState: 'unchanged' as const },
      unitsSold: { current: 0, previous: 0, changePercent: null, comparisonState: 'unchanged' as const },
      returnDeductions: { currentCents: -3546270, previousCents: 0 },
      netCommercialEarning: { currentCents: -3546270, previousCents: 0, changePercent: null, comparisonState: 'negative' as const },
    };
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(returnOnlyData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 3, name: 'Выручка' })).toBeTruthy();
      expect(screen.getByText('Удержания за возвраты')).toBeTruthy();
    });

    expect(screen.queryByText('За выбранный период не было заказов, продаж и возвратов.')).toBeNull();
  });

  it('does not trigger empty state when all primary sales metrics are 0 but returnedUnits > 0', async () => {
    const physicalReturnOnlyData = {
      ...mockOverviewData,
      grossSales: { currentCents: 0, previousCents: 0, changePercent: null, comparisonState: 'unchanged' as const },
      orders: { current: 0, previous: 0, changePercent: null, comparisonState: 'unchanged' as const },
      unitsSold: { current: 0, previous: 0, changePercent: null, comparisonState: 'unchanged' as const },
      returnDeductions: { currentCents: 0, previousCents: 0 },
      netCommercialEarning: { currentCents: 0, previousCents: 0, changePercent: null, comparisonState: 'unchanged' as const },
      returnedUnits: { current: 3, previous: 0 },
      returnRate: { currentPercent: 0, previousPercent: 0 },
    };
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(physicalReturnOnlyData);

    render(
      <MemoryRouter>
        <OverviewTab from="2026-10-01T00:00:00+03:00" to="2026-10-15T00:00:00+03:00" />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 3, name: 'Выручка' })).toBeTruthy();
      expect(screen.getByText('Возвраты товаров')).toBeTruthy();
    });

    expect(screen.getByText('3 шт.')).toBeTruthy();
    expect(screen.queryByText('За выбранный период не было заказов, продаж и возвратов.')).toBeNull();
  });

  it('formats Y-axis ticks correctly in Russian without duplicate labels or English k', () => {
    // Check that close tick values do not collapse to identical '1k ₽'
    const tick700 = formatTimeseriesYAxisTick(700, 'sales');
    const tick1050 = formatTimeseriesYAxisTick(1050, 'sales');
    const tick1400 = formatTimeseriesYAxisTick(1400, 'sales');

    expect(tick700).toMatch(/700\s*₽/);
    expect(tick1050).toMatch(/1\s*050\s*₽/);
    expect(tick1400).toMatch(/1\s*400\s*₽/);
    expect(tick1050).not.toEqual(tick1400);
    expect(tick1050).not.toContain('k');
    expect(tick1400).not.toContain('k');

    // Integer units / orders
    expect(formatTimeseriesYAxisTick(2, 'orders')).toBe('2');
    expect(formatTimeseriesYAxisTick(15, 'units')).toBe('15');
  });

  it('provides accurate date context and grouping hints for income, orders, and sales', () => {
    const incomeContext = getTimeseriesDateContext('income');
    expect(incomeContext.label).toBe('дата финансовой операции');
    expect(incomeContext.groupingHint).toBe('Группировка по дате финансовой операции');

    const ordersContext = getTimeseriesDateContext('orders');
    expect(ordersContext.label).toBe('дата оплаты');
    expect(ordersContext.groupingHint).toBe('Группировка по дате оплаты');

    const salesContext = getTimeseriesDateContext('sales');
    expect(salesContext.label).toBe('дата доставки');
    expect(salesContext.groupingHint).toBe('Группировка по дате доставки');

    const unitsContext = getTimeseriesDateContext('units');
    expect(unitsContext.label).toBe('дата доставки');
    expect(unitsContext.groupingHint).toBe('Группировка по дате доставки');
  });
});
