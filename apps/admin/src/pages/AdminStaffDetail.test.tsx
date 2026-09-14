// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminStaffDetail, generatePassword, countUnicodeChars } from './AdminStaffDetail';
import * as adminApi from '@zamk/api-client/src/admin';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import type { StaffRoleWithPermissions } from '@zamk/api-client/src/types';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn(),
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  getStaffMember: vi.fn(),
  patchStaffMemberProfile: vi.fn(),
  listStaffMembers: vi.fn(),
  listStaffRoles: vi.fn(),
  getStaffMemberPermissions: vi.fn(),
  updateStaffMemberPermissions: vi.fn(),
  updateStaffRole: vi.fn(),
  updateStaffStatus: vi.fn(),
  resetStaffPassword: vi.fn(),
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
  responsibilities: null,
  workNote: null,
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
    vi.mocked(adminApi.patchStaffMemberProfile).mockResolvedValue({ success: true });
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Tabs are exactly: Профиль, Доступ, Учётная запись', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent();
    await waitForHeader();

    expect(screen.getByTestId('tab-profile-content')).toBeDefined();
    expect(screen.getByText('Обязанности пока не указаны')).toBeDefined();
    expect(screen.getByText('Рабочая заметка пока не добавлена')).toBeDefined();
  });

  it('3. Access split editor switches sections and modes without API mutation', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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

    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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

    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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

    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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

    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(blockedMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(archivedMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
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

  it('S: Base 403 on getStaffMember still displays full page forbidden', async () => {
    vi.mocked(adminApi.getStaffMember).mockRejectedValue({ status: 403, code: 'forbidden' });

    renderComponent('user-123');
    await waitFor(() => {
      expect(screen.getByTestId('error-forbidden')).toBeDefined();
    });
    expect(screen.getByText('У вас нет прав для просмотра профилей сотрудников.')).toBeDefined();
  });
});

describe('EMP.1D1 — Employee Account Management Tab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAdminAuth).mockReturnValue(defaultAuthContext);
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
    vi.mocked(adminApi.patchStaffMemberProfile).mockResolvedValue({ success: true });
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read', 'orders.update_status'],
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('A: Account tab renders real employee status, creation date, password change requirement, and system info', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    const accountContent = screen.getByTestId('tab-account-content');
    expect(accountContent).toBeDefined();

    // Status row renders real status
    const statusRow = screen.getByTestId('account-row-status');
    expect(statusRow.textContent).toContain('Активен');

    // Creation date row
    const createdRow = screen.getByTestId('account-row-created-at');
    expect(createdRow.textContent).toContain('15.02.2026');

    // Must change password row
    const passwordRow = screen.getByTestId('account-row-must-change-password');
    expect(passwordRow.textContent).toContain('Не требуется');

    // Collapsed system info
    expect(screen.queryByTestId('system-info-panel')).toBeNull();
    fireEvent.click(screen.getByText('Системная информация'));
    const sysPanel = screen.getByTestId('system-info-panel');
    expect(sysPanel.textContent).toContain('user-123');
    expect(sysPanel.textContent).toContain('manager');
  });

  it('B: active employee shows valid actions (reset password, block, archive) and no unblock/restore', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    const actionsSection = screen.getByTestId('account-actions-section');
    expect(within(actionsSection).getByRole('button', { name: 'Сбросить пароль' })).toBeDefined();
    expect(within(actionsSection).getByRole('button', { name: 'Заблокировать' })).toBeDefined();
    expect(within(actionsSection).queryByRole('button', { name: 'Разблокировать' })).toBeNull();
    expect(within(actionsSection).queryByRole('button', { name: 'Восстановить из архива' })).toBeNull();

    // Danger zone with archive
    const dangerZone = screen.getByTestId('danger-zone-section');
    expect(within(dangerZone).getByRole('button', { name: 'Архивировать сотрудника' })).toBeDefined();
    expect(dangerZone.textContent).toContain('Сотрудник потеряет доступ к Admin. Индивидуальные права сохраняются.');
  });

  it('C: blocked employee shows valid actions (reset password, unblock, archive) and explanation banner', async () => {
    const blockedMember = { ...mockMember, status: 'blocked', staffStatus: 'blocked' };
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(blockedMember);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    const explanation = screen.getByTestId('account-blocked-explanation');
    expect(explanation.textContent).toContain(
      'Учётная запись заблокирована. Настроенные права сохранены, но доступ к Admin не действует до разблокировки.'
    );

    const actionsSection = screen.getByTestId('account-actions-section');
    expect(within(actionsSection).getByRole('button', { name: 'Сбросить пароль' })).toBeDefined();
    expect(within(actionsSection).getByRole('button', { name: 'Разблокировать' })).toBeDefined();
    expect(within(actionsSection).queryByRole('button', { name: 'Заблокировать' })).toBeNull();

    const dangerZone = screen.getByTestId('danger-zone-section');
    expect(within(dangerZone).getByRole('button', { name: 'Архивировать сотрудника' })).toBeDefined();
  });

  it('D: archived behavior matches actual backend semantics (restore button, no danger zone, explanation)', async () => {
    const archivedMember = { ...mockMember, status: 'archived', staffStatus: 'archived' };
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(archivedMember);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    const explanation = screen.getByTestId('account-archived-explanation');
    expect(explanation.textContent).toContain(
      'Учётная запись находится в архиве. Индивидуальные права сохранены, но доступ к Admin закрыт.'
    );

    const actionsSection = screen.getByTestId('account-actions-section');
    expect(within(actionsSection).getByRole('button', { name: 'Восстановить из архива' })).toBeDefined();
    expect(within(actionsSection).queryByRole('button', { name: 'Заблокировать' })).toBeNull();
    expect(within(actionsSection).queryByRole('button', { name: 'Разблокировать' })).toBeNull();

    // Danger zone is not displayed when already archived
    expect(screen.queryByTestId('danger-zone-section')).toBeNull();
  });

  it('E: block requires confirmation modal', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    expect(screen.queryByTestId('block-confirm-modal')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));

    const modal = screen.getByTestId('block-confirm-modal');
    expect(modal).toBeDefined();
    expect(modal.textContent).toContain('Заблокировать сотрудника?');
    expect(modal.textContent).toContain('больше не сможет войти в Admin до разблокировки');
    expect(adminApi.updateStaffStatus).not.toHaveBeenCalled();
  });

  it('F: cancelling block performs no API call and closes modal', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));

    const modal = screen.getByTestId('block-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Отмена' }));

    expect(screen.queryByTestId('block-confirm-modal')).toBeNull();
    expect(adminApi.updateStaffStatus).not.toHaveBeenCalled();
  });

  it('G: successful block updates header + account tab immediately without full reload', async () => {
    vi.mocked(adminApi.updateStaffStatus).mockResolvedValueOnce(undefined as any);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));

    const modal = screen.getByTestId('block-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Заблокировать' }));

    await waitFor(() => {
      expect(adminApi.updateStaffStatus).toHaveBeenCalledWith('user-123', { status: 'blocked' });
    });

    // Modal closed
    expect(screen.queryByTestId('block-confirm-modal')).toBeNull();

    // Account tab status updated immediately
    const statusRow = screen.getByTestId('account-row-status');
    expect(statusRow.textContent).toContain('Заблокирован');

    // Header badge updated immediately
    expect(screen.getByTestId('account-blocked-explanation')).toBeDefined();

    // Actions updated: now shows Разблокировать
    expect(screen.getByRole('button', { name: 'Разблокировать' })).toBeDefined();
  });

  it('H: unblock/reactivate works if supported (calls updateStaffStatus active and restores state)', async () => {
    const blockedMember = { ...mockMember, status: 'blocked', staffStatus: 'blocked' };
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(blockedMember);
    vi.mocked(adminApi.updateStaffStatus).mockResolvedValueOnce(undefined as any);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Разблокировать' }));

    await waitFor(() => {
      expect(adminApi.updateStaffStatus).toHaveBeenCalledWith('user-123', { status: 'active' });
    });

    const statusRow = screen.getByTestId('account-row-status');
    expect(statusRow.textContent).toContain('Активен');
    expect(screen.getByRole('button', { name: 'Заблокировать' })).toBeDefined();
  });

  it('I: archive requires confirmation modal', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    expect(screen.queryByTestId('archive-confirm-modal')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Архивировать сотрудника' }));

    const modal = screen.getByTestId('archive-confirm-modal');
    expect(modal).toBeDefined();
    expect(modal.textContent).toContain('Архивировать сотрудника?');
    expect(modal.textContent).toContain('Аккаунт будет переведён в архив и потеряет доступ к Admin.');
    expect(adminApi.updateStaffStatus).not.toHaveBeenCalled();
  });

  it('J: cancelling archive performs no API call and closes modal', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Архивировать сотрудника' }));

    const modal = screen.getByTestId('archive-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Отмена' }));

    expect(screen.queryByTestId('archive-confirm-modal')).toBeNull();
    expect(adminApi.updateStaffStatus).not.toHaveBeenCalled();
  });

  it('K: successful archive updates canonical state immediately', async () => {
    vi.mocked(adminApi.updateStaffStatus).mockResolvedValueOnce(undefined as any);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Архивировать сотрудника' }));

    const modal = screen.getByTestId('archive-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Архивировать' }));

    await waitFor(() => {
      expect(adminApi.updateStaffStatus).toHaveBeenCalledWith('user-123', { status: 'archived' });
    });

    expect(screen.queryByTestId('archive-confirm-modal')).toBeNull();

    const statusRow = screen.getByTestId('account-row-status');
    expect(statusRow.textContent).toContain('В архиве');
    expect(screen.getByTestId('account-archived-explanation')).toBeDefined();
    expect(screen.queryByTestId('danger-zone-section')).toBeNull();
  });

  it('L: password reset uses existing API (resetStaffPassword)', async () => {
    vi.mocked(adminApi.resetStaffPassword).mockResolvedValueOnce(undefined as any);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить пароль' }));

    const resetModal = screen.getByTestId('reset-password-modal');
    expect(resetModal.textContent).toContain('ivan@zamk.ru');

    // Enter a password with at least 8 characters
    const input = within(resetModal).getByPlaceholderText('Минимум 8 символов');
    fireEvent.change(input, { target: { value: 'SuperSecret123!' } });

    fireEvent.click(within(resetModal).getByRole('button', { name: 'Сбросить пароль' }));

    await waitFor(() => {
      expect(adminApi.resetStaffPassword).toHaveBeenCalledWith('user-123', {
        temporaryPassword: 'SuperSecret123!',
      });
    });
  });

  it('M: temporary password is displayed only from actual backend response (success modal with copy)', async () => {
    vi.mocked(adminApi.resetStaffPassword).mockResolvedValueOnce(undefined as any);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить пароль' }));

    const resetModal = screen.getByTestId('reset-password-modal');
    const input = within(resetModal).getByPlaceholderText('Минимум 8 символов');
    fireEvent.change(input, { target: { value: 'ValidPassword123' } });

    fireEvent.click(within(resetModal).getByRole('button', { name: 'Сбросить пароль' }));

    await waitFor(() => {
      expect(screen.getByTestId('password-success-modal')).toBeDefined();
    });

    const successModal = screen.getByTestId('password-success-modal');
    expect(successModal.textContent).toContain('Новый временный пароль');
    expect(successModal.textContent).toContain('ValidPassword123');
    expect(successModal.textContent).toContain('Передайте пароль сотруднику безопасным способом.');
    expect(within(successModal).getByText('Копировать')).toBeDefined();

    // Must change password row now shows 'Требуется'
    const pwdRow = screen.getByTestId('account-row-must-change-password');
    expect(pwdRow.textContent).toContain('Требуется');

    // Close success modal
    fireEvent.click(within(successModal).getByRole('button', { name: 'Понятно' }));
    expect(screen.queryByTestId('password-success-modal')).toBeNull();
  });

  it('M2: failed password reset does not show temporary password modal', async () => {
    vi.mocked(adminApi.resetStaffPassword).mockRejectedValueOnce({
      status: 500,
      message: 'Failed to reset password',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить пароль' }));

    const resetModal = screen.getByTestId('reset-password-modal');
    const input = within(resetModal).getByPlaceholderText('Минимум 8 символов');
    fireEvent.change(input, { target: { value: 'ValidPassword123' } });

    fireEvent.click(within(resetModal).getByRole('button', { name: 'Сбросить пароль' }));

    await waitFor(() => {
      expect(screen.getByTestId('reset-password-error')).toBeDefined();
    });

    expect(screen.queryByTestId('password-success-modal')).toBeNull();
  });

  it('N: read-only actor does not see unauthorized mutation controls', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      permissions: ['staff.read'],
      hasPermission: (p: string) => p === 'staff.read',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));

    // Data is visible
    expect(screen.getByTestId('account-row-status').textContent).toContain('Активен');
    expect(screen.getByTestId('account-row-created-at')).toBeDefined();

    // Mutation buttons are hidden
    expect(screen.queryByRole('button', { name: 'Сбросить пароль' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Заблокировать' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Разблокировать' })).toBeNull();
    expect(screen.queryByTestId('danger-zone-section')).toBeNull();
  });

  it('O: 403 has human error', async () => {
    vi.mocked(adminApi.updateStaffStatus).mockRejectedValueOnce({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));

    const modal = screen.getByTestId('block-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Заблокировать' }));

    await waitFor(() => {
      expect(screen.getByTestId('modal-error').textContent).toContain(
        'У вас нет права выполнять это действие.'
      );
    });
  });

  it('P: backend safety conflict is preserved (last owner lockout & last permission manager)', async () => {
    // 1. last owner conflict
    vi.mocked(adminApi.updateStaffStatus).mockRejectedValueOnce({
      status: 409,
      code: 'last_owner',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));

    const modal = screen.getByTestId('block-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Заблокировать' }));

    await waitFor(() => {
      expect(screen.getByTestId('modal-error').textContent).toContain(
        'Невозможно заблокировать или архивировать последнего владельца платформы.'
      );
    });

    fireEvent.click(within(modal).getByRole('button', { name: 'Отмена' }));

    // 2. last permission manager conflict on archive
    vi.mocked(adminApi.updateStaffStatus).mockRejectedValueOnce({
      status: 409,
      code: 'last_permission_manager',
    });

    fireEvent.click(screen.getByRole('button', { name: 'Архивировать сотрудника' }));
    const archiveModal = screen.getByTestId('archive-confirm-modal');
    fireEvent.click(within(archiveModal).getByRole('button', { name: 'Архивировать' }));

    await waitFor(() => {
      expect(screen.getByTestId('modal-error').textContent).toContain(
        'Невозможно заблокировать сотрудника: это последний сотрудник с правом управления доступом.'
      );
    });
  });

  it('Q: Access tab is not modified by account workspace', async () => {
    renderComponent('user-123');
    await waitForHeader();

    // Access tab functions normally
    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    expect(screen.getByTestId('tab-access-content')).toBeDefined();
    expect(screen.getAllByText('Заказы').length).toBeGreaterThan(0);
    expect(screen.getByRole('radio', { name: /Просмотр/i })).toBeDefined();
    expect(screen.getByRole('radio', { name: /Работа/i })).toBeDefined();

    // Switch to Account tab and back to Access tab
    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    expect(screen.getByTestId('tab-account-content')).toBeDefined();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    expect(screen.getByTestId('tab-access-content')).toBeDefined();
  });

  it('R: permission set is not mutated by account actions', async () => {
    vi.mocked(adminApi.updateStaffStatus).mockResolvedValueOnce(undefined as any);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));

    const modal = screen.getByTestId('block-confirm-modal');
    fireEvent.click(within(modal).getByRole('button', { name: 'Заблокировать' }));

    await waitFor(() => {
      expect(adminApi.updateStaffStatus).toHaveBeenCalledTimes(1);
    });

    // Verify updateStaffMemberPermissions was NEVER called
    expect(adminApi.updateStaffMemberPermissions).not.toHaveBeenCalled();
  });

  describe('Password Generator Security (Web Crypto)', () => {
    const ALLOWED_ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';

    it('A: password generator does NOT depend on Math.random', () => {
      const mathRandomSpy = vi.spyOn(Math, 'random');
      generatePassword();
      expect(mathRandomSpy).not.toHaveBeenCalled();
      mathRandomSpy.mockRestore();
    });

    it('B: Web Crypto getRandomValues is used', () => {
      const cryptoSpy = vi.spyOn(globalThis.crypto, 'getRandomValues');
      const pwd = generatePassword();
      expect(cryptoSpy).toHaveBeenCalled();
      expect(typeof pwd).toBe('string');
      cryptoSpy.mockRestore();
    });

    it('C: generated password length is exactly 12', () => {
      for (let i = 0; i < 20; i++) {
        const pwd = generatePassword();
        expect(pwd.length).toBe(12);
      }
    });

    it('D: every generated character belongs to the allowed alphabet', () => {
      for (let i = 0; i < 20; i++) {
        const pwd = generatePassword();
        for (const char of pwd) {
          expect(ALLOWED_ALPHABET.includes(char)).toBe(true);
        }
      }
    });

    it('E & F & G & H: Generate button places generated password into the reset form and reset flow sends it & displays it', async () => {
      vi.mocked(adminApi.resetStaffPassword).mockResolvedValueOnce(undefined as any);

      renderComponent('user-123');
      await waitForHeader();

      fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
      fireEvent.click(screen.getByRole('button', { name: 'Сбросить пароль' }));

      const resetModal = screen.getByTestId('reset-password-modal');
      const input = within(resetModal).getByPlaceholderText('Минимум 8 символов') as HTMLInputElement;

      // Click "Сгенерировать"
      fireEvent.click(within(resetModal).getByRole('button', { name: 'Сгенерировать' }));

      const generatedVal = input.value;
      expect(generatedVal.length).toBe(12);
      for (const char of generatedVal) {
        expect(ALLOWED_ALPHABET.includes(char)).toBe(true);
      }

      // Submit form
      fireEvent.click(within(resetModal).getByRole('button', { name: 'Сбросить пароль' }));

      await waitFor(() => {
        expect(adminApi.resetStaffPassword).toHaveBeenCalledWith('user-123', {
          temporaryPassword: generatedVal,
        });
      });

      // Success modal displays exactly the generated password
      const successModal = screen.getByTestId('password-success-modal');
      expect(successModal.textContent).toContain(generatedVal);
    });
  });
});


