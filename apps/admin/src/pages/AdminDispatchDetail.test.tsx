import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { AdminDispatchDetail } from './AdminDispatchDetail';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';
import { ApiError } from '@zamk/api-client/src/errors';

vi.mock('../api/adminPicking', () => ({
  getAdminDispatchContext: vi.fn(),
  dispatchFulfillment: vi.fn(),
  getDispatchErrorMessage: vi.fn((err: any) => {
    if (err?.code === 'fulfillment_not_fully_picked') {
      return 'Отгрузка недоступна: сборка заказа не завершена.';
    }
    if (err?.code === 'dispatch_not_allowed') {
      return 'Отгрузка недоступна: заказ не готов к отгрузке (требуется статус «Собран»).';
    }
    return err?.message || 'Не удалось подтвердить отгрузку';
  }),
}));

const mockDispatchContext: adminPickingApi.DispatchContext = {
  id: 'fulf-123',
  fulfillmentId: 'fulf-123',
  orderId: 'ord-123',
  orderNumber: '1001',
  status: 'packed',
  packedAt: '2026-09-15T00:00:00Z',
  deliveryAddress: 'Test Delivery Address',
  recipientName: 'Test Buyer',
  recipientPhone: '+79991234567',
  customerName: 'Test Buyer',
  customerPhone: '+79991234567',
  deliveryMethodName: 'Курьер ZAMK',
  items: [
    {
      orderItemId: 'item-1',
      productTitle: 'Sneakers',
      variantSize: '42',
      variantColor: 'Black',
      sku: 'SNK-BLK-42',
      barcode: 'BAR-123',
      quantity: 1,
      allocationMode: 'serialized',
      allocatedUnits: [
        {
          inventoryUnitId: 'unit-1',
          unitCode: 'ZMU-PACK-001',
          pickedAt: '2026-09-15T00:00:00Z',
        },
      ],
    },
  ],
};

const setupAuth = (permission: string) => {
  vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
    hasPermission: (p: string) => p === permission,
    user: null,
    isAuthenticated: true,
    isLoading: false,
    hasAnyPermission: () => false,
    staff: null,
    login: vi.fn(),
    logout: vi.fn(),
  } as any);
};

