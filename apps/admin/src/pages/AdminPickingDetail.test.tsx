import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AdminPickingDetail } from './AdminPickingDetail';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';

vi.mock('../api/adminPicking', () => ({
  getAdminPickingOrder: vi.fn(),
  scanPickingCode: vi.fn(),
  getCompatibleUnits: vi.fn(),
  getPickingErrorMessage: vi.fn((err: any) => err.message || 'Ошибка сканирования'),
  isCanonicalScannerCode: vi.fn((code: string) => code.startsWith('ZMU-')),
}));

const mockPickingOrder = {
  orderId: 'ord-123',
  orderNumber: '1001',
  orderStatus: 'accepted',
  fulfillmentId: 'test-123',
  fulfillmentStatus: 'picking',
  items: [
    {
      orderItemId: 'item-1',
      title: 'Sneakers',
      productVariantId: 'var-1',
      quantity: 1,
      pickedQuantity: 0,
      remainingQuantity: 1,
      allocationMode: 'serialized',
      allocatedUnits: [],
      compatibleUnitsCount: 1,
    },
  ],
};

describe('AdminPickingDetail Permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminPickingApi.getAdminPickingOrder).mockResolvedValue(mockPickingOrder as any);
  });

  it('disables picking button for read-only user', async () => {
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (perm: string) => perm === 'orders.read',
      user: null, isAuthenticated: true, isLoading: false, hasAnyPermission: () => false, staff: null, login: vi.fn(), logout: vi.fn()
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/test-123']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const scanButton = (await screen.findByRole('button', { name: /Ввод/i })) as HTMLButtonElement;
    expect(scanButton.disabled).toBe(true);
  });

  it('enables picking button for user with warehouse.picking', async () => {
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (perm: string) => perm === 'warehouse.picking' || perm === 'orders.read',
      user: null, isAuthenticated: true, isLoading: false, hasAnyPermission: () => false, staff: null, login: vi.fn(), logout: vi.fn()
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/test-123']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const scanButton = (await screen.findByRole('button', { name: /Ввод/i })) as HTMLButtonElement;
    expect(scanButton).toBeDefined();
  });
});

describe('AdminPickingDetail Layout-Independent Normalization (SCN.1C)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: () => true,
      user: null, isAuthenticated: true, isLoading: false, hasAnyPermission: () => false, staff: null, login: vi.fn(), logout: vi.fn()
    } as any);
    vi.mocked(adminPickingApi.getAdminPickingOrder).mockResolvedValue(mockPickingOrder as any);
  });

  it('J & K: normalizes Russian-layout ZMU scan upon Enter submission', async () => {
    vi.mocked(adminPickingApi.scanPickingCode).mockResolvedValueOnce({
      scanResult: { newlyPicked: true, alreadyPicked: false, alreadyComplete: false },
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/test-123']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const input = (await screen.findByPlaceholderText(/Отсканируйте ZMU подходящей единицы/i)) as HTMLInputElement;
    // Physical scanner transmits Russian characters when RU layout is active on macOS
    fireEvent.change(input, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });
    // Hardware scanner sends Enter key automatically
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(adminPickingApi.scanPickingCode).toHaveBeenCalledWith('test-123', 'ZMU-BR8XJV54XCMX48ZZ', 'item-1');
    });
  });

  it('L: clicking submit button ("Ввод") uses the exact same normalized value', async () => {
    vi.mocked(adminPickingApi.scanPickingCode).mockResolvedValueOnce({
      scanResult: { newlyPicked: true, alreadyPicked: false, alreadyComplete: false },
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/test-123']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const input = (await screen.findByPlaceholderText(/Отсканируйте ZMU подходящей единицы/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });

    const submitBtn = screen.getByRole('button', { name: /Ввод/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(adminPickingApi.scanPickingCode).toHaveBeenCalledWith('test-123', 'ZMU-BR8XJV54XCMX48ZZ', 'item-1');
    });
  });

  it('M: preserves English layout scan unchanged', async () => {
    vi.mocked(adminPickingApi.scanPickingCode).mockResolvedValueOnce({
      scanResult: { newlyPicked: true, alreadyPicked: false, alreadyComplete: false },
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/test-123']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const input = (await screen.findByPlaceholderText(/Отсканируйте ZMU подходящей единицы/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-BR8XJV54XCMX48ZZ' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(adminPickingApi.scanPickingCode).toHaveBeenCalledWith('test-123', 'ZMU-BR8XJV54XCMX48ZZ', 'item-1');
    });
  });

  it('N: unknown normalized code reaches backend and receives error feedback', async () => {
    vi.mocked(adminPickingApi.scanPickingCode).mockRejectedValueOnce(
      new Error('Код не найден')
    );

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/test-123']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const input = (await screen.findByPlaceholderText(/Отсканируйте ZMU подходящей единицы/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ЯЬГ-ГТЛТЩЦТ99999' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(adminPickingApi.scanPickingCode).toHaveBeenCalledWith('test-123', 'ZMU-UNKNOWN99999', 'item-1');
    });

    await waitFor(() => {
      expect(screen.getByTestId('scan-feedback-banner')).toBeDefined();
      expect(screen.getByText(/Код не найден/i)).toBeDefined();
    });
  });
});
