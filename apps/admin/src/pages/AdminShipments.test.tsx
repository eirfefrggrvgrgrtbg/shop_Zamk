import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminShipments, formatTransitDuration } from './AdminShipments';
import * as adminPickingApi from '../api/adminPicking';
import * as adminShipmentsApi from '../api/adminShipments';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: () => ({
    hasPermission: () => true,
    hasAnyPermission: () => true,
  }),
}));

vi.mock('../api/adminPicking', async () => {
  const actual = await vi.importActual<typeof import('../api/adminPicking')>('../api/adminPicking');
  return {
    ...actual,
    getAdminDispatchQueue: vi.fn(),
  };
});

vi.mock('../api/adminShipments', async () => {
  const actual = await vi.importActual<typeof import('../api/adminShipments')>('../api/adminShipments');
  return {
    ...actual,
    getAdminShipments: vi.fn(),
    getAdminShipment: vi.fn(),
    deliverAdminShipment: vi.fn(),
  };
});

const sampleDispatchQueue: adminPickingApi.DispatchQueueItem[] = [
  {
    fulfillmentId: 'fulf-uuid-2222-bbbb',
    orderId: 'ord-uuid-2222-bbbb',
    orderNumber: 'ORD-100210',
    sellerId: 'seller-uuid-2222',
    sellerName: null,
    status: 'packed',
    orderStatus: 'packed',
    itemsCount: 1,
    unitsCount: 1,
    totalQuantity: 1,
    deliveryMethodName: 'Почта России',
    createdAt: '2026-09-25T10:00:00Z',
    packedAt: '2026-09-25T12:00:00Z',
  },
  {
    fulfillmentId: 'fulf-uuid-1111-aaaa',
    orderId: 'ord-uuid-1111-aaaa',
    orderNumber: 'ORD-100209',
    sellerId: 'seller-uuid-1111',
    sellerName: 'Studio One',
    status: 'packed',
    orderStatus: 'packed',
    itemsCount: 2,
    unitsCount: 3,
    totalQuantity: 3,
    deliveryMethodName: 'СДЭК Курьер',
    createdAt: '2026-09-25T08:00:00Z',
    packedAt: '2026-09-25T09:30:00Z',
  },
];

