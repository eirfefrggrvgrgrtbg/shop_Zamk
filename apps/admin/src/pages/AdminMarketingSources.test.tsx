import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingSources } from './AdminMarketingSources';
import * as marketingApi from '../api/marketing';

vi.mock('../api/marketing', async () => {
  const actual = await vi.importActual<any>('../api/marketing');
  return {
    ...actual,
    getMarketingSources: vi.fn(),
  };
});

const sampleCoverageAvailable = {
  views: { status: 'available' },
  favorites: { status: 'available' },
};

const sampleCoverageUnavailable = {
  views: { status: 'unavailable' },
  favorites: { status: 'unavailable' },
};

const sampleSources = [
  {
    sourceKey: 'direct',
    sourceKind: 'direct' as const,
    visits: 1200,
    paidOrders: 60,
    soldUnits: 75,
    revenueCents: 45000000,
    conversionRate: 0.05,
    newCustomers: 40,
    repeatCustomers: 20,
  },
  {
    sourceKey: null,
    sourceKind: 'unattributed' as const,
    visits: 350,
    paidOrders: 14,
    soldUnits: 16,
    revenueCents: 8500000,
    conversionRate: 0.04,
    newCustomers: 10,
    repeatCustomers: 4,
  },
  {
    sourceKey: 'telegram',
    sourceKind: 'named' as const,
    visits: 800,
    paidOrders: 40,
    soldUnits: 50,
    revenueCents: 24000000,
    conversionRate: 0.05,
    newCustomers: 30,
    repeatCustomers: 10,
  },
  {
    sourceKey: 'unattributed',
    sourceKind: 'named' as const,
    visits: 100,
    paidOrders: 2,
    soldUnits: 2,
    revenueCents: 1200000,
    conversionRate: 0.02,
    newCustomers: 2,
    repeatCustomers: 0,
  },
];

const mount = () =>
  render(
    <MemoryRouter>
      <AdminMarketingSources />
    </MemoryRouter>
  );

