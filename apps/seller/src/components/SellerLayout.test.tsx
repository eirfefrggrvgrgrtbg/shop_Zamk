/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
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

describe('SellerLayout - R1.1B Desktop Top Navigation Shell', () => {
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

  // 1. Desktop permanent sidebar is not rendered
  it('does not render a permanent desktop sidebar aside', () => {
    renderLayout('/dashboard');
    const desktopSidebar = document.querySelector('aside.hidden.md\\:flex, aside.md\\:flex');
    expect(desktopSidebar).toBeNull();
    // Ensure the only aside in the document is the mobile drawer
    const asides = document.querySelectorAll('aside');
    expect(asides.length).toBe(1);
    expect(asides[0].getAttribute('data-testid')).toBe('mobile-drawer');
  });

  // 2. Desktop top navigation renders: Обзор, Ассортимент, Продажи, Данные, Контроль, Профиль
  it('renders desktop top navigation with canonical groups, direct link, and triggers', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    expect(topNav).toBeTruthy();

    expect(within(topNav).getByRole('link', { name: 'Обзор' })).toBeTruthy();
    expect(within(topNav).getByRole('button', { name: /ассортимент/i })).toBeTruthy();
    expect(within(topNav).getByRole('button', { name: /продажи/i })).toBeTruthy();
    expect(within(topNav).getByRole('button', { name: /данные/i })).toBeTruthy();
    expect(within(topNav).getByRole('button', { name: /контроль/i })).toBeTruthy();
    expect(within(topNav).getByRole('button', { name: /профиль/i })).toBeTruthy();
  });

  // 3. Ассортимент dropdown contains: Товары, Остатки, Поставки
  it('opens Ассортимент dropdown on click and renders its items', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const assortmentBtn = within(topNav).getByRole('button', { name: /ассортимент/i });

    expect(assortmentBtn.getAttribute('aria-expanded')).toBe('false');
    expect(within(topNav).queryByRole('menuitem', { name: /товары/i })).toBeNull();

    fireEvent.click(assortmentBtn);
    expect(assortmentBtn.getAttribute('aria-expanded')).toBe('true');

    const productsLink = within(topNav).getByRole('menuitem', { name: /товары/i });
    expect(productsLink.getAttribute('href')).toBe('/products');

    const inventoryLink = within(topNav).getByRole('menuitem', { name: /остатки/i });
    expect(inventoryLink.getAttribute('href')).toBe('/inventory');

    const suppliesLink = within(topNav).getByRole('menuitem', { name: /поставки/i });
    expect(suppliesLink.getAttribute('href')).toBe('/supplies');
  });

  // 4. Продажи dropdown contains: Заказы, Возвраты, Отзывы
  it('opens Продажи dropdown on click and renders its items', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const salesBtn = within(topNav).getByRole('button', { name: /продажи/i });

    fireEvent.click(salesBtn);
    expect(salesBtn.getAttribute('aria-expanded')).toBe('true');

    const ordersLink = within(topNav).getByRole('menuitem', { name: /заказы/i });
    expect(ordersLink.getAttribute('href')).toBe('/orders');

    const returnsLink = within(topNav).getByRole('menuitem', { name: /возвраты/i });
    expect(returnsLink.getAttribute('href')).toBe('/returns');

    const reviewsLink = within(topNav).getByRole('menuitem', { name: /отзывы/i });
    expect(reviewsLink.getAttribute('href')).toBe('/reviews');
  });

  // 5. Данные dropdown contains: Финансы, Аналитика
  it('opens Данные dropdown on click and renders its items', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const dataBtn = within(topNav).getByRole('button', { name: /данные/i });

    fireEvent.click(dataBtn);
    expect(dataBtn.getAttribute('aria-expanded')).toBe('true');

    const financeLink = within(topNav).getByRole('menuitem', { name: /финансы/i });
    expect(financeLink.getAttribute('href')).toBe('/payouts');

    const analyticsLink = within(topNav).getByRole('menuitem', { name: /аналитика/i });
    expect(analyticsLink.getAttribute('href')).toBe('/analytics');
  });

  // 6. Контроль dropdown contains: Предупреждения
  it('opens Контроль dropdown on click and renders its items', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const controlBtn = within(topNav).getByRole('button', { name: /контроль/i });

    fireEvent.click(controlBtn);
    expect(controlBtn.getAttribute('aria-expanded')).toBe('true');

    const warningsLink = within(topNav).getByRole('menuitem', { name: /предупреждения/i });
    expect(warningsLink.getAttribute('href')).toBe('/warnings');
  });

  // 7. Profile dropdown contains: Профиль магазина, Выйти
  it('opens Profile dropdown on click and renders Профиль магазина and Выйти', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const profileBtn = within(topNav).getByRole('button', { name: /профиль/i });

    fireEvent.click(profileBtn);
    expect(profileBtn.getAttribute('aria-expanded')).toBe('true');

    const settingsLink = within(topNav).getByRole('menuitem', { name: /профиль магазина/i });
    expect(settingsLink.getAttribute('href')).toBe('/settings');

    const logoutBtn = within(topNav).getByRole('menuitem', { name: /выйти/i });
    expect(logoutBtn).toBeTruthy();

    fireEvent.click(logoutBtn);
    expect(mockLogout).toHaveBeenCalled();
  });

  // 8. Only one dropdown is open at once
  it('ensures only one dropdown is open at a time', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const assortmentBtn = within(topNav).getByRole('button', { name: /ассортимент/i });
    const salesBtn = within(topNav).getByRole('button', { name: /продажи/i });
    const profileBtn = within(topNav).getByRole('button', { name: /профиль/i });

    // Open assortment
    fireEvent.click(assortmentBtn);
    expect(assortmentBtn.getAttribute('aria-expanded')).toBe('true');
    expect(within(topNav).getByRole('menuitem', { name: /товары/i })).toBeTruthy();

    // Click sales -> assortment closes, sales opens
    fireEvent.click(salesBtn);
    expect(assortmentBtn.getAttribute('aria-expanded')).toBe('false');
    expect(salesBtn.getAttribute('aria-expanded')).toBe('true');
    expect(within(topNav).queryByRole('menuitem', { name: /товары/i })).toBeNull();
    expect(within(topNav).getByRole('menuitem', { name: /заказы/i })).toBeTruthy();

    // Click profile -> sales closes, profile opens
    fireEvent.click(profileBtn);
    expect(salesBtn.getAttribute('aria-expanded')).toBe('false');
    expect(profileBtn.getAttribute('aria-expanded')).toBe('true');
    expect(within(topNav).queryByRole('menuitem', { name: /заказы/i })).toBeNull();
    expect(within(topNav).getByRole('menuitem', { name: /профиль магазина/i })).toBeTruthy();
  });

  // 9. Clicking outside closes dropdown
  it('closes open dropdown when clicking outside', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const assortmentBtn = within(topNav).getByRole('button', { name: /ассортимент/i });

    fireEvent.click(assortmentBtn);
    expect(within(topNav).getByRole('menuitem', { name: /товары/i })).toBeTruthy();

    // Click outside
    fireEvent.mouseDown(screen.getByTestId('child-content'));
    expect(within(topNav).queryByRole('menuitem', { name: /товары/i })).toBeNull();
    expect(assortmentBtn.getAttribute('aria-expanded')).toBe('false');
  });

  // 10. Escape closes dropdown
  it('closes open dropdown when pressing Escape key', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    const assortmentBtn = within(topNav).getByRole('button', { name: /ассортимент/i });

    fireEvent.click(assortmentBtn);
    expect(within(topNav).getByRole('menuitem', { name: /товары/i })).toBeTruthy();

    // Press Escape
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(within(topNav).queryByRole('menuitem', { name: /товары/i })).toBeNull();
    expect(assortmentBtn.getAttribute('aria-expanded')).toBe('false');
  });

  // 11. Active group follows current route
  it('highlights active group and active child items following the current route', () => {
    // 1. Dashboard -> Обзор active
    const { unmount: u1 } = renderLayout('/dashboard');
    const topNav1 = screen.getByTestId('desktop-top-nav');
    const overviewLink = within(topNav1).getByRole('link', { name: 'Обзор' });
    expect(overviewLink.getAttribute('data-active')).toBe('true');
    u1();

    // 2. /products -> Ассортимент active
    const { unmount: u2 } = renderLayout('/products');
    const topNav2 = screen.getByTestId('desktop-top-nav');
    const assortmentBtn = within(topNav2).getByRole('button', { name: /ассортимент/i });
    expect(assortmentBtn.getAttribute('data-active')).toBe('true');
    // Open assortment and check that Товары has active styling
    fireEvent.click(assortmentBtn);
    const productsItem = within(topNav2).getByRole('menuitem', { name: /товары/i });
    expect(productsItem.className).toContain('bg-black');
    expect(productsItem.className).toContain('text-white');
    u2();

    // 3. /orders -> Продажи active
    const { unmount: u3 } = renderLayout('/orders');
    const topNav3 = screen.getByTestId('desktop-top-nav');
    expect(within(topNav3).getByRole('button', { name: /продажи/i }).getAttribute('data-active')).toBe('true');
    u3();

    // 4. /payouts -> Данные active
    const { unmount: u4 } = renderLayout('/payouts');
    const topNav4 = screen.getByTestId('desktop-top-nav');
    expect(within(topNav4).getByRole('button', { name: /данные/i }).getAttribute('data-active')).toBe('true');
    u4();

    // 5. /warnings -> Контроль active
    const { unmount: u5 } = renderLayout('/warnings');
    const topNav5 = screen.getByTestId('desktop-top-nav');
    expect(within(topNav5).getByRole('button', { name: /контроль/i }).getAttribute('data-active')).toBe('true');
    u5();

    // 6. /settings -> Профиль active
    const { unmount: u6 } = renderLayout('/settings');
    const topNav6 = screen.getByTestId('desktop-top-nav');
    expect(within(topNav6).getByRole('button', { name: /профиль/i }).getAttribute('data-active')).toBe('true');
    u6();
  });

  // 13. NotificationBell remains
  it('renders NotificationBell in the desktop top navigation bar', () => {
    renderLayout('/dashboard');
    const topNav = screen.getByTestId('desktop-top-nav');
    expect(within(topNav).getByRole('button', { name: /уведомления/i })).toBeTruthy();
  });

  // 14. Mobile navigation remains reachable
  it('renders mobile menu trigger and mobile drawer with accessible navigation', () => {
    renderLayout('/dashboard');
    const menuBtn = screen.getByRole('button', { name: /открыть меню/i });
    expect(menuBtn).toBeTruthy();

    const drawer = screen.getByTestId('mobile-drawer');
    expect(drawer.className).toContain('-translate-x-full');

    fireEvent.click(menuBtn);
    expect(drawer.className).toContain('translate-x-0');

    // Verify drawer contains canonical links
    expect(within(drawer).getByRole('link', { name: /обзор/i })).toBeTruthy();
    expect(within(drawer).getByRole('link', { name: /товары/i })).toBeTruthy();
    expect(within(drawer).getByRole('link', { name: /заказы/i })).toBeTruthy();
    expect(within(drawer).getByRole('link', { name: /профиль магазина/i })).toBeTruthy();
    expect(within(drawer).getByRole('button', { name: /выйти/i })).toBeTruthy();
  });

  // 15. “Добавить товар” and “Шаблоны” are absent from global nav
  it('does NOT render removed or hidden items (“Добавить товар”, “Шаблоны”) in navigation', () => {
    renderLayout('/dashboard');
    expect(screen.queryByRole('link', { name: /добавить товар/i })).toBeNull();
    expect(screen.queryByRole('link', { name: /шаблоны/i })).toBeNull();
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
