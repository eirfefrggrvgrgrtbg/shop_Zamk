// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminStaffDetail } from './AdminStaffDetail';
import * as adminApi from '@zamk/api-client/src/admin';
import type { StaffMemberView, StaffRoleWithPermissions } from '@zamk/api-client/src/types';
import { STAFF_CAPABILITY_GROUPS } from '../config/staffCapabilities';

vi.mock('@zamk/api-client/src/admin', () => ({
  listStaffMembers: vi.fn(),
  listStaffRoles: vi.fn(),
  getStaffMemberPermissions: vi.fn(),
}));

const mockRoles: StaffRoleWithPermissions[] = [
  {
    id: 'role-manager',
    code: 'manager',
    name: 'Менеджер',
    isSystem: false,
    permissions: ['orders.read', 'products.read', 'warehouse.receiving'],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'role-owner',
    code: 'owner',
    name: 'Владелец',
    isSystem: true,
    permissions: ['staff.read', 'staff.permissions.manage'],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

const mockMember: StaffMemberView = {
  userId: 'user-123',
  name: 'Иван Иванов',
  email: 'ivan@zamk.ru',
  userStatus: 'active',
  mustChangePassword: false,
  staffStatus: 'active',
  roleCode: 'manager',
  roleName: 'Менеджер',
  roleId: 'role-manager',
  permissions: [],
  createdAt: '2026-02-15T12:00:00Z',
  updatedAt: '2026-02-15T12:00:00Z',
};

function renderComponent(userId = 'user-123') {
  return render(
    <MemoryRouter initialEntries={[`/staff/${userId}`]}>
      <Routes>
        <Route path="/staff/:userId" element={<AdminStaffDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('AdminStaffDetail Visual UX (EMP.1C2B1 Rework)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('A. default mode is "Назначенные"', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('Иван Иванов');
    });

    // Check segmented switch: "Назначенные (1)" has active styling/font-semibold
    const assignedBtn = screen.getByRole('button', { name: /Назначенные/ });
    expect(assignedBtn).toBeDefined();
    expect(assignedBtn.className).toContain('font-semibold');
  });

  it('B. unassigned permission is NOT rendered in default assigned view', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Просмотр заказов')).toBeDefined();
    });

    // Unassigned permissions must NOT be rendered in default view
    expect(screen.queryByText('Просмотр товаров')).toBeNull();
    expect(screen.queryByText('Приёмка поставок')).toBeNull();
    expect(screen.queryByText('Создание сотрудников')).toBeNull();
  });

  it('C. assigned permissions are rendered with clean read-only indicator', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read', 'warehouse.receiving'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Просмотр заказов')).toBeDefined();
      expect(screen.getByText('Приёмка поставок')).toBeDefined();
    });

    // Verify there are no checkboxes in read-only mode
    expect(screen.queryByRole('checkbox')).toBeNull();
  });

  it('D. groups with 0 assigned rights are hidden in default view', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    // User only has orders.read (in "orders" group: "Заказы и отправления")
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Заказы и отправления')).toBeDefined();
    });

    // Other groups with 0 assigned rights must be hidden
    expect(screen.queryByText('Сотрудники и роли')).toBeNull();
    expect(screen.queryByText('Склад и логистика')).toBeNull();
    expect(screen.queryByText('Аукционы')).toBeNull();
    expect(screen.queryByText('Финансы и выплаты')).toBeNull();
  });

  it('E. switching to "Все права" exposes all 11 group headers', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Все права/ })).toBeDefined();
    });

    const allBtn = screen.getByRole('button', { name: /Все права/ });
    fireEvent.click(allBtn);

    // All 11 group titles should now be visible in accordion headers
    for (const group of STAFF_CAPABILITY_GROUPS) {
      expect(screen.getByText(group.title)).toBeDefined();
    }
  });

  it('F. all-rights groups are collapsed initially', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Все права/ })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: /Все права/ }));

    // Capabilities inside collapsed groups must not be visible yet
    expect(screen.queryByText('Приёмка поставок')).toBeNull();
    expect(screen.queryByText('Создание сотрудников')).toBeNull();
  });

  it('G. expanding group shows assigned and unassigned capabilities correctly', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Все права/ })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: /Все права/ }));

    // Click on "Заказы и отправления" accordion
    const ordersAccordionHeader = screen.getByText('Заказы и отправления');
    fireEvent.click(ordersAccordionHeader);

    // Assigned capability: "Просмотр заказов"
    expect(screen.getByText('Просмотр заказов')).toBeDefined();
    // Unassigned capability in the same group: "Создание отправлений"
    expect(screen.getByText('Создание отправлений')).toBeDefined();
  });

  it('H. search can find an unassigned capability even while default mode was "Назначенные"', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    // User only has orders.read
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Поиск по правам...')).toBeDefined();
    });

    // In default assigned mode, "Приёмка поставок" is unassigned and not visible
    expect(screen.queryByText('Приёмка поставок')).toBeNull();

    // Type "приёмка" in search
    const searchInput = screen.getByPlaceholderText('Поиск по правам...');
    fireEvent.change(searchInput, { target: { value: 'приёмка' } });

    // Now it finds "Приёмка поставок" in "Склад и логистика"
    expect(screen.getByText('Склад и логистика')).toBeDefined();
    expect(screen.getByText('Приёмка поставок')).toBeDefined();
  });

  it('I. zero direct permissions shows compact empty state rather than 11 empty cards', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: [],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('У сотрудника нет индивидуальных прав доступа.')).toBeDefined();
    });

    // 11 empty cards should NOT be rendered in assigned view
    for (const group of STAFF_CAPABILITY_GROUPS) {
      expect(screen.queryByText(group.title)).toBeNull();
    }
  });

  it('J. preset/custom badge remains correct', async () => {
    // 1. Matches role preset
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read', 'products.read', 'warehouse.receiving'],
    });

    const { unmount } = renderComponent();

    await waitFor(() => {
      expect(screen.getByText('По шаблону')).toBeDefined();
    });
    expect(screen.queryByText('Индивидуально настроено')).toBeNull();

    unmount();

    // 2. Differs from role preset
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Индивидуально настроено')).toBeDefined();
    });
    expect(screen.queryByText('По шаблону')).toBeNull();
  });

  it('K. 403, 404, blocked, and archived existing behavior remains correct', async () => {
    // 1. Blocked warning
    const blockedMember: StaffMemberView = { ...mockMember, staffStatus: 'blocked' };
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [blockedMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: [],
    });

    const { unmount: unmountBlocked } = renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Сотрудник заблокирован/)).toBeDefined();
      expect(screen.getByText('Заблокирован')).toBeDefined();
    });

    unmountBlocked();

    // 2. 403 state
    const err403: any = new Error('Forbidden');
    err403.status = 403;
    err403.code = 'forbidden';
    vi.mocked(adminApi.getStaffMemberPermissions).mockRejectedValue(err403);

    const { unmount: unmount403 } = renderComponent();

    await waitFor(() => {
      expect(
        screen.getByText('У вас нет прав для просмотра прав этого сотрудника.')
      ).toBeDefined();
    });

    unmount403();

    // 3. 404 state
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [] });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: [],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Сотрудник не найден.')).toBeDefined();
    });
  });
});