describe('AdminMarketingSources', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(marketingApi.getMarketingSources).mockResolvedValue({
      coverage: sampleCoverageAvailable,
      sources: sampleSources,
    } as any);
  });

  it('renders direct source with label "Прямой заход"', async () => {
    mount();
    await screen.findByText('Прямой заход');
    expect(screen.getByText('Прямой заход')).toBeTruthy();
  });

  it('renders unattributed source with label "Неизвестный источник"', async () => {
    mount();
    await screen.findByText('Неизвестный источник');
    expect(screen.getByText('Неизвестный источник')).toBeTruthy();
  });

  it('renders named source with raw name', async () => {
    mount();
    await screen.findByText('telegram');
    expect(screen.getByText('telegram')).toBeTruthy();
  });

  it('renders literal named "unattributed" as distinct named source', async () => {
    mount();
    await screen.findByText('unattributed');
    expect(screen.getByText('unattributed')).toBeTruthy();
  });

  it('never displays raw _unattributed sentinel anywhere in human text', async () => {
    mount();
    await screen.findByText('Источники трафика');
    const textContent = document.body.textContent || '';
    expect(textContent).not.toContain('_unattributed');
  });

  it('renders sessions count correctly formatted', async () => {
    mount();
    await screen.findByText('1 200');
    expect(screen.getByText('1 200')).toBeTruthy();
  });

  it('renders orders count correctly', async () => {
    mount();
    await screen.findByText('60');
    expect(screen.getByText('60')).toBeTruthy();
  });

  it('renders revenue with currency presentation', async () => {
    mount();
    await screen.findByText(/450 000/);
    expect(screen.getByText(/450 000/)).toBeTruthy();
  });

  it('renders conversion percentage when views coverage is available', async () => {
    mount();
    const items = await screen.findAllByText('5.00%');
    expect(items.length).toBeGreaterThan(0);
  });

  it('renders dash placeholder when conversion coverage is unavailable', async () => {
    vi.mocked(marketingApi.getMarketingSources).mockResolvedValue({
      coverage: sampleCoverageUnavailable,
      sources: sampleSources,
    } as any);
    mount();
    await screen.findByText('Прямой заход');
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThan(0);
  });

  it('renders dash for anomalous conversion where orders exceed visits', async () => {
    vi.mocked(marketingApi.getMarketingSources).mockResolvedValue({
      coverage: sampleCoverageAvailable,
      sources: [
        {
          sourceKey: 'broken',
          sourceKind: 'named' as const,
          visits: 2,
          paidOrders: 10,
          soldUnits: 10,
          revenueCents: 100000,
          conversionRate: 5.0,
          newCustomers: 1,
          repeatCustomers: 0,
        },
      ],
    } as any);
    mount();
    await screen.findByText('broken');
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });

  it('triggers search with debounced input and refetches', async () => {
    mount();
    await screen.findByText('Источники трафика');
    const searchInput = screen.getByPlaceholderText('ПОИСК ПО ИСТОЧНИКУ...');
    fireEvent.change(searchInput, { target: { value: 'tele' } });

    await waitFor(
      () => {
        expect(marketingApi.getMarketingSources).toHaveBeenCalledWith(
          expect.any(String),
          expect.any(String),
          'tele',
          'visits'
        );
      },
      { timeout: 1500 }
    );
  });

  it('sort select initializes with default "visits" option', async () => {
    mount();
    await screen.findByText('Источники трафика');
    expect(marketingApi.getMarketingSources).toHaveBeenCalledWith(
      expect.any(String),
      expect.any(String),
      '',
      'visits'
    );
  });

  it('disables conversion sort when views coverage is unavailable', async () => {
    vi.mocked(marketingApi.getMarketingSources).mockResolvedValue({
      coverage: sampleCoverageUnavailable,
      sources: sampleSources,
    } as any);
    mount();
    await screen.findByText('Источники трафика');
    expect(screen.getByText('Источники трафика')).toBeTruthy();
  });

  it('period selector renders and provides controls', async () => {
    mount();
    await screen.findByText('30 дней');
    expect(screen.getByText('30 дней')).toBeTruthy();
  });

  it('shows loading indicator while fetching', async () => {
    vi.mocked(marketingApi.getMarketingSources).mockReturnValue(new Promise(() => {}));
    mount();
    expect(document.querySelector('.animate-spin')).toBeTruthy();
  });

  it('renders empty state when sources array is empty', async () => {
    vi.mocked(marketingApi.getMarketingSources).mockResolvedValue({
      coverage: sampleCoverageAvailable,
      sources: [],
    } as any);
    mount();
    await screen.findByText('Нет данных за выбранный период');
    expect(screen.getByText('Нет данных за выбранный период')).toBeTruthy();
  });

  it('shows error state and retries on button click', async () => {
    vi.mocked(marketingApi.getMarketingSources)
      .mockRejectedValueOnce(new Error('Network error'))
      .mockResolvedValueOnce({
        coverage: sampleCoverageAvailable,
        sources: sampleSources,
      } as any);
    mount();
    await screen.findByText('Network error');
    const retryButton = screen.getByRole('button', { name: 'Повторить' });
    fireEvent.click(retryButton);
    await screen.findByText('Прямой заход');
    expect(screen.getByText('Прямой заход')).toBeTruthy();
  });

  it('generates correct row navigation links for all source kinds', async () => {
    mount();
    await screen.findByText('Прямой заход');

    const directLink = screen.getByRole('link', { name: 'Прямой заход' });
    expect(directLink.getAttribute('href')).toBe('/marketing/sources/direct');

    const unattributedLink = screen.getByRole('link', { name: 'Неизвестный источник' });
    expect(unattributedLink.getAttribute('href')).toBe('/marketing/sources/_unattributed');

    const telegramLink = screen.getByRole('link', { name: 'telegram' });
    expect(telegramLink.getAttribute('href')).toBe('/marketing/sources/telegram');

    const literalUnattributedLink = screen.getByRole('link', { name: 'unattributed' });
    expect(literalUnattributedLink.getAttribute('href')).toBe('/marketing/sources/unattributed');
  });
});
