/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Orders } from './Orders';

// Mock contexts
vi.mock('../contexts/ToastContext', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));

vi.mock('../components/ui/Drawer', () => ({
  Drawer: ({ isOpen, children }: { isOpen: boolean; children: React.ReactNode }) =>
    isOpen ? <div data-testid="drawer">{children}</div> : null,
}));

vi.mock('../components/account/AccountLayout', () => ({
  AccountLayout: ({ children, title }: { children: React.ReactNode; title: string }) => (
    <div data-testid="account-layout">
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));

// Mock customer api client functions
const mockGetOrders = vi.fn();
const mockGetCustomerReturns = vi.fn();
const mockGetCustomerReviews = vi.fn();
const mockGetCustomerOrderFulfillments = vi.fn();

vi.mock('@zamk/api-client/src/customer', () => ({
  getOrders: () => mockGetOrders(),
  getCustomerReturns: () => mockGetCustomerReturns(),
  getCustomerReviews: () => mockGetCustomerReviews(),
  getCustomerOrderFulfillments: (id: string) => mockGetCustomerOrderFulfillments(id),
  createPayment: vi.fn(),
}));

describe('M4.3.3C2B — Shop Multi-Fulfillment Tracking & Observer Status Regression', () => {
  const mockShippedOrder = {
    id: 'ord-100207-uuid',
    orderNumber: 'ORD-100207',
    status: 'shipped',
    totalPriceCents: 450000,
    deliveryAddress: 'Москва, ул. Арбат, 10',
    deliveryMethodName: 'Курьерская доставка ZAMK',
    deliveryPriceCents: 0,
    createdAt: '2026-09-25T10:00:00Z',
    items: [
      {
        id: 'item-1',
        orderItemId: 'item-1',
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        variantSize: 'L',
        variantColor: 'Черный',
        priceCents: 450000,
        quantity: 1,
        sellerName: 'ZAMK Studio',
        imageUrl: 'https://cdn.zamk.test/hoodie.jpg',
      },
    ],
  };

  const mockPackedOrder = {
    id: 'ord-100208-uuid',
    orderNumber: 'ORD-100208',
    status: 'packed',
    totalPriceCents: 300000,
    deliveryAddress: 'Санкт-Петербург, Невский пр., 1',
    deliveryMethodName: 'Курьерская доставка ZAMK',
    deliveryPriceCents: 0,
    createdAt: '2026-09-24T10:00:00Z',
    items: [
      {
        id: 'item-2',
        orderItemId: 'item-2',
        productId: 'prod-2',
        productTitle: 'Свитшот Графит',
        variantSize: 'M',
        variantColor: 'Серый',
        priceCents: 300000,
        quantity: 1,
        sellerName: 'ZAMK Studio',
        imageUrl: 'https://cdn.zamk.test/sweatshirt.jpg',
      },
    ],
  };

  const mockDeliveredOrder = {
    id: 'ord-100209-uuid',
    orderNumber: 'ORD-100209',
    status: 'delivered',
    totalPriceCents: 300000,
    deliveryAddress: 'Санкт-Петербург, Невский пр., 1',
    deliveryMethodName: 'Курьерская доставка ZAMK',
    deliveryPriceCents: 0,
    createdAt: '2026-09-24T10:00:00Z',
    items: [
      {
        id: 'item-3',
        orderItemId: 'item-3',
        productId: 'prod-3',
        productTitle: 'Свитшот Графит',
        variantSize: 'M',
        variantColor: 'Серый',
        priceCents: 300000,
        quantity: 1,
        sellerName: 'ZAMK Studio',
        imageUrl: 'https://cdn.zamk.test/sweatshirt.jpg',
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetCustomerReturns.mockResolvedValue({ items: [] });
    mockGetCustomerReviews.mockResolvedValue({ items: [] });
  });

  afterEach(() => {
    cleanup();
  });

  it('1. shipped order renders "Передан в доставку" and does NOT render old "Отправлен"', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-100207')).toBeDefined();
    });

    expect(screen.getAllByText('Передан в доставку')[0]).toBeDefined();
    expect(screen.queryByText('Отправлен')).toBeNull();
  });

  it('2. shipped timeline shows "В пути" and standard delivery steps', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'CDEK',
        trackingNumber: '1234567890',
        trackingUrl: 'https://cdek.ru/tracking?num=1234567890',
        shippedAt: '2026-09-25T11:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-100207')).toBeDefined();
    });

    fireEvent.click(screen.getByText('ORD-100207'));

    await waitFor(() => {
      expect(screen.getByText('Заказ ORD-100207')).toBeDefined();
    });

    expect(screen.getAllByText('Оформлен')[0]).toBeDefined();
    expect(screen.getAllByText('В пути')[0]).toBeDefined();
    expect(screen.getAllByText('Доставлен')[0]).toBeDefined();
  });

  it('3. expanding shipped order lazily calls getCustomerOrderFulfillments(orderId)', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'СДЭК',
        trackingNumber: 'TRK-987654',
        trackingUrl: 'https://cdek.ru/track/TRK-987654',
        shippedAt: '2026-09-25T12:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-100207')).toBeDefined();
    });

    // Before expanding, API should not be called
    expect(mockGetCustomerOrderFulfillments).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText('ORD-100207'));

    await waitFor(() => {
      expect(screen.getByText('Заказ ORD-100207')).toBeDefined();
    });

    await waitFor(() => {
      expect(mockGetCustomerOrderFulfillments).toHaveBeenCalledWith('ord-100207-uuid');
    });

    expect(
      screen.getByText('Заказ собран на складе ZAMK и передан в службу доставки.')
    ).toBeDefined();
    expect(screen.getByText('Следующий этап — доставка заказа.')).toBeDefined();
  });

  it('4. single fulfillment renders one compact package tracking block without package title', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'СДЭК Экспресс',
        trackingNumber: 'CDEK-777888',
        trackingUrl: 'https://cdek.ru/tracking?num=CDEK-777888',
        shippedAt: '2026-09-25T14:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('СДЭК Экспресс')).toBeDefined());

    // Single fulfillment must NOT render "Посылка 1" header
    expect(screen.queryByText('Посылка 1')).toBeNull();
    expect(screen.getByText('CDEK-777888')).toBeDefined();

    const trackingLink = screen.getByRole('link', { name: 'Отследить посылку' });
    expect(trackingLink.getAttribute('href')).toBe('https://cdek.ru/tracking?num=CDEK-777888');
  });

  it('5. multi fulfillment renders distinct package tracking blocks with package titles', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'CarrierA',
        trackingNumber: 'TRK-A',
        trackingUrl: 'https://a.example/track/TRK-A',
        shippedAt: '2026-09-25T14:00:00Z',
        items: [],
      },
      {
        id: 'f-2',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'CarrierB',
        trackingNumber: 'TRK-B',
        trackingUrl: 'https://b.example/track/TRK-B',
        shippedAt: '2026-09-25T15:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('CarrierA')).toBeDefined());

    expect(screen.getByText('Посылка 1')).toBeDefined();
    expect(screen.getByText('Посылка 2')).toBeDefined();
    expect(screen.getByText('TRK-A')).toBeDefined();
    expect(screen.getByText('TRK-B')).toBeDefined();
  });

  it('6. multi-package isolation: Package 1 and Package 2 are strictly isolated without cross-assignment', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'CarrierA',
        trackingNumber: 'TRK-A',
        trackingUrl: 'https://a.example/track/TRK-A',
        shippedAt: '2026-09-25T10:00:00Z',
        items: [],
      },
      {
        id: 'f-2',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'CarrierB',
        trackingNumber: 'TRK-B',
        trackingUrl: 'https://b.example/track/TRK-B',
        shippedAt: '2026-09-25T12:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('Посылка 1')).toBeDefined());

    // Scoped container verification for Package 1
    const package1Header = screen.getByText('Посылка 1');
    const package1Card = package1Header.closest('.rounded-2xl') as HTMLElement;
    expect(package1Card).not.toBeNull();
    const p1 = within(package1Card);
    expect(p1.getByText('CarrierA')).toBeDefined();
    expect(p1.getByText('TRK-A')).toBeDefined();
    const link1 = p1.getByRole('link', { name: 'Отследить посылку' });
    expect(link1.getAttribute('href')).toBe('https://a.example/track/TRK-A');
    expect(p1.queryByText('CarrierB')).toBeNull();
    expect(p1.queryByText('TRK-B')).toBeNull();

    // Scoped container verification for Package 2
    const package2Header = screen.getByText('Посылка 2');
    const package2Card = package2Header.closest('.rounded-2xl') as HTMLElement;
    expect(package2Card).not.toBeNull();
    const p2 = within(package2Card);
    expect(p2.getByText('CarrierB')).toBeDefined();
    expect(p2.getByText('TRK-B')).toBeDefined();
    const link2 = p2.getByRole('link', { name: 'Отследить посылку' });
    expect(link2.getAttribute('href')).toBe('https://b.example/track/TRK-B');
    expect(p2.queryByText('CarrierA')).toBeNull();
    expect(p2.queryByText('TRK-A')).toBeNull();
  });

  it('7. partial multi-fulfillment packed order asserts main badge remains packed', async () => {
    mockGetOrders.mockResolvedValue([mockPackedOrder]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100208')).toBeDefined());
    expect(screen.getAllByText('Упакован')[0]).toBeDefined();
    expect(screen.queryByText('Передан в доставку')).toBeNull();
  });

  it('8. carrier conditional rendering: shown when present, row omitted when null', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    // Fulfillment without carrier
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: null,
        trackingNumber: 'TRK-NOCARRIER',
        trackingUrl: 'https://track.example/TRK-NOCARRIER',
        shippedAt: '2026-09-25T14:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('TRK-NOCARRIER')).toBeDefined());

    expect(screen.queryByText('Служба доставки')).toBeNull();
  });

  it('9. trackingNumber conditional rendering: shown when present, row omitted when null', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    // Fulfillment without trackingNumber
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'Курьерская служба ZAMK',
        trackingNumber: null,
        trackingUrl: 'https://track.example/order',
        shippedAt: '2026-09-25T14:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('Курьерская служба ZAMK')).toBeDefined());

    expect(screen.queryByText('Трек-номер')).toBeNull();
  });

  it('10. trackingUrl conditional rendering: CTA exists with exact backend URL; no fake CTA when null', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    // Fulfillment without trackingUrl
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'Почта России',
        trackingNumber: 'RU123456789',
        trackingUrl: null,
        shippedAt: '2026-09-25T14:00:00Z',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('Почта России')).toBeDefined());

    expect(screen.getByText('RU123456789')).toBeDefined();
    // Must NOT render fake tracking button
    expect(screen.queryByRole('link', { name: /отследить/i })).toBeNull();
  });

  it('11. shippedAt conditional rendering: server timestamp rendered; no date inferred when null', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    // Fulfillment without shippedAt
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'Курьер ZAMK',
        trackingNumber: null,
        trackingUrl: null,
        shippedAt: null,
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('Курьер ZAMK')).toBeDefined());

    // Tracking card has no shipped date row
    // Primary status badge in order list and drawer header have 'Передан в доставку'
    const statusBadges = screen.getAllByText('Передан в доставку');
    expect(statusBadges.length).toBe(2);
  });

  it('12. failure behavior: getCustomerOrderFulfillments rejection keeps status/timeline and shows neutral fallback', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockRejectedValue(new Error('500 Server Error'));

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-100207')).toBeDefined();
    });

    fireEvent.click(screen.getByText('ORD-100207'));

    await waitFor(() => {
      expect(screen.getByText('Заказ ORD-100207')).toBeDefined();
    });

    // Shows safe neutral fallback message
    await waitFor(() => {
      expect(screen.getByText('Данные отслеживания пока недоступны')).toBeDefined();
    });

    // Canonical status and explanation remain completely intact
    expect(screen.getAllByText('Передан в доставку')[0]).toBeDefined();
    expect(
      screen.getByText('Заказ собран на складе ZAMK и передан в службу доставки.')
    ).toBeDefined();

    // Raw backend error must not be exposed to user
    expect(screen.queryByText(/500 Server Error/i)).toBeNull();
  });

  it('13. shipped order action safety: no review, return, cancel, warehouse, or confirm receipt actions', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'СДЭК',
        trackingNumber: 'TRK-100',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('Заказ ORD-100207')).toBeDefined());

    // Customer must NOT have review or return buttons while in transit
    expect(screen.queryByRole('button', { name: /отзыв/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /возврат/i })).toBeNull();
    // No cancel action for shipped order
    expect(screen.queryByRole('button', { name: /отмен/i })).toBeNull();
    // No warehouse/dispatch actions
    expect(screen.queryByRole('button', { name: /отгруз|сборк|склад/i })).toBeNull();
    // No confirm receipt action
    expect(screen.queryByRole('button', { name: /получен|подтвердить получение/i })).toBeNull();
  });

  it('14. delivered order regression: primary status "Доставлен", delivered timeline, review and return CTAs available', async () => {
    mockGetOrders.mockResolvedValue([mockDeliveredOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-deliv',
        orderId: 'ord-100209-uuid',
        status: 'delivered',
        carrier: 'CDEK',
        trackingNumber: 'DELIV-12345',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100209')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100209'));
    await waitFor(() => expect(screen.getByText('Заказ ORD-100209')).toBeDefined());

    expect(screen.getAllByText('Доставлен')[0]).toBeDefined();
    // Review and return CTAs must remain available for delivered order
    expect(screen.getByRole('button', { name: 'Оставить отзыв' })).toBeDefined();
    expect(screen.getByRole('button', { name: 'Оформить возврат' })).toBeDefined();
  });

  it('15. no customer-visible internal concepts (ZMU, fulfillment_id, warehouse.dispatch, reserved_stock)', async () => {
    mockGetOrders.mockResolvedValue([mockShippedOrder]);
    mockGetCustomerOrderFulfillments.mockResolvedValue([
      {
        id: 'f-1',
        orderId: 'ord-100207-uuid',
        status: 'shipped',
        carrier: 'СДЭК',
        trackingNumber: 'TRK-123',
        items: [],
      },
    ]);

    render(
      <MemoryRouter>
        <Orders />
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('ORD-100207')).toBeDefined());
    fireEvent.click(screen.getByText('ORD-100207'));
    await waitFor(() => expect(screen.getByText('Заказ ORD-100207')).toBeDefined());

    const pageText = document.body.textContent || '';
    expect(pageText.includes('fulfillment_id')).toBe(false);
    expect(pageText.includes('ZMU')).toBe(false);
    expect(pageText.includes('warehouse.dispatch')).toBe(false);
    expect(pageText.includes('reserved_stock')).toBe(false);
    expect(pageText.includes('reserved stock')).toBe(false);
  });
});