const sampleShipments: adminShipmentsApi.AdminShipmentView[] = [
  {
    id: 'ship-uuid-shipped-1',
    shipmentId: 'ship-uuid-shipped-1',
    orderId: 'ord-uuid-3333',
    orderNumber: 'ORD-100301',
    fulfillmentId: 'fulf-uuid-3333',
    fulfillmentStatus: 'shipped',
    sellerId: 'seller-uuid-3333',
    sellerName: 'Atelier Nord',
    status: 'shipped',
    statusLabel: 'Отгружен',
    carrier: 'СДЭК',
    trackingNumber: 'TRK-301-CDEK',
    trackingUrl: 'https://cdek.ru/track/TRK-301-CDEK',
    deliveryMethodName: 'СДЭК ПВЗ',
    itemsCount: 2,
    unitsCount: 4,
    packedAt: '2026-09-26T09:00:00Z',
    shippedAt: '2026-09-26T11:15:00Z',
    createdAt: '2026-09-26T11:15:00Z',
    updatedAt: '2026-09-26T11:15:00Z',
  },
  {
    id: 'ship-uuid-shipped-2',
    shipmentId: 'ship-uuid-shipped-2',
    orderId: 'ord-uuid-3334',
    orderNumber: 'ORD-100302',
    fulfillmentId: 'fulf-uuid-3334',
    fulfillmentStatus: 'shipped',
    sellerId: 'seller-uuid-3334',
    sellerName: null,
    status: 'shipped',
    statusLabel: 'Отгружен',
    carrier: 'Boxberry',
    trackingNumber: 'BXB-998877',
    deliveryMethodName: 'Boxberry Курьер',
    itemsCount: 1,
    unitsCount: 1,
    packedAt: '2026-09-26T10:00:00Z',
    shippedAt: '2026-09-26T14:20:00Z',
    createdAt: '2026-09-26T14:20:00Z',
    updatedAt: '2026-09-26T14:20:00Z',
  },
  {
    id: 'ship-uuid-delivered-1',
    shipmentId: 'ship-uuid-delivered-1',
    orderId: 'ord-uuid-4444',
    orderNumber: 'ORD-100401',
    fulfillmentId: 'fulf-uuid-4444',
    fulfillmentStatus: 'delivered',
    sellerId: 'seller-uuid-4444',
    sellerName: 'Maison K',
    status: 'delivered',
    statusLabel: 'Доставлен',
    carrier: 'СДЭК',
    trackingNumber: 'TRK-401-DELIV',
    deliveryMethodName: 'СДЭК Курьер',
    itemsCount: 3,
    unitsCount: 5,
    packedAt: '2026-09-20T08:00:00Z',
    shippedAt: '2026-09-20T10:00:00Z',
    deliveredAt: '2026-09-22T14:00:00Z',
    createdAt: '2026-09-20T10:00:00Z',
    updatedAt: '2026-09-22T14:00:00Z',
  },
  {
    id: 'ship-uuid-delivered-fallback',
    shipmentId: 'ship-uuid-delivered-fallback',
    orderId: 'ord-uuid-4445',
    orderNumber: 'ORD-100402',
    fulfillmentId: 'fulf-uuid-4445',
    fulfillmentStatus: 'delivered',
    sellerId: 'seller-uuid-4445',
    sellerName: null,
    status: 'delivered',
    statusLabel: 'Доставлен',
    carrier: undefined,
    trackingNumber: undefined,
    itemsCount: 1,
    unitsCount: 1,
    shippedAt: '2026-09-22T10:00:00Z',
    deliveredAt: '2026-09-22T14:18:00Z',
    createdAt: '2026-09-22T10:00:00Z',
    updatedAt: '2026-09-22T14:18:00Z',
  },
  {
    id: 'ship-uuid-failed-1',
    shipmentId: 'ship-uuid-failed-1',
    orderId: 'ord-uuid-5555',
    orderNumber: 'ORD-100501',
    fulfillmentId: 'fulf-uuid-5555',
    fulfillmentStatus: 'shipped',
    sellerId: 'seller-uuid-5555',
    sellerName: 'Urban Thread',
    status: 'failed',
    statusLabel: 'Ошибка',
    carrier: 'DPD',
    trackingNumber: 'DPD-FAIL-501',
    deliveryMethodName: 'DPD Курьер',
    itemsCount: 1,
    unitsCount: 2,
    shippedAt: '2026-09-21T12:00:00Z',
    createdAt: '2026-09-21T12:00:00Z',
    updatedAt: '2026-09-23T09:00:00Z',
  },
  {
    id: 'ship-uuid-cancelled-1',
    shipmentId: 'ship-uuid-cancelled-1',
    orderId: 'ord-uuid-5556',
    orderNumber: 'ORD-100502',
    fulfillmentId: 'fulf-uuid-5556',
    fulfillmentStatus: 'cancelled',
    sellerId: 'seller-uuid-5556',
    sellerName: 'Forma Lab',
    status: 'cancelled',
    statusLabel: 'Отменен',
    carrier: 'СДЭК',
    trackingNumber: 'TRK-502-CANC',
    itemsCount: 1,
    unitsCount: 1,
    createdAt: '2026-09-21T13:00:00Z',
    updatedAt: '2026-09-21T16:00:00Z',
  },
];

const mockGetAdminShipmentDetail = (id: string): adminShipmentsApi.AdminShipmentView => {
  const found = sampleShipments.find((s) => s.id === id);
  if (!found) {
    throw new Error('Отправление не найдено');
  }
  return {
    ...found,
    customerName: 'Иван Иванов',
    customerPhone: '+7 (999) 111-22-33',
    deliveryAddress: 'г. Москва, ул. Арбат, д. 10',
    items: [
      {
        orderItemId: `item-1-${id}`,
        productId: 'prod-1',
        variantId: 'var-1',
        productTitle: 'Шерстяное пальто',
        imageUrl: 'https://example.com/coat.jpg',
        variantColor: 'Черный',
        variantSize: 'M',
        quantity: 1,
      },
      {
        orderItemId: `item-2-${id}`,
        productId: 'prod-2',
        variantId: 'var-2',
        productTitle: 'Шелковый шарф',
        imageUrl: null,
        variantColor: null,
        variantSize: null,
        quantity: 2,
      },
    ],
  };
};

