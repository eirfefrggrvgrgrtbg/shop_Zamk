import { Component, type ReactNode } from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, act, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingOverview } from './AdminMarketingOverview';
import * as api from '../api/marketing';
import { customRange } from '../components/marketing/MarketingPeriodControl';

vi.mock('../api/marketing');

const emptyOverviewResponse: api.AnalyticsOverviewResponse = {
  current: {
    visits: 0,
    paidOrders: 0,
    soldUnits: 0,
    revenueCents: 0,
    aovCents: 0,
    conversionRateBps: 0,
    newCustomers: 0,
    repeatCustomers: 0,
    returnsCount: 0,
    returnedUnits: 0,
    returnedAmountCents: 0,
  },
  previous: {
    visits: 0,
    paidOrders: 0,
    soldUnits: 0,
    revenueCents: 0,
    aovCents: 0,
    conversionRateBps: 0,
    newCustomers: 0,
    repeatCustomers: 0,
    returnsCount: 0,
    returnedUnits: 0,
    returnedAmountCents: 0,
  },
};

const populatedOverviewResponse: api.AnalyticsOverviewResponse = {
  current: {
    visits: 1250,
    paidOrders: 50,
    soldUnits: 80,
    revenueCents: 25000000,
    aovCents: 500000,
    conversionRateBps: 400,
    newCustomers: 35,
    repeatCustomers: 15,
    returnsCount: 2,
    returnedUnits: 3,
    returnedAmountCents: 1500000,
  },
  previous: {
    visits: 1000,
    paidOrders: 40,
    soldUnits: 60,
    revenueCents: 20000000,
    aovCents: 500000,
    conversionRateBps: 400,
    newCustomers: 30,
    repeatCustomers: 10,
    returnsCount: 1,
    returnedUnits: 1,
    returnedAmountCents: 500000,
  },
};

const populatedSourcesResponse: api.AnalyticsSourcesResponse = {
  sources: [
    {
      source: 'Direct',
      visits: 300,
      paidOrders: 15,
      revenueCents: 7500000,
    },
    {
      source: 'Unattributed',
      visits: 100,
      paidOrders: 5,
      revenueCents: 2500000,
    },
    {
      source: 'telegram',
      visits: 850,
      paidOrders: 30,
      revenueCents: 15000000,
    },
  ],
};

const populatedCampaignsResponse: api.AnalyticsCampaignsResponse = {
  campaigns: [
    {
      campaignId: '11111111-1111-4111-8111-111111111111',
      name: 'Summer Sale 2026',
      visits: 850,
      paidOrders: 30,
      revenueCents: 15000000,
    },
    {
      campaignId: null,
      name: 'Без кампании',
      visits: 400,
      paidOrders: 20,
      revenueCents: 10000000,
    },
  ],
};


class CrashProbe extends Component<{ children: ReactNode }, { error: string }> {
  state = { error: '' };
  static getDerivedStateFromError(error: Error) { return { error: error.message }; }
  render() { return this.state.error ? <div role="alert">{this.state.error}</div> : this.props.children; }
}
const mount = () => render(<MemoryRouter><CrashProbe><aside>Admin shell</aside><AdminMarketingOverview /></CrashProbe></MemoryRouter>);
const revenue = () => screen.getByTestId('marketing-revenue').textContent;
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.getMarketingOverview).mockResolvedValue(populatedOverviewResponse);
  vi.mocked(api.getMarketingSources).mockResolvedValue(populatedSourcesResponse);
  vi.mocked(api.getMarketingCampaigns).mockResolvedValue(populatedCampaignsResponse);
  if (typeof HTMLDialogElement !== "undefined") { HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); }; }
});
afterEach(cleanup);

