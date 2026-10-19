import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { FinanceTab } from './FinanceTab';
import * as analyticsApi from '../../api/selleranalytics';
import type { OverviewResponse } from '../../api/selleranalytics';

vi.mock('../../api/selleranalytics', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/selleranalytics')>();
  return {
    ...actual,
    getAnalyticsOverview: vi.fn(),
  };
});

function createMockData(overrides: Partial<OverviewResponse> = {}): OverviewResponse {
  return {
    period: {
      from: '2026-07-19T00:00:00+03:00',
      to: '2026-10-17T00:00:00+03:00',
      timezone: 'Europe/Moscow',
    },
    hasHistoricalSales: true,
    grossSales: { currentCents: 8038400, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
    orders: { current: 10, previous: 0, changePercent: 0, comparisonState: 'unchanged' },
    deliveredOrders: { current: 8, previous: 0 },
    unitsSold: { current: 12, previous: 0, changePercent: 0, comparisonState: 'unchanged' },
    averageOrderValue: { currentCents: 1004800, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
    commission: { currentCents: 723456, previousCents: 0 },
    sellerEarningBeforeReturns: { currentCents: 7314944, previousCents: 0 },
    returnDeductions: { currentCents: -3546270, previousCents: 0 },
    otherAdjustments: { currentCents: 1182090, previousCents: 0 },
    netCommercialEarning: { currentCents: 3768674, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
    returnedUnits: { current: 3, previous: 0 },
    returnRate: { currentPercent: 25.0, previousPercent: 0 },
    timeseries: [],
    insights: [],
    ...overrides,
  };
}

function normalize(text: string | null | undefined): string {
  return (text || '').replace(/[\s\u00a0]+/g, ' ').trim();
}

describe('FinanceTab Component & Formula Contract Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state initially', () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockReturnValue(new Promise(() => {}));
    const { container } = render(<FinanceTab from="2026-07-19" to="2026-10-17" />);
    expect(container.querySelector('.animate-pulse')).toBeTruthy();
  });

  it('renders error state on fetch failure and allows retry', async () => {
    vi.mocked(analyticsApi.getAnalyticsOverview).mockRejectedValueOnce(new Error('Network error'));
    render(<FinanceTab from="2026-07-19" to="2026-10-17" />);

    await waitFor(() => {
      expect(screen.getByText('Не удалось загрузить финансовую аналитику')).toBeTruthy();
      expect(screen.getByText('Network error')).toBeTruthy();
    });

    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValueOnce(createMockData());
    screen.getByText('Повторить попытку').click();

    await waitFor(() => {
      expect(screen.getByText('Структура доходов')).toBeTruthy();
    });
  });

  it('Scenario 1: No returns, no adjustments — exact waterfall equality', async () => {
    const data = createMockData({
      grossSales: { currentCents: 1000000, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
      commission: { currentCents: 100000, previousCents: 0 },
      sellerEarningBeforeReturns: { currentCents: 900000, previousCents: 0 },
      returnDeductions: { currentCents: 0, previousCents: 0 },
      otherAdjustments: { currentCents: 0, previousCents: 0 },
      netCommercialEarning: { currentCents: 900000, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
      returnedUnits: { current: 0, previous: 0 },
      returnRate: { currentPercent: 0, previousPercent: 0 },
    });
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(data);

    render(<FinanceTab from="2026-07-19" to="2026-10-17" />);

    await waitFor(() => {
      expect(normalize(screen.getByTestId('gross-sales').textContent)).toBe('10 000 ₽');
      expect(normalize(screen.getByTestId('commission').textContent)).toBe('− 1 000 ₽');
      expect(normalize(screen.getByTestId('seller-earning').textContent)).toBe('9 000 ₽');
      expect(normalize(screen.getByTestId('return-deductions').textContent)).toBe('− 0 ₽');
      expect(normalize(screen.getByTestId('net-commercial-earning').textContent)).toBe('9 000 ₽');
    });

    // Other adjustments card should NOT be rendered when otherAdjustments is 0
    expect(screen.queryByTestId('other-adjustments-card')).toBeNull();
  });

  it('Scenario 2: Returns only (no other adjustments) — deductions correctly subtotaled', async () => {
    const data = createMockData({
      grossSales: { currentCents: 1000000, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
      commission: { currentCents: 100000, previousCents: 0 },
      sellerEarningBeforeReturns: { currentCents: 900000, previousCents: 0 },
      returnDeductions: { currentCents: -300000, previousCents: 0 },
      otherAdjustments: { currentCents: 0, previousCents: 0 },
      netCommercialEarning: { currentCents: 600000, previousCents: 0, changePercent: 0, comparisonState: 'unchanged' },
      returnedUnits: { current: 1, previous: 0 },
      returnRate: { currentPercent: 10.0, previousPercent: 0 },
    });
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(data);

    render(<FinanceTab from="2026-07-19" to="2026-10-17" />);

    await waitFor(() => {
      expect(normalize(screen.getByTestId('seller-earning').textContent)).toBe('9 000 ₽');
      expect(normalize(screen.getByTestId('return-deductions').textContent)).toBe('− 3 000 ₽');
      expect(normalize(screen.getByTestId('net-commercial-earning').textContent)).toBe('6 000 ₽');
    });

    expect(screen.queryByTestId('other-adjustments-card')).toBeNull();
  });

  it('Scenario 3: 90-day screenshot case with positive other adjustments (+11 820,90 ₽)', async () => {
    const data = createMockData();
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(data);

    render(<FinanceTab from="2026-07-19" to="2026-10-17" />);

    await waitFor(() => {
      // 1. Core waterfall strictly equals 80 384 - 7 234,56 = 73 149,44 and 73 149,44 - 35 462,70 = 37 686,74
      expect(normalize(screen.getByTestId('gross-sales').textContent)).toBe('80 384 ₽');
      expect(normalize(screen.getByTestId('commission').textContent)).toBe('− 7 234,56 ₽');
      expect(normalize(screen.getByTestId('seller-earning').textContent)).toBe('73 149,44 ₽');
      expect(normalize(screen.getByTestId('return-deductions').textContent)).toBe('− 35 462,7 ₽');
      expect(normalize(screen.getByTestId('net-commercial-earning').textContent)).toBe('37 686,74 ₽');

      // 2. Other adjustments is visually separated in its own dedicated card
      expect(screen.getByTestId('other-adjustments-card')).toBeTruthy();
      expect(normalize(screen.getByTestId('other-adjustments').textContent)).toBe('+ 11 820,9 ₽');
      expect(normalize(screen.getByTestId('total-with-adjustments').textContent)).toBe('49 507,64 ₽');

      // 3. Truthful explanation is present
      expect(screen.getByText(/не входят в коммерческий результат от продаж товаров/i)).toBeTruthy();
    });
  });

  it('Scenario 4: Returns with negative other adjustments (e.g. penalty/debit)', async () => {
    const data = createMockData({
      otherAdjustments: { currentCents: -500000, previousCents: 0 },
    });
    vi.mocked(analyticsApi.getAnalyticsOverview).mockResolvedValue(data);

    render(<FinanceTab from="2026-07-19" to="2026-10-17" />);

    await waitFor(() => {
      expect(normalize(screen.getByTestId('net-commercial-earning').textContent)).toBe('37 686,74 ₽');
      expect(screen.getByTestId('other-adjustments-card')).toBeTruthy();
      expect(normalize(screen.getByTestId('other-adjustments').textContent)).toBe('− 5 000 ₽');
      // 37 686,74 - 5 000 = 32 686,74
      expect(normalize(screen.getByTestId('total-with-adjustments').textContent)).toBe('32 686,74 ₽');
    });
  });
});
