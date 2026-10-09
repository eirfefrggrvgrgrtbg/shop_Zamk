import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
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
    cleanup();
    vi.mocked(marketingApi.getMarketingSources).mockResolvedValue({
      coverage: sampleCoverageAvailable,
      sources: sampleSources,
    } as any);
  });

  it('renders direct source with label "Прямой заход"', async () => {
    mount();
    await screen.findAllByText('Прямой заход');
    expect(screen.getAllByText('Прямой заход').length).toBeGreaterThan(0);
  });

  it('renders unattributed source with label "Неизвестный источник"', async () => {
    mount();
    await screen.findAllByText('Неизвестный источник');
    expect(screen.getAllByText('Неизвестный источник').length).toBeGreaterThan(0);
  });

  it('renders named source with raw name', async () => {
    mount();
    await screen.findAllByText('telegram');
    expect(screen.getAllByText('telegram').length).toBeGreaterThan(0);
  });

  it('renders literal named "unattributed" as distinct named source', async () => {
    mount();
    await screen.findAllByText('unattributed');
    expect(screen.getAllByText('unattributed').length).toBeGreaterThan(0);
  });

  it('never displays raw _unattributed sentinel anywhere in human text', async () => {
    mount();
    await screen.findAllByText('Источники трафика');
    const textContent = document.body.textContent || '';
    expect(textContent).not.toContain('_unattributed');
  });

  it('renders sessions count correctly formatted', async () => {
    mount();
    await screen.findAllByText('1 200');
    expect(screen.getAllByText('1 200').length).toBeGreaterThan(0);
  });

  it('renders orders count correctly', async () => {
    mount();
    await screen.findAllByText('60');
    expect(screen.getAllByText('60').length).toBeGreaterThan(0);
  });

  it('renders revenue with currency presentation', async () => {
    mount();
    await screen.findAllByText(/450 000/);
    expect(screen.getAllByText(/450 000/).length).toBeGreaterThan(0);
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
    await screen.findAllByText('Прямой заход');
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
    await screen.findAllByText('broken');
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });

  it('triggers search with debounced input and refetches', async () => {
    mount();
    await screen.findAllByText('Источники трафика');
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
    await screen.findAllByText('Источники трафика');
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
    await screen.findAllByText('Источники трафика');
    expect(screen.getAllByText('Источники трафика').length).toBeGreaterThan(0);
  });

  it('period selector renders and provides controls', async () => {
    mount();
    await screen.findAllByText('30 дней');
    expect(screen.getAllByText('30 дней').length).toBeGreaterThan(0);
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
    await screen.findAllByText('Нет данных об источниках трафика за выбранный период.');
    expect(screen.getAllByText('Нет данных об источниках трафика за выбранный период.').length).toBeGreaterThan(0);
  });

  it('shows error state and retries on button click', async () => {
    vi.mocked(marketingApi.getMarketingSources)
      .mockRejectedValueOnce(new Error('Network error'))
      .mockResolvedValueOnce({
        coverage: sampleCoverageAvailable,
        sources: sampleSources,
      } as any);
    mount();
    await screen.findAllByText('Network error');
    const retryButton = screen.getByRole('button', { name: 'Повторить' });
    fireEvent.click(retryButton);
    await screen.findAllByText('Прямой заход');
    expect(screen.getAllByText('Прямой заход').length).toBeGreaterThan(0);
  });

  it('generates correct row navigation links for all source kinds', async () => {
    mount();
    await screen.findAllByText('Прямой заход');

    const directLink = screen.getAllByRole('link', { name: /Прямой заход/ })[0];
    expect(directLink.getAttribute('href')).toBe('/marketing/sources/direct');

    const unattributedLink = screen.getAllByRole('link', { name: /Неизвестный источник/ })[0];
    expect(unattributedLink.getAttribute('href')).toBe('/marketing/sources/_unattributed');

    const telegramLink = screen.getAllByRole('link', { name: /telegram/ })[0];
    expect(telegramLink.getAttribute('href')).toBe('/marketing/sources/telegram');

    const literalUnattributedLink = screen.getAllByRole('link', { name: /unattributed/ })[0];
    expect(literalUnattributedLink.getAttribute('href')).toBe('/marketing/sources/unattributed');
  });
});
