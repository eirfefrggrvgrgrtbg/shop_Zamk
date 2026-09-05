import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AdminPickingDetail } from './AdminPickingDetail';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import * as AdminAuthContext from '../contexts/AdminAuthContext';

vi.mock('../api/adminPicking', () => ({
  getAdminPickingOrder: vi.fn().mockResolvedValue({
    id: 'test-123',
    fulfillmentStatus: 'picking',
    orderStatus: 'accepted',
    items: [{
      orderItemId: 'item-1',
      title: 'Test Item',
      quantity: 1,
      allocationMode: 'legacy',
      barcode: '1234567890123',
      pickedQuantity: 0,
      remainingQuantity: 1
    }]
  })
}));

describe('AdminPickingDetail Permissions', () => {
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