describe('EMP.1D2C — Employee Responsibilities & Internal Work Note UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAdminAuth).mockReturnValue(defaultAuthContext);
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.patchStaffMemberProfile).mockResolvedValue({ success: true });
  });

  afterEach(() => {
    cleanup();
  });

  it('A: AdminStaffDetail uses getStaffMember for detail hydration', async () => {
    renderComponent('user-123');
    await waitForHeader();

    expect(adminApi.getStaffMember).toHaveBeenCalledWith('user-123');
  });

  it('B: responsibilities returned by backend render in Profile', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Руководство отделом приёмки и инвентаризации',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('display-responsibilities').textContent).toBe(
      'Руководство отделом приёмки и инвентаризации'
    );
  });

  it('C: workNote returned by backend renders in Profile', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      workNote: 'График работы 2/2 с 9:00 до 21:00',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('display-work-note').textContent).toBe(
      'График работы 2/2 с 9:00 до 21:00'
    );
  });

  it('D: multiline formatting is preserved', async () => {
    const multilineText = 'Линия 1: Первичная приёмка\nЛиния 2: Контроль маркировки\nЛиния 3: Склад';
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: multilineText,
    });

    renderComponent('user-123');
    await waitForHeader();

    const el = screen.getByTestId('display-responsibilities');
    expect(el.textContent).toBe(multilineText);
    expect(el.className).toContain('whitespace-pre-wrap');
  });

  it('E: null responsibilities shows "Обязанности пока не указаны"', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: null,
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('display-responsibilities-empty').textContent?.trim()).toBe(
      'Обязанности пока не указаны'
    );
  });

  it('F: null workNote shows "Рабочая заметка пока не добавлена"', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      workNote: null,
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('display-work-note-empty').textContent?.trim()).toBe(
      'Рабочая заметка пока не добавлена'
    );
  });

  it('G: actor with staff.update sees Edit button', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      hasPermission: (p: string) => p === 'staff.update' || p === 'staff.read',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getAllByRole('button', { name: 'Редактировать' })[0]).toBeDefined();
  });

  it('H: actor without staff.update does NOT see Edit button', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      hasPermission: (p: string) => p === 'staff.read',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.queryByRole('button', { name: 'Редактировать' })).toBeNull();
  });

  it('I: Edit populates draft from canonical values', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Контроль заказов',
      workNote: 'Работает удаленно',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);

    const respTextarea = screen.getByTestId('textarea-responsibilities') as HTMLTextAreaElement;
    const noteTextarea = screen.getByTestId('textarea-work-note') as HTMLTextAreaElement;

    expect(respTextarea.value).toBe('Контроль заказов');
    expect(noteTextarea.value).toBe('Работает удаленно');
  });

  it('J: typing changes only local draft before Save', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);

    const respTextarea = screen.getByTestId('textarea-responsibilities') as HTMLTextAreaElement;
    fireEvent.change(respTextarea, { target: { value: 'Новые обязанности' } });

    expect(respTextarea.value).toBe('Новые обязанности');
    expect(adminApi.patchStaffMemberProfile).not.toHaveBeenCalled();
  });

  it('K: Cancel performs zero PATCH calls', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Тест' } });
    fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));

    expect(adminApi.patchStaffMemberProfile).not.toHaveBeenCalled();
  });

  it('L: Cancel restores canonical display', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Канонические обязанности',
      workNote: null,
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Случайный текст' } });
    fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));

    expect(screen.queryByTestId('textarea-responsibilities')).toBeNull();
    expect(screen.getByTestId('display-responsibilities').textContent).toBe('Канонические обязанности');
  });

  it('M: changing only responsibilities sends only responsibilities', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Старые обязанности',
      workNote: 'Заметка без изменений',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), {
      target: { value: 'Новые обязанности' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        responsibilities: 'Новые обязанности',
      });
    });
  });

  it('N: changing only workNote sends only workNote', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Обязанности без изменений',
      workNote: 'Старая заметка',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-work-note'), {
      target: { value: 'Обновлённая заметка' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        workNote: 'Обновлённая заметка',
      });
    });
  });

  it('O: changing both sends both', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Старые обязанности',
      workNote: 'Старая заметка',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Новые об' } });
    fireEvent.change(screen.getByTestId('textarea-work-note'), { target: { value: 'Новая зам' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        responsibilities: 'Новые об',
        workNote: 'Новая зам',
      });
    });
  });

  it('P: clearing responsibilities sends null', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Ранее заданные обязанности',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: '   ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        responsibilities: null,
      });
    });
  });

  it('Q: clearing workNote sends null', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      workNote: 'Ранее заданная заметка',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-work-note'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        workNote: null,
      });
    });
  });

  it('R: unchanged normalized draft performs zero PATCH calls and exits edit mode', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      responsibilities: 'Обязанности',
      workNote: 'Заметка',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    // Add surrounding whitespace that trims to identical value
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: '  Обязанности  ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    expect(adminApi.patchStaffMemberProfile).not.toHaveBeenCalled();
    expect(screen.queryByTestId('textarea-responsibilities')).toBeNull();
  });

  it('S & T & U & V: successful save flow (PATCH -> refetch -> truth -> exit edit -> success message)', async () => {
    const updatedMember = {
      ...mockMember,
      responsibilities: 'Сохранённые обязанности',
      workNote: 'Сохранённая заметка',
    };

    vi.mocked(adminApi.patchStaffMemberProfile).mockResolvedValue({ success: true });
    vi.mocked(adminApi.getStaffMember)
      .mockResolvedValueOnce(mockMember)
      .mockResolvedValueOnce(updatedMember);

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Сохранённые обязанности' } });
    fireEvent.change(screen.getByTestId('textarea-work-note'), { target: { value: 'Сохранённая заметка' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalled();
      expect(adminApi.getStaffMember).toHaveBeenCalledTimes(2);
    });

    // U. Exits edit mode
    expect(screen.queryByTestId('textarea-responsibilities')).toBeNull();

    // T. Visible canonical truth
    expect(screen.getByTestId('display-responsibilities').textContent).toBe('Сохранённые обязанности');
    expect(screen.getByTestId('display-work-note').textContent).toBe('Сохранённая заметка');

    // V. Success message appears
    expect(screen.getByTestId('profile-success-banner').textContent).toContain('Профиль сотрудника обновлён.');
  });

  it('W: 403 preserves draft and displays human error', async () => {
    vi.mocked(adminApi.patchStaffMemberProfile).mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Черновик' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(screen.getByTestId('profile-error-banner')).toBeDefined();
    });

    expect(screen.getByTestId('profile-error-banner').textContent).toContain(
      'У вас нет права редактировать профиль сотрудника.'
    );
    // Draft preserved
    expect((screen.getByTestId('textarea-responsibilities') as HTMLTextAreaElement).value).toBe('Черновик');
  });

  it('X: 400 preserves draft and displays error message', async () => {
    vi.mocked(adminApi.patchStaffMemberProfile).mockRejectedValue({
      status: 400,
      code: 'validation_error',
      message: 'Некорректный формат данных',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Особый черновик' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(screen.getByTestId('profile-error-banner')).toBeDefined();
    });

    expect(screen.getByTestId('profile-error-banner').textContent).toContain('Некорректный формат данных');
    expect((screen.getByTestId('textarea-responsibilities') as HTMLTextAreaElement).value).toBe('Особый черновик');
  });

  it('Y: network failure preserves draft', async () => {
    vi.mocked(adminApi.patchStaffMemberProfile).mockRejectedValue(new Error('Failed to fetch'));

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: 'Текст при обрыве связи' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(screen.getByTestId('profile-error-banner')).toBeDefined();
    });

    expect(screen.getByTestId('profile-error-banner').textContent).toContain(
      'Не удалось сохранить профиль. Попробуйте ещё раз.'
    );
    expect((screen.getByTestId('textarea-responsibilities') as HTMLTextAreaElement).value).toBe(
      'Текст при обрыве связи'
    );
  });

  it('Z: exactly 4000 Unicode characters allowed and Save remains enabled', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);

    const exact4000 = 'Ж'.repeat(4000);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: exact4000 } });

    expect(screen.getByTestId('responsibilities-char-counter').textContent).toBe('4000 / 4000');
    const saveBtn = screen.getByRole('button', { name: 'Сохранить' }) as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(false);
  });

  it('AA: >4000 Unicode characters disables Save and shows validation error without API call', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getAllByRole('button', { name: 'Редактировать' })[0]);

    const over4000 = 'Ж'.repeat(4001);
    fireEvent.change(screen.getByTestId('textarea-responsibilities'), { target: { value: over4000 } });

    expect(screen.getByTestId('responsibilities-char-counter').textContent).toBe('4001 / 4000');
    expect(screen.getByText('Превышен лимит в 4000 символов')).toBeDefined();

    const saveBtn = screen.getByRole('button', { name: 'Сохранить' }) as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(true);

    fireEvent.click(saveBtn);
    expect(adminApi.patchStaffMemberProfile).not.toHaveBeenCalled();
  });

  it('AB: Unicode counter handles multibyte/emoji correctly', () => {
    expect(countUnicodeChars('👋🌍')).toBe(2);
    expect(countUnicodeChars('Привет')).toBe(6);
    expect(countUnicodeChars('A\uD83D\uDE00B')).toBe(3); // Surrogate pair counted as 1 emoji
  });

  it('AC: blocked target remains editable with staff.update', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      status: 'blocked',
      staffStatus: 'blocked',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('blocked-banner')).toBeDefined();
    const editBtn = screen.getAllByRole('button', { name: 'Редактировать' })[0];
    expect(editBtn).toBeDefined();

    fireEvent.click(editBtn);
    expect(screen.getByTestId('profile-edit-mode')).toBeDefined();
  });

  it('AD: archived target remains editable with staff.update', async () => {
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      status: 'archived',
      staffStatus: 'archived',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('archived-banner')).toBeDefined();
    const editBtn = screen.getAllByRole('button', { name: 'Редактировать' })[0];
    expect(editBtn).toBeDefined();

    fireEvent.click(editBtn);
    expect(screen.getByTestId('profile-edit-mode')).toBeDefined();
  });

  it('AE: Access tab regression remains green', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    expect(screen.getByTestId('split-access-editor')).toBeDefined();
  });

  it('AF: Account tab regression remains green', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    expect(screen.getByTestId('tab-account-content')).toBeDefined();
    expect(screen.getByRole('button', { name: 'Сбросить пароль' })).toBeDefined();
  });

  it('AG: AdminStaffDetail calls getStaffMember with exact route userId and no legacy record ID substitution', async () => {
    const targetUserId = '0152e515-28b2-4da6-9ffe-af65e6492858';
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      userId: targetUserId,
      name: 'Тест Поддержка',
    });

    renderComponent(targetUserId);

    await waitFor(() => {
      expect(adminApi.getStaffMember).toHaveBeenCalledWith(targetUserId);
    });

    expect(adminApi.getStaffMember).toHaveBeenCalledTimes(1);
    expect(adminApi.getStaffMember).not.toHaveBeenCalledWith(expect.not.stringMatching(targetUserId));
  });

  it('AH: direct route hydration works without list navigation state', async () => {
    const targetUserId = 'cad55a1a-919e-4f33-95fc-400b549a904a';
    vi.mocked(adminApi.getStaffMember).mockResolvedValue({
      ...mockMember,
      userId: targetUserId,
      name: 'Тест Склад',
    });

    // Directly rendering without any location state (simulating direct link / browser reload)
    renderComponent(targetUserId);

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('Тест Склад');
    });

    expect(screen.getByTestId('tab-profile-content')).toBeDefined();
    expect(adminApi.getStaffMember).toHaveBeenCalledWith(targetUserId);
  });

  it('AI: read mode renders an Edit button in both Responsibilities and Work Note sections', async () => {
    renderComponent('user-123');
    await waitForHeader();

    const editButtons = screen.getAllByRole('button', { name: 'Редактировать' });
    expect(editButtons.length).toBe(2);
  });

  it('AJ: both Edit buttons are hidden when actor lacks staff.update capability', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      permissions: ['staff.read'],
      hasPermission: (perm: string) => perm === 'staff.read',
    });

    renderComponent('user-123');
    await waitForHeader();

    expect(screen.queryByRole('button', { name: 'Редактировать' })).toBeNull();
  });

  it('AK: clicking Responsibilities Edit enters shared edit mode', async () => {
    renderComponent('user-123');
    await waitForHeader();

    const editButtons = screen.getAllByRole('button', { name: 'Редактировать' });
    fireEvent.click(editButtons[0]);

    expect(screen.getByTestId('profile-edit-mode')).toBeDefined();
    expect(screen.getByTestId('textarea-responsibilities')).toBeDefined();
    expect(screen.getByTestId('textarea-work-note')).toBeDefined();
    expect(screen.queryByRole('button', { name: 'Редактировать' })).toBeNull();
  });

  it('AL & AM: clicking Work Note Edit enters the SAME shared edit mode exposing BOTH textareas', async () => {
    renderComponent('user-123');
    await waitForHeader();

    const editButtons = screen.getAllByRole('button', { name: 'Редактировать' });
    // Click the second Edit button (Work Note section)
    fireEvent.click(editButtons[1]);

    expect(screen.getByTestId('profile-edit-mode')).toBeDefined();
    expect(screen.getByTestId('textarea-responsibilities')).toBeDefined();
    expect(screen.getByTestId('textarea-work-note')).toBeDefined();
    expect(screen.queryByRole('button', { name: 'Редактировать' })).toBeNull();
  });

  it('AN: saving from Work Note initiated edit uses the same single PATCH flow without duplicate save logic', async () => {
    renderComponent('user-123');
    await waitForHeader();

    const editButtons = screen.getAllByRole('button', { name: 'Редактировать' });
    fireEvent.click(editButtons[1]);

    fireEvent.change(screen.getByTestId('textarea-work-note'), {
      target: { value: 'Обновлённая заметка' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        workNote: 'Обновлённая заметка',
      });
    });

    expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledTimes(1);
  });
});

