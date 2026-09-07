import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminReturnReceiving } from './AdminReturnReceiving';
import * as adminReturnsApi from '../api/adminReturns';
import type {
  AdminReturnReceivingState,
  AdminReturnReceivingItem,
} from '@zamk/api-client/src/types';
import { ApiError } from '@zamk/api-client/src/errors';



import { useAdminAuth } from '../contexts/AdminAuthContext';
vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn(),
}));

vi.mock('../utils/audio', () => ({
  playBeepSound: vi.fn(),
}));

const mockSerializedItem: AdminReturnReceivingItem = {
  returnItem: {
    id: 'ri-101',
    returnId: 'ret-100193-id',
    orderItemId: 'oi-wool-coat',
    quantity: 1,
    priceCents: 1299000,
    subtotalPriceCents: 1299000,
  },
  productTitle: 'Dev Wool Coat',
  productImageUrl: 'https://minio.local/media/coat.jpg',
  variantSize: 'M',
  variantColor: 'Graphite',
  sku: 'DEV-COAT-M',
  priceCents: 1299000,
  allocationMode: 'serialized',
  outboundAllocations: [
    {
      id: 'alloc-101',
      unitCode: 'ZMU-DEVCOAT101',
      status: 'shipped',
      unitStatus: 'shipped',
    },
  ],
  scannedUnits: [],
  requestedQuantity: 1,
  scannedQuantity: 0,
  remainingQuantity: 1,
  notReceivedQuantity: 1,
  acceptedQuantity: 0,
  damagedQuantity: 0,
  rejectedQuantity: 0,
  canFinalize: false,
};

const mockLegacyItem: AdminReturnReceivingItem = {
  returnItem: {
    id: 'ri-leg-1',
    returnId: 'ret-100193-id',
    orderItemId: 'oi-leg-item',
    quantity: 5,
    priceCents: 50000,
    subtotalPriceCents: 250000,
  },
  productTitle: 'Dev Basic T-Shirt',
  variantSize: 'L',
  variantColor: 'White',
  sku: 'DEV-TSHIRT-L',
  priceCents: 50000,
  allocationMode: 'legacy',
  outboundAllocations: [],
  scannedUnits: [],
  requestedQuantity: 5,
  scannedQuantity: 0,
  remainingQuantity: 5,
  notReceivedQuantity: 5,
  acceptedQuantity: 0,
  damagedQuantity: 0,
  rejectedQuantity: 0,
  canFinalize: false,
};

const mockReceivingStateApproved: AdminReturnReceivingState = {
  return: {
    id: 'ret-100193-id',
    orderId: 'ord-100193-id',
    orderNumber: 'ORD-100193',
    status: 'approved',
    reason: 'damaged',
    comment: 'Damaged zipper on receipt',
  },
  orderNumber: 'ORD-100193',
  items: [mockSerializedItem],
  totalRequested: 1,
  totalScanned: 0,
  totalRemaining: 1,
  serializedRequested: 1,
  serializedScanned: 0,
  legacyRequested: 0,
  canFinalize: false,
};

const mockReceivingStateActive: AdminReturnReceivingState = {
  ...mockReceivingStateApproved,
  return: {
    ...mockReceivingStateApproved.return,
    status: 'receiving',
    receivingStartedAt: '2026-08-31T11:00:00Z',
  },
};

