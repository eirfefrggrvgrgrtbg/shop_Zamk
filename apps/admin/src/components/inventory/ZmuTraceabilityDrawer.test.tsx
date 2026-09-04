import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { ZmuTraceabilityDrawer } from './ZmuTraceabilityDrawer';
import type { AdminInventoryUnitTraceability } from '../../api/adminInventory';

// Mock getAdminInventoryUnitTraceability
vi.mock('../../api/adminInventory', async () => {
  const actual = await vi.importActual('../../api/adminInventory');
  return {
    ...actual,
    getAdminInventoryUnitTraceability: vi.fn().mockImplementation((unitCode: string) => {
      if (unitCode === 'ZMU-XUJBQQ5ADSW4BWTX') {
        return Promise.resolve(mockStaleUnitTraceability);
      }
      if (unitCode === 'ZMU-WJEFXRQDGPYY6JF7') {
        return Promise.resolve(mockLiveUnitTraceability);
      }
      if (unitCode === 'ZMU-PARTIAL-123') {
        return Promise.resolve(mockPartialHistoryTraceability);
      }
      return Promise.reject(new Error('Unit not found'));
    }),
  };
});

// Forensic Stale Unit
const mockStaleUnitTraceability: AdminInventoryUnitTraceability = {
  identity: {
    id: 'bfb72d4b-b5c4-40fc-bd69-f9c303a03852',
    unitCode: 'ZMU-XUJBQQ5ADSW4BWTX',
    variantId: '3b37fd2c-40d7-364b-892d-5ef4e3905afd',
    productId: '2a6fa985-dae0-39eb-ad87-253f982e84f1',
    productTitle: 'Dev Wool Coat',
    variantName: 'M · Graphite',
    sku: 'DEV-SKU-0',
    barcode: 'ZMK-DEV-0001',
    size: 'M',
    color: 'Graphite',
    sellerId: '44444444-4444-4444-8444-444444444444',
    sellerName: 'ZAMK Dev Seller',
    source: 'seller',
  },
  currentState: {
    status: 'warehouse',
    availability: 'free',
    location: 'Не ведётся',
    isStaleAllocation: true,
    healthIssue: 'stale_active_allocation',
  },
  origin: {
    supplyId: '810d9299-e2b4-436f-9f10-355df27ac762',
    supplyNumber: 'SUP-001197',
    supplyStatus: 'completed',
    receivedAt: '2026-08-25T14:18:52Z',
  },
  currentContext: {
    liveAllocation: undefined,
    staleAllocation: {
      id: '4df2106f-32d4-4469-bae6-4d6bd22f6da7',
      orderId: 'e94d9db8-60b1-4e06-851a-e96d3490174e',
      orderNumber: 'ORD-100193',
      orderStatus: 'delivered',
      fulfillmentId: 'b0f663be-6ab2-4731-860c-8c4885f696ad',
      fulfillmentStatus: 'delivered',
      pickedAt: '2026-08-25T15:00:00Z',
    },
  },
  timeline: [
    {
      id: 'ret-fin-1',
      type: 'return_received',
      category: 'physical',
      eventName: 'Возвращена на склад',
      description: 'Возврат RET-583FB821: единица проверена и возвращена в свободный остаток (ресток)',
      timestamp: '2026-08-31T09:40:20Z',
      sourceEntity: 'return_item_units',
      referenceNumber: 'RET-583FB821',
      referenceId: 'ret-123',
      actorRole: 'staff',
      link: '/returns?id=ret-123',
    },
    {
      id: 'ret-scan-1',
      type: 'return_unit_scanned',
      category: 'operation',
      eventName: 'Единица отсканирована при приёмке возврата',
      description: 'Физическая единица отсканирована на складе при приёмке возврата RET-583FB821',
      timestamp: '2026-08-31T09:40:03Z',
      sourceEntity: 'return_item_units',
      referenceNumber: 'RET-583FB821',
      referenceId: 'ret-123',
      actorRole: 'staff',
      link: '/returns?id=ret-123',
    },
    {
      id: 'ret-rec-start-1',
      type: 'return_receiving_started',
      category: 'operation',
      eventName: 'Начата приёмка возврата',
      description: 'Сотрудник склада начал приёмку возврата RET-583FB821',
      timestamp: '2026-08-31T09:39:48Z',
      sourceEntity: 'returns',
      referenceNumber: 'RET-583FB821',
      referenceId: 'ret-123',
      actorRole: 'staff',
      link: '/returns?id=ret-123',
    },
    {
      id: 'ret-app-1',
      type: 'return_approved',
      category: 'order_lifecycle',
      eventName: 'Возврат согласован',
      description: 'Возврат RET-583FB821 одобрен к приёмке на складе',
      timestamp: '2026-08-30T11:02:26Z',
      sourceEntity: 'returns',
      referenceNumber: 'RET-583FB821',
      referenceId: 'ret-123',
      actorRole: 'staff',
      link: '/returns?id=ret-123',
    },
    {
      id: 'ret-req-1',
      type: 'return_requested',
      category: 'order_lifecycle',
      eventName: 'Запрошен возврат',
      description: 'Покупатель запросил возврат по причине: Брак',
      timestamp: '2026-08-30T10:50:13Z',
      sourceEntity: 'returns',
      referenceNumber: 'RET-583FB821',
      referenceId: 'ret-123',
      actorRole: 'customer',
      link: '/returns?id=ret-123',
    },
    {
      id: 'alloc-1',
      type: 'allocation_created',
      category: 'commitment',
      eventName: 'Назначена заказу',
      description: 'Единица назначена на комплектацию заказа ORD-100193 (Оплачен)',
      timestamp: '2026-08-29T15:21:50Z',
      sourceEntity: 'order_item_allocations',
      referenceNumber: 'ORD-100193',
      referenceId: 'e94d9db8-60b1-4e06-851a-e96d3490174e',
      actorRole: 'system',
      link: '/orders/e94d9db8-60b1-4e06-851a-e96d3490174e',
    },
    {
      id: 'rec-scan-1',
      type: 'received',
      category: 'physical',
      eventName: 'Принята на склад',
      description: 'Физическая единица принята на склад при приёмке поставки SUP-001197',
      timestamp: '2026-08-25T17:18:52Z',
      sourceEntity: 'supply_receiving_scans',
      referenceNumber: 'SUP-001197',
      referenceId: '810d9299-e2b4-436f-9f10-355df27ac762',
      actorRole: 'staff',
      actorName: 'Warehouse Staff',
      link: '/supply-receiving?id=810d9299-e2b4-436f-9f10-355df27ac762',
    },
    {
      id: 'sup-1',
      type: 'inbound_created',
      category: 'physical',
      eventName: 'Ожидается поступление',
      description: 'Единица заявлена продавцом в поставке SUP-001197',
      timestamp: '2026-08-25T17:18:01Z',
      sourceEntity: 'seller_supplies',
      referenceNumber: 'SUP-001197',
      referenceId: '810d9299-e2b4-436f-9f10-355df27ac762',
      actorRole: 'seller',
      link: '/supply-receiving?id=810d9299-e2b4-436f-9f10-355df27ac762',
    },
  ],
  hasPartialHistory: false,
};

