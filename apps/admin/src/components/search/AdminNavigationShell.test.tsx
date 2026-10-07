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

  it('3. Both Marketing and Support visible: renders stacked center column with Marketing on top and Support on bottom', () => {
    mockAuth([
      'dashboard.read',
      'support.read',
      'marketing.campaigns.read',
      'orders.read',
      'returns.read',
      'payments.read',
      'refunds.read',
      'warehouse.picking',
      'staff.read',
    ]);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const palette = screen.getByTestId('admin-search-palette');

    // Dedicated Center Stack container exists
    const centerStack = within(palette).getByTestId('admin-nav-center-stack');
    expect(centerStack).toBeDefined();

    // Marketing card is inside center stack (top half)
    const marketingGroup = within(centerStack).getByTestId('admin-nav-group-Маркетинг');
    expect(marketingGroup).toBeDefined();
    expect(within(marketingGroup).getByText('Сводка')).toBeDefined();
    expect(within(marketingGroup).getByText('Кампании')).toBeDefined();
    expect(within(marketingGroup).getByTestId('admin-nav-item--marketing')).toBeDefined();
    expect(within(marketingGroup).getByTestId('admin-nav-item--marketing-campaigns')).toBeDefined();

    // Support card is inside center stack (bottom half)
    const supportGroup = within(centerStack).getByTestId('admin-nav-group-Поддержка');
    expect(supportGroup).toBeDefined();
    expect(within(supportGroup).getAllByText('Поддержка')).toHaveLength(2);
    expect(within(supportGroup).getByTestId('admin-nav-item--support')).toBeDefined();

    // Verify ordering of all groups across catalog
    const groupElements = within(palette).getAllByTestId(/^admin-nav-group-/);
    const groupTitles = groupElements.map((el) => el.getAttribute('data-testid'));
    expect(groupTitles).toEqual([
      'admin-nav-group-Коммерция и сервисы',
      'admin-nav-group-Маркетинг',
      'admin-nav-group-Поддержка',
      'admin-nav-group-СКЛАД',
      'admin-nav-group-Администрирование',
    ]);
  });

  it('3b. Marketing-only state: center column contains only Marketing, without empty Support shell', () => {
    mockAuth([
      'dashboard.read',
      'marketing.campaigns.read',
      'orders.read',
      'warehouse.picking',
      'staff.read',
    ], 'admin');
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const palette = screen.getByTestId('admin-search-palette');

    const centerStack = within(palette).getByTestId('admin-nav-center-stack');
    expect(within(centerStack).getByTestId('admin-nav-group-Маркетинг')).toBeDefined();
    expect(within(centerStack).queryByTestId('admin-nav-group-Поддержка')).toBeNull();

    // Support is completely absent from DOM
    expect(within(palette).queryByTestId('admin-nav-group-Поддержка')).toBeNull();
    expect(within(palette).queryByTestId('admin-nav-item--support')).toBeNull();
  });

  it('3c. Support-only state: center column contains only Support, without empty Marketing shell', () => {
    mockAuth([
      'dashboard.read',
      'support.read',
      'orders.read',
      'warehouse.picking',
      'staff.read',
    ], 'support');
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const palette = screen.getByTestId('admin-search-palette');

    const centerStack = within(palette).getByTestId('admin-nav-center-stack');
    expect(within(centerStack).getByTestId('admin-nav-group-Поддержка')).toBeDefined();
    expect(within(centerStack).queryByTestId('admin-nav-group-Маркетинг')).toBeNull();

    // Marketing is completely absent from DOM
    expect(within(palette).queryByTestId('admin-nav-group-Маркетинг')).toBeNull();
    expect(within(palette).queryByTestId('admin-nav-item--marketing')).toBeNull();
  });

  it('3d. Neither Marketing nor Support state: center stack is completely omitted', () => {
    mockAuth([
      'warehouse.picking',
      'warehouse.packing',
      'warehouse.dispatch',
      'inventory.read',
    ], 'warehouse_operator');
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const palette = screen.getByTestId('admin-search-palette');

    // Warehouse group exists
    expect(within(palette).getByTestId('admin-nav-group-СКЛАД')).toBeDefined();

    // Center stack, Marketing, Support are completely absent
    expect(within(palette).queryByTestId('admin-nav-center-stack')).toBeNull();
    expect(within(palette).queryByTestId('admin-nav-group-Маркетинг')).toBeNull();
    expect(within(palette).queryByTestId('admin-nav-group-Поддержка')).toBeNull();
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

    // Click Support link from palette
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

  it('7. Read visibility vs mutation authority: Support and Marketing follow RBAC strictly', () => {
    // 7a. User without support.read or marketing.campaigns.read
    mockAuth(['orders.read']);
    const { unmount: unmount1 } = render(
      <MemoryRouter initialEntries={['/orders']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    expect(screen.queryByTestId('admin-nav-group-Маркетинг')).toBeNull();
    expect(screen.queryByTestId('admin-nav-group-Поддержка')).toBeNull();
    expect(screen.queryByTestId('admin-nav-item--support')).toBeNull();

    // Search for support without permission -> still null
    const input1 = screen.getByTestId('admin-search-input');
    fireEvent.change(input1, { target: { value: 'поддерж' } });
    expect(screen.queryByTestId('admin-nav-item--support')).toBeNull();
    unmount1();

    // 7b. User with support.read & marketing.campaigns.read
    mockAuth(['support.read', 'marketing.campaigns.read']);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    // Both visible in center stack
    expect(screen.getByTestId('admin-nav-group-Маркетинг')).toBeDefined();
    expect(screen.getByTestId('admin-nav-group-Поддержка')).toBeDefined();

    // Search "поддерж" discovers Support
    const input2 = screen.getByTestId('admin-search-input');
    fireEvent.change(input2, { target: { value: 'поддерж' } });
    expect(screen.getByTestId('admin-nav-item--support')).toBeDefined();
  });

  it('8. Keyboard navigation: ArrowDown traverses across Commerce -> Marketing -> Support -> Warehouse -> Staff', () => {
    mockAuth([
      'dashboard.read',
      'marketing.campaigns.read',
      'support.read',
      'warehouse.picking',
      'staff.read',
    ]);
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout>
          <Routes>
            <Route path="/dashboard" element={<div>Dashboard Page</div>} />
            <Route path="/marketing" element={<div>Marketing Overview</div>} />
            <Route path="/marketing/campaigns" element={<div>Marketing Campaigns</div>} />
            <Route path="/support" element={<div>Support Page Workspace</div>} />
            <Route path="/fulfillment/picking" element={<div>Picking Screen</div>} />
            <Route path="/staff" element={<div>Staff Screen</div>} />
          </Routes>
          <LocationTracker />
        </AdminLayout>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('admin-nav-trigger'));
    const input = screen.getByTestId('admin-search-input');

    // Initial state: index 0 (Главная /dashboard)
    // Press ArrowDown 1 time -> Marketing: Сводка (/marketing)
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(screen.queryByTestId('admin-search-palette')).toBeNull();
    expect(screen.getByTestId('current-pathname').textContent).toBe('/marketing');
    expect(screen.getByText('Marketing Overview')).toBeDefined();
  });
});
