// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminStaffDetail } from './AdminStaffDetail';
import * as adminApi from '@zamk/api-client/src/admin';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import type { StaffRoleWithPermissions } from '@zamk/api-client/src/types';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn(),
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  listStaffMembers: vi.fn(),
  listStaffRoles: vi.fn(),
  getStaffMemberPermissions: vi.fn(),
  updateStaffMemberPermissions: vi.fn(),
}));

const mockRoles: StaffRoleWithPermissions[] = [
  {
    id: 'role-manager',
    code: 'manager',
    name: 'Менеджер',
    isSystem: false,
    permissions: ['orders.read', 'orders.update_status'],
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

const mockMember: any = {
  userId: 'user-123',
  name: 'Иван Иванов',
  fullName: 'Иван Иванов',
  email: 'ivan@zamk.ru',
  status: 'active',
  staffStatus: 'active',
  mustChangePassword: false,
  roleId: 'role-manager',
  staffRoleId: 'role-manager',
  roleCode: 'manager',
  roleName: 'Менеджер',
  createdAt: '2026-02-15T12:00:00Z',
  updatedAt: '2026-02-15T12:00:00Z',
};

const defaultAuthContext: ReturnType<typeof useAdminAuth> = {
  user: {
    id: 'actor-current-id',
    email: 'actor@zamk.ru',
    name: 'Admin User',
    role: 'admin',
    status: 'active',
  },
  staff: {
    roleCode: 'owner',
    roleName: 'Владелец',
    status: 'active',
    permissions: ['staff.read', 'staff.permissions.manage'],
  },
  permissions: ['staff.read', 'staff.permissions.manage'],
  isAuthenticated: true,
  isLoading: false,
  error: null,
  hasPermission: () => true,
  hasAnyPermission: () => true,
  isOwner: () => true,
  isCoOwner: () => false,
  login: vi.fn(),
  logout: vi.fn(),
  refreshSession: vi.fn(),
  changePassword: vi.fn(),
  reloadStaff: vi.fn(),
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

async function waitForHeader() {
  await waitFor(() => {
    expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('Иван Иванов');
  });
}

describe('AdminStaffDetail Employee Workspace Shell (EMP.1C3C2R.1 & R.2A)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAdminAuth).mockReturnValue(defaultAuthContext);
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Tabs are exactly: Профиль, Доступ, Учётная запись', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();
    await waitForHeader();

    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((t) => t.textContent?.trim())).toEqual([
      'Профиль',
      'Доступ',
      'Учётная запись',
    ]);
  });

  it('2. Профиль is default active tab and shows empty state for responsibilities', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();
    await waitForHeader();

    expect(screen.getByTestId('tab-profile-content')).toBeDefined();
    expect(screen.getByText('Обязанности пока не указаны.')).toBeDefined();
    expect(screen.getByText('Рабочая заметка пока не добавлена.')).toBeDefined();
  });

  it('3. Access split editor switches sections and modes without API mutation', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    expect(screen.getByTestId('split-access-editor')).toBeDefined();

    // Select closed mode
    fireEvent.click(screen.getByRole('radio', { name: /Закрыт/i }));
    expect(screen.getByTestId('dirty-bottom-bar')).toBeDefined();

    // No mutation API called
    expect(adminApi.updateStaffMemberPermissions).not.toHaveBeenCalled();
  });

  it('A: useAdminAuth uses real typed context shape', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();
    await waitForHeader();

    expect(vi.mocked(useAdminAuth)).toHaveBeenCalled();
    const ctx = vi.mocked(useAdminAuth).mock.results[0].value;
    expect(ctx.user?.id).toBe('actor-current-id');
    expect(ctx.user?.role).toBe('admin');
    expect(typeof ctx.reloadStaff).toBe('function');
  });

  it('B: initial permissions GET 403 does NOT destroy base employee profile', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    // Full page forbidden must NOT be shown
    expect(screen.queryByTestId('error-forbidden')).toBeNull();
    expect(screen.queryByTestId('error-not-found')).toBeNull();

    // Base profile remains fully functional
    expect(screen.getByTestId('tab-profile-content')).toBeDefined();
    const profileTab = screen.getByTestId('tab-profile-content');
    expect(within(profileTab).getByText('ivan@zamk.ru')).toBeDefined();

    // Account tab remains accessible
    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    expect(screen.getByTestId('tab-account-content')).toBeDefined();
  });

  it('C: initial permissions GET 403 renders non-editable Access forbidden state', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    // Compact forbidden panel is rendered
    expect(screen.getByTestId('access-forbidden-panel')).toBeDefined();
    expect(screen.getByText('У вас нет права управлять доступом сотрудников.')).toBeDefined();
    expect(
      screen.getByText('Для изменения индивидуальных прав требуется право «Управление правами сотрудников».')
    ).toBeDefined();

    // No editable controls or dirty bars
    expect(screen.queryByTestId('split-access-editor')).toBeNull();
    expect(screen.queryByTestId('dirty-bottom-bar')).toBeNull();
    expect(screen.queryByRole('radio', { name: /Работа/i })).toBeNull();
    expect(adminApi.updateStaffMemberPermissions).not.toHaveBeenCalled();
  });

  it('D: PUT 403 enters permission-management-forbidden state', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(screen.getByText('У вас больше нет права изменять доступ сотрудников.')).toBeDefined();
    });

    // Enters forbidden panel state
    expect(screen.getByTestId('access-forbidden-panel')).toBeDefined();
    expect(screen.queryByTestId('split-access-editor')).toBeNull();
  });

  it('E: PUT 403 does NOT retry GET permissions', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(screen.getByText('У вас больше нет права изменять доступ сотрудников.')).toBeDefined();
    });

    // Exactly 1 call from initial load; NO retry GET
    expect(adminApi.getStaffMemberPermissions).toHaveBeenCalledTimes(1);
  });

  it('F: PUT 403 leaves no active Save/Continue controls', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(screen.getByText('У вас больше нет права изменять доступ сотрудников.')).toBeDefined();
    });

    expect(screen.queryByTestId('change-preview-modal')).toBeNull();
    expect(screen.queryByTestId('dirty-bottom-bar')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Сохранить изменения' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Продолжить' })).toBeNull();
  });

  it('G: successful self-revoke performs exactly one PUT', async () => {
    // Current actor is user-123
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      user: { ...defaultAuthContext.user!, id: 'user-123' },
    });

    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read', 'staff.permissions.manage'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('section-item-staff'));
    fireEvent.click(screen.getByLabelText(/Управление индивидуальными правами/i));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    // Self-revoke modal confirms
    expect(screen.getByTestId('self-revoke-confirm-modal')).toBeDefined();
    fireEvent.click(screen.getByRole('button', { name: 'Отключить и сохранить' }));

    await waitFor(() => {
      expect(adminApi.updateStaffMemberPermissions).toHaveBeenCalledTimes(1);
    });

    const [, payload] = vi.mocked(adminApi.updateStaffMemberPermissions).mock.calls[0];
    expect(payload.permissions).not.toContain('staff.permissions.manage');
    expect(payload.permissions).toContain('staff.read');
  });

  it('H: successful self-revoke immediately disables further access mutation UI', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      user: { ...defaultAuthContext.user!, id: 'user-123' },
    });

    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read', 'staff.permissions.manage'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('section-item-staff'));
    fireEvent.click(screen.getByLabelText(/Управление индивидуальными правами/i));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));
    fireEvent.click(screen.getByRole('button', { name: 'Отключить и сохранить' }));

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });

    expect(screen.getByText('Доступ сотрудника обновлён.')).toBeDefined();
    // Access tab is now locked down with forbidden panel
    expect(screen.getByTestId('access-forbidden-panel')).toBeDefined();
    expect(screen.queryByTestId('split-access-editor')).toBeNull();
    expect(screen.queryByTestId('dirty-bottom-bar')).toBeNull();
    expect(defaultAuthContext.reloadStaff).toHaveBeenCalled();
  });

  it('I: self-revoke with staff.read remaining keeps base employee profile visible', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      user: { ...defaultAuthContext.user!, id: 'user-123' },
    });

    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read', 'staff.permissions.manage'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('section-item-staff'));
    fireEvent.click(screen.getByLabelText(/Управление индивидуальными правами/i));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));
    fireEvent.click(screen.getByRole('button', { name: 'Отключить и сохранить' }));

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });

    // Profile tab remains intact
    fireEvent.click(screen.getByRole('tab', { name: 'Профиль' }));
    expect(screen.getByTestId('tab-profile-content')).toBeDefined();
    const profileTab = screen.getByTestId('tab-profile-content');
    expect(within(profileTab).getByText('Иван Иванов')).toBeDefined();

    // Account tab remains intact
    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    expect(screen.getByTestId('tab-account-content')).toBeDefined();
  });

  it('J: no second PUT can be triggered after successful self-revoke', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      user: { ...defaultAuthContext.user!, id: 'user-123' },
    });

    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read', 'staff.permissions.manage'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('section-item-staff'));
    fireEvent.click(screen.getByLabelText(/Управление индивидуальными правами/i));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));
    fireEvent.click(screen.getByRole('button', { name: 'Отключить и сохранить' }));

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });

    // Verify there are no buttons or dirty controls left
    expect(screen.queryByRole('button', { name: 'Сохранить изменения' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Продолжить' })).toBeNull();
    expect(adminApi.updateStaffMemberPermissions).toHaveBeenCalledTimes(1);
  });

  it('K: ordinary successful save remains unchanged', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read', 'orders.update_status'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });

    // Ordinary save keeps editor active and usable
    expect(screen.getByTestId('split-access-editor')).toBeDefined();
    expect(screen.queryByTestId('access-forbidden-panel')).toBeNull();
  });

  it('L: 409 still preserves editable draft', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['staff.read', 'staff.permissions.manage'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockRejectedValue({
      status: 409,
      code: 'last_permission_manager',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('section-item-staff'));
    fireEvent.click(screen.getByLabelText(/Управление индивидуальными правами/i));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(
        screen.getByText(
          'Невозможно отключить управление правами. В системе должен оставаться хотя бы один активный сотрудник с правом управления доступом.'
        )
      ).toBeDefined();
    });

    // Draft preserved, dirty bar still active
    expect(screen.getByTestId('dirty-bottom-bar')).toBeDefined();
    expect(screen.queryByTestId('save-success-banner')).toBeNull();
  });

  it('M: 400 preserves draft and shows error', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockRejectedValue({
      status: 400,
      code: 'validation_error',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(
        screen.getByText('Не удалось сохранить доступ. Проверьте выбранные действия.')
      ).toBeDefined();
    });

    expect(screen.getByTestId('dirty-bottom-bar')).toBeDefined();
  });

  it('N: network error preserves draft and allows retry', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions)
      .mockRejectedValueOnce(new Error('Network disconnected'))
      .mockResolvedValueOnce({
        userId: 'user-123',
        permissions: ['orders.read', 'orders.update_status'],
      });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));

    const saveBtn = screen.getByRole('button', { name: 'Сохранить изменения' });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(
        screen.getByText('Не удалось сохранить изменения. Попробуйте ещё раз.')
      ).toBeDefined();
    });

    // Retry
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });
  });

  it('O: blocked target can save access and displays subtle note', async () => {
    const blockedMember = { ...mockMember, status: 'blocked', staffStatus: 'blocked' };
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [blockedMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read', 'orders.update_status'],
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('blocked-banner')).toBeDefined();
    expect(
      screen.getAllByText('Настройки сохранятся, но доступ не будет действовать до активации учётной записи.').length
    ).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });
  });

  it('P: archived target can save access and displays subtle note', async () => {
    const archivedMember = { ...mockMember, status: 'archived', staffStatus: 'archived' };
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [archivedMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read', 'orders.update_status'],
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('archived-banner')).toBeDefined();
    expect(
      screen.getAllByText('Настройки сохранятся, но доступ не будет действовать до активации учётной записи.').length
    ).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }));

    await waitFor(() => {
      expect(screen.getByTestId('save-success-banner')).toBeDefined();
    });
  });

  it('Q: Cancel before save still restores original without PUT', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));
    expect(screen.getByTestId('dirty-bottom-bar')).toBeDefined();

    // Click Cancel in sticky bottom bar
    const dirtyBar = screen.getByTestId('dirty-bottom-bar');
    fireEvent.click(within(dirtyBar).getByRole('button', { name: 'Отмена' }));

    expect(screen.queryByTestId('dirty-bottom-bar')).toBeNull();
    expect(adminApi.updateStaffMemberPermissions).not.toHaveBeenCalled();
  });

  it('R: navigation dirty warning triggers window.confirm', async () => {
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: [mockMember] });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    const confirmSpy = vi.spyOn(window, 'confirm');

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByRole('radio', { name: /Работа/i }));

    confirmSpy.mockReturnValueOnce(false);
    const backLink = screen.getByText(/К списку сотрудников/);
    fireEvent.click(backLink);

    expect(confirmSpy).toHaveBeenCalledWith('Есть несохранённые изменения. Выйти без сохранения?');
    confirmSpy.mockRestore();
  });

  it('S: Base 403 on listStaffMembers still displays full page forbidden', async () => {
    vi.mocked(adminApi.listStaffMembers).mockRejectedValue({ status: 403, code: 'forbidden' });

    renderComponent('user-123');
    await waitFor(() => {
      expect(screen.getByTestId('error-forbidden')).toBeDefined();
    });
    expect(screen.getByText('У вас нет прав для просмотра профилей сотрудников.')).toBeDefined();
  });
});