// Forensic Live Allocated Unit
const mockLiveUnitTraceability: AdminInventoryUnitTraceability = {
  identity: {
    id: '6b8e8083-a4e8-4579-aac2-5d578023090c',
    unitCode: 'ZMU-WJEFXRQDGPYY6JF7',
    variantId: '3b37fd2c-40d7-364b-892d-5ef4e3905afd',
    productId: '2a6fa985-dae0-39eb-ad87-253f982e84f1',
    productTitle: 'Dev Wool Coat',
    variantName: 'M · Graphite',
    sku: 'DEV-SKU-0',
    barcode: 'ZMK-DEV-0001',
    size: 'M',
    color: 'Graphite',
    sellerId: '44444444-4444-4444-8444-444444444444',
    sellerName: 'ZAMK Dev Seller',
    source: 'seller',
  },
  currentState: {
    status: 'warehouse',
    availability: 'allocated',
    location: 'Не ведётся',
    isStaleAllocation: false,
  },
  origin: {
    supplyId: '810d9299-e2b4-436f-9f10-355df27ac762',
    supplyNumber: 'SUP-001201',
    supplyStatus: 'completed',
    receivedAt: '2026-09-03T15:40:29Z',
  },
  currentContext: {
    liveAllocation: {
      id: '7bdf248b-ba5c-4381-8bbb-8a267d6fa2c3',
      orderId: 'afa4e753-c7e5-422f-a477-605cd387efc9',
      orderNumber: 'ORD-100196',
      orderStatus: 'paid',
      fulfillmentId: '228aadaf-e051-4255-8cdf-d596195d1d5e',
      fulfillmentStatus: 'paid',
      pickedAt: undefined, // Not picked yet!
    },
  },
  timeline: [
    {
      id: 'alloc-live-1',
      type: 'allocation_created',
      category: 'commitment',
      eventName: 'Назначена заказу',
      description: 'Единица назначена на комплектацию заказа ORD-100196 (Оплачен)',
      timestamp: '2026-09-03T16:00:00Z',
      sourceEntity: 'order_item_allocations',
      referenceNumber: 'ORD-100196',
      referenceId: 'afa4e753-c7e5-422f-a477-605cd387efc9',
      actorRole: 'system',
      link: '/orders/afa4e753-c7e5-422f-a477-605cd387efc9',
    },
    {
      id: 'rec-scan-live-1',
      type: 'received',
      category: 'physical',
      eventName: 'Принята на склад',
      description: 'Физическая единица принята на склад при приёмке поставки SUP-001201',
      timestamp: '2026-09-03T15:40:29Z',
      sourceEntity: 'supply_receiving_scans',
      referenceNumber: 'SUP-001201',
      referenceId: '810d9299-e2b4-436f-9f10-355df27ac762',
      actorRole: 'staff',
      actorName: 'Warehouse Staff',
      link: '/supply-receiving?id=810d9299-e2b4-436f-9f10-355df27ac762',
    },
    {
      id: 'sup-live-1',
      type: 'inbound_created',
      category: 'physical',
      eventName: 'Ожидается поступление',
      description: 'Единица заявлена продавцом в поставке SUP-001201',
      timestamp: '2026-09-03T15:00:00Z',
      sourceEntity: 'seller_supplies',
      referenceNumber: 'SUP-001201',
      referenceId: '810d9299-e2b4-436f-9f10-355df27ac762',
      actorRole: 'seller',
      link: '/supply-receiving?id=810d9299-e2b4-436f-9f10-355df27ac762',
    },
  ],
  hasPartialHistory: false,
};

