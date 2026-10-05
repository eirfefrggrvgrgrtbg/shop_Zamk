// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { OrderQuickView } from '../../components/support/OrderQuickView';
import * as adminOrdersApi from '../../api/adminOrders';

vi.mock('../../api/adminOrders', () => ({
  getAdminOrder: vi.fn(),
}));

const mockOrderView: adminOrdersApi.AdminOrderView = {
  id: 'ord-100',
  orderNumber: 'ZMK-100481',
  status: 'paid',
  statusLabel: 'Оплачен',
  paymentStatus: 'paid',
  paymentStatusLabel: 'Оплачен картой',
  fulfillmentsCount: 0,
  itemPositionsCount: 1,
  unitsCount: 2,
  sourceType: 'online',
  customerName: 'Анна Иванова',
  customerEmail: 'anna@example.com',
  customerPhone: '+7 999 123-45-67',
  deliveryAddress: 'г. Москва, ул. Ленина, д. 1',
  deliveryMethodName: 'Курьер ZAMK',
  totalAmount: 4500,
  totalPriceCents: 450000,
  currency: 'RUB',
  items: [
    {
      id: 'item-1',
      orderId: 'ord-100',
      productId: 'prod-1',
      productVariantId: 'var-1',
      sellerId: 'seller-1',
      title: 'Кроссовки Urban Runner',
      productSlug: 'urban-runner',
      priceCents: 225000,
      quantity: 2,
      subtotalPriceCents: 450000,
      createdAt: '2026-10-03T10:00:00Z',
    },
  ],
};

describe('OrderQuickView Component Contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state initially while fetching order', async () => {
    (adminOrdersApi.getAdminOrder as any).mockReturnValue(new Promise(() => {}));

    render(
      <MemoryRouter>
        <OrderQuickView orderId="ord-100" onClose={vi.fn()} />
      </MemoryRouter>
    );

    expect(screen.getByText('Загрузка данных заказа...')).toBeDefined();
  });

  it('renders error state when order fetch fails', async () => {
    (adminOrdersApi.getAdminOrder as any).mockRejectedValue(new Error('Сетевой сбой при загрузке'));

    render(
      <MemoryRouter>
        <OrderQuickView orderId="ord-100" onClose={vi.fn()} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Сетевой сбой при загрузке')).toBeDefined();
    });
  });

  it('renders canonical order details and items on success', async () => {
    (adminOrdersApi.getAdminOrder as any).mockResolvedValue(mockOrderView);

    render(
      <MemoryRouter>
        <OrderQuickView orderId="ord-100" onClose={vi.fn()} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Заказ ZMK-100481')).toBeDefined();
      expect(screen.getByText('Оплачен')).toBeDefined();
      expect(screen.getByText('Оплачен картой')).toBeDefined();
      expect(screen.getByText('4500 ₽')).toBeDefined();
      expect(screen.getByText('Анна Иванова')).toBeDefined();
      expect(screen.getByText('anna@example.com')).toBeDefined();
      expect(screen.getByText('+7 999 123-45-67')).toBeDefined();
      expect(screen.getByText('Курьер ZAMK')).toBeDefined();
      expect(screen.getByText('г. Москва, ул. Ленина, д. 1')).toBeDefined();
      expect(screen.getByText('Кроссовки Urban Runner')).toBeDefined();
      expect(screen.getByText('2 шт.')).toBeDefined();
    });
  });

  it('provides explicit full order card link pointing to /orders/:id in a new tab', async () => {
    (adminOrdersApi.getAdminOrder as any).mockResolvedValue(mockOrderView);

    render(
      <MemoryRouter>
        <OrderQuickView orderId="ord-100" onClose={vi.fn()} />
      </MemoryRouter>
    );

    const fullLink = screen.getByRole('link', { name: /Открыть полную карточку заказа/i });
    expect(fullLink.getAttribute('href')).toBe('/orders/ord-100');
    expect(fullLink.getAttribute('target')).toBe('_blank');
    expect(fullLink.getAttribute('rel')).toBe('noopener noreferrer');
  });

  it('triggers onClose when close button is clicked', async () => {
    (adminOrdersApi.getAdminOrder as any).mockResolvedValue(mockOrderView);
    const onClose = vi.fn();

    render(
      <MemoryRouter>
        <OrderQuickView orderId="ord-100" onClose={onClose} />
      </MemoryRouter>
    );

    const closeBtn = screen.getByTitle('Закрыть Quick View');
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
