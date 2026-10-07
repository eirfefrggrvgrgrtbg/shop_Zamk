import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingDesigners } from './AdminMarketingDesigners';
import * as apiClient from '@zamk/api-client';
import * as opsApi from '../api/adminOperations';

vi.mock('@zamk/api-client', async () => {
  const actual = await vi.importActual<any>('@zamk/api-client');
  return {
    ...actual,
    getAdminDesignerAnalytics: vi.fn(),
  };
});
vi.mock('../api/adminOperations');

const sampleCoverageAvailable = {
  views: { status: 'available' },
  favorites: { status: 'available' },
  addToCart: { status: 'available' },
};

const sampleCoveragePartial = {
  views: { status: 'partial', trackedFrom: '2026-10-01T00:00:00Z' },
  favorites: { status: 'partial', trackedFrom: '2026-10-01T00:00:00Z' },
  addToCart: { status: 'partial', trackedFrom: '2026-10-01T00:00:00Z' },
};

const sampleCoverageUnavailable = {
  views: { status: 'unavailable' },
  favorites: { status: 'unavailable' },
  addToCart: { status: 'unavailable' },
};

const sampleDesigners = [
  {
    designerId: 'dsgn_12345678',
    designerName: 'Acme Studio',
    productsCount: 5,
    primaryImage: 'https://example.com/designer.jpg',
    views: 120,
    favorites: 15,
    addToCart: 8,
    purchases: 6,
    soldUnits: 8,
    revenueCents: 8000000,
    conversionRate: 0.05,
    returns: 1,
    previousRevenueCents: 4000000,
    revenueChangePct: 100.0,
  },
  {
    designerId: 'dsgn_abcdef01',
    designerName: 'Declining Brand',
    productsCount: 2,
    primaryImage: null,
    views: 80,
    favorites: 5,
    addToCart: 2,
    purchases: 1,
    soldUnits: 1,
    revenueCents: 2000000,
    conversionRate: 0.0125,
    returns: 0,
    previousRevenueCents: 4000000,
    revenueChangePct: -50.0,
  },
  {
    designerId: 'dsgn_99999999',
    designerName: 'New Designer',
    productsCount: 1,
    primaryImage: null,
    views: 0,
    favorites: 0,
    addToCart: 0,
    purchases: 0,
    soldUnits: 0,
    revenueCents: 0,
    conversionRate: 0,
    returns: 0,
    previousRevenueCents: 0,
    revenueChangePct: undefined,
  },
];

const mockCategories = [
  { id: 'cat-1', name: 'Coats' },
  { id: 'cat-2', name: 'Dresses' },
];

const mount = () =>
  render(
    <MemoryRouter>
      <AdminMarketingDesigners />
    </MemoryRouter>
  );

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(opsApi.getAdminCategories).mockResolvedValue(mockCategories as any);
  vi.mocked(apiClient.getAdminDesignerAnalytics).mockResolvedValue({
    coverage: sampleCoverageAvailable,
    designers: sampleDesigners,
  } as any);
});

afterEach(cleanup);

