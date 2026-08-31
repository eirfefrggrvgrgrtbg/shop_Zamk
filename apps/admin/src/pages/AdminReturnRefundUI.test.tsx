import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminReturns } from './AdminReturns';
import * as adminReturnsApi from '../api/adminReturns';
import type { AdminReturn, AdminReturnRefundQuote } from '../api/adminReturns';
import { ApiError } from '@zamk/api-client/src/errors';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: () => ({
    user: { id: 'admin-1', role: 'admin', email: 'admin@zamk.local' },
    staff: { id: 'staff-1', permissions: ['returns.read', 'returns.update_status', 'refunds.create'] },
    permissions: ['returns.read', 'returns.update_status', 'refunds.create'],
    isAuthenticated: true,
    isLoading: false,
    error: null,
    hasPermission: () => true,
    hasAnyPermission: () => true,
    isOwner: () => true,
    isCoOwner: () => false,
  }),
}));

vi.mock('../api/adminTimeline', () => ({
  getAdminReturnTimeline: () => Promise.resolve({ entityType: 'return', entityId: '', canonicalIdentifier: '', events: [] }),
}));

describe('Admin Returns Refund UI (M5.4B)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  const mockReturn: AdminReturn = {
    id: 'ret-100193-id',
    orderId: 'ord-100193-id',
    orderNumber: 'ORD-100193',
    status: 'item_received',
    reason: 'damaged',
    customerName: 'Nikita Osipov',
    customerEmail: 'customer@zamk.local',
    createdAt: '2026-08-30T12:00:00Z',
    items: [
      {
        id: 'ri-1',
        orderItemId: 'oi-1',
        productTitle: 'Dev Wool Coat',
        quantity: 1,
        priceCents: 1500000,
        subtotalPriceCents: 1500000,
      },
    ],
  };

  const mockEligibleQuote: AdminReturnRefundQuote = {
    returnId: 'ret-100193-id',
    orderNumber: 'ORD-100193',
    currency: 'RUB',
    items: [
      {
        orderItemId: 'oi-1',
        productTitle: 'Dev Wool Coat',
        mode: 'serialized',
        requestedQuantity: 1,
        refundableQuantity: 1,
        unitPriceCents: 1500000,
        refundCents: 1500000,
      },
    ],
    productsRefundCents: 1500000,
    deliveryRefundCents: 0,
    totalRefundCents: 1500000,
    alreadyRefundedCents: 0,
    remainingRefundableCents: 1500000,
    canRefund: true,
    blockingReason: null,
  };

  it('renders refund quote card with items breakdown and totals when eligible', async () => {
    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(mockEligibleQuote);

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-refund-card')).toBeDefined();
    });

    const refundCard = screen.getByTestId('return-refund-card');
    expect(refundCard.textContent).toContain('Возврат средств');
    expect(refundCard.textContent).toContain('Доступен');
    expect(refundCard.textContent).toContain('Dev Wool Coat');
    expect(refundCard.textContent).toContain('Поштучный учёт');
    expect(refundCard.textContent).toContain('1 шт.');
    expect(refundCard.textContent).toContain('Товары:');
    expect(refundCard.textContent).toContain('Доставка:');
    expect(refundCard.textContent).toContain('Итого к возврату:');
    expect(screen.getByRole('button', { name: /Запустить возврат средств/i })).toBeDefined();
  });

  it('opens confirmation modal and creates refund reservation on submit', async () => {
    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(mockEligibleQuote);
    const createRefundSpy = vi.spyOn(adminReturnsApi, 'createAdminRefundForReturn').mockResolvedValue({
      id: 'ref-new-1',
      orderId: 'ord-100193-id',
      status: 'pending',
      amountCents: 1500000,
      currency: 'RUB',
      reason: 'Defective lining',
      createdAt: '2026-08-31T10:00:00Z',
    });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Запустить возврат средств/i })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: /Запустить возврат средств/i }));

    expect(screen.getByText('Запустить возврат средств?')).toBeDefined();
    expect(screen.getByText('После подтверждения возврат будет поставлен в обработку.')).toBeDefined();

    const input = screen.getByPlaceholderText(/Например: брак при производстве/i);
    fireEvent.change(input, { target: { value: 'Defective lining' } });

    fireEvent.click(screen.getByRole('button', { name: /^Запустить возврат$/i }));

    await waitFor(() => {
      expect(createRefundSpy).toHaveBeenCalledWith('ret-100193-id', 'Defective lining');
    });

    await waitFor(() => {
      expect(screen.getByText('Возврат средств поставлен в обработку.')).toBeDefined();
    });
  });

  it('renders pending state purely based on latestRefundStatus with unrelated blockingReason', async () => {
    const pendingQuote: AdminReturnRefundQuote = {
      ...mockEligibleQuote,
      canRefund: false,
      latestRefundStatus: 'pending',
      blockingReason: 'Нестандартная строка ответа от бэкенда без ключевых слов',
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(pendingQuote);

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-refund-card')).toBeDefined();
    });

    const refundCard = screen.getByTestId('return-refund-card');
    expect(refundCard.textContent).toContain('Обрабатывается');
    expect(refundCard.textContent).toContain('Возврат средств обрабатывается');
    expect(refundCard.textContent).toContain('Возврат зарегистрирован и ожидает обработки платежной системой.');
    expect(screen.queryByRole('button', { name: /Запустить возврат средств/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Повторить возврат средств/i })).toBeNull();
  });

  it('renders retry button when latestRefundStatus is failed and canRefund is true', async () => {
    const failedRetryQuote: AdminReturnRefundQuote = {
      ...mockEligibleQuote,
      canRefund: true,
      latestRefundStatus: 'failed',
      blockingReason: null,
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(failedRetryQuote);

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-refund-card')).toBeDefined();
    });

    const refundCard = screen.getByTestId('return-refund-card');
    expect(refundCard.textContent).toContain('Доступен');
    const retryBtn = screen.getByRole('button', { name: /Повторить возврат средств/i });
    expect(retryBtn).toBeDefined();

    // Open modal and check retry modal wording
    fireEvent.click(retryBtn);
    expect(screen.getByText('Повторить возврат средств?')).toBeDefined();
    expect(screen.getByRole('button', { name: /^Повторить возврат$/i })).toBeDefined();
  });

  it('POST success refetches quote to pending and renders Обрабатывается (never Выполнен)', async () => {
    let quoteFetchCount = 0;
    const pendingQuote: AdminReturnRefundQuote = {
      ...mockEligibleQuote,
      canRefund: false,
      latestRefundStatus: 'pending',
      blockingReason: 'Возврат средств уже зарезервирован и ожидает обработки',
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockImplementation(() => {
      quoteFetchCount++;
      if (quoteFetchCount > 1) {
        return Promise.resolve(pendingQuote);
      }
      return Promise.resolve(mockEligibleQuote);
    });

    const createRefundSpy = vi.spyOn(adminReturnsApi, 'createAdminRefundForReturn').mockResolvedValue({
      id: 'ref-new-1',
      orderId: 'ord-100193-id',
      status: 'pending',
      amountCents: 1500000,
      currency: 'RUB',
      reason: 'Defective lining',
      createdAt: '2026-08-31T10:00:00Z',
    });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Запустить возврат средств/i })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: /Запустить возврат средств/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Запустить возврат$/i }));

    await waitFor(() => {
      expect(createRefundSpy).toHaveBeenCalled();
    });

    await waitFor(() => {
      const card = screen.getByTestId('return-refund-card');
      expect(card.textContent).toContain('Обрабатывается');
      expect(card.textContent).toContain('Возврат средств обрабатывается');
      expect(card.textContent).toContain('Возврат зарегистрирован и ожидает обработки платежной системой.');
      // Must NOT render Выполнен!
      expect(card.textContent).not.toContain('Выполнен');
      expect(screen.queryByRole('button', { name: /Запустить возврат средств/i })).toBeNull();
    });
  });

  it('renders succeeded state when latestRefundStatus is succeeded', async () => {
    const succeededQuote: AdminReturnRefundQuote = {
      ...mockEligibleQuote,
      canRefund: false,
      latestRefundStatus: 'succeeded',
      latestRefundProcessedAt: '2026-08-31T10:05:00Z',
      blockingReason: 'Возврат средств уже выполнен',
      alreadyRefundedCents: 1500000,
      remainingRefundableCents: 0,
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(succeededQuote);

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-refund-card')).toBeDefined();
    });

    const refundCard = screen.getByTestId('return-refund-card');
    expect(refundCard.textContent).toContain('Выполнен');
    expect(refundCard.textContent).toContain('Возврат средств выполнен');
    expect(screen.queryByRole('button', { name: /Запустить возврат средств/i })).toBeNull();
  });

  it('renders blocked state when return is approved but not received at warehouse', async () => {
    const approvedReturn: AdminReturn = {
      ...mockReturn,
      status: 'approved',
    };
    const blockedQuote: AdminReturnRefundQuote = {
      ...mockEligibleQuote,
      canRefund: false,
      latestRefundStatus: null,
      blockingReason: 'Возврат средств доступен только после приёмки товара на складе.',
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([approvedReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(approvedReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(blockedQuote);

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-refund-card')).toBeDefined();
    });

    const refundCard = screen.getByTestId('return-refund-card');
    expect(refundCard.textContent).toContain('Недоступен');
    expect(refundCard.textContent).toContain('Возврат средств недоступен');
    expect(refundCard.textContent).toContain('Возврат средств доступен только после приёмки товара на складе.');
    expect(screen.queryByRole('button', { name: /Запустить возврат средств/i })).toBeNull();
  });

  it('handles refund creation error gracefully without crashing dossier', async () => {
    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockReturn]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(mockEligibleQuote);
    vi.spyOn(adminReturnsApi, 'createAdminRefundForReturn').mockRejectedValue(
      new ApiError('Allocation invariant broken', 'refund_allocation_invariant', 400),
    );

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100193-id']}>
        <AdminReturns />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Запустить возврат средств/i })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: /Запустить возврат средств/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Запустить возврат$/i }));

    await waitFor(() => {
      expect(screen.getByText(/Несогласованное состояние резервирования: количество единиц не соответствует заказу./i)).toBeDefined();
    });

    expect(screen.getByTestId('return-refund-card')).toBeDefined();
  });
});