describe('formatTransitDuration helper', () => {
  it('formats less than 1 minute', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-20T10:00:45Z';
    expect(formatTransitDuration(start, end)).toBe('В пути: < 1 мин.');
  });

  it('formats minutes only (< 1 hour)', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-20T10:03:00Z';
    expect(formatTransitDuration(start, end)).toBe('В пути: 3 мин.');
  });

  it('formats hours and minutes (< 24 hours)', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-20T14:18:00Z';
    expect(formatTransitDuration(start, end)).toBe('В пути: 4 ч. 18 мин.');
  });

  it('formats exact hours without trailing 0 minutes', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-20T14:00:00Z';
    expect(formatTransitDuration(start, end)).toBe('В пути: 4 ч.');
  });

  it('formats days and hours (>= 24 hours)', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-22T14:00:00Z';
    expect(formatTransitDuration(start, end)).toBe('В пути: 2 дн. 4 ч.');
  });

  it('formats exact days without trailing 0 hours', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-22T10:00:00Z';
    expect(formatTransitDuration(start, end)).toBe('В пути: 2 дн.');
  });

  it('returns null for missing, invalid, or backwards timestamps', () => {
    expect(formatTransitDuration(null, '2026-09-20T10:00:00Z')).toBeNull();
    expect(formatTransitDuration('2026-09-20T10:00:00Z', null)).toBeNull();
    expect(formatTransitDuration('invalid-date', '2026-09-20T10:00:00Z')).toBeNull();
    expect(formatTransitDuration('2026-09-22T10:00:00Z', '2026-09-20T10:00:00Z')).toBeNull();
  });
});

