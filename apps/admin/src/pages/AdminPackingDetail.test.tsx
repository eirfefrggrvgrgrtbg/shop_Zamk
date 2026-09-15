import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AdminPackingDetail } from './AdminPackingDetail';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';

vi.mock('../api/adminPicking', () => ({
  getAdminPickingOrder: vi.fn(),
  packFulfillment: vi.fn(),
  getPackingErrorMessage: vi.fn((err: any) => err.message || 'Ошибка упаковки'),
}));

const mockPickingOrder: adminPickingApi.PickingOrder = {
  orderId: 'ord-123',
  orderNumber: '1001',
  orderStatus: 'assembling',
  fulfillmentId: 'fulf-123',
  fulfillmentStatus: 'assembling',
  items: [
    {
      orderItemId: 'item-1',
      productVariantId: 'var-1',
      title: 'Sneakers',
      quantity: 1,
      pickedQuantity: 1,
      remainingQuantity: 0,
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

describe('AdminPackingDetail Capability Guard & Least Privilege', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminPickingApi.getAdminPickingOrder).mockResolvedValue(mockPickingOrder);
  });

  it('disables packing button when employee lacks warehouse.packing capability (e.g. orders.read only)', async () => {
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
      <MemoryRouter initialEntries={['/fulfillment/packing/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/packing/:id" element={<AdminPackingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const packButton = (await screen.findByRole('button', { name: /Подтвердить упаковку/i })) as HTMLButtonElement;
    expect(packButton).toBeDefined();
    expect(packButton.disabled).toBe(true);
  });

  it('enables packing button and allows pack confirmation for employee with warehouse.packing', async () => {
    vi.mocked(adminPickingApi.packFulfillment).mockResolvedValue({
      fulfillmentId: 'fulf-123',
      orderId: 'ord-123',
      fulfillmentStatus: 'packed',
      orderStatus: 'packed',
      packedAt: '2026-09-15T00:00:00Z',
    });

    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (p: string) => p === 'warehouse.packing',
      user: null,
      isAuthenticated: true,
      isLoading: false,
      hasAnyPermission: () => false,
      staff: null,
      login: vi.fn(),
      logout: vi.fn(),
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/packing/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/packing/:id" element={<AdminPackingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    const packButton = (await screen.findByRole('button', { name: /Подтвердить упаковку/i })) as HTMLButtonElement;
    expect(packButton.disabled).toBe(false);

    fireEvent.click(packButton);

    await waitFor(() => {
      expect(adminPickingApi.packFulfillment).toHaveBeenCalledWith('fulf-123');
    });

    expect(await screen.findByText('Упаковка завершена')).toBeDefined();
  });

  it('renders packed state with packedAt using operational PickingOrder without commercial fulfillment DTO', async () => {
    vi.mocked(adminPickingApi.getAdminPickingOrder).mockResolvedValue({
      ...mockPickingOrder,
      fulfillmentStatus: 'packed',
      orderStatus: 'packed',
      packedAt: '2026-09-15T12:30:00Z',
    });

    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (p: string) => p === 'warehouse.packing',
      user: null,
      isAuthenticated: true,
      isLoading: false,
      hasAnyPermission: () => false,
      staff: null,
      login: vi.fn(),
      logout: vi.fn(),
    } as any);

    render(
      <MemoryRouter initialEntries={['/fulfillment/packing/fulf-123']}>
        <Routes>
          <Route path="/fulfillment/packing/:id" element={<AdminPackingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    expect(await screen.findByText('Упаковка завершена')).toBeDefined();
    expect(screen.getByText(/Сборка переведена в статус «Упакован»/i)).toBeDefined();
    expect(screen.getByRole('link', { name: /Перейти к отгрузке/i })).toBeDefined();
    const packingQueueLinks = screen.getAllByRole('link', { name: /К очереди упаковки/i });
    expect(packingQueueLinks.length).toBeGreaterThanOrEqual(1);
    expect(packingQueueLinks[0].getAttribute('href')).toBe('/fulfillment/packing');
    expect(adminPickingApi.getAdminPickingOrder).toHaveBeenCalledWith('fulf-123');
  });
});
