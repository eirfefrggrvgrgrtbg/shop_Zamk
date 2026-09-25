/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerOrders } from './SellerOrders';
import { getSellerOrders, getSellerOrderSummary } from '@zamk/api-client/src/seller';
import type { SellerOrder } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerOrders: vi.fn(),
  getSellerOrderSummary: vi.fn(),
}));

describe('SellerOrders — Canonical Status Presentation and Shipped Observer State', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerOrderSummary).mockResolvedValue({
      todayUnits: 2,
      todayOrders: 2,
      last7dGross: 120000,
      last30dGross: 500000,
      returnsCount: 0,
      returnsAmount: 0,
    });
  });

  const createOrder = (overrides: Partial<SellerOrder> = {}): SellerOrder => ({
    id: 'ord-uuid-12345',
    orderNumber: 'ORD-100207',
    createdAt: '2026-09-25T11:00:00Z',
    commercialStatus: 'shipped',
    deliveryStatus: 'shipped',
    sellerItemCount: 1,
    sellerUnits: 1,
    sellerGrossAmount: 122200,
    sellerRefundAmount: 0,
    sellerNetAmount: 122200,
    items: [
      {
        id: 'item-1',
        orderId: 'ord-uuid-12345',
        productId: 'prod-1',
        productVariantId: 'var-1',
        sellerId: 'seller-1',
        title: 'Худи оверсайз',
        productSlug: 'hoodie',
        variantSize: 'L',
        variantColor: 'Черный',
        sku: 'HD-BLK-L',
        imageUrl: undefined,
        priceCents: 122200,
        quantity: 1,
        subtotalPriceCents: 122200,
        createdAt: '2026-09-25T11:00:00Z',
      },
    ],
    ...overrides,
  });

  it('A. assembling renders "Собирается" and does NOT render raw "assembling"', async () => {
    const order = createOrder({
      id: 'ord-assembling',
      orderNumber: 'ORD-100208',
      commercialStatus: 'assembling',
      deliveryStatus: 'assembling',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Собирается')).toBeTruthy();
    });

    expect(screen.queryByText('assembling')).toBeNull();
  });

  it('B. packed renders "Собран" and does NOT render raw "packed"', async () => {
    const order = createOrder({
      id: 'ord-packed',
      orderNumber: 'ORD-100209',
      commercialStatus: 'packed',
      deliveryStatus: 'packed',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Собран')).toBeTruthy();
    });

    expect(screen.queryByText('packed')).toBeNull();
  });

  it('C. shipped renders "Отгружен" and does NOT render raw "shipped"', async () => {
    const order = createOrder({
      id: 'ord-shipped',
      orderNumber: 'ORD-100207',
      commercialStatus: 'shipped',
      deliveryStatus: 'shipped',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Отгружен')).toBeTruthy();
    });

    expect(screen.queryByText('shipped')).toBeNull();
  });

  it('D. delivered renders "Доставлен" and does NOT render raw "delivered"', async () => {
    const order = createOrder({
      id: 'ord-delivered',
      orderNumber: 'ORD-100204',
      commercialStatus: 'delivered',
      deliveryStatus: 'delivered',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      // Primary status badge has "Доставлен"
      expect(screen.getAllByText('Доставлен').length).toBeGreaterThanOrEqual(1);
    });

    expect(screen.queryByText('delivered')).toBeNull();
  });

  it('E. shipped expanded/detail state shows observer guidance (dispatch happened, no seller action required)', async () => {
    const order = createOrder({
      id: 'ord-shipped-expanded',
      orderNumber: 'ORD-100207',
      commercialStatus: 'shipped',
      deliveryStatus: 'shipped',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('#ORD-100207')).toBeTruthy();
    });

    // Expand order row
    const row = screen.getByText('#ORD-100207').closest('tr')!;
    fireEvent.click(row);

    // Observer guidance banner must be present
    const notice = await screen.findByTestId('shipped-observer-notice');
    expect(notice.textContent).toContain('Заказ отгружен со склада ZAMK и передан в доставку');
    expect(notice.textContent).toContain('От продавца действий не требуется');
  });

  it('F. shipped state renders NO warehouse action button in Seller UI', async () => {
    const order = createOrder({
      id: 'ord-shipped-actions',
      orderNumber: 'ORD-100207',
      commercialStatus: 'shipped',
      deliveryStatus: 'shipped',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('#ORD-100207')).toBeTruthy();
    });

    // Expand
    const row = screen.getByText('#ORD-100207').closest('tr')!;
    fireEvent.click(row);

    // Assert absence of warehouse action buttons
    expect(screen.queryByText(/отгрузить/i)).toBeNull();
    expect(screen.queryByText(/подтвердить отгрузку/i)).toBeNull();
    expect(screen.queryByText(/передать курьеру/i)).toBeNull();
    expect(screen.queryByText(/mark shipped/i)).toBeNull();
    expect(screen.queryByText(/dispatch/i)).toBeNull();
  });

  it('G. existing shipment sub-status "Передан в доставку" remains correct', async () => {
    const order = createOrder({
      id: 'ord-delivery-subbadge',
      orderNumber: 'ORD-100207',
      commercialStatus: 'shipped',
      deliveryStatus: 'shipped',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [order],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('🚚 Передан в доставку')).toBeTruthy();
    });
  });

  it('H. return/refund statuses remain unchanged', async () => {
    const returnOrder = createOrder({
      id: 'ord-has-return',
      orderNumber: 'ORD-100201',
      commercialStatus: 'has_return',
      deliveryStatus: 'delivered',
    });
    const fullyReturnedOrder = createOrder({
      id: 'ord-fully-returned',
      orderNumber: 'ORD-100202',
      commercialStatus: 'fully_returned',
      deliveryStatus: 'delivered',
    });

    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [returnOrder, fullyReturnedOrder],
      totalCount: 2,
    });

    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Есть возврат')).toBeTruthy();
      expect(screen.getByText('Возвращён')).toBeTruthy();
    });
  });
});