// Partial History Unit
const mockPartialHistoryTraceability: AdminInventoryUnitTraceability = {
  identity: {
    id: 'partial-id-1',
    unitCode: 'ZMU-PARTIAL-123',
    variantId: '3b37fd2c-40d7-364b-892d-5ef4e3905afd',
    productId: '2a6fa985-dae0-39eb-ad87-253f982e84f1',
    productTitle: 'Dev Wool Coat',
    variantName: 'M · Graphite',
    sku: 'DEV-SKU-0',
    barcode: 'ZMK-DEV-0001',
    size: 'M',
    color: 'Graphite',
    sellerId: '44444444-4444-4444-8444-444444444444',
    sellerName: 'ZAMK Dev Seller',
    source: 'seller',
  },
  currentState: {
    status: 'warehouse',
    availability: 'free',
    location: 'Не ведётся',
    isStaleAllocation: false,
  },
  currentContext: {},
  timeline: [],
  hasPartialHistory: true,
};

describe('ZmuTraceabilityDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders nothing when closed or no unitCode', () => {
    const { container } = render(
      <MemoryRouter>
        <ZmuTraceabilityDrawer unitCode={null} isOpen={false} onClose={() => {}} />
      </MemoryRouter>
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders stale forensic unit ZMU-XUJBQQ5ADSW4BWTX correctly', async () => {
    render(
      <MemoryRouter>
        <ZmuTraceabilityDrawer unitCode="ZMU-XUJBQQ5ADSW4BWTX" isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    // Header & Identity
    await waitFor(() => {
      expect(screen.getByText('ZMU-XUJBQQ5ADSW4BWTX')).toBeDefined();
    });
    expect(screen.getByText(/Dev Wool Coat/)).toBeDefined();
    expect(screen.getByText(/M · Graphite/)).toBeDefined();

    // Badges in header
    expect(screen.getAllByText('На складе')[0]).toBeDefined();
    expect(screen.getAllByText('Свободна')[0]).toBeDefined();
    expect(screen.getAllByText(/Старое назначение/i)[0]).toBeDefined();

    // Current State Summary block
    expect(screen.getByText(/Текущее состояние единицы/i)).toBeDefined();
    expect(screen.getByText('Не ведётся')).toBeDefined();
    expect(screen.getAllByText('SUP-001197').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/Историческое:/i)).toBeDefined();

    // Diagnostic alert callout
    expect(screen.getByText(/Текущее расхождение: Старое назначение/i)).toBeDefined();
    expect(screen.getByText(/не было закрыто/i)).toBeDefined();

    // Quick Action: Free Scanner link (canonical route)
    const scannerLink = screen.getByRole('link', { name: /Открыть в сканере/i });
    expect(scannerLink.getAttribute('href')).toContain('/warehouse/free-scan?q=ZMU-XUJBQQ5ADSW4BWTX');

    // Canonical Timeline events (stable recorded historical facts only)
    expect(screen.getByText('История единицы (8)')).toBeDefined();
    expect(screen.getByText('Возвращена на склад')).toBeDefined();
    expect(screen.getByText('Единица отсканирована при приёмке возврата')).toBeDefined();
    expect(screen.getByText('Начата приёмка возврата')).toBeDefined();
    expect(screen.getByText('Возврат согласован')).toBeDefined();
    expect(screen.getByText('Запрошен возврат')).toBeDefined();
    expect(screen.getByText('Назначена заказу')).toBeDefined();
    expect(screen.getByText('Принята на склад')).toBeDefined();
    expect(screen.getByText('Ожидается поступление')).toBeDefined();

    // Category badges
    expect(screen.getAllByText('ФИЗИЧЕСКОЕ').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('ОБЯЗАТЕЛЬСТВО')).toBeDefined();
    expect(screen.getAllByText('ОПЕРАЦИЯ').length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('ЗАКАЗ / ВОЗВРАТ').length).toBeGreaterThanOrEqual(2);

    // Humanized Actor Presentation (no raw staff/customer/system role tags)
    expect(screen.getByText('Warehouse Staff · Сотрудник')).toBeDefined();
    expect(screen.getAllByText('Сотрудник').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Система').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Покупатель').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Продавец').length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText(/\(staff\)/)).toBeNull();
    expect(screen.queryByText(/\(customer\)/)).toBeNull();
    expect(screen.queryByText(/\(system\)/)).toBeNull();

    // Raw DB table names must be hidden from primary visible UI
    expect(screen.queryByText('return_item_units')).toBeNull();
    expect(screen.queryByText('order_item_allocations')).toBeNull();
    expect(screen.queryByText('seller_supplies')).toBeNull();
    expect(screen.queryByText('supply_receiving_scans')).toBeNull();
  });

  it('renders live allocated forensic unit ZMU-WJEFXRQDGPYY6JF7 correctly with no fake future events', async () => {
    render(
      <MemoryRouter>
        <ZmuTraceabilityDrawer unitCode="ZMU-WJEFXRQDGPYY6JF7" isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-WJEFXRQDGPYY6JF7')).toBeDefined();
    });

    // Current availability
    expect(screen.getAllByText('Назначена заказу')[0]).toBeDefined();

    // Current Context showing live allocation
    expect(screen.getAllByText('ORD-100196').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Оплачен')).toBeDefined();
    expect(screen.getByText('Ожидает сборки')).toBeDefined();

    // Timeline only has 3 events (no fake picked / shipped)
    expect(screen.getByText('История единицы (3)')).toBeDefined();
    expect(screen.queryByText('Собрана на складе')).toBeNull();
    expect(screen.queryByText('Отгружена со склада')).toBeNull();
  });

  it('renders partial history notice when hasPartialHistory is true', async () => {
    render(
      <MemoryRouter>
        <ZmuTraceabilityDrawer unitCode="ZMU-PARTIAL-123" isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-PARTIAL-123')).toBeDefined();
    });

    expect(
      screen.getByText('История этой единицы сохранена не полностью. Отображаются только зафиксированные события.')
    ).toBeDefined();
    expect(screen.getByText(/Для этой единицы пока не зафиксировано событий жизненного цикла/i)).toBeDefined();
  });
});