describe('EMP.1D3A — Move Access Template Application Into Employee Detail', () => {
  const managerPresetPerms = ['orders.read', 'orders.update_status'];
  const ownerPresetPerms = ['staff.read', 'staff.permissions.manage'];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAdminAuth).mockReturnValue(defaultAuthContext);
    vi.mocked(adminApi.getStaffMember).mockResolvedValue(mockMember);
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });
    vi.mocked(adminApi.updateStaffRole).mockResolvedValue(undefined as any);
  });

  afterEach(() => {
    cleanup();
  });

  it('A: Access tab renders current template name', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    const templateHeading = screen.getByText('Назначенный шаблон доступа');
    expect(templateHeading).toBeDefined();
    expect(templateHeading.parentElement?.textContent).toContain('Менеджер');
  });

  it('B: "По шаблону" shown when direct permissions equal template preset', async () => {
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: managerPresetPerms,
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    const badges = screen.getAllByText('По шаблону');
    expect(badges.length).toBeGreaterThanOrEqual(1);
  });

  it('C: "Настроено вручную" shown when direct permissions differ from template preset', async () => {
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ['orders.read'],
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    const badges = screen.getAllByText('Настроено вручную');
    expect(badges.length).toBeGreaterThanOrEqual(1);
  });

  it('D: authorized actor sees "Применить другой шаблон"', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    const applyBtn = screen.getByTestId('apply-template-btn');
    expect(applyBtn).toBeDefined();
    expect(applyBtn.textContent).toContain('Применить другой шаблон');
  });

  it('E: unauthorized actor does not see template application action', async () => {
    vi.mocked(useAdminAuth).mockReturnValue({
      ...defaultAuthContext,
      hasPermission: (perm: string) => perm !== 'staff.update',
    });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    expect(screen.queryByTestId('apply-template-btn')).toBeNull();
  });

  it('F & G: clicking opens template selector populated with existing role templates', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('apply-template-btn'));

    expect(screen.getByTestId('template-modal')).toBeDefined();
    const select = screen.getByTestId('select-access-template');
    expect(select).toBeDefined();
    expect(select.textContent).toContain('Менеджер');
    expect(select.textContent).toContain('Владелец');
  });

  it('H, I & J: choosing template shows confirmation with replacement warning and diff counts', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('apply-template-btn'));

    const select = screen.getByTestId('select-access-template');
    fireEvent.change(select, { target: { value: 'owner' } });

    const preview = screen.getByTestId('template-confirmation-preview');
    expect(preview).toBeDefined();
    expect(preview.textContent).toContain('Применение шаблона заменит текущий набор индивидуальных прав сотрудника.');
    expect(preview.textContent).toContain('Владелец');

    const diffCounts = screen.getByTestId('template-diff-counts');
    expect(diffCounts.textContent).toContain('Добавится: 2 прав');
    expect(diffCounts.textContent).toContain('Удалится: 1 прав');
  });

  it('K, L, M, N & O: confirming calls updateStaffRole, refetches member and permissions, updates Access UI and shows success banner', async () => {
    const updatedMember = {
      ...mockMember,
      roleId: 'role-owner',
      roleCode: 'owner',
      roleName: 'Владелец',
    };

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('apply-template-btn'));

    const select = screen.getByTestId('select-access-template');
    fireEvent.change(select, { target: { value: 'owner' } });

    vi.mocked(adminApi.getStaffMember).mockResolvedValue(updatedMember);
    vi.mocked(adminApi.getStaffMemberPermissions).mockResolvedValue({
      userId: 'user-123',
      permissions: ownerPresetPerms,
    });

    fireEvent.click(screen.getByTestId('confirm-apply-template-btn'));

    await waitFor(() => {
      expect(adminApi.updateStaffRole).toHaveBeenCalledWith('user-123', { roleCode: 'owner' });
    });
    expect(adminApi.updateStaffRole).toHaveBeenCalledTimes(1);

    await waitFor(() => {
      expect(screen.queryByTestId('template-modal')).toBeNull();
    });

    expect(adminApi.getStaffMember).toHaveBeenCalledWith('user-123');
    expect(adminApi.getStaffMemberPermissions).toHaveBeenCalledWith('user-123');

    expect(screen.getByTestId('save-success-banner')).toBeDefined();
    expect(screen.getByTestId('save-success-banner').textContent).toContain('Шаблон доступа применён.');

    const templateHeading = screen.getByText('Назначенный шаблон доступа');
    expect(templateHeading.parentElement?.textContent).toContain('Владелец');
    expect(screen.getAllByText('По шаблону').length).toBeGreaterThanOrEqual(1);
  });

  it('P: 403 error is handled with humanized message', async () => {
    vi.mocked(adminApi.updateStaffRole).mockRejectedValue({ status: 403 });

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('apply-template-btn'));

    fireEvent.change(screen.getByTestId('select-access-template'), { target: { value: 'owner' } });
    fireEvent.click(screen.getByTestId('confirm-apply-template-btn'));

    await waitFor(() => {
      expect(screen.getByTestId('template-modal-error')).toBeDefined();
    });
    expect(screen.getByTestId('template-modal-error').textContent).toContain(
      'У вас нет права изменять шаблон доступа сотрудника.'
    );
  });

  it('Q: network error does not corrupt current Access state', async () => {
    vi.mocked(adminApi.updateStaffRole).mockRejectedValue(new Error('Failed to fetch'));

    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));
    fireEvent.click(screen.getByTestId('apply-template-btn'));

    fireEvent.change(screen.getByTestId('select-access-template'), { target: { value: 'owner' } });
    fireEvent.click(screen.getByTestId('confirm-apply-template-btn'));

    await waitFor(() => {
      expect(screen.getByTestId('template-modal-error')).toBeDefined();
    });
    expect(screen.getByTestId('template-modal-error').textContent).toContain(
      'Не удалось применить шаблон доступа. Попробуйте ещё раз.'
    );

    fireEvent.click(screen.getByTestId('cancel-template-btn'));
    expect(screen.queryByTestId('template-modal')).toBeNull();

    const templateHeading = screen.getByText('Назначенный шаблон доступа');
    expect(templateHeading.parentElement?.textContent).toContain('Менеджер');
    expect(screen.getAllByText('Настроено вручную').length).toBeGreaterThanOrEqual(1);
  });

  it('R: action is disabled and displays warning when Access draft is dirty', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Доступ' }));

    fireEvent.click(screen.getByRole('radio', { name: /Закрыт/i }));
    expect(screen.getByTestId('dirty-bottom-bar')).toBeDefined();

    const applyBtn = screen.getByTestId('apply-template-btn') as HTMLButtonElement;
    expect(applyBtn.disabled).toBe(true);

    const warning = screen.getByTestId('apply-template-dirty-warning');
    expect(warning).toBeDefined();
    expect(warning.textContent).toContain('Сначала сохраните или отмените изменения доступа.');
  });

  it('S: Profile tab regression remains green', async () => {
    renderComponent('user-123');
    await waitForHeader();

    expect(screen.getByTestId('tab-profile-content')).toBeDefined();

    const editButtons = screen.getAllByRole('button', { name: 'Редактировать' });
    fireEvent.click(editButtons[0]);

    fireEvent.change(screen.getByTestId('textarea-responsibilities'), {
      target: { value: 'Новые обязанности' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(adminApi.patchStaffMemberProfile).toHaveBeenCalledWith('user-123', {
        responsibilities: 'Новые обязанности',
      });
    });
  });

  it('T: Account tab regression remains green', async () => {
    renderComponent('user-123');
    await waitForHeader();

    fireEvent.click(screen.getByRole('tab', { name: 'Учётная запись' }));
    expect(screen.getByTestId('tab-account-content')).toBeDefined();

    const resetBtn = screen.getByRole('button', { name: 'Сбросить пароль' });
    expect(resetBtn).toBeDefined();
    fireEvent.click(resetBtn);

    expect(screen.getByTestId('reset-password-modal')).toBeDefined();
  });
});
