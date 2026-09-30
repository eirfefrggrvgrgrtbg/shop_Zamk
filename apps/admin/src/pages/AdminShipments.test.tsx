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
    const end = '2026-09-22T14:00:00Z'; // 52 hours = 2 days 4 hours
    expect(formatTransitDuration(start, end)).toBe('В пути: 2 дн. 4 ч.');
  });

  it('formats exact days without trailing 0 hours', () => {
    const start = '2026-09-20T10:00:00Z';
    const end = '2026-09-22T10:00:00Z'; // 48 hours = exactly 2 days
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

    // C: Packed fulfillments appear ONLY in "К отправке"
    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByText('ORD-100210')).toBeDefined();
    expect(screen.queryByText('ORD-100301')).toBeNull();
    expect(screen.queryByText('ORD-100401')).toBeNull();
    expect(screen.queryByText('ORD-100501')).toBeNull();

    // D: Shipped shipments appear ONLY in "В пути"
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.getByText('ORD-100302')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();
    expect(screen.queryByText('ORD-100401')).toBeNull();
    expect(screen.queryByText('ORD-100501')).toBeNull();

    // E: Delivered shipments appear ONLY in "Доставлены"
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.getByText('ORD-100401')).toBeDefined();
    expect(screen.getByText('ORD-100402')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();
    expect(screen.queryByText('ORD-100301')).toBeNull();
    expect(screen.queryByText('ORD-100501')).toBeNull();

    // F: Failed and cancelled shipments appear ONLY in "Проблемы"
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

    // G & H: orderNumber primary and sellerName secondary shown
    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByText('Studio One')).toBeDefined();

    // I: null sellerName -> "Продавец"
    expect(screen.getByText('ORD-100210')).toBeDefined();
    expect(screen.getByText('Продавец')).toBeDefined();

    // J: UUIDs are not rendered in list text
    const pageText = screen.getByTestId('admin-shipments-page').textContent || '';
    expect(pageText).not.toContain('fulf-uuid-1111-aaaa');
    expect(pageText).not.toContain('ord-uuid-1111-aaaa');
    expect(pageText).not.toContain('seller-uuid-1111');
    expect(pageText).not.toContain('seller-uuid-2222');

    // Also verify on "В пути" tab
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

    // ORD-100401 has carrier 'СДЭК' and tracking 'TRK-401-DELIV'
    const table = screen.getByRole('table');
    expect(within(table).getByText('СДЭК')).toBeDefined();
    expect(within(table).getByText('TRK-401-DELIV')).toBeDefined();

    // ORD-100402 has missing carrier and tracking -> fallbacks
    expect(within(table).getByText('Служба не указана')).toBeDefined();
    expect(within(table).getByText('Трек не указан')).toBeDefined();
  });

  it('K, L, M, N: displays itemsCount/unitsCount, carrier/tracking, packedAt, deliveredAt, and derived transit duration', async () => {
    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    // K & M: "К отправке" shows itemsCount, unitsCount, deliveryMethodName, packedAt, and deterministic FIFO sort (oldest packed first)
    expect(await screen.findByText('2 позиции')).toBeDefined();
    expect(screen.getByText('3 единицы')).toBeDefined();
    expect(screen.getByText('1 позиция')).toBeDefined();
    expect(screen.getByText('1 единица')).toBeDefined();
    expect(screen.getByText('СДЭК Курьер')).toBeDefined();

    // Verify FIFO sort on "К отправке": ORD-100209 (packed 09:30) appears before ORD-100210 (packed 12:00)
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

    // L: "В пути" shows carrier, trackingNumber, and shippedAt
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const inTransitTable = screen.getByRole('table');
    expect(within(inTransitTable).getByText('СДЭК')).toBeDefined();
    expect(within(inTransitTable).getByText('TRK-301-CDEK')).toBeDefined();
    expect(within(inTransitTable).getByText('Boxberry')).toBeDefined();
    expect(within(inTransitTable).getByText('BXB-998877')).toBeDefined();
    expect(within(inTransitTable).getByText('4 единицы')).toBeDefined();

    // N: "Доставлены" shows deliveredAt and derived transit duration ("В пути: 2 дн. 4 ч." for ORD-100401 and "В пути: 4 ч. 18 мин." for ORD-100402)
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

    // "К отправке" has "Действие" header and action links
    expect(await screen.findByText('ORD-100209')).toBeDefined();
    expect(screen.getByRole('columnheader', { name: 'Действие' })).toBeDefined();

    // "В пути" has "Действие" header and action buttons
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    expect(screen.getByRole('columnheader', { name: 'Действие' })).toBeDefined();

    // "Доставлены" does NOT have "Действие" header
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.queryByRole('columnheader', { name: 'Действие' })).toBeNull();

    // "Проблемы" does NOT have "Действие" header
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

    // O: Packed action links to existing dispatch flow (/fulfillment/dispatch/:id)
    const dispatchLinks = await screen.findAllByRole('link', { name: /Перейти к отгрузке/i });
    expect(dispatchLinks).toHaveLength(2);
    expect(dispatchLinks[0].getAttribute('href')).toBe('/fulfillment/dispatch/fulf-uuid-1111-aaaa');

    // P: Delivery confirmation on "В пути" opens modal and invokes deliverAdminShipment
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const deliverButtons = screen.getAllByRole('button', { name: /Подтвердить доставку/i });
    expect(deliverButtons).toHaveLength(2);

    // Clicking second button (ORD-100302 is newer shippedAt, so index 1 is ORD-100301)
    fireEvent.click(deliverButtons[1]);
    expect(screen.getByText('Подтвердить доставку?')).toBeDefined();
    expect(screen.getAllByText('TRK-301-CDEK').length).toBeGreaterThanOrEqual(2);

    // Confirm inside modal
    const modalConfirmBtns = screen.getAllByRole('button', { name: /Подтвердить доставку/i });
    fireEvent.click(modalConfirmBtns[modalConfirmBtns.length - 1]);

    await waitFor(() => {
      expect(adminShipmentsApi.deliverAdminShipment).toHaveBeenCalledWith('ship-uuid-shipped-1');
    });

    // Q: Delivered rows have NO operational delivery button
    fireEvent.click(screen.getByTestId('shipments-tab-delivered'));
    expect(screen.queryByRole('button', { name: /Подтвердить доставку/i })).toBeNull();
    expect(screen.queryByRole('link', { name: /Перейти к отгрузке/i })).toBeNull();

    // R: Problem rows do not invent recovery action
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

    // On "К отправке": placeholder is "Заказ или продавец" and carrier select is hidden
    const searchInputToDispatch = screen.getByPlaceholderText('Заказ или продавец');
    expect(searchInputToDispatch).toBeDefined();
    expect(screen.queryByLabelText('Служба доставки')).toBeNull();

    // S: Search by order number on "К отправке"
    fireEvent.change(searchInputToDispatch, { target: { value: '100210' } });
    expect(screen.getByText('ORD-100210')).toBeDefined();
    expect(screen.queryByText('ORD-100209')).toBeNull();

    // V: Clearing filters restores rows
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить' }));
    expect(screen.getByText('ORD-100209')).toBeDefined();
    expect(screen.getByText('ORD-100210')).toBeDefined();

    // On "В пути": placeholder is "Заказ или трек-номер" and carrier select is visible
    fireEvent.click(screen.getByTestId('shipments-tab-in_transit'));
    const searchInputInTransit = screen.getByPlaceholderText('Заказ или трек-номер');
    expect(searchInputInTransit).toBeDefined();
    expect(screen.getByLabelText('Служба доставки')).toBeDefined();

    // T: Search by tracking number on "В пути"
    fireEvent.change(searchInputInTransit, { target: { value: 'bxb-998877' } });
    expect(screen.getByText('ORD-100302')).toBeDefined();
    expect(screen.queryByText('ORD-100301')).toBeNull();

    // U: Carrier filter on "В пути"
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить' }));
    const carrierSelect = screen.getByLabelText('Служба доставки');
    fireEvent.change(carrierSelect, { target: { value: 'СДЭК' } });
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.queryByText('ORD-100302')).toBeNull();

    // W: Zero filtered result -> "Ничего не найдено"
    fireEvent.change(searchInputInTransit, { target: { value: 'NON-EXISTENT-99999' } });
    expect(screen.getByText('Ничего не найдено')).toBeDefined();

    // Clicking "Сбросить фильтры" inside empty state restores rows
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить фильтры' }));
    expect(screen.getByText('ORD-100301')).toBeDefined();
    expect(screen.getByText('ORD-100302')).toBeDefined();
  });

  it('X & Y: enforces PII boundary in list and uses exact problem status labels without inventing failure reasons', async () => {
    const shipmentsWithExtraFields: adminShipmentsApi.AdminShipmentView[] = [
      {
        ...sampleShipments[4], // failed
        customerName: 'Секретный Покупатель',
        customerPhone: '+79991112233',
        deliveryAddress: 'г. Москва, ул. Тайная, д. 42, кв. 10',
      },
      sampleShipments[5], // cancelled
    ];
    vi.mocked(adminShipmentsApi.getAdminShipments).mockResolvedValue(shipmentsWithExtraFields);

    render(
      <MemoryRouter>
        <AdminShipments />
      </MemoryRouter>
    );

    expect(await screen.findByText('ORD-100209')).toBeDefined();
    fireEvent.click(screen.getByTestId('shipments-tab-problems'));

    // Y: Exact problem status labels ("Ошибка доставки", "Отправка отменена"), no fabricated failure reasons
    expect(screen.getByText('Ошибка доставки')).toBeDefined();
    expect(screen.getByText('Отправка отменена')).toBeDefined();

    const pageText = screen.getByTestId('admin-shipments-page').textContent || '';
    expect(pageText).not.toMatch(/утеряна|повреждена|возвращается курьером|причина отмены/i);

    // X: No recipient PII appears in list
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