describe('AdminShipments Outbound Workspace (SHIPMENTS UX.3C1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue(sampleDispatchQueue);
    vi.mocked(adminShipmentsApi.getAdminShipments).mockResolvedValue(sampleShipments);
    vi.mocked(adminShipmentsApi.getAdminShipment).mockImplementation(async (id: string) =>
      mockGetAdminShipmentDetail(id)
    );
  });

  it('A & B: renders 4 lifecycle tabs with accurate counts and defaults to "К отправке"', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByRole('heading', { name: 'Отправления' })).toBeDefined();

    const tabToDispatch = screen.getByTestId('shipments-tab-to_dispatch');
    const tabInTransit = screen.getByTestId('shipments-tab-in_transit');
    const tabDelivered = screen.getByTestId('shipments-tab-delivered');
    const tabProblems = screen.getByTestId('shipments-tab-problems');

    expect(within(tabToDispatch).getByText('К отправке')).toBeDefined();
    expect(within(tabInTransit).getByText('В пути')).toBeDefined();
    expect(within(tabDelivered).getByText('Доставлены')).toBeDefined();
    expect(within(tabProblems).getByText('Проблемы')).toBeDefined();

    expect(screen.getByTestId('shipments-tab-count-to_dispatch').textContent).toBe('2');
    expect(screen.getByTestId('shipments-tab-count-in_transit').textContent).toBe('2');
    expect(screen.getByTestId('shipments-tab-count-delivered').textContent).toBe('2');
    expect(screen.getByTestId('shipments-tab-count-problems').textContent).toBe('2');

    expect(tabToDispatch.getAttribute('aria-selected')).toBe('true');
  });

  it('C, D, E, F: isolates rows strictly into their lifecycle tab buckets', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByText('ORD-100210')).toBeDefined();
    expect(screen.queryByText('ORD-100301')).toBeNull();
    expect(screen.queryByText('ORD-100401')).toBeNull();
    expect(screen.queryByText('ORD-100501')).toBeNull();

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.getByText('ORD-100302')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();
    expect(screen.queryByText('ORD-100401')).toBeNull();
    expect(screen.queryByText('ORD-100501')).toBeNull();

    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.getByText('ORD-100401')).toBeDefined();
    expect(screen.getByText('ORD-100402')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();
    expect(screen.queryByText('ORD-100301')).toBeNull();
    expect(screen.queryByText('ORD-100501')).toBeNull();

    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    expect(screen.getByText('ORD-100501')).toBeDefined();
    expect(screen.getByText('ORD-100502')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();
    expect(screen.queryByText('ORD-100301')).toBeNull();
    expect(screen.queryByText('ORD-100401')).toBeNull();
  });

  it('G, H, I, J: renders human identity (orderNumber primary, sellerName secondary), falls back to "Продавец", and hides UUIDs', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByText('Studio One')).toBeDefined();

    expect(screen.getByText('ORD-100210')).toBeDefined();
    expect(screen.getByText('Продавец')).toBeDefined();

    const pageText = screen.getByTestId('admin-shipments-page').textContent || '';
    expect(pageText).not.toContain('fulf-uuid-1111-aaaa');
    expect(pageText).not.toContain('ord-uuid-1111-aaaa');
    expect(pageText).not.toContain('seller-uuid-1111');
    expect(pageText).not.toContain('seller-uuid-2222');

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const inTransitText = screen.getByTestId('admin-shipments-page').textContent || '';
    expect(inTransitText).not.toContain('ship-uuid-shipped-1');
    expect(inTransitText).not.toContain('fulf-uuid-3333');
    expect(inTransitText).not.toContain('ord-uuid-3333');
    expect(inTransitText).not.toContain('seller-uuid-3333');
  });

  it('Carrier / Tracking hierarchy and fallbacks: renders carrier primary and tracking secondary without prominent standalone "—"', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));

    const table = screen.getByRole('table');
    expect(within(table).getByText('СДЭК')).toBeDefined();
    expect(within(table).getByText('TRK-401-DELIV')).toBeDefined();

    expect(within(table).getByText('Служба не указана')).toBeDefined();
    expect(within(table).getByText('Трек не указан')).toBeDefined();
  });

  it('K, L, M, N: displays itemsCount/unitsCount, carrier/tracking, packedAt, deliveredAt, and derived transit duration', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('2 позиции')).toBeDefined();
    expect(screen.getByText('3 единицы')).toBeDefined();
    expect(screen.getByText('1 позиция')).toBeDefined();
    expect(screen.getByText('1 единица')).toBeDefined();
    expect(screen.getByText('СДЭК Курьер')).toBeDefined();

    const rows = screen.getAllByRole('row');
    expect(rows[1].textContent).toContain('ORD-100209');
    expect(rows[2].textContent).toContain('ORD-100210');

    const formattedPackedAt = new Date('2026-09-25T09:30:00Z').toLocaleString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
    expect(screen.getByText(formattedPackedAt)).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const inTransitTable = screen.getByRole('table');
    expect(within(inTransitTable).getByText('СДЭК')).toBeDefined();
    expect(within(inTransitTable).getByText('TRK-301-CDEK')).toBeDefined();
    expect(within(inTransitTable).getByText('Boxberry')).toBeDefined();
    expect(within(inTransitTable).getByText('BXB-998877')).toBeDefined();
    expect(within(inTransitTable).getByText('4 единицы')).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    const formattedDeliveredAt = new Date('2026-09-22T14:00:00Z').toLocaleString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
    expect(screen.getByText(formattedDeliveredAt)).toBeDefined();
    expect(screen.getByText('В пути: 2 дн. 4 ч.')).toBeDefined();
    expect(screen.getByText('В пути: 4 ч. 18 мин.')).toBeDefined();
    expect(screen.getByText('3 позиции')).toBeDefined();
    expect(screen.getByText('5 единиц')).toBeDefined();
  });

  it('Action column visibility: present on "К отправке" and "В пути", removed from "Доставлены" and "Проблемы"', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByRole('columnheader', { name: 'Действие' })).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    expect(screen.getByRole('columnheader', { name: 'Действие' })).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.queryByRole('columnheader', { name: 'Действие' })).toBeNull();

    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    expect(screen.queryByRole('columnheader', { name: 'Действие' })).toBeNull();
  });

  it('O, P, Q, R: preserves dispatch navigation and delivery confirmation modal while keeping delivered and problem tabs read-only', async () => {
    vi.mocked(adminShipmentsApi.deliverAdminShipment).mockResolvedValue({
      shipmentId: 'ship-uuid-shipped-1',
      fulfillmentId: 'fulf-uuid-3333',
      orderId: 'ord-uuid-3333',
      shipmentStatus: 'delivered',
      fulfillmentStatus: 'delivered',
      orderStatus: 'delivered',
      deliveredAt: '2026-09-27T10:00:00Z',
    });

    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    const dispatchLinks = await screen.findAllByRole('link', { name: /Перейти к отгрузке/i });
    expect(dispatchLinks).toHaveLength(2);
    expect(dispatchLinks[0].getAttribute('href')).toBe('/fulfillment/dispatch/fulf-uuid-1111-aaaa');

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const deliverButtons = screen.getAllByRole('button', { name: /Подтвердить доставку/i });
    expect(deliverButtons).toHaveLength(2);

    fireEvent.click(deliverButtons[1]);
    expect(screen.getByText('Подтвердить доставку?')).toBeDefined();
    expect(screen.getAllByText('TRK-301-CDEK').length).toBeGreaterThanOrEqual(2);

    const modalConfirmBtns = screen.getAllByRole('button', { name: /Подтвердить доставку/i });
    fireEvent.click(modalConfirmBtns[modalConfirmBtns.length - 1]);

    await waitFor(() => {
      expect(adminShipmentsApi.deliverAdminShipment).toHaveBeenCalledWith('ship-uuid-shipped-1');
    });

    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.queryByRole('button', { name: /Подтвердить доставку/i })).toBeNull();
    expect(screen.queryByRole('link', { name: /Перейти к отгрузке/i })).toBeNull();

    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    expect(screen.queryByRole('button', { name: /Подтвердить доставку/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Повторить|Восстановить|Переотправить/i })).toBeNull();
  });

  it('S, T, U, V, W: supports dynamic search placeholder, carrier filter visibility, filter reset, and "Ничего не найдено"', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();

    const searchInputToDispatch = screen.getByPlaceholderText('Заказ или продавец');
    expect(searchInputToDispatch).toBeDefined();
    expect(screen.queryByLabelText('Служба доставки')).toBeNull();

    fireEvent.change(searchInputToDispatch, { target: { value: '100210' } });
    expect(screen.getByText('ORD-100210')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Сбросить' }));
    expect(screen.getByText('ORD-100209')).toBeDefined();
    expect(screen.getByText('ORD-100210')).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const searchInputInTransit = screen.getByPlaceholderText('Заказ или трек-номер');
    expect(searchInputInTransit).toBeDefined();
    expect(screen.getByLabelText('Служба доставки')).toBeDefined();

    fireEvent.change(searchInputInTransit, { target: { value: 'bxb-998877' } });
    expect(screen.getByText('ORD-100302')).toBeDefined();
    expect(screen.queryByText('ORD-100301')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Сбросить' }));
    const carrierSelect = screen.getByLabelText('Служба доставки');
    fireEvent.change(carrierSelect, { target: { value: 'СДЭК' } });
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.queryByText('ORD-100302')).toBeNull();

    fireEvent.change(searchInputInTransit, { target: { value: 'NON-EXISTENT-99999' } });
    expect(screen.getByText('Ничего не найдено')).toBeDefined();

    fireEvent.click(screen.getByRole('button', { name: 'Сбросить фильтры' }));
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.getByText('ORD-100302')).toBeDefined();
  });

  it('X & Y: enforces PII boundary in list and uses exact problem status labels without inventing failure reasons', async () => {
    const shipmentsWithExtraFields: adminShipmentsApi.AdminShipmentView[] = [
      {
        ...sampleShipments[4],
        customerName: 'Секретный Покупатель',
        customerPhone: '+79991112233',
        deliveryAddress: 'г. Москва, ул. Тайная, д. 42, кв. 10',
      },
      sampleShipments[5],
    ];
    vi.mocked(adminShipmentsApi.getAdminShipments).mockResolvedValue(shipmentsWithExtraFields);

    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();
    fireEvent.click(screen.getByTestId('shipments-tab-problems'));

    expect(screen.getByText('Ошибка доставки')).toBeDefined();
    expect(screen.getByText('Отправка отменена')).toBeDefined();

    const pageText = screen.getByTestId('admin-shipments-page').textContent || '';
    expect(pageText).not.toMatch(/утеряна|повреждена|возвращается курьером|причина отмены/i);

    expect(pageText).not.toContain('Секретный Покупатель');
    expect(pageText).not.toContain('+79991112233');
    expect(pageText).not.toContain('ул. Тайная');
  });

  it('renders tab-specific empty states when buckets are empty and maps enriched AdminShipment DTOs', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue([]);
    vi.mocked(adminShipmentsApi.getAdminShipments).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('Нет отправлений, ожидающих передачи')).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    expect(screen.getByText('Сейчас нет отправлений в пути')).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.getByText('Доставленных отправлений пока нет')).toBeDefined();

    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    expect(screen.getByText('Проблемных отправлений нет')).toBeDefined();

    const mapped = adminShipmentsApi.mapAdminShipment({
      id: 'ship-1',
      shipmentId: 'ship-1',
      orderId: 'ord-1',
      orderNumber: 'ORD-500',
      fulfillmentId: 'fulf-1',
      fulfillmentStatus: 'shipped',
      sellerId: 'sel-1',
      sellerName: 'Brand X',
      status: 'shipped',
      carrier: 'СДЭК',
      trackingNumber: 'TRK-500',
      deliveryMethodName: 'СДЭК Курьер',
      itemsCount: 2,
      unitsCount: 3,
      packedAt: '2026-09-25T10:00:00Z',
      shippedAt: '2026-09-25T12:00:00Z',
    });
    expect(mapped.orderNumber).toBe('ORD-500');
    expect(mapped.sellerName).toBe('Brand X');
    expect(mapped.itemsCount).toBe(2);
    expect(mapped.unitsCount).toBe(3);
    expect(mapped.deliveryMethodName).toBe('СДЭК Курьер');
  });
});