describe('Marketing overview range regression and hierarchy', () => {
  it('A/B: Today → 7d → 30d keeps the shell and renders each successful range', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');
    for (const name of ['Сегодня', '7 дней', '30 дней']) {
      fireEvent.click(screen.getByRole('button', { name }));
      await waitFor(() => expect(screen.queryByTestId('overview-loading')).toBeNull());
      expect(screen.getByRole('button', { name }).getAttribute('aria-pressed')).toBe('true');
      expect(screen.getByText('Admin shell')).toBeTruthy();
      expect(revenue()).toContain('250');
    }
    const calls = vi.mocked(api.getMarketingOverview).mock.calls;
    expect(calls).toHaveLength(4);
    expect(Date.parse(calls[2][1]) - Date.parse(calls[2][0])).toBe(7 * 86400000);
    expect(Date.parse(calls[3][1]) - Date.parse(calls[3][0])).toBe(30 * 86400000);
  });

  it('root cause regression: empty Go slices after a range change cannot throw null.map', async () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {});
    mount();
    await screen.findByText('Summer Sale 2026');
    vi.mocked(api.getMarketingSources).mockResolvedValue({ sources: null } as unknown as api.AnalyticsSourcesResponse);
    vi.mocked(api.getMarketingCampaigns).mockResolvedValue({ campaigns: null } as unknown as api.AnalyticsCampaignsResponse);
    fireEvent.click(screen.getByRole('button', { name: '7 дней' }));
    await waitFor(() => expect(screen.getAllByText('Нет данных за этот период')).toHaveLength(2));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByText('Admin shell')).toBeTruthy();
    expect(errors).not.toHaveBeenCalled();
    errors.mockRestore();
  });

  it('C: custom dates are applied atomically with an exclusive next-day upper bound via custom calendar', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');
    fireEvent.click(screen.getByRole('button', { name: 'Период' }));

    // Ensure native date input is NOT used
    expect(screen.queryByLabelText('Начало')).toBeNull();
    expect(screen.queryByLabelText('Окончание')).toBeNull();

    // Custom calendar months are visible
    expect(screen.getByRole('heading', { name: 'Период аналитики' })).toBeTruthy();
    expect(screen.getByTestId('calendar-start-summary')).toBeTruthy();
    expect(screen.getByTestId('calendar-end-summary')).toBeTruthy();

    // Select start and end days via calendar buttons
    const day1 = screen.getByRole('button', { name: '2026-09-01' });
    const day7 = screen.getByRole('button', { name: '2026-09-07' });
    fireEvent.click(day1);
    fireEvent.click(day7);

    expect(api.getMarketingOverview).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: 'Применить' }));
    await waitFor(() => expect(api.getMarketingOverview).toHaveBeenLastCalledWith('2026-09-01T00:00:00.000Z', '2026-09-08T00:00:00.000Z'));
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('custom calendar: cancel preserves previous range, month navigation works', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');
    fireEvent.click(screen.getByRole('button', { name: 'Период' }));

    // Initial month is September 2026
    expect(screen.getByText(/Сентябрь 2026/)).toBeTruthy();

    // Month navigation
    const prevBtn = screen.getByRole('button', { name: 'Предыдущий месяц' });
    const nextBtn = screen.getAllByRole('button', { name: 'Следующий месяц' })[0];
    fireEvent.click(prevBtn);
    expect(screen.getByText(/Август 2026/)).toBeTruthy();
    fireEvent.click(nextBtn);
    expect(screen.getByText(/Сентябрь 2026/)).toBeTruthy();

    // Clicking Cancel does not call API and preserves state
    fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    expect(api.getMarketingOverview).toHaveBeenCalledTimes(1);
  });

  it('invalid custom dates stay local and never serialize an invalid Date', async () => {
    expect(customRange('', '')).toBeNull();
    expect(customRange('bad', '2026-01-01')).toBeNull();
    expect(customRange('2026-02-30', '2026-03-02')).toBeNull();
    expect(customRange('2026-03-03', '2026-03-02')).toBeNull();
  });

  it('D/W: failed refresh preserves the previous snapshot and retries the selected range', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');
    const oldRevenue = revenue();
    const pending = deferred<api.AnalyticsOverviewResponse>();
    vi.mocked(api.getMarketingOverview).mockReturnValueOnce(pending.promise);
    fireEvent.click(screen.getByRole('button', { name: '7 дней' }));
    expect(revenue()).toBe(oldRevenue);
    expect(screen.getByText('Обновляем...')).toBeTruthy();
    expect(screen.getByTestId('marketing-hero')).toBeTruthy();
    await act(async () => pending.reject(new Error('offline')));
    expect(screen.getByRole('alert').textContent).toContain('Не удалось обновить данные');
    expect(revenue()).toBe(oldRevenue);
    vi.mocked(api.getMarketingOverview).mockResolvedValue(emptyOverviewResponse);
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
    expect(revenue()).toContain('0');
    expect(api.getMarketingOverview).toHaveBeenCalledTimes(3);
  });

  it.each(['resolve', 'reject'] as const)('E: stale request %s cannot replace newer values, errors or loading', async result => {
    mount();
    await screen.findByText('Summer Sale 2026');
    const stale = deferred<api.AnalyticsOverviewResponse>();
    vi.mocked(api.getMarketingOverview).mockReturnValueOnce(stale.promise).mockResolvedValueOnce(emptyOverviewResponse);
    fireEvent.click(screen.getByRole('button', { name: '7 дней' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сегодня' }));
    await waitFor(() => expect(screen.queryByTestId('overview-loading')).toBeNull());
    const latest = revenue();
    await act(async () => { if (result === 'resolve') stale.resolve(populatedOverviewResponse); else stale.reject(new Error('stale failure')); });
    expect(revenue()).toBe(latest);
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.queryByTestId('overview-loading')).toBeNull();
  });

  it('F: null overview preserves successful data with a recoverable error', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');
    const old = revenue();
    vi.mocked(api.getMarketingOverview).mockResolvedValue(null as unknown as api.AnalyticsOverviewResponse);
    fireEvent.click(screen.getByRole('button', { name: 'Сегодня' }));
    await screen.findByRole('alert');
    expect(revenue()).toBe(old);
    expect(screen.getByText('Admin shell')).toBeTruthy();
  });

  it('G/H/V: empty zero metrics and zero previous revenue have no NaN/Infinity or invented delta', async () => {
    vi.mocked(api.getMarketingOverview).mockResolvedValue(emptyOverviewResponse);
    vi.mocked(api.getMarketingSources).mockResolvedValue({ sources: [] });
    vi.mocked(api.getMarketingCampaigns).mockResolvedValue({ campaigns: [] });
    mount();
    await waitFor(() => expect(screen.getAllByText('Нет данных за этот период')).toHaveLength(2));
    expect(document.body.textContent).not.toMatch(/NaN|Infinity/);
    expect(revenue()).toContain('0');
    expect(screen.getAllByText('0.00%').length).toBeGreaterThan(0);
  });

  it('H: partial/null/nonfinite metric values render safely', async () => {
    vi.mocked(api.getMarketingOverview).mockResolvedValue({ current: { revenueCents: Infinity, paidOrders: null, conversionRateBps: NaN }, previous: {} } as unknown as api.AnalyticsOverviewResponse);
    mount();
    await screen.findByText('Summer Sale 2026');
    expect(revenue()).toBe('—');
    expect(document.body.textContent).not.toMatch(/NaN|Infinity/);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('N–R: revenue hierarchy, Russian tabs, compact quality, ranked sources and campaign performance', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');
    expect(screen.getByRole('heading', { name: 'Качество продаж' })).toBeTruthy();
    const navigation = screen.getByRole('navigation', { name: 'Разделы маркетинга' });
    expect(within(navigation).getByRole('link', { name: 'Сводка' })).toBeTruthy();
    expect(within(navigation).getByRole('link', { name: 'Кампании' })).toBeTruthy();
    expect(document.body.textContent).not.toContain('Overview');
    const sources = screen.getByRole('region', { name: 'Показатели источников' });
    expect(within(sources).getAllByRole('row')[0].textContent).toContain('telegram');
    expect(within(sources).getByText('68%')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Лучшие кампании' })).toBeTruthy();
    const topCampaigns = screen.getByRole('region', { name: 'Показатели кампаний' });
    expect(within(topCampaigns).getByText('Summer Sale 2026')).toBeTruthy();
    expect(within(topCampaigns).getByText('30')).toBeTruthy(); expect(within(topCampaigns).getByText('Заказы')).toBeTruthy();
    expect(within(topCampaigns).getByText('3.52%')).toBeTruthy(); expect(within(topCampaigns).getByText('Конверсия')).toBeTruthy();
    expect(within(topCampaigns).getByText(/150\s*000\s*₽/)).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Summer Sale 2026' }).getAttribute('href')).toContain('/marketing/campaigns/11111111');
  });

  it('W: initial failure keeps dashboard geometry and retry control visible', async () => {
    vi.mocked(api.getMarketingOverview).mockRejectedValue(new Error('offline'));
    mount();
    await screen.findByRole('alert');
    expect(screen.getByTestId('marketing-hero')).toBeTruthy();
    expect(screen.getByText('Admin shell')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeTruthy();
    expect(screen.getByRole('region', { name: 'Показатели источников' })).toBeTruthy();
  });

  it('A: Overview core data loads with trend success', async () => {
    vi.mocked(api.getMarketingTrend).mockResolvedValue({
      trend: [
        { date: '2026-05-01', revenueCents: 5000000, paidOrders: 10 },
        { date: '2026-05-02', revenueCents: 10000000, paidOrders: 20 },
      ],
    });
    mount();
    await screen.findByText('Summer Sale 2026');
    expect(revenue()).toContain('250');
    expect(screen.getAllByText('50').length).toBeGreaterThan(0);
    expect(screen.queryByTestId('trend-error-area')).toBeNull();
    expect(screen.getByText('2026-05-01')).toBeTruthy();
    expect(screen.getByText('2026-05-02')).toBeTruthy();
  });

  it('B: trend failure does NOT blank successful core analytics', async () => {
    vi.mocked(api.getMarketingTrend).mockRejectedValue(new Error('trend failure'));
    mount();
    await screen.findByText('Summer Sale 2026');
    // Core analytics remain fully visible
    expect(revenue()).toContain('250');
    expect(screen.getAllByText('50').length).toBeGreaterThan(0);
    expect(screen.getByText('telegram')).toBeTruthy();
    // Top error banner is NOT shown
    expect(screen.queryByRole('alert')).toBeNull();
    // Trend area shows isolated error with retry button
    const trendErr = screen.getByTestId('trend-error-area');
    expect(within(trendErr).getByText('Не удалось загрузить динамику')).toBeTruthy();
    expect(within(trendErr).getByRole('button', { name: 'Повторить' })).toBeTruthy();
  });

  it('C: core Overview failure shows error banner and keeps geometry', async () => {
    vi.mocked(api.getMarketingOverview).mockRejectedValue(new Error('core failure'));
    mount();
    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain('Не удалось обновить данные');
    expect(screen.getByTestId('marketing-hero')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeTruthy();
  });

  it('D: isolated trend retry succeeds and renders trend without reloading core', async () => {
    vi.mocked(api.getMarketingTrend).mockRejectedValueOnce(new Error('trend failure'));
    mount();
    await screen.findByText('Summer Sale 2026');
    const trendErr = screen.getByTestId('trend-error-area');
    expect(within(trendErr).getByText('Не удалось загрузить динамику')).toBeTruthy();

    vi.mocked(api.getMarketingTrend).mockResolvedValueOnce({
      trend: [
        { date: '2026-05-01', revenueCents: 5000000, paidOrders: 10 },
      ],
    });
    fireEvent.click(within(trendErr).getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(screen.queryByTestId('trend-error-area')).toBeNull());
    expect(screen.getByText('2026-05-01')).toBeTruthy();
    expect(revenue()).toContain('250');
  });

  it('trend chart: revenue is default, allows switching to orders with integer scale and human dates', async () => {
    vi.mocked(api.getMarketingTrend).mockResolvedValue({
      trend: [
        { date: '2026-05-01', revenueCents: 5000000, paidOrders: 10 },
        { date: '2026-05-02', revenueCents: 10000000, paidOrders: 20 },
      ],
    });
    mount();
    await screen.findByText('Summer Sale 2026');

    // Heading and switcher
    expect(screen.getByRole('heading', { name: /Динамика/ })).toBeTruthy();
    const revBtn = screen.getByRole('button', { name: 'Выручка' });
    const ordBtn = screen.getByRole('button', { name: 'Заказы' });

    // Revenue is default
    expect(revBtn.getAttribute('aria-pressed')).toBe('true');
    expect(ordBtn.getAttribute('aria-pressed')).toBe('false');
    expect(screen.getByText('Выручка по дням')).toBeTruthy();

    // Human-readable dates
    expect(screen.getByText('1 май')).toBeTruthy();
    expect(screen.getByText('2 май')).toBeTruthy();

    // Switch to Orders
    fireEvent.click(ordBtn);
    expect(ordBtn.getAttribute('aria-pressed')).toBe('true');
    expect(revBtn.getAttribute('aria-pressed')).toBe('false');
    expect(screen.getByText('Заказы по дням')).toBeTruthy();
  });

  it('trend chart: zero data renders baseline and restrained empty message per mode', async () => {
    vi.mocked(api.getMarketingTrend).mockResolvedValue({
      trend: [
        { date: '2026-05-01', revenueCents: 0, paidOrders: 0 },
        { date: '2026-05-02', revenueCents: 0, paidOrders: 0 },
      ],
    });
    mount();
    await screen.findByText('Summer Sale 2026');

    // Revenue mode zero state
    expect(screen.getByText('Нет выручки за выбранный период')).toBeTruthy();

    // Switch to Orders mode
    fireEvent.click(screen.getByRole('button', { name: 'Заказы' }));
    expect(screen.getByText('Нет заказов за выбранный период')).toBeTruthy();
  });

  it('anomalous conversion coverage (visits < paidOrders): renders warning banner and prevents absurd percentage in KPI', async () => {
    vi.mocked(api.getMarketingOverview).mockResolvedValue({
      current: {
        visits: 2,
        paidOrders: 8,
        soldUnits: 10,
        revenueCents: 4000000,
        aovCents: 500000,
        conversionRateBps: 40000,
        newCustomers: 6,
        repeatCustomers: 2,
        returnsCount: 0,
        returnedUnits: 0,
        returnedAmountCents: 0,
      },
      previous: emptyOverviewResponse.previous,
    });
    mount();
    await screen.findByText('Summer Sale 2026');

    // KPI shows "—"
    expect(screen.getAllByText('Конверсия').length).toBeGreaterThan(0);
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
    expect(screen.queryByText('400.00%')).toBeNull();
    expect(screen.queryByText('400%')).toBeNull();

    // Warning banner is visible and contains factual counts
    const notice = screen.getByTestId('anomalous-conversion-notice');
    expect(within(notice).getByText('Недостаточно данных')).toBeTruthy();
    expect(within(notice).getByText('Покрытие аналитики низкое')).toBeTruthy();
    expect(within(notice).getByText(/2 сессии на 8 оплаченных заказов/)).toBeTruthy();
    expect(within(notice).getByText(/Недостаточно данных для достоверной конверсии/)).toBeTruthy();
  });

  it('normal conversion coverage (visits >= paidOrders): shows normal percentage and no warning banner', async () => {
    mount();
    await screen.findByText('Summer Sale 2026');

    // Warning banner is not present
    expect(screen.queryByTestId('anomalous-conversion-notice')).toBeNull();

    // Normal conversion shown
    expect(screen.getByText('4.00%')).toBeTruthy();
  });

  it('zero sessions with paid orders: treats coverage as insufficient and renders warning banner', async () => {
    vi.mocked(api.getMarketingOverview).mockResolvedValue({
      current: {
        visits: 0,
        paidOrders: 5,
        soldUnits: 5,
        revenueCents: 2500000,
        aovCents: 500000,
        conversionRateBps: 0,
        newCustomers: 5,
        repeatCustomers: 0,
        returnsCount: 0,
        returnedUnits: 0,
        returnedAmountCents: 0,
      },
      previous: emptyOverviewResponse.previous,
    });
    mount();
    await screen.findByText('Summer Sale 2026');

    // KPI shows "—"
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);

    // Warning banner is visible with zero visits and 5 orders
    const notice = screen.getByTestId('anomalous-conversion-notice');
    expect(within(notice).getByText('Недостаточно данных')).toBeTruthy();
    expect(within(notice).getByText('Покрытие аналитики низкое')).toBeTruthy();
    expect(within(notice).getByText(/0 сессий на 5 оплаченных заказов/)).toBeTruthy();
    expect(within(notice).getByText(/Недостаточно данных для достоверной конверсии/)).toBeTruthy();
  });
});