describe('AdminMarketingDesigners Component Tests', () => {
  it('1. loading state is rendered while request is in flight', async () => {
    let finish: any;
    vi.mocked(apiClient.getAdminDesignerAnalytics).mockReturnValue(
      new Promise(resolve => {
        finish = resolve;
      })
    );

    mount();
    // Spinner is present during loading
    expect(document.querySelector('.animate-spin')).toBeTruthy();

    finish({ coverage: sampleCoverageAvailable, designers: [] });
    await waitFor(() => expect(document.querySelector('.animate-spin')).toBeNull());
  });

  it('2. populated rows are rendered with tabular metrics and revenue formatting', async () => {
    mount();
    await screen.findByText('Acme Studio');

    const table = screen.getByRole('table');
    const acmeName = within(table).getByText('Acme Studio');
    const acmeRow = acmeName.closest('tr')!;
    expect(acmeRow).toBeTruthy();

    expect(within(acmeRow).getByText('5')).toBeTruthy(); // productsCount
    expect(within(acmeRow).getByText(/80\s*000/)).toBeTruthy(); // revenue formatting (80,000 ₽)
    expect(within(acmeRow).getByText('+100%')).toBeTruthy(); // growth presentation
    expect(within(acmeRow).getByText('6')).toBeTruthy(); // purchases
    expect(within(acmeRow).getByText('8 шт')).toBeTruthy(); // soldUnits
    expect(within(acmeRow).getByText('120')).toBeTruthy(); // views
    expect(within(acmeRow).getByText('15')).toBeTruthy(); // favorites
    expect(within(acmeRow).getByText('5.00%')).toBeTruthy(); // conversion
    expect(within(acmeRow).getByText('1')).toBeTruthy(); // returns
  });

  it('3. delta and growth presentation handles positive, negative, and unavailable previous revenue', async () => {
    mount();
    await screen.findByText('Acme Studio');

    const table = screen.getByRole('table');
    // Acme Studio: +100%
    const growthElem = within(table).getByText('+100%');
    expect(growthElem).toBeTruthy();
    expect(growthElem.className).toContain('text-green-700');

    // Declining Brand: -50%
    const declineElem = within(table).getByText('-50%');
    expect(declineElem).toBeTruthy();
    expect(declineElem.className).toContain('text-amber-700');

    // New Designer (previousRevenueCents = 0): "—"
    const dashElements = within(table).getAllByText('—');
    expect(dashElements.length).toBeGreaterThanOrEqual(1);
  });

  it('4. no raw UUID is rendered in designer table', async () => {
    mount();
    await screen.findByText('Acme Studio');

    const table = screen.getByRole('table');
    const tableText = table.textContent || '';
    const uuidRegex = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;
    expect(uuidRegex.test(tableText)).toBe(false);
  });

  it('5. empty state is rendered when no designers match period', async () => {
    vi.mocked(apiClient.getAdminDesignerAnalytics).mockResolvedValue({
      coverage: sampleCoverageAvailable,
      designers: [],
    } as any);

    mount();
    await screen.findByText('Нет данных за выбранный период');
  });

  it('6. error state and retry button refetches data upon click', async () => {
    vi.mocked(apiClient.getAdminDesignerAnalytics).mockRejectedValueOnce(new Error('Network error'));

    mount();
    await screen.findByText('Не удалось загрузить аналитику дизайнеров');
    expect(apiClient.getAdminDesignerAnalytics).toHaveBeenCalledTimes(1);

    const retryBtn = screen.getByRole('button', { name: 'Повторить' });
    expect(retryBtn).toBeTruthy();

    vi.mocked(apiClient.getAdminDesignerAnalytics).mockResolvedValueOnce({
      coverage: sampleCoverageAvailable,
      designers: sampleDesigners,
    } as any);

    fireEvent.click(retryBtn);

    await waitFor(() => {
      expect(apiClient.getAdminDesignerAnalytics).toHaveBeenCalledTimes(2);
      expect(screen.queryByText('Не удалось загрузить аналитику дизайнеров')).toBeNull();
      expect(screen.getByText('Acme Studio')).toBeTruthy();
    });
  });

  it('7. partial coverage renders dot indicator and legend', async () => {
    vi.mocked(apiClient.getAdminDesignerAnalytics).mockResolvedValue({
      coverage: sampleCoveragePartial,
      designers: sampleDesigners,
    } as any);

    mount();
    await screen.findByText('Acme Studio');

    // Legend is shown
    expect(screen.getByText('Поведенческие данные собраны не за весь период')).toBeTruthy();

    // Partial metric has dot indicator
    const partialItems = document.querySelectorAll('.group\\/partial');
    expect(partialItems.length).toBeGreaterThan(0);
  });

  it('8. unavailable coverage renders dash with tooltip and unavailable != zero', async () => {
    vi.mocked(apiClient.getAdminDesignerAnalytics).mockResolvedValue({
      coverage: sampleCoverageUnavailable,
      designers: sampleDesigners,
    } as any);

    mount();
    await screen.findByText('Acme Studio');

    // Legend is NOT shown for unavailable
    expect(screen.queryByText('Поведенческие данные собраны не за весь период')).toBeNull();

    // Unavailable views and favorites render dash "—" with tooltip
    const unavailSpans = document.querySelectorAll('span[title="Недостаточно данных"]');
    expect(unavailSpans.length).toBeGreaterThanOrEqual(2);
  });

  it('9. search input updates and triggers refetch', async () => {
    mount();
    await screen.findByText('Acme Studio');

    const searchInput = screen.getByPlaceholderText('ПОИСК ПО ИМЕНИ...');
    fireEvent.change(searchInput, { target: { value: 'Acme' } });

    await waitFor(() => {
      expect(apiClient.getAdminDesignerAnalytics).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        'revenue',
        'Acme',
        ''
      );
    });
  });

  it('10. category filter selects option and triggers refetch', async () => {
    mount();
    await screen.findByText('Acme Studio');

    const catSelectTrigger = screen.getByTestId('select-Все категории');
    fireEvent.click(catSelectTrigger);

    const coatsOption = await screen.findByRole('option', { name: 'Coats' });
    fireEvent.click(coatsOption);

    await waitFor(() => {
      expect(apiClient.getAdminDesignerAnalytics).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        'revenue',
        '',
        'cat-1'
      );
    });
  });

  it('11. sort selects option and triggers refetch', async () => {
    mount();
    await screen.findByText('Acme Studio');

    const sortTrigger = screen.getByTestId('select-Сортировка');
    fireEvent.click(sortTrigger);

    const growthOption = await screen.findByRole('option', { name: 'Рост выручки' });
    fireEvent.click(growthOption);

    await waitFor(() => {
      expect(apiClient.getAdminDesignerAnalytics).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        'revenue_growth',
        '',
        ''
      );
    });
  });

  it('12. sort options for incomplete coverage are disabled with explanation', async () => {
    vi.mocked(apiClient.getAdminDesignerAnalytics).mockResolvedValue({
      coverage: sampleCoveragePartial, // favorites not available
      designers: sampleDesigners,
    } as any);

    mount();
    await screen.findByText('Acme Studio');

    const sortTrigger = screen.getByTestId('select-Сортировка');
    fireEvent.click(sortTrigger);

    const favOption = await screen.findByRole('option', { name: /По избранному/ });
    expect(favOption.getAttribute('aria-disabled')).toBe('true');
    expect(favOption.textContent).toContain('Недостаточно данных для сортировки');
  });
});
