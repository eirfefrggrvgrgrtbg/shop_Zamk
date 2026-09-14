import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useParams } from 'react-router-dom';
import { AdminStaff } from './AdminStaff';
import * as adminApi from '@zamk/api-client/src/admin';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import type { StaffMemberView, StaffRoleWithPermissions } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/admin');
vi.mock('../contexts/AdminAuthContext');

const mockMembers: StaffMemberView[] = [
  {
    userId: '0152e515-28b2-4da6-9ffe-af65e6492858',
    name: 'Тест Поддержка',
    email: 'support@test.local',
    userStatus: 'active',
    mustChangePassword: false,
    staffStatus: 'active',
    roleCode: 'support',
    roleName: 'Поддержка',
    roleId: '0d509e38-a5f4-46ae-9067-908c1469a0ff',
    permissions: ['staff.read'],
    createdAt: '2026-09-05T22:11:33.442492+03:00',
    updatedAt: '2026-09-14T12:44:03.326024+03:00',
  },
  {
    userId: 'cad55a1a-919e-4f33-95fc-400b549a904a',
    name: 'Тест Склад',
    email: 'warehouse@test.local',
    userStatus: 'active',
    mustChangePassword: false,
    staffStatus: 'active',
    roleCode: 'warehouse_operator',
    roleName: 'Оператор склада',
    roleId: '9c16af58-de7f-4f17-abb0-04eb40674662',
    permissions: ['inventory.read'],
    createdAt: '2026-09-05T22:10:49.226957+03:00',
    updatedAt: '2026-09-05T22:10:49.226957+03:00',
  },
];

const mockRoles: StaffRoleWithPermissions[] = [
  {
    id: '0d509e38-a5f4-46ae-9067-908c1469a0ff',
    code: 'support',
    name: 'Поддержка',
    isSystem: true,
    permissions: ['staff.read'],
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-01T00:00:00Z',
  },
];

const defaultAuthContext: ReturnType<typeof useAdminAuth> = {
  user: {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'admin@zamk.local',
    name: 'Local Admin',
    role: 'admin',
    status: 'active',
  },
  staff: {
    roleCode: 'owner',
    roleName: 'Владелец',
    status: 'active',
    permissions: ['staff.read', 'staff.create', 'staff.update', 'staff.block'],
  },
  permissions: ['staff.read', 'staff.create', 'staff.update', 'staff.block'],
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

function DummyDetail() {
  const { userId } = useParams<{ userId: string }>();
  return <div data-testid="dummy-detail">Detail for {userId}</div>;
}

describe('AdminStaff List Link Identifier Regressions (EMP.1D2C)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAdminAuth).mockReturnValue(defaultAuthContext);
    vi.mocked(adminApi.listStaffMembers).mockResolvedValue({ items: mockMembers });
    vi.mocked(adminApi.listStaffRoles).mockResolvedValue({ items: mockRoles });
  });

  afterEach(() => {
    cleanup();
  });

  it('A: staff list employee link uses canonical userId in href', async () => {
    render(
      <MemoryRouter initialEntries={['/staff']}>
        <Routes>
          <Route path="/staff" element={<AdminStaff />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Тест Поддержка')).toBeDefined();
    });

    const supportLink = screen.getByTestId('staff-link-0152e515-28b2-4da6-9ffe-af65e6492858') as HTMLAnchorElement;
    expect(supportLink).toBeDefined();
    expect(supportLink.getAttribute('href')).toBe('/staff/0152e515-28b2-4da6-9ffe-af65e6492858');

    const warehouseLink = screen.getByTestId('staff-link-cad55a1a-919e-4f33-95fc-400b549a904a') as HTMLAnchorElement;
    expect(warehouseLink).toBeDefined();
    expect(warehouseLink.getAttribute('href')).toBe('/staff/cad55a1a-919e-4f33-95fc-400b549a904a');
  });

  it('B: clicking employee link navigates to /staff/:userId and passes that exact userId', async () => {
    render(
      <MemoryRouter initialEntries={['/staff']}>
        <Routes>
          <Route path="/staff" element={<AdminStaff />} />
          <Route path="/staff/:userId" element={<DummyDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Тест Поддержка')).toBeDefined();
    });

    const supportLink = screen.getByTestId('staff-link-0152e515-28b2-4da6-9ffe-af65e6492858');
    fireEvent.click(supportLink);

    await waitFor(() => {
      expect(screen.getByTestId('dummy-detail')).toBeDefined();
    });

    expect(screen.getByTestId('dummy-detail').textContent).toBe('Detail for 0152e515-28b2-4da6-9ffe-af65e6492858');
  });

  it('D: no legacy staff-record ID is substituted in link construction', async () => {
    render(
      <MemoryRouter initialEntries={['/staff']}>
        <Routes>
          <Route path="/staff" element={<AdminStaff />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Тест Поддержка')).toBeDefined();
    });

    const links = screen.getAllByRole('link');
    const staffMemberLinks = links.filter((l) => l.getAttribute('href')?.startsWith('/staff/'));
    expect(staffMemberLinks.length).toBe(2);

    const hrefs = staffMemberLinks.map((l) => l.getAttribute('href'));
    expect(hrefs).toContain('/staff/0152e515-28b2-4da6-9ffe-af65e6492858');
    expect(hrefs).toContain('/staff/cad55a1a-919e-4f33-95fc-400b549a904a');
  });
});
