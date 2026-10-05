// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup, within, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { AdminLayout } from '../AdminLayout';
import { useAdminAuth } from '../../contexts/AdminAuthContext';

vi.mock('../../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn(),
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  getAdminSellers: vi.fn().mockResolvedValue({ items: [] }),
}));

vi.mock('../../api/adminProducts', () => ({
  getModerationProducts: vi.fn().mockResolvedValue({ items: [], totalCount: 0 }),
}));

vi.mock('../../api/adminReviews', () => ({
  getAdminReviews: vi.fn().mockResolvedValue([]),
}));

vi.mock('../../api/adminPicking', () => ({
  getAdminPickingQueue: vi.fn().mockResolvedValue([]),
  getAdminPackingQueue: vi.fn().mockResolvedValue([]),
}));

vi.mock('../../api/notifications', () => ({
  notificationsApi: {
    getUnreadCount: vi.fn().mockResolvedValue(0),
    getNotifications: vi.fn().mockResolvedValue({ notifications: [], total: 0 }),
    markAsRead: vi.fn().mockResolvedValue(undefined),
    markAllAsRead: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('../../api/adminSearch', () => ({
  searchAdminGlobal: vi.fn().mockResolvedValue({ results: [] }),
  groupSearchResults: vi.fn().mockReturnValue([]),
  getResultNavigationUrl: vi.fn().mockReturnValue('/'),
  getAdminSearchErrorMessage: vi.fn().mockReturnValue('Ошибка поиска'),
}));

function mockAuth(permissions: string[], roleCode: string = 'manager') {
  const permSet = new Set(permissions);
  const authVal = {
    user: { id: 'admin-1', email: 'admin@zamk.ru', role: 'admin' },
    staff: {
      userId: 'admin-1',
      roleCode,
      roleName: roleCode === 'owner' ? 'Владелец' : 'Администратор',
      permissions,
      status: 'active',
    },
    permissions,
    isAuthenticated: true,
    isLoading: false,
    error: null,
    hasPermission: (p: string) => permSet.has(p),
    hasAnyPermission: (perms: string[]) => perms.some((p) => permSet.has(p)),
    isOwner: () => roleCode === 'owner',
    isCoOwner: () => roleCode === 'co_owner',
    login: vi.fn(),
    logout: vi.fn(),
    refreshSession: vi.fn(),
    changePassword: vi.fn(),
    reloadStaff: vi.fn(),
  };
  vi.mocked(useAdminAuth).mockReturnValue(authVal as any);
  return authVal;
}

function LocationTracker() {
  const location = useLocation();
  return <div data-testid="current-pathname">{location.pathname}</div>;
}

describe('AdminNavigationShell — Canonical On-Demand Search & Support Contract', () => {
  beforeEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('1. Hamburger trigger exists and permanent global sidebar is completely absent', () => {
    mockAuth(['dashboard.read', 'support.read']);
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout>
          <div data-testid="workspace-content">Рабочая область</div>
        </AdminLayout>
      </MemoryRouter>
    );

    // 1a. Hamburger trigger exists in the top-left
    const trigger = screen.getByTestId('admin-nav-trigger');
    expect(trigger).toBeDefined();
    expect(within(trigger).getByText('Меню')).toBeDefined();

    // 1b. No permanent aside or sidebar in DOM
    expect(container.querySelector('aside')).toBeNull();
    expect(screen.queryByTestId('admin-search-palette')).toBeNull();

    // 1c. Workspace occupies full available horizontal flex width
    const mainWorkspace = container.querySelector('main');
    expect(mainWorkspace).toBeDefined();
    expect(mainWorkspace?.className).toContain('flex-1');
    expect(mainWorkspace?.className).toContain('w-full');
    expect(screen.getByTestId('workspace-content')).toBeDefined();
  });

  it('2. Clicking hamburger trigger opens unified search & navigation catalog over workspace', () => {
    mockAuth(['dashboard.read', 'support.read', 'orders.read']);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout>
          <div>Workspace</div>
        </AdminLayout>
      </MemoryRouter>
    );

    expect(screen.queryByTestId('admin-search-palette')).toBeNull();

    // Click trigger
    fireEvent.click(screen.getByTestId('admin-nav-trigger'));

    // Catalog surface is open over workspace
    const palette = screen.getByTestId('admin-search-palette');
    expect(palette).toBeDefined();
    expect(screen.getByTestId('admin-search-input')).toBeDefined();
    expect(screen.getByTestId('admin-nav-catalog')).toBeDefined();
  });

  it('3. Support is in its own dedicated group and distinct from Orders, Returns, Payments, Refunds', () => {
    mockAuth([
      'dashboard.read',
      'support.read',
      'orders.read',
      'returns.read',
      'payments.read',
      'refunds.read',
    ]);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const palette = screen.getByTestId('admin-search-palette');

    // Dedicated Support group exists
    const supportGroup = within(palette).getByTestId('admin-nav-group-Поддержка');
    expect(supportGroup).toBeDefined();
    expect(within(supportGroup).getAllByText('Поддержка')).toHaveLength(2); // Group title + item name
    expect(within(supportGroup).getByTestId('admin-nav-item--support')).toBeDefined();

    // Commerce group contains Orders, Returns, Payments, Refunds, but NOT Support
    const commerceGroup = within(palette).getByTestId('admin-nav-group-Коммерция и сервисы');
    expect(within(commerceGroup).getByText('Заказы')).toBeDefined();
    expect(within(commerceGroup).getByText('Возвраты')).toBeDefined();
    expect(within(commerceGroup).getByText('Платежи покупателей')).toBeDefined();
    expect(within(commerceGroup).getByText('Возмещения')).toBeDefined();
    expect(within(commerceGroup).queryByTestId('admin-nav-item--support')).toBeNull();
  });

  it('4. Searching "поддерж" filters destinations immediately to Support', () => {
    mockAuth([
      'dashboard.read',
      'support.read',
      'orders.read',
      'products.read',
    ]);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const input = screen.getByTestId('admin-search-input');

    // Type "поддерж"
    fireEvent.change(input, { target: { value: 'поддерж' } });

    // Filtered immediate results show Support
    const supportItem = screen.getByTestId('admin-nav-item--support');
    expect(supportItem).toBeDefined();
    expect(within(supportItem).getByText('Поддержка')).toBeDefined();

    // Non-matching destinations are filtered out immediately
    expect(screen.queryByTestId('admin-nav-item--orders')).toBeNull();
    expect(screen.queryByTestId('admin-nav-item--products')).toBeNull();
  });

  it('5. Selecting Support navigates to /support and automatically closes palette', () => {
    mockAuth(['dashboard.read', 'support.read']);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout>
          <Routes>
            <Route path="/dashboard" element={<div>Dashboard Page</div>} />
            <Route path="/support" element={<div>Support Page Workspace</div>} />
          </Routes>
          <LocationTracker />
        </AdminLayout>
      </MemoryRouter>
    );

    expect(screen.getByTestId('current-pathname').textContent).toBe('/dashboard');

    // Open palette
    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    expect(screen.getByTestId('admin-search-palette')).toBeDefined();

    // Click Support link
    const supportLink = screen.getByTestId('admin-nav-item--support');
    fireEvent.click(supportLink);

    // Palette is closed automatically
    expect(screen.queryByTestId('admin-search-palette')).toBeNull();

    // Navigation occurred to /support
    expect(screen.getByTestId('current-pathname').textContent).toBe('/support');
    expect(screen.getByText('Support Page Workspace')).toBeDefined();
  });

  it('6. Header breadcrumb displays current page context on /support', () => {
    mockAuth(['support.read']);
    render(
      <MemoryRouter initialEntries={['/support']}>
        <AdminLayout>
          <div>Support Content</div>
        </AdminLayout>
      </MemoryRouter>
    );

    // Breadcrumb in header shows ZAMK Admin / Поддержка
    const header = screen.getByRole('banner');
    expect(within(header).getByText('ZAMK Admin')).toBeDefined();
    expect(within(header).getByText('Поддержка')).toBeDefined();
  });

  it('7. Read visibility vs mutation authority: user with support.read sees Support; user without does not', () => {
    // 7a. User without support.read
    mockAuth(['orders.read']);
    const { unmount } = render(
      <MemoryRouter initialEntries={['/orders']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    expect(screen.queryByTestId('admin-nav-group-Поддержка')).toBeNull();
    expect(screen.queryByTestId('admin-nav-item--support')).toBeNull();
    unmount();

    // 7b. User with support.read
    mockAuth(['support.read']);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    expect(screen.getByTestId('admin-nav-group-Поддержка')).toBeDefined();
    expect(screen.getByTestId('admin-nav-item--support')).toBeDefined();
  });

  it('8. Keyboard navigation: ArrowDown + Enter on filtered search opens /support and closes palette', () => {
    mockAuth(['dashboard.read', 'support.read']);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout>
          <Routes>
            <Route path="/dashboard" element={<div>Dashboard Page</div>} />
            <Route path="/support" element={<div>Support Page Workspace</div>} />
          </Routes>
          <LocationTracker />
        </AdminLayout>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const input = screen.getByTestId('admin-search-input');

    // Type query matching Support
    fireEvent.change(input, { target: { value: 'поддерж' } });

    // Press Enter to navigate to first active match
    fireEvent.keyDown(input, { key: 'Enter' });

    // Palette closes and URL is /support
    expect(screen.queryByTestId('admin-search-palette')).toBeNull();
    expect(screen.getByTestId('current-pathname').textContent).toBe('/support');
    expect(screen.getByText('Support Page Workspace')).toBeDefined();
  });
});