describe('AdminReturnReceiving Component & Flows', () => {
  beforeEach(() => {
    (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
      hasPermission: (_p: string) => true,
      hasAnyPermission: (_ps: string[]) => true,
    });
    vi.clearAllMocks();
  });

  it('renders approved state with "Начать приёмку" CTA and triggers startAdminReturnReceiving', async () => {
    const startSpy = vi.spyOn(adminReturnsApi, 'startAdminReturnReceiving').mockResolvedValue(undefined);
    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockImplementation(() => {
      if (startSpy.mock.calls.length > 0) {
        return Promise.resolve(mockReceivingStateActive);
      }
      return Promise.resolve(mockReceivingStateApproved);
    });

    render(
      <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Складская приёмка возврата')).toBeDefined();
    });

    expect(screen.getByText('ORD-100193')).toBeDefined();
    expect(screen.getByText('Dev Wool Coat')).toBeDefined();
    expect(screen.getByText('Graphite')).toBeDefined();
    expect(screen.getByText('DEV-COAT-M')).toBeDefined();
    expect(screen.getByRole('button', { name: /Начать приёмку/i })).toBeDefined();

    // Trigger Start
    fireEvent.click(screen.getByRole('button', { name: /Начать приёмку/i }));

    await waitFor(() => {
      expect(startSpy).toHaveBeenCalledWith('ret-100193-id');
    });

    await waitFor(() => {
      expect(screen.getByText('Сканирование единиц товара (ZMU)')).toBeDefined();
    });
  });

  it('scans ZMU code and handles successful scan with disposition options', async () => {
    const scannedState: AdminReturnReceivingState = {
      ...mockReceivingStateActive,
      items: [
        {
          ...mockSerializedItem,
          scannedUnits: [
            {
              id: 'scan-u-101',
              returnItemId: 'ri-101',
              orderItemAllocationId: 'alloc-101',
              unitCode: 'ZMU-DEVCOAT101',
              scannedAt: '2026-08-31T11:05:00Z',
              createdAt: '2026-08-31T11:05:00Z',
              updatedAt: '2026-08-31T11:05:00Z',
              disposition: undefined,
            },
          ],
          scannedQuantity: 1,
          remainingQuantity: 0,
          notReceivedQuantity: 0,
          canFinalize: false,
        },
      ],
      totalScanned: 1,
      totalRemaining: 0,
      serializedScanned: 1,
      canFinalize: false,
    };

    const scanSpy = vi.spyOn(adminReturnsApi, 'scanAdminReturnUnit').mockResolvedValue({
      scannedUnit: scannedState.items[0].scannedUnits[0],
      item: scannedState.items[0],
      canFinalize: false,
    });

    let fetchCount = 0;
    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockImplementation(() => {
      fetchCount++;
      if (fetchCount > 1) {
        return Promise.resolve(scannedState);
      }
      return Promise.resolve(mockReceivingStateActive);
    });

    render(
      <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX')).toBeDefined();
    });

    const input = screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX');
    fireEvent.change(input, { target: { value: 'ZMU-DEVCOAT101' } });
    fireEvent.click(screen.getByRole('button', { name: /^Сканировать$/i }));

    await waitFor(() => {
      expect(scanSpy).toHaveBeenCalledWith('ret-100193-id', 'ZMU-DEVCOAT101');
    });

    await waitFor(() => {
      expect(screen.getByText('ZMU-DEVCOAT101')).toBeDefined();
      expect(screen.getByText('Требуется решение')).toBeDefined();
      expect(screen.getByRole('button', { name: 'Вернуть в продажу' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Повреждён' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Отклонить возврат' })).toBeDefined();
    });
  });

  it('updates unit disposition to restock and updates condition note', async () => {
    const inspectedState: AdminReturnReceivingState = {
      ...mockReceivingStateActive,
      items: [
        {
          ...mockSerializedItem,
          scannedUnits: [
            {
              id: 'scan-u-101',
              returnItemId: 'ri-101',
              orderItemAllocationId: 'alloc-101',
              unitCode: 'ZMU-DEVCOAT101',
              scannedAt: '2026-08-31T11:05:00Z',
              createdAt: '2026-08-31T11:05:00Z',
              updatedAt: '2026-08-31T11:05:00Z',
              disposition: 'restock',
              inspectedCondition: 'Brand new, tags intact',
            },
          ],
          scannedQuantity: 1,
          remainingQuantity: 0,
          notReceivedQuantity: 0,
          acceptedQuantity: 1,
          canFinalize: true,
        },
      ],
      totalScanned: 1,
      totalRemaining: 0,
      serializedScanned: 1,
      canFinalize: true,
    };

    const inspectSpy = vi.spyOn(adminReturnsApi, 'inspectSerializedReturnUnit').mockResolvedValue(undefined);

    const initialScannedState: AdminReturnReceivingState = {
      ...mockReceivingStateActive,
      items: [
        {
          ...mockSerializedItem,
          scannedUnits: [
            {
              id: 'scan-u-101',
              returnItemId: 'ri-101',
              orderItemAllocationId: 'alloc-101',
              unitCode: 'ZMU-DEVCOAT101',
              scannedAt: '2026-08-31T11:05:00Z',
              createdAt: '2026-08-31T11:05:00Z',
              updatedAt: '2026-08-31T11:05:00Z',
              disposition: undefined,
            },
          ],
          scannedQuantity: 1,
          remainingQuantity: 0,
          notReceivedQuantity: 0,
          canFinalize: false,
        },
      ],
      totalScanned: 1,
      totalRemaining: 0,
      serializedScanned: 1,
      canFinalize: false,
    };

    let fetchCount = 0;
    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockImplementation(() => {
      fetchCount++;
      if (fetchCount > 1) {
        return Promise.resolve(inspectedState);
      }
      return Promise.resolve(initialScannedState);
    });

    render(
      <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Вернуть в продажу' })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Вернуть в продажу' }));

    await waitFor(() => {
      expect(inspectSpy).toHaveBeenCalledWith('ret-100193-id', 'scan-u-101', {
        disposition: 'restock',
        inspectedCondition: undefined,
      });
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Завершить приёмку/i })).toBeDefined();
      expect(screen.getByRole('button', { name: /Завершить приёмку/i }).hasAttribute('disabled')).toBe(false);
    });
  });

  it('finalizes receiving via confirmation modal and updates Return status to item_received without auto-refund', async () => {
    const readyState: AdminReturnReceivingState = {
      ...mockReceivingStateActive,
      items: [
        {
          ...mockSerializedItem,
          scannedUnits: [
            {
              id: 'scan-u-101',
              returnItemId: 'ri-101',
              orderItemAllocationId: 'alloc-101',
              unitCode: 'ZMU-DEVCOAT101',
              scannedAt: '2026-08-31T11:05:00Z',
              createdAt: '2026-08-31T11:05:00Z',
              updatedAt: '2026-08-31T11:05:00Z',
              disposition: 'restock',
            },
          ],
          scannedQuantity: 1,
          remainingQuantity: 0,
          notReceivedQuantity: 0,
          acceptedQuantity: 1,
          canFinalize: true,
        },
      ],
      totalScanned: 1,
      totalRemaining: 0,
      serializedScanned: 1,
      canFinalize: true,
    };

    const finalizedState: AdminReturnReceivingState = {
      ...readyState,
      return: {
        ...readyState.return,
        status: 'item_received',
      },
      canFinalize: false,
    };

    const finalizeSpy = vi.spyOn(adminReturnsApi, 'finalizeAdminReturnReceiving').mockResolvedValue(undefined);
    const refundSpy = vi.spyOn(adminReturnsApi, 'createAdminRefundForReturn');

    let fetchCount = 0;
    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockImplementation(() => {
      fetchCount++;
      if (fetchCount > 1) {
        return Promise.resolve(finalizedState);
      }
      return Promise.resolve(readyState);
    });

    render(
      <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Завершить приёмку/i })).toBeDefined();
    });

    // Click Finalize to open confirmation modal
    fireEvent.click(screen.getByRole('button', { name: /Завершить приёмку/i }));

    expect(screen.getByText('Подтверждение завершения приёмки')).toBeDefined();
    expect(screen.getByText('Вернуть на склад (в продажу):')).toBeDefined();
    expect(screen.getAllByText('1 шт.').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('button', { name: /Подтвердить и завершить/i })).toBeDefined();

    // Confirm Finalize
    fireEvent.click(screen.getByRole('button', { name: /Подтвердить и завершить/i }));

    await waitFor(() => {
      expect(finalizeSpy).toHaveBeenCalledWith('ret-100193-id');
    });

    await waitFor(() => {
      expect(screen.getAllByText('Приёмка завершена').length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('Товар принят')).toBeDefined();
    });

    // CRITICAL: Prove NO automatic refund call was made
    expect(refundSpy).not.toHaveBeenCalled();
  });

  it('supports partial serialized receiving (Q=3, 2 scanned, 1 unscanned) and displays notReceived count in modal', async () => {
    const partialSerializedItem: AdminReturnReceivingItem = {
      returnItem: {
        id: 'ri-part-3',
        returnId: 'ret-part-id',
        orderItemId: 'oi-part-item',
        quantity: 3,
        priceCents: 100000,
        subtotalPriceCents: 300000,
      },
      productTitle: 'Dev Premium Hoodie',
      variantSize: 'L',
      variantColor: 'Black',
      sku: 'DEV-HOODIE-L',
      priceCents: 100000,
      allocationMode: 'serialized',
      outboundAllocations: [
        { id: 'alloc-1', unitCode: 'ZMU-HD-1', status: 'shipped', unitStatus: 'shipped' },
        { id: 'alloc-2', unitCode: 'ZMU-HD-2', status: 'shipped', unitStatus: 'shipped' },
        { id: 'alloc-3', unitCode: 'ZMU-HD-3', status: 'shipped', unitStatus: 'shipped' },
      ],
      scannedUnits: [
        {
          id: 'scan-1',
          returnItemId: 'ri-part-3',
          orderItemAllocationId: 'alloc-1',
          unitCode: 'ZMU-HD-1',
          disposition: 'restock',
          createdAt: '2026-08-31T11:00:00Z',
          updatedAt: '2026-08-31T11:00:00Z',
        },
        {
          id: 'scan-2',
          returnItemId: 'ri-part-3',
          orderItemAllocationId: 'alloc-2',
          unitCode: 'ZMU-HD-2',
          disposition: 'damaged',
          createdAt: '2026-08-31T11:00:00Z',
          updatedAt: '2026-08-31T11:00:00Z',
        },
      ],
      requestedQuantity: 3,
      scannedQuantity: 2,
      remainingQuantity: 1,
      notReceivedQuantity: 1,
      acceptedQuantity: 1,
      damagedQuantity: 1,
      rejectedQuantity: 0,
      canFinalize: true,
    };

    const partialState: AdminReturnReceivingState = {
      return: {
        id: 'ret-part-id',
        orderId: 'ord-part-id',
        orderNumber: 'ORD-PART-300',
        status: 'receiving',
      },
      orderNumber: 'ORD-PART-300',
      items: [partialSerializedItem],
      totalRequested: 3,
      totalScanned: 2,
      totalRemaining: 1,
      serializedRequested: 3,
      serializedScanned: 2,
      legacyRequested: 0,
      canFinalize: true,
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(partialState);
    const finalizeSpy = vi.spyOn(adminReturnsApi, 'finalizeAdminReturnReceiving').mockResolvedValue(undefined);

    render(
      <MemoryRouter initialEntries={['/returns/ret-part-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-PART-300')).toBeDefined();
    });

    // Check finalize with discrepancy button is enabled
    const finalizeBtn = screen.getByRole('button', { name: /Завершить с расхождением/i });
    expect(finalizeBtn.hasAttribute('disabled')).toBe(false);

    // Open modal
    fireEvent.click(finalizeBtn);

    expect(screen.getByText('Подтверждение приёмки с расхождением')).toBeDefined();
    expect(screen.getByText('Вернуть на склад (в продажу):')).toBeDefined();
    expect(screen.getAllByText(/Не получено/i).length).toBeGreaterThanOrEqual(1);

    // Confirm finalize
    fireEvent.click(screen.getByRole('button', { name: /Подтвердить расхождение и завершить/i }));

    await waitFor(() => {
      expect(finalizeSpy).toHaveBeenCalledWith('ret-part-id');
    });
  });

  it('supports zero-received serialized return (Q=1, 0 scanned) with discrepancy notice, NOT ready green completion', async () => {
    const zeroScannedItem: AdminReturnReceivingItem = {
      ...mockSerializedItem,
      scannedUnits: [],
      requestedQuantity: 1,
      scannedQuantity: 0,
      remainingQuantity: 1,
      notReceivedQuantity: 1,
      canFinalize: true,
    };

    const zeroReceivedState: AdminReturnReceivingState = {
      return: {
        id: 'ret-zero-id',
        orderId: 'ord-zero-id',
        orderNumber: 'ORD-ZERO-001',
        status: 'receiving',
      },
      orderNumber: 'ORD-ZERO-001',
      items: [zeroScannedItem],
      totalRequested: 1,
      totalScanned: 0,
      totalRemaining: 1,
      serializedRequested: 1,
      serializedScanned: 0,
      legacyRequested: 0,
      canFinalize: true,
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(zeroReceivedState);
    const finalizeSpy = vi.spyOn(adminReturnsApi, 'finalizeAdminReturnReceiving').mockResolvedValue(undefined);

    render(
      <MemoryRouter initialEntries={['/returns/ret-zero-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-ZERO-001')).toBeDefined();
    });

    // Verify readiness state does NOT advertise normal full completion
    expect(screen.queryByText('Все отсканированные единицы проверены. Вы можете завершить приёмку.')).toBeNull();
    expect(screen.queryByRole('button', { name: /^Завершить приёмку$/i })).toBeNull();

    // Verify zero-scanned notice and explicit discrepancy button
    expect(screen.getByText('Товары ещё не отсканированы')).toBeDefined();
    const discrepancyBtn = screen.getByRole('button', { name: /Завершить без товара \(0 из 1\)/i });
    expect(discrepancyBtn.hasAttribute('disabled')).toBe(false);

    fireEvent.click(discrepancyBtn);

    expect(screen.getByText('Подтверждение приёмки без товара (0 получено)')).toBeDefined();
    expect(screen.getByText(/Внимание: ни один товар не был принят!/i)).toBeDefined();
    expect(screen.getByText(/Возврат денежных средств покупателю будет полностью заблокирован/i)).toBeDefined();
    expect(screen.getAllByText(/Не получено/i).length).toBeGreaterThanOrEqual(1);

    fireEvent.click(screen.getByRole('button', { name: /Подтвердить отсутствие товара и завершить/i }));

    await waitFor(() => {
      expect(finalizeSpy).toHaveBeenCalledWith('ret-zero-id');
    });
  });

  it('regression: matches exact owner screenshot state for ORD-100196 (warehouse_operator, 1 requested, 0 scanned, 1 not received)', async () => {
    const screenshotItem: AdminReturnReceivingItem = {
      ...mockSerializedItem,
      scannedUnits: [],
      requestedQuantity: 1,
      scannedQuantity: 0,
      remainingQuantity: 1,
      notReceivedQuantity: 1,
      canFinalize: true,
    };

    const screenshotState: AdminReturnReceivingState = {
      return: {
        id: 'ret-100196-id',
        orderId: 'ord-100196-id',
        orderNumber: 'ORD-100196',
        status: 'receiving',
        receivingStartedAt: '2026-09-07T06:45:00Z',
        reason: 'damaged',
      },
      orderNumber: 'ORD-100196',
      items: [screenshotItem],
      totalRequested: 1,
      totalScanned: 0,
      totalRemaining: 1,
      serializedRequested: 1,
      serializedScanned: 0,
      legacyRequested: 0,
      canFinalize: true,
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(screenshotState);

    render(
      <MemoryRouter initialEntries={['/returns/ret-100196-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('ORD-100196')).toBeDefined();
    });

    // 1. Meta cards reflect 1 requested, 0 scanned, 1 not received
    expect(screen.getByText('1 шт.')).toBeDefined();
    expect(screen.getByText('0 шт.')).toBeDefined();

    // 2. ZMU input is visible for scanning
    expect(screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX')).toBeDefined();

    // 3. Footer MUST NOT say "Все отсканированные единицы проверены. Вы можете завершить приёмку."
    expect(screen.queryByText('Все отсканированные единицы проверены. Вы можете завершить приёмку.')).toBeNull();

    // 4. Normal green "Завершить приёмку" button MUST NOT be rendered
    expect(screen.queryByRole('button', { name: /^Завершить приёмку$/i })).toBeNull();

    // 5. Explicit zero-scan discrepancy message is displayed
    expect(screen.getByText('Товары ещё не отсканированы')).toBeDefined();
    expect(screen.getByText(/Отсканируйте ZMU-код единицы для приёмки/i)).toBeDefined();

    // 6. Explicit discrepancy button is displayed
    const zeroBtn = screen.getByRole('button', { name: /Завершить без товара \(0 из 1\)/i });
    expect(zeroBtn).toBeDefined();
    expect(zeroBtn.className).toContain('bg-amber-100');
    expect(zeroBtn.className).not.toContain('bg-green-600');
  });

  it('renders scan error message with mapped human text when invalid ZMU is scanned', async () => {
    vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(mockReceivingStateActive);
    const invalidErr = new ApiError('Not found in return', 'invalid_zmu', 400);
    vi.spyOn(adminReturnsApi, 'scanAdminReturnUnit').mockRejectedValue(invalidErr);

    render(
      <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
        <Routes>
          <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX')).toBeDefined();
    });

    const input = screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX');
    fireEvent.change(input, { target: { value: 'ZMU-WRONGCODE' } });
    fireEvent.click(screen.getByRole('button', { name: /^Сканировать$/i }));

    await waitFor(() => {
      expect(screen.getByText('Код ZMU не найден или не принадлежит данному возврату.')).toBeDefined();
    });
  });

  describe('RBAC boundary enforcement — Support vs Warehouse', () => {
    it('enforces strict read-only boundary for Support role (returns.read + returns.update_status, NO warehouse.returns)', async () => {
      // Setup support permissions
      (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
        hasPermission: (p: string) => p === 'returns.read' || p === 'returns.update_status',
        hasAnyPermission: (ps: string[]) => ps.some(p => p === 'returns.read' || p === 'returns.update_status'),
      });

      const scanSpy = vi.spyOn(adminReturnsApi, 'scanAdminReturnUnit');
      const startSpy = vi.spyOn(adminReturnsApi, 'startAdminReturnReceiving');
      const legacySpy = vi.spyOn(adminReturnsApi, 'inspectLegacyReturnItem');
      const unitSpy = vi.spyOn(adminReturnsApi, 'inspectSerializedReturnUnit');

      // State with both serialized & legacy items in receiving status
      const mixedReceivingState: AdminReturnReceivingState = {
        ...mockReceivingStateActive,
        items: [
          mockSerializedItem,
          {
            ...mockLegacyItem,
            returnItem: { ...mockLegacyItem.returnItem, acceptedQuantity: 2, damagedQuantity: 1, rejectedQuantity: 0 },
            acceptedQuantity: 2,
            damagedQuantity: 1,
            rejectedQuantity: 0,
            notReceivedQuantity: 2,
          },
        ],
      };

      vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(mixedReceivingState);

      render(
        <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
          <Routes>
            <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
          </Routes>
        </MemoryRouter>,
      );

      // 1. Prove read model is visible
      await waitFor(() => {
        expect(screen.getByText('ORD-100193')).toBeDefined();
        expect(screen.getByText('Dev Wool Coat')).toBeDefined();
        expect(screen.getByText('Dev Basic T-Shirt')).toBeDefined();
        expect(screen.getByText('Товар повреждён')).toBeDefined();
      });

      // 2. Prove Start Receiving button is unavailable
      expect(screen.queryByRole('button', { name: /Начать приёмку/i })).toBeNull();

      // 3. Prove Scanner is disabled and shows read-only notice
      const scannerInput = screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX') as HTMLInputElement;
      expect(scannerInput.disabled).toBe(true);
      expect(screen.getByText(/Режим только для чтения: у вас нет прав на физическое сканирование на складе./i)).toBeDefined();

      // 4. Prove Enter / form submit on scanner does NOT call scan API
      fireEvent.change(scannerInput, { target: { value: 'ZMU-TEST-CODE' } });
      fireEvent.submit(scannerInput.closest('form')!);
      expect(scanSpy).not.toHaveBeenCalled();

      // 5. Prove unit disposition buttons ("Вернуть в продажу", "Повреждён", "Отклонить возврат") are NOT available
      expect(screen.queryByRole('button', { name: /Вернуть в продажу/i })).toBeNull();
      expect(screen.queryByRole('button', { name: /Повреждён \(Брак\)/i })).toBeNull();
      expect(screen.queryByRole('button', { name: /Отклонить возврат/i })).toBeNull();

      // 6. Prove legacy quantity inputs and save button are NOT available (read-only summary shown)
      expect(screen.queryByRole('button', { name: /Сохранить количества/i })).toBeNull();
      expect(screen.queryByLabelText(/Принято в продажу/i)).toBeNull();
      expect(unitSpy).not.toHaveBeenCalled();
      expect(legacySpy).not.toHaveBeenCalled();
      expect(startSpy).not.toHaveBeenCalled();

      // 7. Prove Finalize button is NOT available
      expect(screen.queryByRole('button', { name: /Завершить приёмку/i })).toBeNull();
    });

    it('enforces Start Receiving is hidden for Support when return is approved', async () => {
      (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
        hasPermission: (p: string) => p === 'returns.read' || p === 'returns.update_status',
        hasAnyPermission: (ps: string[]) => ps.some(p => p === 'returns.read' || p === 'returns.update_status'),
      });

      vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(mockReceivingStateApproved);

      render(
        <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
          <Routes>
            <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
          </Routes>
        </MemoryRouter>,
      );

      await waitFor(() => {
        expect(screen.getByText('ORD-100193')).toBeDefined();
      });

      expect(screen.queryByRole('button', { name: /Начать приёмку/i })).toBeNull();
    });

    it('enables full operational controls for Warehouse role (has warehouse.returns)', async () => {
      (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
        permissions: ['warehouse.returns'],
        isAuthenticated: true,
        isLoading: false,
        error: null,
        hasPermission: (p: string) => p === 'warehouse.returns',
        hasAnyPermission: (ps: string[]) => ps.some(p => p === 'warehouse.returns'),
      });

      vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue(mockReceivingStateActive);

      render(
        <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
          <Routes>
            <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
          </Routes>
        </MemoryRouter>,
      );

      await waitFor(() => {
        expect(screen.getByText('ORD-100193')).toBeDefined();
      });

      // Scanner is active & enabled
      const scannerInput = screen.getByPlaceholderText('Например: ZMU-XUJBQQ5ADSW4BWTX') as HTMLInputElement;
      expect(scannerInput.disabled).toBe(false);

      // Finalize button is visible
      expect(screen.getByRole('button', { name: /Завершить приёмку/i })).toBeDefined();
    });
  });

  describe('Customer Comment Privacy / RBAC in Receiving UI', () => {
    it('hides customer comment block completely for warehouse-only role (has warehouse.returns, no returns.read)', async () => {
      (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
        permissions: ['warehouse.returns'],
        isAuthenticated: true,
        isLoading: false,
        error: null,
        hasPermission: (p: string) => p === 'warehouse.returns',
        hasAnyPermission: (ps: string[]) => ps.some(p => p === 'warehouse.returns'),
      });

      // Even if mock data contained a comment, warehouse UI must never render it
      vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue({
        ...mockReceivingStateApproved,
        return: {
          ...mockReceivingStateApproved.return,
          comment: 'Secret customer feedback',
        },
      });

      render(
        <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
          <Routes>
            <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
          </Routes>
        </MemoryRouter>,
      );

      await waitFor(() => {
        expect(screen.getByText('ORD-100193')).toBeDefined();
      });

      // Customer comment header and text must be completely absent
      expect(screen.queryByText(/Комментарий покупателя/i)).toBeNull();
      expect(screen.queryByText(/Secret customer feedback/i)).toBeNull();
      expect(screen.queryByText(/Комментарий покупателя не указан/i)).toBeNull();

      // Return reason and product details must remain visible
      expect(screen.getByText('Товар повреждён')).toBeDefined();
      expect(screen.getByText('Dev Wool Coat')).toBeDefined();
    });

    it('renders customer comment block for Support role (has returns.read)', async () => {
      (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
        permissions: ['returns.read'],
        isAuthenticated: true,
        isLoading: false,
        error: null,
        hasPermission: (p: string) => p === 'returns.read',
        hasAnyPermission: (ps: string[]) => ps.some(p => p === 'returns.read'),
      });

      vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue({
        ...mockReceivingStateApproved,
        return: {
          ...mockReceivingStateApproved.return,
          comment: 'Visible customer comment for support',
        },
      });

      render(
        <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
          <Routes>
            <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
          </Routes>
        </MemoryRouter>,
      );

      await waitFor(() => {
        expect(screen.getByText('ORD-100193')).toBeDefined();
      });

      expect(screen.getByText(/Комментарий покупателя/i)).toBeDefined();
      expect(screen.getByText('Visible customer comment for support')).toBeDefined();
      expect(screen.getByText('Товар повреждён')).toBeDefined();
    });

    it('renders fallback when customer comment is null for Support role', async () => {
      (useAdminAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
        permissions: ['returns.read'],
        isAuthenticated: true,
        isLoading: false,
        error: null,
        hasPermission: (p: string) => p === 'returns.read',
        hasAnyPermission: (ps: string[]) => ps.some(p => p === 'returns.read'),
      });

      vi.spyOn(adminReturnsApi, 'getAdminReturnReceivingState').mockResolvedValue({
        ...mockReceivingStateApproved,
        return: {
          ...mockReceivingStateApproved.return,
          comment: undefined,
        },
      });

      render(
        <MemoryRouter initialEntries={['/returns/ret-100193-id/receiving']}>
          <Routes>
            <Route path="/returns/:id/receiving" element={<AdminReturnReceiving />} />
          </Routes>
        </MemoryRouter>,
      );

      await waitFor(() => {
        expect(screen.getByText('ORD-100193')).toBeDefined();
      });

      expect(screen.getByText('Комментарий покупателя не указан')).toBeDefined();
    });
  });
});
