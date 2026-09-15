import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AdminDispatchDetail } from './AdminDispatchDetail';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';

vi.mock('../api/adminPicking', () => ({
  getAdminDispatchContext: vi.fn(),
  dispatchFulfillment: vi.fn(),
  getDispatchErrorMessage: vi.fn((err: any) => err.message || 'Ошибка отгрузки'),
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

describe('AdminDispatchDetail Capability Guard & Least Privilege', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(mockDispatchContext);
  });

  it('disables dispatch button when employee lacks warehouse.dispatch capability (e.g. orders.read only)', async () => {
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (p: string) => p === 'orders.read',
      user: null,
      isAuthenticated: true,
      isLoading: false,
      hasAnyPermission: () => false,
      staff: null,
      login: vi.fn(),
      logout: vi.fn(),
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const dispatchButton = (await screen.findByRole('button', { name: /Подтвердить отгрузку/i })) as HTMLButtonElement;
    expect(dispatchButton).toBeDefined();
    expect(dispatchButton.disabled).toBe(true);
    expect(adminPickingApi.getAdminDispatchContext).toHaveBeenCalledWith('fulf-123');
  });

  it('enables dispatch button and allows dispatch confirmation for employee with only warehouse.dispatch', async () => {
    vi.mocked(adminPickingApi.dispatchFulfillment).mockResolvedValue({
      fulfillmentId: 'fulf-123',
      orderId: 'ord-123',
      shipmentId: 'ship-123',
      fulfillmentStatus: 'shipped',
      orderStatus: 'shipped',
      shipmentStatus: 'shipped',
      shippedAt: '2026-09-15T00:00:00Z',
    });

    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (p: string) => p === 'warehouse.dispatch',
      user: null,
      isAuthenticated: true,
      isLoading: false,
      hasAnyPermission: () => false,
      staff: null,
      login: vi.fn(),
      logout: vi.fn(),
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const dispatchButton = (await screen.findByRole('button', { name: /Подтвердить отгрузку/i })) as HTMLButtonElement;
    expect(dispatchButton.disabled).toBe(false);

    fireEvent.click(dispatchButton);

    // Confirmation modal button: "Да, подтвердить отгрузку"
    const modalConfirmBtn = await screen.findByRole('button', { name: /Да, подтвердить отгрузку/i });
    fireEvent.click(modalConfirmBtn);

    await waitFor(() => {
      expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledWith('fulf-123');
    });

    expect(await screen.findByText('Отгрузка со склада выполнена')).toBeDefined();
    expect(screen.getByText('ZMU-PACK-001')).toBeDefined();
  });
});
