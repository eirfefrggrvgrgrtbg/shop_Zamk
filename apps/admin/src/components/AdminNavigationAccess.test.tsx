// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminLayout } from './AdminLayout';
import { AdminProtectedRoute } from './AdminProtectedRoute';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import {
  STAFF_SCREEN_ACCESS_RULES,
  getStaffScreenAccessRule,
  getStaffScreenVisibility,
  isScreenRuleVisible,
  isScreenVisibleWithPermissions,
} from '../config/staffWorkModules';
import { getAllCapabilityKeys } from '../config/staffCapabilities';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn(),
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  getAdminSellers: vi.fn().mockResolvedValue({ items: [] }),
}));

vi.mock('../api/adminProducts', () => ({
  getModerationProducts: vi.fn().mockResolvedValue({ items: [], totalCount: 0 }),
}));

vi.mock('../api/adminReviews', () => ({
  getAdminReviews: vi.fn().mockResolvedValue([]),
}));

vi.mock('../api/adminPicking', () => ({
  getAdminPickingQueue: vi.fn().mockResolvedValue([]),
}));

function mockAuth(permissions: string[], roleCode: string = 'manager') {
  const permSet = new Set(permissions);
  const authVal = {
    user: { id: 'user-1', email: 'test@zamk.ru', role: 'admin' },
    staff: {
      userId: 'user-1',
      roleCode,
      roleName: roleCode === 'owner' ? 'Владелец' : 'Менеджер',
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

function getSidebar() {
  const layout = screen.getByTestId('admin-layout');
  const aside = layout.querySelector('aside');
  if (!aside) throw new Error('aside not found');
  return aside;
}

describe('EMP.1C3C2R.2B — Admin Navigation and Route Guards Alignment', () => {
  beforeEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  // A. 24 screen rules are consumed by navigation/route helpers
  it('A: all 24 canonical screen rules are consumed by getStaffScreenVisibility', () => {
    expect(STAFF_SCREEN_ACCESS_RULES).toHaveLength(24);

    for (const rule of STAFF_SCREEN_ACCESS_RULES) {
      const visibility = getStaffScreenVisibility(rule.route);
      expect(visibility, `Rule for route ${rule.route} must have defined visibility`).toBeDefined();
      expect(visibility).toEqual(rule.visibility);

      const byKey = getStaffScreenVisibility(rule.key);
      expect(byKey, `Rule for key ${rule.key} must have defined visibility`).toBeDefined();
      expect(byKey).toEqual(rule.visibility);
    }
  });

  // B. no duplicate local route-permission registry introduced
  it('B: guarantees navigation derives directly from STAFF_SCREEN_ACCESS_RULES without separate registry', () => {
    const allScreenRoutes = STAFF_SCREEN_ACCESS_RULES.map((r) => r.route);
    for (const route of allScreenRoutes) {
      const rule = getStaffScreenAccessRule(route);
      expect(rule).toBeDefined();
      expect(rule?.route).toBe(route);
    }
    expect(getStaffScreenAccessRule('/phantom/route')).toBeUndefined();
    expect(getStaffScreenVisibility('/phantom/route')).toBeUndefined();
  });

  // C. picking uses warehouse.picking
  it('C: picking route and navigation visibility strictly require warehouse.picking', () => {
    const pickingVisibility = getStaffScreenVisibility('/fulfillment/picking');
    expect(pickingVisibility).toBe('warehouse.picking');

    mockAuth(['warehouse.picking']);
    render(
      <MemoryRouter initialEntries={['/fulfillment/picking']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Сборка заказов')).toBeDefined();
  });

  // D. order receiving uses warehouse.receiving
  it('D: order receiving route visibility strictly requires warehouse.receiving', () => {
    const orderRecVisibility = getStaffScreenVisibility('/orders/receiving');
    expect(orderRecVisibility).toBe('warehouse.receiving');

    mockAuth(['warehouse.receiving']);
    render(
      <MemoryRouter initialEntries={['/orders/receiving']}>
        <Routes>
          <Route
            path="/orders/receiving"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/orders/receiving')}>
                <div>Order Receiving Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Order Receiving Screen')).toBeDefined();
  });

  // E. supply receiving uses inventory.receipt
  it('E: supply receiving route and navigation visibility require inventory.receipt', () => {
    const supplyRecVisibility = getStaffScreenVisibility('/supplies/receiving');
    expect(supplyRecVisibility).toBe('inventory.receipt');

    mockAuth(['inventory.receipt']);
    render(
      <MemoryRouter initialEntries={['/supplies/receiving']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Приемка поставок')).toBeDefined();
  });

  // F. orders.read does not imply picking
  it('F: orders.read alone does NOT expose picking navigation or access to /fulfillment/picking', () => {
    mockAuth(['orders.read']);
    render(
      <MemoryRouter initialEntries={['/fulfillment/picking']}>
        <AdminLayout>
          <Routes>
            <Route
              path="/fulfillment/picking"
              element={
                <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/picking')}>
                  <div>Picking Queue</div>
                </AdminProtectedRoute>
              }
            />
          </Routes>
        </AdminLayout>
      </MemoryRouter>
    );

    const sidebar = getSidebar();
    expect(within(sidebar).getByText('Заказы')).toBeDefined();
    expect(within(sidebar).queryByText('Сборка заказов')).toBeNull();

    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Picking Queue')).toBeNull();
  });

  // G. orders.read does not imply order receiving
  it('G: orders.read alone does NOT grant access to /orders/receiving', () => {
    mockAuth(['orders.read']);
    render(
      <MemoryRouter initialEntries={['/orders/receiving']}>
        <Routes>
          <Route
            path="/orders/receiving"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/orders/receiving')}>
                <div>Order Receiving Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Order Receiving Screen')).toBeNull();
  });

  // H. inventory.read does not imply supply receiving
  it('H: inventory.read alone does NOT expose supply receiving in navigation or route guard', () => {
    mockAuth(['inventory.read']);
    render(
      <MemoryRouter initialEntries={['/supplies/receiving']}>
        <AdminLayout>
          <Routes>
            <Route
              path="/supplies/receiving"
              element={
                <AdminProtectedRoute permission={getStaffScreenVisibility('/supplies/receiving')}>
                  <div>Supply Receiving Screen</div>
                </AdminProtectedRoute>
              }
            />
          </Routes>
        </AdminLayout>
      </MemoryRouter>
    );

    const sidebar = getSidebar();
    expect(within(sidebar).getByText('Остатки / Склад')).toBeDefined();
    expect(within(sidebar).queryByText('Приемка поставок')).toBeNull();
    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Supply Receiving Screen')).toBeNull();
  });

  // I. dashboard OR works
  it('I: dashboard multi-capability OR rule works correctly', () => {
    const dashboardRule = getStaffScreenVisibility('/dashboard');
    expect(dashboardRule).toEqual(['dashboard.read', 'analytics.read']);

    // 1. dashboard.read only -> visible
    mockAuth(['dashboard.read']);
    const { unmount: unmount1 } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Главная')).toBeDefined();
    unmount1();

    // 2. analytics.read only -> visible
    mockAuth(['analytics.read']);
    const { unmount: unmount2 } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Главная')).toBeDefined();
    unmount2();

    // 3. neither -> hidden
    mockAuth(['orders.read']);
    const { unmount: unmount3 } = render(
      <MemoryRouter initialEntries={['/orders']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).queryByText('Главная')).toBeNull();
    unmount3();
  });

  // J. catalog OR works
  it('J: catalog multi-capability OR rule works correctly', () => {
    const catalogRule = getStaffScreenVisibility('/catalog');
    expect(catalogRule).toEqual(['categories.read', 'brands.read']);

    // 1. categories.read -> visible
    mockAuth(['categories.read']);
    const { unmount: unmount1 } = render(
      <MemoryRouter initialEntries={['/catalog']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Категории и бренды')).toBeDefined();
    unmount1();

    // 2. brands.read -> visible
    mockAuth(['brands.read']);
    const { unmount: unmount2 } = render(
      <MemoryRouter initialEntries={['/catalog']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Категории и бренды')).toBeDefined();
    unmount2();

    // 3. neither -> hidden
    mockAuth(['orders.read']);
    const { unmount: unmount3 } = render(
      <MemoryRouter initialEntries={['/orders']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).queryByText('Категории и бренды')).toBeNull();
    unmount3();
  });

  // K. returns OR works
  it('K: returns multi-capability OR rule works correctly', () => {
    const returnsRule = getStaffScreenVisibility('/returns');
    expect(returnsRule).toEqual(['returns.read', 'warehouse.returns']);

    // 1. returns.read -> visible
    mockAuth(['returns.read']);
    const { unmount: unmount1 } = render(
      <MemoryRouter initialEntries={['/returns']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Возвраты')).toBeDefined();
    unmount1();

    // 2. warehouse.returns -> visible
    mockAuth(['warehouse.returns']);
    const { unmount: unmount2 } = render(
      <MemoryRouter initialEntries={['/returns']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).getByText('Возвраты')).toBeDefined();
    unmount2();

    // 3. neither -> hidden
    mockAuth(['orders.read']);
    const { unmount: unmount3 } = render(
      <MemoryRouter initialEntries={['/orders']}>
        <AdminLayout />
      </MemoryRouter>
    );
    expect(within(getSidebar()).queryByText('Возвраты')).toBeNull();
    unmount3();
  });

  // L. staff.read still exposes staff screen
  it('L: staff.read exposes Сотрудники in navigation and route guard', () => {
    mockAuth(['staff.read']);
    render(
      <MemoryRouter initialEntries={['/staff']}>
        <AdminLayout>
          <Routes>
            <Route
              path="/staff"
              element={
                <AdminProtectedRoute permission={getStaffScreenVisibility('/staff')}>
                  <div>Staff Directory Screen</div>
                </AdminProtectedRoute>
              }
            />
          </Routes>
        </AdminLayout>
      </MemoryRouter>
    );

    expect(within(getSidebar()).getByText('Сотрудники')).toBeDefined();
    expect(screen.getByText('Staff Directory Screen')).toBeDefined();
  });

  // M. staff.permissions.manage alone does not substitute staff.read
  it('M: staff.permissions.manage alone does NOT substitute staff.read for navigation or route access', () => {
    mockAuth(['staff.permissions.manage']);
    render(
      <MemoryRouter initialEntries={['/staff']}>
        <AdminLayout>
          <Routes>
            <Route
              path="/staff"
              element={
                <AdminProtectedRoute permission={getStaffScreenVisibility('/staff')}>
                  <div>Staff Directory Screen</div>
                </AdminProtectedRoute>
              }
            />
          </Routes>
        </AdminLayout>
      </MemoryRouter>
    );

    expect(within(getSidebar()).queryByText('Сотрудники')).toBeNull();
    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Staff Directory Screen')).toBeNull();
  });

  // N. screenless capabilities do not create sidebar entries
  it('N: screenless capabilities do not create phantom sidebar entries', () => {
    mockAuth([
      'security.read',
      'testing.manage',
      'storefront.manage',
      'auctions.manage_settings',
    ]);
    render(
      <MemoryRouter initialEntries={['/']}>
        <AdminLayout />
      </MemoryRouter>
    );

    const sidebar = getSidebar();
    expect(within(sidebar).queryByText('Безопасность')).toBeNull();
    expect(within(sidebar).queryByText('Тестирование')).toBeNull();
    expect(within(sidebar).queryByText('Витрина')).toBeNull();
    expect(within(sidebar).queryByText('Настройки аукционов')).toBeNull();
    // And staff group heading should be hidden since no staff nav items are visible
    expect(within(sidebar).queryByText('Администрирование')).toBeNull();
  });

  // O. sidebar preview and actual navigation derive from same screen rules
  it('O: sidebar preview and actual navigation use identical visibility calculations', () => {
    const testPermissionSets: string[][] = [
      ['orders.read'],
      ['warehouse.picking', 'inventory.receipt'],
      ['dashboard.read', 'categories.read', 'staff.read'],
      ['roles.read', 'audit.read'],
      [],
    ];

    for (const perms of testPermissionSets) {
      for (const rule of STAFF_SCREEN_ACCESS_RULES) {
        const previewVisible = isScreenVisibleWithPermissions(rule.visibility, perms);

        const permSet = new Set(perms);
        const navVisible = isScreenRuleVisible(
          rule.visibility,
          (p) => permSet.has(p),
          (array) => array.some((p) => permSet.has(p))
        );

        expect(
          navVisible,
          `Rule ${rule.key} visibility mismatch between preview helper and nav helper for perms: ${perms.join(',')}`
        ).toBe(previewVisible);
      }
    }
  });

  // P. direct forbidden route is denied
  it('P: direct forbidden route access renders forbidden message without silent redirect', () => {
    mockAuth(['orders.read']); // Has orders, not settings
    render(
      <MemoryRouter initialEntries={['/settings']}>
        <AdminProtectedRoute permission={getStaffScreenVisibility('/settings')}>
          <div>Settings Sensitive Page</div>
        </AdminProtectedRoute>
      </MemoryRouter>
    );

    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.getByText('У вас нет доступа к этому разделу.')).toBeDefined();
    expect(screen.queryByText('Settings Sensitive Page')).toBeNull();
  });

  // Q. current full-access admin still sees expected navigation
  it('Q: full-access admin sees all expected sidebar items and group heading', () => {
    const allCaps = getAllCapabilityKeys();
    mockAuth(allCaps);

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AdminLayout />
      </MemoryRouter>
    );

    const sidebar = getSidebar();

    const expectedNavNames = [
      'Главная',
      'Продавцы',
      'Аукционы',
      'Товары',
      'Модерация',
      'Категории и бренды',
      'Заказы',
      'Сборка заказов',
      'Доставка / Отгрузки',
      'Остатки / Склад',
      'Приемка поставок',
      'Платежи покупателей',
      'Возвраты',
      'Возмещения',
      'Выплаты продавцам',
      'Сводные отчеты',
      'Доступы и роли',
      'Сотрудники',
      'Журнал действий',
    ];

    for (const name of expectedNavNames) {
      expect(within(sidebar).getByText(name)).toBeDefined();
    }

    expect(within(sidebar).getByText('Администрирование')).toBeDefined();
  });

  // Owner without role bypass
  it('verifies owner role does NOT bypass capability check at frontend navigation level', () => {
    mockAuth(['orders.read'], 'owner');

    render(
      <MemoryRouter initialEntries={['/orders']}>
        <AdminLayout />
      </MemoryRouter>
    );

    const sidebar = getSidebar();
    expect(within(sidebar).queryByText('Сборка заказов')).toBeNull();
    expect(within(sidebar).getByText('Заказы')).toBeDefined();
  });

  // R. packing route visibility and guard requires warehouse.packing
  it('R: packing route guard strictly requires warehouse.packing and rejects orders.read or other roles', () => {
    const packingVisibility = getStaffScreenVisibility('/fulfillment/packing');
    expect(packingVisibility).toBe('warehouse.packing');

    // 1. Operator with warehouse.packing -> access granted
    mockAuth(['warehouse.packing']);
    const { unmount: unmount1 } = render(
      <MemoryRouter initialEntries={['/fulfillment/packing/fulf-123']}>
        <Routes>
          <Route
            path="/fulfillment/packing/:id"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/packing')}>
                <div>Packing Detail Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Packing Detail Screen')).toBeDefined();
    unmount1();

    // 2. Operator with only orders.read -> access denied
    mockAuth(['orders.read']);
    const { unmount: unmount2 } = render(
      <MemoryRouter initialEntries={['/fulfillment/packing/fulf-123']}>
        <Routes>
          <Route
            path="/fulfillment/packing/:id"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/packing')}>
                <div>Packing Detail Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Packing Detail Screen')).toBeNull();
    unmount2();

    // 3. Operator with warehouse.dispatch only -> access denied
    mockAuth(['warehouse.dispatch']);
    const { unmount: unmount3 } = render(
      <MemoryRouter initialEntries={['/fulfillment/packing/fulf-123']}>
        <Routes>
          <Route
            path="/fulfillment/packing/:id"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/packing')}>
                <div>Packing Detail Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Packing Detail Screen')).toBeNull();
    unmount3();
  });

  // S. dispatch route visibility and guard requires warehouse.dispatch
  it('S: dispatch route guard strictly requires warehouse.dispatch and rejects orders.read or other roles', () => {
    const dispatchVisibility = getStaffScreenVisibility('/fulfillment/dispatch');
    expect(dispatchVisibility).toBe('warehouse.dispatch');

    // 1. Operator with warehouse.dispatch -> access granted
    mockAuth(['warehouse.dispatch']);
    const { unmount: unmount1 } = render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route
            path="/fulfillment/dispatch/:id"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/dispatch')}>
                <div>Dispatch Detail Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Dispatch Detail Screen')).toBeDefined();
    unmount1();

    // 2. Operator with only orders.read -> access denied
    mockAuth(['orders.read']);
    const { unmount: unmount2 } = render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route
            path="/fulfillment/dispatch/:id"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/dispatch')}>
                <div>Dispatch Detail Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Dispatch Detail Screen')).toBeNull();
    unmount2();

    // 3. Operator with warehouse.packing only -> access denied
    mockAuth(['warehouse.packing']);
    const { unmount: unmount3 } = render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route
            path="/fulfillment/dispatch/:id"
            element={
              <AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/dispatch')}>
                <div>Dispatch Detail Screen</div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Dispatch Detail Screen')).toBeNull();
    unmount3();
  });
});
