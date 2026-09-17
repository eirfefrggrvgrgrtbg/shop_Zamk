/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { SellerLayout } from './SellerLayout';
import { getSellerBalance } from '@zamk/api-client/src/seller';

const mockLogout = vi.fn();
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    user: { id: 'seller-1', name: 'Test Seller', role: 'seller' },
    logout: mockLogout,
  }),
}));

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerMe: vi.fn().mockResolvedValue({ seller: { status: 'active' } }),
  getSellerBalance: vi.fn(),
  getSellerPayouts: vi.fn(),
}));

vi.mock('../api/notifications', () => ({
  notificationsApi: {
    getNotifications: vi.fn().mockResolvedValue({ items: [], totalCount: 0 }),
    getUnreadCount: vi.fn().mockResolvedValue(0),
    markRead: vi.fn(),
    markAllRead: vi.fn(),
  },
}));

describe('SellerLayout - R1.1 & R1.1A Global Shell & Header Balance', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerBalance).mockResolvedValue({
      availableCents: 4728400,
      grossSalesCents: 0,
      commissionCents: 0,
      adjustmentsCents: 0,
      frozenCents: 0,
      paidCents: 0,
      currency: 'RUB',
    } as any);
  });

  const renderLayout = (initialPath = '/dashboard') => {
    return render(
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="*"
            element={
              <SellerLayout>
                <div data-testid="child-content">Page Content</div>
              </SellerLayout>
            }
          />
        </Routes>
      </MemoryRouter>
    );
  };

  it('renders child content', () => {
    renderLayout('/dashboard');
    expect(screen.getByTestId('child-content')).toBeTruthy();
    expect(screen.getByText('Page Content')).toBeTruthy();
  });

  it('renders NotificationBell in header, and does NOT render GlobalMoneyStrip', () => {
    renderLayout('/dashboard');
    // Notification bell is present
    expect(screen.getByRole('button', { name: /уведомления/i })).toBeTruthy();
    // Old Money strip fields (multi-currency, frozen, in transit) are absent
    expect(screen.queryByText(/в пути/i)).toBeNull();
    expect(screen.queryByText(/заморожено/i)).toBeNull();
  });

  it('renders exactly the 5 canonical navigation groups with section titles', () => {
    renderLayout('/dashboard');

    expect(screen.getAllByText('Ориентация').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Ассортимент').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Продажи').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Данные').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Контроль').length).toBeGreaterThan(0);
  });

  it('renders all canonical navigation links in their designated groups', () => {
    renderLayout('/dashboard');

    // Group 1: Ориентация
    const dashboardLinks = screen.getAllByRole('link', { name: /панель продавца/i });
    expect(dashboardLinks.some(link => link.getAttribute('href') === '/dashboard')).toBe(true);

    // Group 2: Ассортимент
    const productLinks = screen.getAllByRole('link', { name: /товары/i });
    expect(productLinks.some(link => link.getAttribute('href') === '/products')).toBe(true);

    const inventoryLinks = screen.getAllByRole('link', { name: /остатки/i });
    expect(inventoryLinks.some(link => link.getAttribute('href') === '/inventory')).toBe(true);

    const supplyLinks = screen.getAllByRole('link', { name: /поставки/i });
    expect(supplyLinks.some(link => link.getAttribute('href') === '/supplies')).toBe(true);

    // Group 3: Продажи
    const orderLinks = screen.getAllByRole('link', { name: /заказы/i });
    expect(orderLinks.some(link => link.getAttribute('href') === '/orders')).toBe(true);

    const returnLinks = screen.getAllByRole('link', { name: /возвраты/i });
    expect(returnLinks.some(link => link.getAttribute('href') === '/returns')).toBe(true);

    const reviewLinks = screen.getAllByRole('link', { name: /отзывы/i });
    expect(reviewLinks.some(link => link.getAttribute('href') === '/reviews')).toBe(true);

    // Group 4: Данные
    // Note: there are now two links to /payouts: the sidebar "Финансы" and the header balance link
    const financeLinks = screen.getAllByRole('link', { name: /финансы/i });
    expect(financeLinks.some(link => link.getAttribute('href') === '/payouts')).toBe(true);

    const analyticsLinks = screen.getAllByRole('link', { name: /аналитика/i });
    expect(analyticsLinks.some(link => link.getAttribute('href') === '/analytics')).toBe(true);

    // Group 5: Контроль
    const warningLinks = screen.getAllByRole('link', { name: /предупреждения/i });
    expect(warningLinks.some(link => link.getAttribute('href') === '/warnings')).toBe(true);
  });

  it('does NOT render removed or hidden items from sidebar navigation', () => {
    renderLayout('/dashboard');

    // "Добавить товар" removed from sidebar
    expect(screen.queryByRole('link', { name: /добавить товар/i })).toBeNull();

    // "Шаблоны" hidden from sidebar
    expect(screen.queryByRole('link', { name: /шаблоны/i })).toBeNull();

    // "Выплаты" renamed to "Финансы"
    expect(screen.queryByRole('link', { name: /^выплаты$/i })).toBeNull();
  });

  it('renders "Профиль магазина" in the lower profile/footer section linking to /settings', () => {
    renderLayout('/dashboard');

    const profileLinks = screen.getAllByRole('link', { name: /профиль магазина/i });
    expect(profileLinks.length).toBeGreaterThan(0);
    expect(profileLinks.some(link => link.getAttribute('href') === '/settings')).toBe(true);
  });

  it('renders logout button and calls logout on click', () => {
    renderLayout('/dashboard');

    const logoutBtns = screen.getAllByRole('button', { name: /выйти/i });
    expect(logoutBtns.length).toBeGreaterThan(0);

    fireEvent.click(logoutBtns[0]);
    expect(mockLogout).toHaveBeenCalled();
  });

  it('highlights the active link correctly based on current path', () => {
    renderLayout('/orders');

    const orderLinks = screen.getAllByRole('link', { name: /заказы/i });
    expect(orderLinks.some(link => link.className.includes('bg-black') && link.className.includes('text-white'))).toBe(true);

    const productLinks = screen.getAllByRole('link', { name: /товары/i });
    expect(productLinks.every(link => !link.className.includes('bg-black'))).toBe(true);
  });

  it('highlights /settings when on settings route', () => {
    renderLayout('/settings');

    const profileLinks = screen.getAllByRole('link', { name: /профиль магазина/i });
    expect(profileLinks.some(link => link.className.includes('bg-black') && link.className.includes('text-white'))).toBe(true);
  });

  // ==================================================
  // R1.1A - Compact Available Balance in Header Tests
  // ==================================================

  it('renders compact finance link in header pointing to /payouts', async () => {
    renderLayout('/dashboard');

    await waitFor(() => {
      const links = screen.getAllByRole('link');
      const headerPayoutLink = links.find(
        (link) => link.getAttribute('href') === '/payouts' && link.getAttribute('title')?.includes('Доступно к выплате')
      );
      expect(headerPayoutLink).toBeTruthy();
    });
  });

  it('displays formatted positive available balance and accessible title', async () => {
    vi.mocked(getSellerBalance).mockResolvedValue({
      availableCents: 4728400,
    } as any);

    renderLayout('/dashboard');

    await waitFor(() => {
      // 47 284 ₽ (accounting for non-breaking space)
      expect(screen.getByText(/47[\s\u00a0]284[\s\u00a0]₽/)).toBeTruthy();
    });

    const headerLink = screen.getByTitle(/Доступно к выплате: 47[\s\u00a0]284[\s\u00a0]₽\. Открыть финансы/);
    expect(headerLink).toBeTruthy();
    expect(headerLink.getAttribute('href')).toBe('/payouts');
  });

  it('displays real 0 ₽ when backend returns 0 availableCents', async () => {
    vi.mocked(getSellerBalance).mockResolvedValue({
      availableCents: 0,
    } as any);

    renderLayout('/dashboard');

    await waitFor(() => {
      expect(screen.getByText('0 ₽')).toBeTruthy();
    });

    const headerLink = screen.getByTitle('Доступно к выплате: 0 ₽. Открыть финансы');
    expect(headerLink).toBeTruthy();
    expect(headerLink.getAttribute('href')).toBe('/payouts');
  });

  it('displays signed negative available balance and does not label it as debt', async () => {
    vi.mocked(getSellerBalance).mockResolvedValue({
      availableCents: -1182100,
    } as any);

    renderLayout('/dashboard');

    await waitFor(() => {
      // Signed negative value −11 821 ₽
      expect(screen.getByText(/−[\s\u00a0]*11[\s\u00a0]821[\s\u00a0]₽/)).toBeTruthy();
    });

    const headerLink = screen.getByTitle(/Баланс: −[\s\u00a0]*11[\s\u00a0]821[\s\u00a0]₽\. Открыть финансы/);
    expect(headerLink).toBeTruthy();
    expect(headerLink.getAttribute('href')).toBe('/payouts');

    // Crucial requirement: Do NOT call it Долг, Задолженность, Доступно к выплате negative
    expect(screen.queryByText(/долг/i)).toBeNull();
    expect(screen.queryByText(/задолженность/i)).toBeNull();
    expect(screen.queryByText(/доступно к выплате/i)).toBeNull();
  });

  it('does not display fake 0 ₽ while loading and renders placeholder skeleton', () => {
    // Unresolved promise
    vi.mocked(getSellerBalance).mockReturnValue(new Promise(() => {}));

    renderLayout('/dashboard');

    // Never show fake 0 ₽
    expect(screen.queryByText('0 ₽')).toBeNull();

    // Shows subtle skeleton
    expect(screen.getByTestId('header-balance-skeleton')).toBeTruthy();

    // Link still exists and points to /payouts
    const headerLink = screen.getByTitle('Загрузка баланса');
    expect(headerLink).toBeTruthy();
    expect(headerLink.getAttribute('href')).toBe('/payouts');
  });

  it('does not display fake 0 ₽ on error and renders neutral fallback — ₽', async () => {
    vi.mocked(getSellerBalance).mockRejectedValue(new Error('Network error'));

    renderLayout('/dashboard');

    await waitFor(() => {
      expect(screen.getByText('— ₽')).toBeTruthy();
    });

    // Never show fake 0 ₽
    expect(screen.queryByText('0 ₽')).toBeNull();

    // Link still exists and points to /payouts
    const headerLink = screen.getByTitle('Баланс недоступен. Открыть финансы');
    expect(headerLink).toBeTruthy();
    expect(headerLink.getAttribute('href')).toBe('/payouts');
  });
});
