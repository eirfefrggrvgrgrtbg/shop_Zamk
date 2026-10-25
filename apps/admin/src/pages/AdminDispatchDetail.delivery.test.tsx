// FBO.DELIVERY.HANDOFF.1 — the dispatch screen shows the order's full delivery snapshot to the warehouse operator.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminDispatchDetail } from './AdminDispatchDetail';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';

vi.mock('../api/adminPicking', () => ({
  getAdminDispatchContext: vi.fn(),
  dispatchFulfillment: vi.fn(),
  getDispatchErrorMessage: vi.fn((e: any) => e?.message || 'error'),
}));

const base: adminPickingApi.DispatchContext = {
  id: 'fulf-1', fulfillmentId: 'fulf-1', orderId: 'ord-1', orderNumber: 'ORD-100500', status: 'packed', packedAt: '2026-09-15T00:00:00Z',
  deliveryAddress: 'г. Москва, ул. Тверская, д. 1, кв. 12', recipientName: 'Анна Иванова', recipientPhone: '+7 999 000-11-22',
  customerName: 'Анна Иванова', customerPhone: '+7 999 000-11-22', deliveryMethodName: 'Курьер ZAMK',
  items: [{ orderItemId: 'i1', productTitle: 'Худи', quantity: 1, allocationMode: 'legacy' }],
};

const screenPart = () => within(screen.getByTestId('dispatch-detail-page'));
const page = () => render(
  <MemoryRouter initialEntries={['/fulfillment/dispatch/fulf-1']}>
    <Routes><Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} /></Routes>
  </MemoryRouter>,
);

describe('AdminDispatchDetail — delivery data', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (p: string) => p === 'warehouse.dispatch', user: null, isAuthenticated: true, isLoading: false,
      hasAnyPermission: () => false, staff: null, login: vi.fn(), logout: vi.fn(),
    } as never);
  });

  it('shows recipient, phone, address, postal code, courier comment, order and parcel numbers, carrier and track', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue({
      ...base, deliveryPostalCode: '101000', deliveryCourierComment: 'Домофон 12, позвонить за час',
      parcelNumber: 2, parcelsTotal: 3, carrier: 'СДЭК', trackingNumber: 'TRK-0001',
    });
    page();
    await screen.findByTestId('dispatch-detail-page');
    expect(screenPart().getByText('Анна Иванова')).toBeDefined();
    expect(screenPart().getByText('+7 999 000-11-22')).toBeDefined();
    expect(screenPart().getByText('г. Москва, ул. Тверская, д. 1, кв. 12')).toBeDefined();
    expect(screen.getByTestId('dispatch-postal-code').textContent).toContain('101000');
    expect(screen.getByTestId('dispatch-courier-comment').textContent).toContain('Домофон 12, позвонить за час');
    expect(screen.getAllByText(/ORD-100500/).length).toBeGreaterThan(0); // the order number is in the page header
    expect(screen.getByTestId('dispatch-parcel').textContent).toContain('2 из 3');
    expect(screen.getByTestId('dispatch-carrier').textContent).toContain('СДЭК');
    expect(screen.getByTestId('dispatch-tracking').textContent).toContain('TRK-0001');
  });

  it('an old order without the new fields: clear «not specified» states, no carrier or track rows, dispatch still offered', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue({ ...base, parcelNumber: 1, parcelsTotal: 1 });
    page();
    await screen.findByTestId('dispatch-detail-page');
    expect(screen.getByTestId('dispatch-postal-code').textContent).toContain('не указан');
    expect(screen.getByTestId('dispatch-courier-comment').textContent).toContain('Не указан');
    expect(screen.queryByTestId('dispatch-carrier')).toBeNull();
    expect(screen.queryByTestId('dispatch-tracking')).toBeNull();
    expect(screen.getByRole('button', { name: /Подтвердить отгрузку/i })).toBeDefined();
  });

  it('a long comment is shown as text, never as markup', async () => {
    const long = '<b>важно</b> ' + 'очень длинный комментарий '.repeat(18);
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue({ ...base, deliveryCourierComment: long, parcelNumber: 1, parcelsTotal: 1 });
    page();
    await screen.findByTestId('dispatch-detail-page');
    const box = screenPart().getByTestId('dispatch-courier-comment');
    expect(box.textContent).toContain('<b>важно</b>');
    expect(box.querySelector('b')).toBeNull();
  });
});
