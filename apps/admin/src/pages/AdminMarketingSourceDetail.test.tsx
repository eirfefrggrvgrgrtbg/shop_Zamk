import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminMarketingSourceDetail } from './AdminMarketingSourceDetail';
import * as marketingApi from '../api/marketing';

vi.mock('../api/marketing', async () => {
  const actual = await vi.importActual<any>('../api/marketing');
  return {
    ...actual,
    getMarketingSourceDetail: vi.fn(),
  };
});

const sampleCoverageAvailable = {
  views: { status: 'available' },
  favorites: { status: 'available' },
};

const sampleDetailDirect = {
  source: {
    sourceKey: 'direct',
    sourceKind: 'direct' as const,
    visits: 1200,
    paidOrders: 60,
    soldUnits: 75,
    revenueCents: 45000000,
    conversionRate: 0.05,
    conversionRateBps: 500,
    newCustomers: 40,
    repeatCustomers: 20,
  },
  coverage: sampleCoverageAvailable,
  trend: [
    { date: '2026-10-01', revenueCents: 15000000, paidOrders: 20 },
    { date: '2026-10-02', revenueCents: 30000000, paidOrders: 40 },
  ],
  topProducts: [
    {
      productId: 'prod_12345678abcdef01',
      name: 'Шелковое платье',
      designerName: 'Acme Studio',
      purchases: 25,
      soldUnits: 25,
      revenueCents: 20000000,
      views: 300,
      favorites: 40,
      addToCart: 30,
      returns: 0,
      conversionRate: 0.08,
      primaryImage: 'https://example.com/dress.jpg',
    },
  ],
  topDesigners: [
    {
      designerId: 'dsgn_12345678',
      designerName: 'Acme Studio',
      productsCount: 3,
      primaryImage: 'https://example.com/designer.jpg',
      views: 500,
      favorites: 60,
      addToCart: 40,
      purchases: 35,
      soldUnits: 40,
      revenueCents: 28000000,
      conversionRate: 0.07,
      returns: 1,
    },
  ],
  campaigns: [
    {
      campaignId: 'cmp_12345',
      name: 'Осенний сейл',
      visits: 600,
      paidOrders: 30,
      revenueCents: 22000000,
    },
  ],
};

const sampleDetailUnattributed = {
  source: {
    sourceKey: null,
    sourceKind: 'unattributed' as const,
    visits: 400,
    paidOrders: 15,
    soldUnits: 18,
    revenueCents: 9000000,
    conversionRate: 0.0375,
    conversionRateBps: 375,
    newCustomers: 12,
    repeatCustomers: 3,
  },
  coverage: sampleCoverageAvailable,
  trend: [
    { date: '2026-10-01', revenueCents: 4500000, paidOrders: 7 },
    { date: '2026-10-02', revenueCents: 4500000, paidOrders: 8 },
  ],
  topProducts: [],
  topDesigners: [],
  campaigns: [
    {
      campaignId: null,
      name: 'Без кампании',
      visits: 400,
      paidOrders: 15,
      revenueCents: 9000000,
    },
  ],
};

const sampleDetailNamed = {
  source: {
    sourceKey: 'telegram',
    sourceKind: 'named' as const,
    visits: 900,
    paidOrders: 45,
    soldUnits: 55,
    revenueCents: 32000000,
    conversionRate: 0.05,
    conversionRateBps: 500,
    newCustomers: 35,
    repeatCustomers: 10,
  },
  coverage: sampleCoverageAvailable,
  trend: [],
  topProducts: [],
  topDesigners: [],
  campaigns: [],
};

const mount = (route: string) =>
  render(
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route path="/marketing/sources/:source" element={<AdminMarketingSourceDetail />} />
      </Routes>
    </MemoryRouter>
  );