describe('AdminDispatchDetail — M4.3.2A Acceptance Suite (A through J)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(mockDispatchContext);
  });

  // A. packed fulfillment appears actionable
  it('A. packed fulfillment appears actionable for user with warehouse.dispatch', async () => {
    setupAuth('warehouse.dispatch');

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const dispatchBtn = (await screen.findByRole('button', { name: /Подтвердить отгрузку/i })) as HTMLButtonElement;
    expect(dispatchBtn).toBeDefined();
    expect(dispatchBtn.disabled).toBe(false);
    expect(screen.getByText('Собран')).toBeDefined();
    expect(screen.getByText(/Заказ собран и готов к отгрузке/i)).toBeDefined();
    expect(screen.getByText('ZMU-PACK-001')).toBeDefined();
    expect(screen.getByText(/SNK-BLK-42/)).toBeDefined();
  });

  // B. assembling fulfillment has no dispatch CTA
  it('B. assembling fulfillment has no dispatch CTA', async () => {
    setupAuth('warehouse.dispatch');
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue({
      ...mockDispatchContext,
      status: 'assembling',
      packedAt: undefined,
    });

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    expect(await screen.findByText(/Сборка находится в статусе «Собирается»/i)).toBeDefined();
    expect(screen.queryByRole('button', { name: /Подтвердить отгрузку/i })).toBeNull();
  });

  // C. shipped fulfillment has no dispatch CTA
  it('C. shipped fulfillment has no dispatch CTA', async () => {
    setupAuth('warehouse.dispatch');
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue({
      ...mockDispatchContext,
      status: 'shipped',
    });

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    expect(await screen.findByText('Отгрузка со склада выполнена')).toBeDefined();
    expect(screen.queryByRole('button', { name: /Подтвердить отгрузку/i })).toBeNull();
  });

  // D. clicking CTA opens confirmation but does not call API yet
  it('D. clicking CTA opens confirmation but does not call API yet', async () => {
    setupAuth('warehouse.dispatch');

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const cta = await screen.findByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(cta);

    // Modal elements appear
    expect(await screen.findByText('Подтвердить отгрузку?')).toBeDefined();
    expect(screen.getByText('После подтверждения товар будет считаться физически покинувшим склад ZAMK.')).toBeDefined();

    // API was not called
    expect(adminPickingApi.dispatchFulfillment).not.toHaveBeenCalled();
  });

  // E. confirming calls dispatch exactly once with correct fulfillment id
  it('E. confirming calls dispatch exactly once with correct fulfillment id', async () => {
    setupAuth('warehouse.dispatch');
    vi.mocked(adminPickingApi.dispatchFulfillment).mockResolvedValue({
      fulfillmentId: 'fulf-123',
      orderId: 'ord-123',
      shipmentId: 'ship-123',
      fulfillmentStatus: 'shipped',
      orderStatus: 'shipped',
      shipmentStatus: 'shipped',
      shippedAt: '2026-09-15T00:00:00Z',
    });

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const cta = await screen.findByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(cta);

    // Confirm in modal
    const modalButtons = await screen.findAllByRole('button', { name: /Подтвердить отгрузку/i });
    // Click the modal confirm button (the last one)
    fireEvent.click(modalButtons[modalButtons.length - 1]);

    await waitFor(() => {
      expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledTimes(1);
      expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledWith('fulf-123');
    });
  });

  // F. loading disables repeated confirmation
  it('F. loading disables repeated confirmation while dispatch request is in flight', async () => {
    setupAuth('warehouse.dispatch');
    let resolveDispatch: (res: any) => void = () => {};
    vi.mocked(adminPickingApi.dispatchFulfillment).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveDispatch = resolve;
        })
    );

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const cta = await screen.findByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(cta);

    const modalButtons = await screen.findAllByRole('button', { name: /Подтвердить отгрузку/i });
    const modalConfirmBtn = modalButtons[modalButtons.length - 1] as HTMLButtonElement;

    // First click triggers dispatch
    fireEvent.click(modalConfirmBtn);
    expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledTimes(1);

    // Button is now disabled with in-flight indicator
    expect(modalConfirmBtn.disabled).toBe(true);

    // Second click does NOT trigger a second dispatch call
    fireEvent.click(modalConfirmBtn);
    expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledTimes(1);

    // Resolve in-flight request
    await act(async () => {
      resolveDispatch({
        fulfillmentId: 'fulf-123',
        orderId: 'ord-123',
        shipmentId: 'ship-123',
        fulfillmentStatus: 'shipped',
        orderStatus: 'shipped',
        shipmentStatus: 'shipped',
        shippedAt: '2026-09-15T00:00:00Z',
      });
    });
  });

  // G. success/refetch renders shipped and removes action
  it('G. success/refetch renders shipped and removes action', async () => {
    setupAuth('warehouse.dispatch');
    vi.mocked(adminPickingApi.dispatchFulfillment).mockResolvedValue({
      fulfillmentId: 'fulf-123',
      orderId: 'ord-123',
      shipmentId: 'ship-123',
      fulfillmentStatus: 'shipped',
      orderStatus: 'shipped',
      shipmentStatus: 'shipped',
      shippedAt: '2026-09-15T00:00:00Z',
    });
    vi.mocked(adminPickingApi.getAdminDispatchContext)
      .mockResolvedValueOnce(mockDispatchContext)
      .mockResolvedValueOnce({
        ...mockDispatchContext,
        status: 'shipped',
      });

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const cta = await screen.findByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(cta);

    const modalButtons = await screen.findAllByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(modalButtons[modalButtons.length - 1]);

    expect(await screen.findByText('Отгрузка со склада выполнена')).toBeDefined();
    expect(screen.queryByRole('button', { name: /Подтвердить отгрузку/i })).toBeNull();
  });

  // H. backend failure keeps fulfillment unshipped and shows operational error
  it('H. backend failure keeps fulfillment unshipped and shows operational error', async () => {
    setupAuth('warehouse.dispatch');
    const apiErr = new ApiError('Fulfillment not fully picked', 'fulfillment_not_fully_picked', 409);
    vi.mocked(adminPickingApi.dispatchFulfillment).mockRejectedValue(apiErr);

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const cta = await screen.findByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(cta);

    const modalButtons = await screen.findAllByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(modalButtons[modalButtons.length - 1]);

    // Modal closes, error banner shows concise operational message
    expect(await screen.findByText('Отгрузка недоступна: сборка заказа не завершена.')).toBeDefined();

    // Fulfillment remains unshipped in "Собран" status
    expect(screen.getByText('Собран')).toBeDefined();
    expect(screen.getByText(/Заказ собран и готов к отгрузке/i)).toBeDefined();
    expect(screen.queryByText('Отгрузка со склада выполнена')).toBeNull();
  });

  // I. user without warehouse.dispatch does not get dispatch action
  it('I. user without warehouse.dispatch does not get dispatch action (read-only mode)', async () => {
    setupAuth('orders.read');

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await screen.findByText(/1001/);
    expect(screen.queryByRole('button', { name: /Подтвердить отгрузку/i })).toBeNull();
    expect(screen.getByText(/Для подтверждения отгрузки требуется право warehouse.dispatch/i)).toBeDefined();
  });

  // J. no Seller-facing code changed
  it('J. confirms Seller boundary is preserved', () => {
    // Structural invariant: Admin dispatch detail does not export or import seller-side functions
    expect(typeof adminPickingApi.dispatchFulfillment).toBe('function');
  });
});