describe('Shipment Read-Only Detail Drawer (SHIPMENTS UX.3D1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue(sampleDispatchQueue);
    vi.mocked(adminShipmentsApi.getAdminShipments).mockResolvedValue(sampleShipments);
    vi.mocked(adminShipmentsApi.getAdminShipment).mockImplementation(async (id: string) =>
      mockGetAdminShipmentDetail(id)
    );
  });

  it('A, B, C, D: opens drawer for shipped, delivered, problem rows, and NOT for packed fulfillments', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();

    // D: Packed row in "К отправке" does NOT open shipment drawer
    const packedRow = screen.getByText('ORD-100209').closest('tr');
    expect(packedRow).toBeDefined();
    fireEvent.click(packedRow!);
    expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    expect(adminShipmentsApi.getAdminShipment).not.toHaveBeenCalled();

    // A: Shipped row opens drawer
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const shippedRow = (await screen.findByText('ORD-100301')).closest('tr');
    fireEvent.click(shippedRow!);

    expect(await screen.findByTestId('shipment-detail-drawer')).toBeDefined();
    expect(adminShipmentsApi.getAdminShipment).toHaveBeenCalledWith('ship-uuid-shipped-1');

    // Close drawer via close button
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));
    await waitFor(() => {
      expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    });

    // B: Delivered row opens drawer
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    const deliveredRow = (await screen.findByText('ORD-100401')).closest('tr');
    fireEvent.click(deliveredRow!);

    expect(await screen.findByTestId('shipment-detail-drawer')).toBeDefined();
    expect(adminShipmentsApi.getAdminShipment).toHaveBeenCalledWith('ship-uuid-delivered-1');

    // Close drawer via backdrop click
    fireEvent.click(screen.getByTestId('shipment-drawer-backdrop'));
    await waitFor(() => {
      expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    });

    // C: Failed/Problem row opens drawer
    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    const failedRow = (await screen.findByText('ORD-100501')).closest('tr');
    fireEvent.click(failedRow!);

    expect(await screen.findByTestId('shipment-detail-drawer')).toBeDefined();
    expect(adminShipmentsApi.getAdminShipment).toHaveBeenCalledWith('ship-uuid-failed-1');
  });

  it('Keyboard accessibility: opens drawer on Enter or Space key press on shipment row', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));

    const shippedRow = (await screen.findByText('ORD-100301')).closest('tr');
    expect(shippedRow).toBeDefined();

    // Trigger via Enter key
    fireEvent.keyDown(shippedRow!, { key: 'Enter' });
    expect(await screen.findByTestId('shipment-detail-drawer')).toBeDefined();
    expect(adminShipmentsApi.getAdminShipment).toHaveBeenCalledWith('ship-uuid-shipped-1');

    // Close via Escape key
    fireEvent.keyDown(window, { key: 'Escape' });
    await waitFor(() => {
      expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    });
  });

  it('Action buttons in row do NOT trigger drawer opening (event propagation stopped)', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));

    await screen.findByText('ORD-100301');
    const deliverBtns = screen.getAllByRole('button', { name: /Подтвердить доставку/i });
    expect(deliverBtns.length).toBeGreaterThan(0);

    // Clicking "Подтвердить доставку" opens the confirmation modal, NOT the drawer
    fireEvent.click(deliverBtns[0]);
    expect(screen.getByText('Подтвердить доставку?')).toBeDefined();
    expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    expect(adminShipmentsApi.getAdminShipment).not.toHaveBeenCalled();
  });

  it('E, F, G, H: Header renders orderNumber primary, sellerName secondary (or fallback "Продавец"), correct badge, and hides UUID heading', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));

    // Open ORD-100301 (has sellerName: Atelier Nord)
    const shippedRow = (await screen.findByText('ORD-100301')).closest('tr');
    fireEvent.click(shippedRow!);

    const drawer = await screen.findByTestId('shipment-detail-drawer');
    const heading = within(drawer).getByRole('heading', { level: 2 });
    expect(heading.textContent).toBe('ORD-100301');
    expect(within(drawer).getByText('Atelier Nord')).toBeDefined();
    expect(within(drawer).getByTestId('shipment-drawer-status-badge').textContent).toBe('В пути');

    // H: No UUID in drawer heading
    expect(heading.textContent).not.toContain('ship-uuid-shipped-1');
    expect(heading.textContent).not.toContain('ord-uuid-3333');

    // Close drawer
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));

    // Open ORD-100302 (null sellerName -> fallback "Продавец")
    const shippedRow2 = (await screen.findByText('ORD-100302')).closest('tr');
    fireEvent.click(shippedRow2!);

    const drawer2 = await screen.findByTestId('shipment-detail-drawer');
    expect(within(drawer2).getByRole('heading', { level: 2 }).textContent).toBe('ORD-100302');
    expect(within(drawer2).getByText('Продавец')).toBeDefined();
  });

  it('Current State summary block: renders accurate canonical state text without invented reasons', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');

    // 1. Shipped
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);
    expect(
      within(await screen.findByTestId('shipment-drawer-current-state')).getByText(
        'Отправление передано в доставку'
      )
    ).toBeDefined();
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));

    // 2. Delivered
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    fireEvent.click((await screen.findByText('ORD-100401')).closest('tr')!);
    expect(
      within(await screen.findByTestId('shipment-drawer-current-state')).getByText(
        'Отправление доставлено'
      )
    ).toBeDefined();
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));

    // 3. Failed
    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    fireEvent.click((await screen.findByText('ORD-100501')).closest('tr')!);
    expect(
      within(await screen.findByTestId('shipment-drawer-current-state')).getByText(
        'Доставка не завершена'
      )
    ).toBeDefined();
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));

    // 4. Cancelled
    fireEvent.click((await screen.findByText('ORD-100502')).closest('tr')!);
    expect(
      within(await screen.findByTestId('shipment-drawer-current-state')).getByText(
        'Отправка отменена'
      )
    ).toBeDefined();
  });

  it('I, J, K, L: Delivery block renders carrier, tracking, tracking URL link, timestamps, and missing fallbacks', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));

    // Open ORD-100301 (has full carrier, tracking, and safe URL)
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);
    const deliveryBlock = await screen.findByTestId('shipment-drawer-delivery');

    expect(within(deliveryBlock).getByText('Служба доставки')).toBeDefined();
    expect(within(deliveryBlock).getByText('СДЭК')).toBeDefined();
    expect(within(deliveryBlock).getByText('TRK-301-CDEK')).toBeDefined();

    // K: Tracking URL is safe external link
    const trackLink = within(deliveryBlock).getByRole('link', { name: /Отследить/i });
    expect(trackLink.getAttribute('href')).toBe('https://cdek.ru/track/TRK-301-CDEK');
    expect(trackLink.getAttribute('target')).toBe('_blank');

    // Close and open ORD-100402 with missing carrier and tracking
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    fireEvent.click((await screen.findByText('ORD-100402')).closest('tr')!);

    const fallbackDeliveryBlock = await screen.findByTestId('shipment-drawer-delivery');
    expect(within(fallbackDeliveryBlock).getByText('Не указана')).toBeDefined();
    expect(within(fallbackDeliveryBlock).getAllByText('Не указан')).toHaveLength(2);
  });

  it('M, N, O, P: Contents block displays product items, quantity, clean variant attributes, and summary counts', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);

    const contents = await screen.findByTestId('shipment-drawer-contents');

    // P: itemsCount & unitsCount summary (ORD-100301 has 2 items, 4 units)
    expect(within(contents).getByText(/2 позиции · 4 единицы/i)).toBeDefined();

    // M: product title, image, and quantity
    expect(within(contents).getByText('Шерстяное пальто')).toBeDefined();
    expect(within(contents).getByText('1 шт.')).toBeDefined();
    const coatImg = within(contents).getByRole('img', { name: 'Шерстяное пальто' });
    expect(coatImg.getAttribute('src')).toBe('https://example.com/coat.jpg');

    // N: color + size
    expect(within(contents).getByText('Черный · M')).toBeDefined();

    // O: item 2 has missing color and size -> does NOT render empty separator
    expect(within(contents).getByText('Шелковый шарф')).toBeDefined();
    expect(within(contents).getByText('2 шт.')).toBeDefined();
    const contentsText = contents.textContent || '';
    expect(contentsText).not.toContain('undefined');
    expect(contentsText).not.toContain('null');
  });

  it('Q & R: Recipient PII is displayed exclusively inside drawer and never leaks to list', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));

    // Before opening drawer: PII does NOT exist anywhere in document
    expect(screen.queryByText('Иван Иванов')).toBeNull();
    expect(screen.queryByText('+7 (999) 111-22-33')).toBeNull();
    expect(screen.queryByText('г. Москва, ул. Арбат, д. 10')).toBeNull();

    // Open drawer
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);

    // Q: Recipient details exist inside recipient section
    const recipientSection = await screen.findByTestId('shipment-drawer-recipient');
    expect(within(recipientSection).getByText('Иван Иванов')).toBeDefined();
    expect(within(recipientSection).getByText('+7 (999) 111-22-33')).toBeDefined();
    expect(within(recipientSection).getByText('г. Москва, ул. Арбат, д. 10')).toBeDefined();

    // Close drawer
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));
    await waitFor(() => {
      expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    });

    // R: List remains strictly PII-free
    const listText = screen.getByTestId('admin-shipments-page').textContent || '';
    expect(listText).not.toContain('Иван Иванов');
    expect(listText).not.toContain('+7 (999) 111-22-33');
    expect(listText).not.toContain('ул. Арбат');
  });

  it('S, T, U: Timeline displays exact canonical timestamps, does not fake delivered timestamp, and marks exception for problem states', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');

    // Shipped shipment
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);

    const timelineShipped = await screen.findByTestId('shipment-drawer-timeline');
    expect(within(timelineShipped).getByText('Упаковано')).toBeDefined();
    expect(within(timelineShipped).getByText('Передано в доставку')).toBeDefined();
    expect(within(timelineShipped).getByText('Доставлено')).toBeDefined();

    // T: Shipped shipment has NOT been delivered -> does not fake deliveredAt
    expect(within(timelineShipped).getByText('Не доставлено')).toBeDefined();
    expect(screen.queryByTestId('shipment-drawer-timeline-exception')).toBeNull();

    // Close and open Failed shipment
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));
    fireEvent.click(screen.getByTestId('shipments-tab-problems'));
    fireEvent.click((await screen.findByText('ORD-100501')).closest('tr')!);

    // U: Exception block shown without fabricating reasons
    const exceptionBlock = await screen.findByTestId('shipment-drawer-timeline-exception');
    expect(within(exceptionBlock).getByText('Ошибка доставки')).toBeDefined();
    expect(within(exceptionBlock).getByText('Доставка не завершена')).toBeDefined();
    expect(exceptionBlock.textContent).not.toMatch(/утеряна|повреждена|курьер/i);
  });

  it('V & W: Technical section is collapsed by default and contains shipmentId, fulfillmentId, orderId', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);

    const techSection = await screen.findByTestId('shipment-drawer-technical');
    const detailsEl = techSection.querySelector('details');
    expect(detailsEl).toBeDefined();

    // V: Collapsed by default
    expect(detailsEl?.hasAttribute('open')).toBe(false);

    // W: Contains required IDs
    expect(within(techSection).getByText('ship-uuid-shipped-1')).toBeDefined();
    expect(within(techSection).getByText('fulf-uuid-3333')).toBeDefined();
    expect(within(techSection).getByText('ord-uuid-3333')).toBeDefined();

    // Copy button
    const copyBtns = within(techSection).getAllByTitle(/Скопировать/i);
    expect(copyBtns.length).toBeGreaterThanOrEqual(3);
  });

  it('X & Y: Handles loading and error states with a retry button', async () => {
    let rejectPromise: ((err: Error) => void) | null = null;
    vi.mocked(adminShipmentsApi.getAdminShipment).mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectPromise = reject;
        })
    );

    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    fireEvent.click((await screen.findByText('ORD-100301')).closest('tr')!);

    // X: Loading state rendered
    expect(screen.getByTestId('shipment-drawer-loading')).toBeDefined();

    // Trigger rejection
    rejectPromise!(new Error('Network failure'));

    // Y: Error state rendered with retry button
    expect(await screen.findByTestId('shipment-drawer-error')).toBeDefined();
    expect(screen.getByText('Не удалось загрузить данные отправления')).toBeDefined();

    const retryBtn = screen.getByRole('button', { name: /Повторить попытку/i });
    expect(retryBtn).toBeDefined();

    // Retry invokes getAdminShipment again
    vi.mocked(adminShipmentsApi.getAdminShipment).mockResolvedValueOnce(
      mockGetAdminShipmentDetail('ship-uuid-shipped-1')
    );
    fireEvent.click(retryBtn);

    expect(await screen.findByTestId('shipment-drawer-delivery')).toBeDefined();
  });

  it('Z & AA: Opening and closing drawer preserves tab, search, and carrier filter state', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    await screen.findByText('ORD-100209');

    // Switch tab to "В пути"
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));

    // Type in search query
    const searchInput = screen.getByPlaceholderText('Заказ или трек-номер');
    fireEvent.change(searchInput, { target: { value: '301' } });

    // Select carrier filter
    const carrierSelect = screen.getByLabelText('Служба доставки');
    fireEvent.change(carrierSelect, { target: { value: 'СДЭК' } });

    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.queryByText('ORD-100302')).toBeNull();

    // Open drawer
    fireEvent.click(screen.getByText('ORD-100301').closest('tr')!);
    expect(await screen.findByTestId('shipment-detail-drawer')).toBeDefined();

    // Close drawer
    fireEvent.click(screen.getByTestId('shipment-drawer-close'));
    await waitFor(() => {
      expect(screen.queryByTestId('shipment-detail-drawer')).toBeNull();
    });

    // AA: Tab, search query, and carrier filter remain completely intact!
    expect(screen.getByTestId('shipments-tab-in_transit').getAttribute('aria-selected')).toBe(
      'true'
    );
    expect((screen.getByPlaceholderText('Заказ или трек-номер') as HTMLInputElement).value).toBe(
      '301'
    );
    expect((screen.getByLabelText('Служба доставки') as HTMLSelectElement).value).toBe('СДЭК');
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.queryByText('ORD-100302')).toBeNull();
  });
});