describe('AdminMarketingSourceDetail', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    cleanup();
  });

  it('renders direct source with label "Прямой заход"', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    await screen.findByRole('heading', { level: 1, name: /Прямой заход/i });
    expect(screen.getAllByText('Прямой заход').length).toBeGreaterThan(0);
  });

  it('renders unattributed detail using sentinel without leaking sentinel to UI', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailUnattributed as any);
    mount('/marketing/sources/_unattributed');
    await screen.findByRole('heading', { level: 1, name: /Неизвестный источник/i });
    expect(screen.getAllByText('Неизвестный источник').length).toBeGreaterThan(0);
    const textContent = document.body.textContent || '';
    expect(textContent).not.toContain('_unattributed');
  });

  it('renders named source detail with raw name', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailNamed as any);
    mount('/marketing/sources/telegram');
    await screen.findByRole('heading', { level: 1, name: /telegram/i });
    expect(screen.getAllByText('telegram').length).toBeGreaterThan(0);
  });

  it('renders summary KPI metrics (sessions, orders, conversion, revenue)', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    await screen.findAllByText('1 200');
    expect(screen.getAllByText('1 200').length).toBeGreaterThan(0);
    expect(screen.getAllByText('60').length).toBeGreaterThan(0);
    expect(screen.getAllByText('5.00%').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/450 000/).length).toBeGreaterThan(0);
  });

  it('renders trend chart section', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    await screen.findAllByText('Динамика');
    expect(screen.getAllByText('Динамика').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Выручка' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Заказы' })).toBeTruthy();
  });

  it('renders top products with names and purchases, omitting UUIDs', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    await screen.findAllByText('Шелковое платье');
    expect(screen.getAllByText('Шелковое платье').length).toBeGreaterThan(0);
    expect(screen.getAllByText('25').length).toBeGreaterThan(0);
    const textContent = document.body.textContent || '';
    expect(textContent).not.toContain('prod_12345678abcdef01');
  });

  it('renders top designers with name and purchases, omitting UUIDs', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    const designerNames = await screen.findAllByText('Acme Studio');
    expect(designerNames.length).toBeGreaterThan(0);
    expect(screen.getAllByText('35').length).toBeGreaterThan(0);
    const textContent = document.body.textContent || '';
    expect(textContent).not.toContain('dsgn_12345678');
  });

  it('renders campaigns section for source', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    await screen.findAllByText('Осенний сейл');
    expect(screen.getAllByText('Осенний сейл').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/600/).length).toBeGreaterThan(0);
  });

  it('renders explicit campaign under unattributed source', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailUnattributed as any);
    mount('/marketing/sources/_unattributed');
    await screen.findAllByText('Без кампании');
    expect(screen.getAllByText('Без кампании').length).toBeGreaterThan(0);
  });

  it('renders nested empty states when products, designers, or campaigns are empty', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailNamed as any);
    mount('/marketing/sources/telegram');
    await screen.findAllByText('Нет кампаний в этом источнике');
    expect(screen.getAllByText('Нет кампаний в этом источнике').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Нет данных о дизайнерах').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Нет данных о товарах').length).toBeGreaterThan(0);
  });

  it('period control is rendered in header', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockResolvedValue(sampleDetailDirect as any);
    mount('/marketing/sources/direct');
    await screen.findAllByText('30 дней');
    expect(screen.getAllByText('30 дней').length).toBeGreaterThan(0);
  });

  it('shows loading spinner when data is in flight', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockReturnValue(new Promise(() => {}));
    mount('/marketing/sources/direct');
    expect(screen.getAllByText('Загрузка данных').length).toBeGreaterThan(0);
  });

  it('renders error state and link back to sources list on failure', async () => {
    vi.mocked(marketingApi.getMarketingSourceDetail).mockRejectedValue(new Error('Source fetch failed'));
    mount('/marketing/sources/direct');
    await screen.findAllByText('Source fetch failed');
    expect(screen.getAllByText('Source fetch failed').length).toBeGreaterThan(0);
    expect(screen.getByRole('link', { name: 'К списку источников' })).toBeTruthy();
  });
});
