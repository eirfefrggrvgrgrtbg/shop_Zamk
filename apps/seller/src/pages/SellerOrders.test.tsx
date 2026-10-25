/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
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
    fulfillmentStage: 'shipped',
    nextStepOwner: 'zamk',
    hasReturns: false,
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

  const renderPage = () =>
    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

  it('H. a partial return is shown per item and does not turn the delivered order into "Возвращён"', async () => {
    const base = createOrder({}).items![0];
    const order = createOrder({
      id: 'ord-partial',
      orderNumber: 'ORD-100201',
      commercialStatus: 'delivered',
      deliveryStatus: 'delivered',
      fulfillmentStage: 'delivered',
      nextStepOwner: 'none',
      hasReturns: true,
      sellerItemCount: 2,
      sellerUnits: 4,
      items: [
        { ...base, id: 'i-ret', title: 'Футболка', quantity: 3, returnRefundedUnits: 1, returnRequestedUnits: 0, returnInProgressUnits: 0, fulfillmentStage: 'delivered' },
        { ...base, id: 'i-ok', title: 'Кепка', quantity: 1, returnRefundedUnits: 0, returnRequestedUnits: 0, returnInProgressUnits: 0, fulfillmentStage: 'delivered' },
      ],
    });
    vi.mocked(getSellerOrders).mockResolvedValueOnce({ items: [order], totalCount: 1 });
    renderPage();

    await waitFor(() => expect(screen.getByText('#ORD-100201')).toBeTruthy());
    expect(screen.queryByText('Возвращён')).toBeNull();
    expect(screen.getAllByText('Доставлен').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByTestId('order-has-returns')).toBeTruthy();

    fireEvent.click(screen.getByText('#ORD-100201').closest('tr')!);
    const items = await screen.findAllByTestId('order-item');
    expect(items).toHaveLength(2);
    const [returned, clean] = items;
    expect(within(returned).getByTestId('item-return-ref').textContent).toBe('Возмещено: 1 шт.');
    expect(within(clean).queryByTestId('item-return-ref')).toBeNull();
    expect(within(clean).queryByTestId('item-return-req')).toBeNull();
  });

  it('I. a return request, a return in progress and a refund are three different labels', async () => {
    const base = createOrder({}).items![0];
    const order = createOrder({
      id: 'ord-ret-kinds',
      orderNumber: 'ORD-100301',
      deliveryStatus: 'delivered',
      fulfillmentStage: 'delivered',
      hasReturns: true,
      items: [{ ...base, returnRequestedUnits: 1, returnInProgressUnits: 2, returnRefundedUnits: 3 }],
    });
    vi.mocked(getSellerOrders).mockResolvedValueOnce({ items: [order], totalCount: 1 });
    renderPage();
    await waitFor(() => expect(screen.getByText('#ORD-100301')).toBeTruthy());
    fireEvent.click(screen.getByText('#ORD-100301').closest('tr')!);
    expect((await screen.findByTestId('item-return-req')).textContent).toBe('Запрошен возврат: 1 шт.');
    expect(screen.getByTestId('item-return-prog').textContent).toBe('Возврат в обработке: 2 шт.');
    expect(screen.getByTestId('item-return-ref').textContent).toBe('Возмещено: 3 шт.');
  });

  it('J. a delivered order without returns shows no return marker', async () => {
    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [createOrder({ orderNumber: 'ORD-100302', deliveryStatus: 'delivered', fulfillmentStage: 'delivered', nextStepOwner: 'none' })],
      totalCount: 1,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('#ORD-100302')).toBeTruthy());
    expect(screen.queryByTestId('order-has-returns')).toBeNull();
    expect(screen.getByText('Действий не требуется')).toBeTruthy();
  });

  it('K. FBO: the detail says ZAMK warehouse does the work and offers no warehouse action', async () => {
    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [createOrder({ orderNumber: 'ORD-100303', deliveryStatus: 'packed', fulfillmentStage: 'processing' })],
      totalCount: 1,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('#ORD-100303')).toBeTruthy());
    fireEvent.click(screen.getByText('#ORD-100303').closest('tr')!);
    const notice = await screen.findByTestId('fbo-notice');
    expect(notice.textContent).toContain('Сборку, упаковку и отправку выполняет склад ZAMK');
    expect(screen.queryByRole('button', { name: /собрать|упаковать|отгрузить|подтвердить/i })).toBeNull();
  });

  it('L. filters ask the server, reset the page and keep the real status names', async () => {
    vi.mocked(getSellerOrders).mockResolvedValue({ items: [], totalCount: 0 });
    renderPage();
    await waitFor(() => expect(getSellerOrders).toHaveBeenCalledWith({ filter: 'all', limit: 20, offset: 0 }));

    for (const [id, label] of [['new', 'Новые'], ['processing', 'В обработке ZAMK'], ['shipped', 'Отгружены'], ['delivered', 'Доставлены'], ['returns', 'С возвратами']] as const) {
      fireEvent.click(screen.getByRole('button', { name: label }));
      await waitFor(() => expect(getSellerOrders).toHaveBeenLastCalledWith({ filter: id, limit: 20, offset: 0 }));
      expect(screen.getByRole('button', { name: label }).getAttribute('aria-pressed')).toBe('true');
    }
    await waitFor(() => expect(screen.getByText('В этом разделе заказов нет.')).toBeTruthy());
  });

  it('M. pagination uses the server total and requests the next page by offset', async () => {
    const many = Array.from({ length: 20 }, (_, i) => createOrder({ id: `o-${i}`, orderNumber: `ORD-P${i}` }));
    vi.mocked(getSellerOrders).mockResolvedValue({ items: many, totalCount: 45 });
    renderPage();
    await waitFor(() => expect(screen.getByText('#ORD-P0')).toBeTruthy());
    expect(screen.getByTestId('orders-pagination').textContent).toContain('1–20 из 45');
    fireEvent.click(screen.getByRole('button', { name: 'Вперёд' }));
    await waitFor(() => expect(getSellerOrders).toHaveBeenLastCalledWith({ filter: 'all', limit: 20, offset: 20 }));
    await waitFor(() => expect(screen.getByTestId('orders-pagination').textContent).toContain('21–40 из 45'));
  });

  it('N. a load error shows a message and a retry that recovers', async () => {
    vi.mocked(getSellerOrders)
      .mockRejectedValueOnce(Object.assign(new Error('boom'), { status: 500 }))
      .mockResolvedValueOnce({ items: [createOrder({ orderNumber: 'ORD-100304' })], totalCount: 1 });
    renderPage();
    expect((await screen.findByRole('alert')).textContent).toContain('Не удалось загрузить данные продаж.');
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(screen.getByText('#ORD-100304')).toBeTruthy());
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('O. a seller with no orders sees the empty state', async () => {
    vi.mocked(getSellerOrders).mockResolvedValueOnce({ items: [], totalCount: 0 });
    renderPage();
    await waitFor(() => expect(screen.getByText('У вас пока нет продаж.')).toBeTruthy());
  });

  it('P. a paid order shows a Russian sub-status, never the raw "paid", and no duplicate label', async () => {
    vi.mocked(getSellerOrders).mockResolvedValueOnce({
      items: [
        createOrder({ id: 'o-paid', orderNumber: 'ORD-100305', deliveryStatus: 'paid', fulfillmentStage: 'paid' }),
        createOrder({ id: 'o-del', orderNumber: 'ORD-100306', deliveryStatus: 'delivered', fulfillmentStage: 'delivered', nextStepOwner: 'none' }),
      ],
      totalCount: 2,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('#ORD-100305')).toBeTruthy());
    expect(screen.queryByText('paid')).toBeNull();
    expect(screen.queryByText(/🚚 paid/)).toBeNull();
    expect(screen.getByText('🚚 Ожидает сборки на складе ZAMK')).toBeTruthy();
    expect(screen.queryByText('🚚 Доставлен')).toBeNull();
  });
});
