vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn().mockReturnValue({
    hasPermission: () => true,
    user: null,
    isAuthenticated: true,
    isLoading: false,
    hasAnyPermission: () => false,
    staff: null,
    login: vi.fn(),
    logout: vi.fn()
  })
}));
// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AdminPickingQueue } from './AdminPickingQueue';
import { AdminPickingDetail } from './AdminPickingDetail';
import * as pickingApi from '../api/adminPicking';
import { ApiError } from '@zamk/api-client/src/errors';

describe('Admin Guided Picking Foundation (M6.1)', () => {
  it('queue renders actionable orders with correct status badges and CTAs', async () => {
    const mockQueue: pickingApi.PickingQueueItem[] = [
      {
        fulfillmentId: 'fulf-1',
        orderId: 'ord-1',
        orderNumber: '1001',
        status: 'paid',
        sellerName: 'ZAMK Store',
        createdAt: '2026-09-02T10:00:00Z',
        itemPositionsCount: 1,
        totalQuantity: 1,
        pickedQuantity: 0,
        remainingQuantity: 1,
        progressPercent: 0,
        isComplete: false,
      },
      {
        fulfillmentId: 'fulf-2',
        orderId: 'ord-2',
        orderNumber: '1002',
        status: 'assembling',
        sellerName: 'Fashion Brand',
        createdAt: '2026-09-02T09:30:00Z',
        itemPositionsCount: 2,
        totalQuantity: 5,
        pickedQuantity: 2,
        remainingQuantity: 3,
        progressPercent: 40,
        isComplete: false,
      },
    ];

    vi.spyOn(pickingApi, 'getAdminPickingQueue').mockResolvedValue(mockQueue);

    render(
      <MemoryRouter>
        <AdminPickingQueue />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Заказ #1001')).toBeDefined();
      expect(screen.getByText('Заказ #1002')).toBeDefined();
    });

    const statusBadges = screen.getAllByTestId('picking-status-badge');
    expect(statusBadges[0].textContent).toContain('Требует сборки');
    expect(statusBadges[1].textContent).toContain('Сборка начата');

    expect(screen.getByText('Начать сборку')).toBeDefined();
    expect(screen.getByText('Продолжить сборку')).toBeDefined();
  });

  it('serialized screen visibly shows expected ZMU and guided instructions', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-1',
      orderNumber: '1001',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-1',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-1',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          imageUrl: 'https://example.com/coat.jpg',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'serialized',
          allocatedUnits: [
            {
              inventoryUnitId: 'u-1',
              unitCode: 'ZMU-DEV-COAT-001',
              pickedAt: null,
            },
          ],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockOrder);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-1']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('guided-picking-target')).toBeDefined();
    });

    expect(screen.getByText('СЕЙЧАС НУЖНО ВЗЯТЬ')).toBeDefined();
    expect(screen.getAllByText('Dev Wool Coat').length).toBeGreaterThan(0);
    expect(screen.getAllByText('M · Graphite').length).toBeGreaterThan(0);
    expect(screen.getByText(/ПОДХОДЯЩИХ ЕДИНИЦ НА СКЛАДЕ/)).toBeDefined();
    expect(screen.getByText('Возьмите любую свободную единицу')).toBeDefined();
    expect(screen.getByText('Возьмите любую свободную единицу этого варианта и отсканируйте её ZMU.')).toBeDefined();
    expect(screen.getByTestId('view-compatible-units-btn')).toBeDefined();
  });

  it('legacy item does NOT show a fake ZMU and shows barcode/sku prompt', async () => {
    const mockLegacyOrder: pickingApi.PickingOrder = {
      orderId: 'ord-2',
      orderNumber: '1002',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-2',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-2',
          title: 'Basic Cotton Sock',
          productVariantId: 'var-2',
          variantSize: 'L',
          variantColor: 'White',
          sku: 'SKU-SOCK-WHT-L',
          barcode: '4607001234567',
          quantity: 2,
          pickedQuantity: 0,
          remainingQuantity: 2,
          allocationMode: 'legacy',
          allocatedUnits: [],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockLegacyOrder);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-2']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('guided-picking-target')).toBeDefined();
    });

    expect(screen.getAllByText('Basic Cotton Sock').length).toBeGreaterThan(0);
    expect(screen.getByText('ОЖИДАЕМЫЙ ШТРИХКОД (ZMK)')).toBeDefined();
    expect(screen.getByTestId('expected-unit-code').textContent).toBe('4607001234567');
    expect(screen.queryByText(/ZMU-/)).toBeNull();
    expect(screen.getByText('Отсканируйте штрихкод товара')).toBeDefined();
  });

  it('wrong scan stays on current task and displays mapped human error message', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-1',
      orderNumber: '1001',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-1',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-1',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'serialized',
          allocatedUnits: [
            {
              inventoryUnitId: 'u-1',
              unitCode: 'ZMU-DEV-COAT-001',
              pickedAt: null,
            },
          ],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockOrder);
    vi.spyOn(pickingApi, 'scanPickingCode').mockRejectedValue(
      new ApiError('Unit is not allocated to this fulfillment', 'unit_not_allocated_to_fulfillment', 409)
    );

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-1']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Возьмите любую свободную единицу')).toBeDefined();
    });

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU подходящей единицы/);
    fireEvent.change(input, { target: { value: 'ZMU-WRONG-999' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(screen.getByTestId('scan-feedback-banner')).toBeDefined();
      expect(screen.getByText('Эта единица не относится к текущей сборке')).toBeDefined();
    });

    // Stays on same target
    expect(screen.getByText('Возьмите любую свободную единицу')).toBeDefined();
  });

  it('correct scan advances to the next unfinished item', async () => {
    const initialOrder: pickingApi.PickingOrder = {
      orderId: 'ord-1',
      orderNumber: '1001',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-1',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-1',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'serialized',
          allocatedUnits: [
            {
              inventoryUnitId: 'u-1',
              unitCode: 'ZMU-DEV-COAT-001',
              pickedAt: null,
            },
          ],
        },
        {
          orderItemId: 'oi-2',
          title: 'Leather Belt',
          productVariantId: 'var-2',
          variantSize: '95',
          variantColor: 'Black',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'serialized',
          allocatedUnits: [
            {
              inventoryUnitId: 'u-2',
              unitCode: 'ZMU-BELT-002',
              pickedAt: null,
            },
          ],
        },
      ],
    };

    const updatedOrder: pickingApi.PickingOrder = {
      ...initialOrder,
      fulfillmentStatus: 'assembling',
      items: [
        {
          ...initialOrder.items[0],
          pickedQuantity: 1,
          remainingQuantity: 0,
          allocatedUnits: [
            {
              inventoryUnitId: 'u-1',
              unitCode: 'ZMU-DEV-COAT-001',
              pickedAt: '2026-09-02T10:05:00Z',
            },
          ],
        },
        initialOrder.items[1],
      ],
    };

    const getOrderSpy = vi.spyOn(pickingApi, 'getAdminPickingOrder');
    getOrderSpy.mockResolvedValueOnce(initialOrder).mockResolvedValueOnce(updatedOrder);

    vi.spyOn(pickingApi, 'scanPickingCode').mockResolvedValue({
      fulfillmentId: 'fulf-1',
      orderId: 'ord-1',
      scanResult: {
        code: 'ZMU-DEV-COAT-001',
        type: 'serialized',
        orderItemId: 'oi-1',
        newlyPicked: true,
        alreadyPicked: false,
        alreadyComplete: false,
      },
      item: {
        quantity: 1,
        pickedQuantity: 1,
        remainingQuantity: 0,
        allocationMode: 'serialized',
      },
      fulfillmentProgress: {
        totalQuantity: 2,
        pickedQuantity: 1,
        remainingQuantity: 1,
        isComplete: false,
      },
    });

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-1']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText('Dev Wool Coat').length).toBeGreaterThan(0);
      expect(screen.getByText('Возьмите любую свободную единицу')).toBeDefined();
    });

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU подходящей единицы/);
    fireEvent.change(input, { target: { value: 'ZMU-DEV-COAT-001' } });
    fireEvent.submit(input.closest('form')!);

    // Automatically advances to next unfinished item
    await waitFor(() => {
      expect(screen.getAllByText('Leather Belt').length).toBeGreaterThan(0);
      expect(screen.getByText('Возьмите любую свободную единицу')).toBeDefined();
    });
  });

  it('canonical queue filtering excludes 1/1 completed and terminal orders', async () => {
    const mixedQueue: pickingApi.PickingQueueItem[] = [
      {
        fulfillmentId: 'fulf-active',
        orderId: 'ord-active',
        orderNumber: '5001',
        status: 'paid',
        orderStatus: 'paid',
        createdAt: '2026-09-02T10:00:00Z',
        itemPositionsCount: 1,
        totalQuantity: 2,
        pickedQuantity: 1,
        remainingQuantity: 1,
        progressPercent: 50,
        isComplete: false,
      },
    ];

    vi.spyOn(pickingApi, 'getAdminPickingQueue').mockResolvedValue(mixedQueue);

    render(
      <MemoryRouter>
        <AdminPickingQueue />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Заказ #5001')).toBeDefined();
    });

    // Verify only the actionable order is rendered
    expect(screen.queryByText('Заказ #9999')).toBeNull();
  });

  it('legacy picking displays real accepted ZMK barcode when available', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-100',
      orderNumber: '100194',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-100',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-legacy-zmk',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          sku: 'DEV-SKU-0',
          barcode: 'ZMK-DEV-0001',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'legacy',
          allocatedUnits: [],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockOrder);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-100']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ОЖИДАЕМЫЙ ШТРИХКОД (ZMK)')).toBeDefined();
      expect(screen.getByTestId('expected-unit-code').textContent).toBe('ZMK-DEV-0001');
      expect(screen.getByText('Отсканируйте штрихкод товара')).toBeDefined();
    });
  });

  it('legacy picking with placeholder barcode (000000000000) falls back to real SKU', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-101',
      orderNumber: '100194',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-101',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-placeholder-barcode',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          sku: 'DEV-SKU-0',
          barcode: '000000000000',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'legacy',
          allocatedUnits: [],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockOrder);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-101']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      // Must NOT display 000000000000
      expect(screen.queryByText('000000000000')).toBeNull();
      // Must display real SKU
      expect(screen.getByText('ОЖИДАЕМЫЙ АРТИКУЛ (SKU)')).toBeDefined();
      expect(screen.getByTestId('expected-unit-code').textContent).toBe('DEV-SKU-0');
      expect(screen.getByText('Отсканируйте артикул (SKU) товара')).toBeDefined();
    });
  });

  it('legacy picking with placeholder barcode and absent SKU shows operational exception and disables input', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-102',
      orderNumber: '100195',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-102',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-no-code',
          title: 'Mystery Item',
          productVariantId: 'var-none',
          barcode: '000000000000',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'legacy',
          allocatedUnits: [],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockOrder);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-102']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      // Must NOT display 000000000000
      expect(screen.queryByText('000000000000')).toBeNull();
      // Must display operational exception
      expect(screen.getByTestId('expected-unit-code').textContent).toBe('У позиции нет сканируемого складского кода');
      expect(screen.getByText(/У товара отсутствует штрихкод и артикул для сканирования/)).toBeDefined();
    });

    const input = screen.getByPlaceholderText('Сканирование недоступно: нет кода позиции') as HTMLInputElement;
    expect(input.disabled).toBe(true);
    const submitBtn = screen.getByRole('button', { name: 'Ввод' }) as HTMLButtonElement;
    expect(submitBtn.disabled).toBe(true);
  });

  it('opens drawer and displays compatible units list with allocated and free badges', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-1',
      orderNumber: '1001',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-1',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-1',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          quantity: 1,
          pickedQuantity: 0,
          remainingQuantity: 1,
          allocationMode: 'serialized',
          compatibleUnitsCount: 2,
          allocatedUnits: [
            {
              inventoryUnitId: 'u-1',
              unitCode: 'ZMU-DEV-COAT-001',
              pickedAt: null,
            },
          ],
        },
      ],
    };

    const mockUnits: pickingApi.CompatibleUnit[] = [
      {
        inventoryUnitId: 'u-1',
        unitCode: 'ZMU-DEV-COAT-001',
        productVariantId: 'var-1',
        availability: 'allocated_to_current_item',
      },
      {
        inventoryUnitId: 'u-2',
        unitCode: 'ZMU-DEV-COAT-002',
        productVariantId: 'var-1',
        availability: 'free',
      },
    ];

    vi.spyOn(pickingApi, 'getAdminPickingOrder').mockResolvedValue(mockOrder);
    vi.spyOn(pickingApi, 'getCompatibleUnits').mockResolvedValue(mockUnits);

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-1']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('view-compatible-units-btn')).toBeDefined();
    });

    fireEvent.click(screen.getByTestId('view-compatible-units-btn'));

    await waitFor(() => {
      const drawer = screen.getByTestId('compatible-units-drawer');
      expect(drawer).toBeDefined();
      expect(screen.getAllByText('ZMU-DEV-COAT-001').length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('Назначена этому заказу')).toBeDefined();
      expect(screen.getByText('ZMU-DEV-COAT-002')).toBeDefined();
      expect(screen.getByText('Свободна')).toBeDefined();
    });

    // Close drawer
    fireEvent.click(screen.getByTestId('close-compatible-units-btn'));
    expect(screen.queryByTestId('compatible-units-drawer')).toBeNull();
  });

  it('scanning free compatible ZMU shows substitution human message and advances', async () => {
    const mockOrder: pickingApi.PickingOrder = {
      orderId: 'ord-1',
      orderNumber: '1001',
      orderStatus: 'paid',
      fulfillmentId: 'fulf-1',
      fulfillmentStatus: 'paid',
      items: [
        {
          orderItemId: 'oi-1',
          title: 'Dev Wool Coat',
          productVariantId: 'var-1',
          variantSize: 'M',
          variantColor: 'Graphite',
          quantity: 2,
          pickedQuantity: 0,
          remainingQuantity: 2,
          allocationMode: 'serialized',
          compatibleUnitsCount: 3,
          allocatedUnits: [
            {
              inventoryUnitId: 'u-1',
              unitCode: 'ZMU-DEV-COAT-001',
              pickedAt: null,
            },
            {
              inventoryUnitId: 'u-2',
              unitCode: 'ZMU-DEV-COAT-002',
              pickedAt: null,
            },
          ],
        },
      ],
    };

    const updatedOrder: pickingApi.PickingOrder = {
      ...mockOrder,
      items: [
        {
          ...mockOrder.items[0],
          pickedQuantity: 1,
          remainingQuantity: 1,
          allocatedUnits: [
            {
              inventoryUnitId: 'u-3',
              unitCode: 'ZMU-DEV-COAT-FREE-888',
              pickedAt: '2026-09-02T10:00:00Z',
            },
            {
              inventoryUnitId: 'u-2',
              unitCode: 'ZMU-DEV-COAT-002',
              pickedAt: null,
            },
          ],
        },
      ],
    };

    vi.spyOn(pickingApi, 'getAdminPickingOrder')
      .mockResolvedValueOnce(mockOrder)
      .mockResolvedValueOnce(updatedOrder);

    vi.spyOn(pickingApi, 'scanPickingCode').mockResolvedValue({
      fulfillmentId: 'fulf-1',
      orderId: 'ord-1',
      scanResult: {
        code: 'ZMU-DEV-COAT-FREE-888',
        type: 'serialized',
        orderItemId: 'oi-1',
        newlyPicked: true,
        alreadyPicked: false,
        alreadyComplete: false,
        substituted: true,
      },
      item: {
        quantity: 2,
        pickedQuantity: 1,
        remainingQuantity: 1,
        allocationMode: 'serialized',
      },
      fulfillmentProgress: {
        totalQuantity: 2,
        pickedQuantity: 1,
        remainingQuantity: 1,
        isComplete: false,
      },
    });

    render(
      <MemoryRouter initialEntries={['/fulfillment/picking/fulf-1']}>
        <Routes>
          <Route path="/fulfillment/picking/:id" element={<AdminPickingDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Возьмите любую свободную единицу')).toBeDefined();
    });

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU подходящей единицы/);
    fireEvent.change(input, { target: { value: 'ZMU-DEV-COAT-FREE-888' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(screen.getByTestId('scan-feedback-banner')).toBeDefined();
      expect(screen.getByText('Единица выбрана и добавлена в сборку')).toBeDefined();
    });
  });
});
