import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { VariantInventoryDrawer } from './VariantInventoryDrawer';
import type { AdminInventoryView } from '../../api/adminInventory';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock getAdminInventoryItem to return the item directly
vi.mock('../../api/adminInventory', async () => {
  const actual = await vi.importActual('../../api/adminInventory');
  return {
    ...actual,
    getAdminInventoryItem: vi.fn().mockImplementation((id: string) => {
      if (id === 'legacy-item-id') {
        return Promise.resolve(mockLegacyItem);
      }
      return Promise.resolve(mockWoolCoatItem);
    }),
    getAdminInventoryMovements: vi.fn().mockResolvedValue([
      {
        id: 'mov-1',
        type: 'receipt',
        quantity: 10,
        reason: 'Initial stock',
        createdAt: '2026-08-25T14:00:00Z',
      },
    ]),
    getActiveInventoryReconciliation: vi.fn().mockResolvedValue(null),
    listInventoryReconciliations: vi.fn().mockResolvedValue([]),
    startInventoryReconciliation: vi.fn().mockResolvedValue({ id: 'reconcile-session-1' }),
    getAdminInventoryUnitTraceability: vi.fn().mockImplementation((unitCode: string) => {
      return Promise.resolve({
        identity: {
          id: 'unit-id-1',
          unitCode,
          productTitle: 'Dev Wool Coat',
          variantName: 'M · Graphite',
        },
        currentState: {
          status: 'warehouse',
          availability: 'free',
          location: 'Не ведётся',
          isStaleAllocation: false,
        },
        currentContext: {},
        timeline: [],
        hasPartialHistory: false,
      });
    }),
  };
});

const mockWoolCoatItem: AdminInventoryView = {
  id: '57e77b5f-c67a-37dd-a47d-bb88a2ff17c5',
  productId: '2a6fa985-dae0-39eb-ad87-253f982e84f1',
  productTitle: 'Dev Wool Coat',
  productVariantId: '3b37fd2c-40d7-364b-892d-5ef4e3905afd',
  variant: 'DEV-SKU-0 / M / Graphite',
  sku: 'DEV-SKU-0',
  barcode: 'ZMK-DEV-0001',
  size: 'M',
  color: 'Graphite',
  sellerId: '44444444-4444-4444-8444-444444444444',
  sellerName: 'ZAMK Dev Seller',
  source: 'seller',
  totalStock: 29,
  reservedStock: 2,
  availableStock: 27,
  aggregate: { total: 29, reserved: 2, available: 27 },
  physical: {
    warehouse: 4,
    allocated: 1,
    picked: 0,
    free: 3,
    expected: 1,
    damaged: 1,
    writtenOff: 0,
    shipped: 2,
    staleAllocated: 1,
  },
  legacy: { onHand: 25, reserved: 1, available: 24 },
  accountingMode: 'mixed',
  health: {
    status: 'warning',
    issues: ['stale_active_allocation'],
  },
  physicalUnits: [
    {
      id: '24e27f10-ff3d-4da1-a287-9d8306b3c733',
      unitCode: 'ZMU-BRFEA757ZAMUQYVW',
      status: 'warehouse',
      createdAt: '2026-09-03T15:36:12Z',
      availability: 'free',
      isStaleAllocation: false,
      supplyLineage: {
        supplyId: '810d9299-e2b4-436f-9f10-355df27ac762',
        supplyNumber: 'SUP-001201',
        supplyStatus: 'completed',
        receivedAt: '2026-09-03T15:40:19Z',
      },
    },
    {
      id: '6b8e8083-a4e8-4579-aac2-5d578023090c',
      unitCode: 'ZMU-WJEFXRQDGPYY6JF7',
      status: 'warehouse',
      createdAt: '2026-09-03T15:36:12Z',
      availability: 'allocated',
      isStaleAllocation: false,
      liveAllocation: {
        id: '7bdf248b-ba5c-4381-8bbb-8a267d6fa2c3',
        orderId: 'afa4e753-c7e5-422f-a477-605cd387efc9',
        orderNumber: 'ORD-100196',
        orderStatus: 'paid',
        fulfillmentId: '228aadaf-e051-4255-8cdf-d596195d1d5e',
        fulfillmentStatus: 'paid',
      },
      supplyLineage: {
        supplyId: '810d9299-e2b4-436f-9f10-355df27ac762',
        supplyNumber: 'SUP-001201',
        supplyStatus: 'completed',
        receivedAt: '2026-09-03T15:40:29Z',
      },
    },
    {
      id: 'bfb72d4b-b5c4-40fc-bd69-f9c303a03852',
      unitCode: 'ZMU-XUJBQQ5ADSW4BWTX',
      status: 'warehouse',
      createdAt: '2026-08-25T17:18:01Z',
      availability: 'free',
      isStaleAllocation: true,
      staleAllocation: {
        id: '4df2106f-32d4-4469-bae6-4d6bd22f6da7',
        orderId: 'e94d9db8-60b1-4e06-851a-e96d3490174e',
        orderNumber: 'ORD-100193',
        orderStatus: 'delivered',
        fulfillmentId: '66976e97-a584-4ffb-8f2d-73c8c1565f7e',
        fulfillmentStatus: 'delivered',
        pickedAt: '2026-08-29T20:56:11Z',
      },
      supplyLineage: {
        supplyId: 'a131dc11-c90b-431f-a538-f8f012d03308',
        supplyNumber: 'SUP-001197',
        supplyStatus: 'completed',
        receivedAt: '2026-08-25T17:18:34Z',
      },
    },
    {
      id: '4e164e43-bf8f-4320-a212-5f268160f26c',
      unitCode: 'ZMU-28E9HXCS7C5TZ893',
      status: 'expected',
      createdAt: '2026-08-25T17:18:01Z',
      availability: 'unavailable_expected',
      isStaleAllocation: false,
      supplyLineage: {
        supplyId: 'a131dc11-c90b-431f-a538-f8f012d03308',
        supplyNumber: 'SUP-001197',
        supplyStatus: 'completed',
      },
    },
    {
      id: '6354e6d1-b172-4e78-92ef-81f31df39cdc',
      unitCode: 'ZMU-CQRB7WXP2G6T2YD9',
      status: 'damaged',
      createdAt: '2026-08-25T17:18:01Z',
      availability: 'unavailable_damaged',
      isStaleAllocation: false,
      supplyLineage: {
        supplyId: 'a131dc11-c90b-431f-a538-f8f012d03308',
        supplyNumber: 'SUP-001197',
        supplyStatus: 'completed',
        receivedAt: '2026-08-25T17:18:43Z',
      },
    },
    {
      id: '95ebc828-7dd8-4ecc-afd9-3c40c5e62b68',
      unitCode: 'ZMU-C5MXPTQ7WH8WZYQP',
      status: 'shipped',
      createdAt: '2026-08-25T17:18:01Z',
      availability: 'unavailable_shipped',
      isStaleAllocation: false,
      supplyLineage: {
        supplyId: 'a131dc11-c90b-431f-a538-f8f012d03308',
        supplyNumber: 'SUP-001197',
        supplyStatus: 'completed',
        receivedAt: '2026-08-25T17:18:30Z',
      },
    },
  ],
};

const mockLegacyItem: AdminInventoryView = {
  id: 'legacy-item-id',
  productId: 'legacy-prod-id',
  productTitle: 'Legacy Simple T-Shirt',
  productVariantId: 'legacy-variant-id',
  variant: 'L / White',
  sku: 'LEG-TSHIRT-L',
  barcode: 'ZMK-LEG-0001',
  size: 'L',
  color: 'White',
  sellerId: '44444444-4444-4444-8444-444444444444',
  sellerName: 'ZAMK Dev Seller',
  source: 'seller',
  totalStock: 50,
  reservedStock: 5,
  availableStock: 45,
  aggregate: { total: 50, reserved: 5, available: 45 },
  physical: {
    warehouse: 0,
    allocated: 0,
    picked: 0,
    free: 0,
    expected: 0,
    damaged: 0,
    writtenOff: 0,
    shipped: 0,
    staleAllocated: 0,
  },
  legacy: { onHand: 50, reserved: 5, available: 45 },
  accountingMode: 'legacy',
  health: { status: 'healthy', issues: [] },
  physicalUnits: [],
};

describe('VariantInventoryDrawer', () => {
  it('renders drawer header with Russian accounting labels and tooltips', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    expect(screen.getByText('Dev Wool Coat')).toBeDefined();
    expect(screen.getByText('M · Graphite')).toBeDefined();
    expect(screen.getByText('ZMK: ZMK-DEV-0001')).toBeDefined();
    expect(screen.getByText('SKU: DEV-SKU-0')).toBeDefined();

    // Russian accounting badge with tooltip
    const mixedBadge = screen.getByText('Смешанный');
    expect(mixedBadge).toBeDefined();
    expect(mixedBadge.getAttribute('title')).toBe('Часть остатка учитывается по ZMU, часть — количественно.');

    expect(screen.getByText('Расхождение')).toBeDefined();
  });

  it('renders top balance section correctly', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    expect(screen.getByText('Складской баланс')).toBeDefined();
    expect(screen.getByText('Коммерческий итог')).toBeDefined();
    expect(screen.getByText('Физические ZMU')).toBeDefined();
    expect(screen.getByText('БЕЗ ZMU')).toBeDefined();
    expect(screen.getByText('Вне склада')).toBeDefined();
  });

  it('renders stale allocation diagnostic callout with Russian order status', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    expect(screen.getByText(/Обнаружены расхождения в учёте:/i)).toBeDefined();
    expect(screen.getByText(/Физическая единица содержит активное назначение на завершённый заказ./i)).toBeDefined();
    expect(screen.getByText('ZMU-XUJBQQ5ADSW4BWTX')).toBeDefined();
    expect(screen.getByText('ORD-100193')).toBeDefined();
    // Russian status formatted
    expect(screen.getByText(/Доставлен/i)).toBeDefined();
    // Ensure no raw English (delivered)
    expect(screen.queryByText(/\(delivered\)/i)).toBeNull();
  });

  it('renders 5-column physical units table with combined context and origin', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-BRFEA757ZAMUQYVW')).toBeDefined();
    });

    // 5 Columns verified
    expect(screen.getByText('ZMU')).toBeDefined();
    expect(screen.getByText('Физическое состояние')).toBeDefined();
    expect(screen.getByText('Доступность')).toBeDefined();
    expect(screen.getByText('Текущий контекст')).toBeDefined();
    expect(screen.getByText('Происхождение')).toBeDefined();

    // Live allocated unit with link and combined status
    expect(screen.getByText('ZMU-WJEFXRQDGPYY6JF7')).toBeDefined();
    expect(screen.getByText('ORD-100196')).toBeDefined();
    expect(screen.getByText('Оплачен')).toBeDefined();
    expect(screen.getByText('Ожидает сборки')).toBeDefined();

    // Stale allocated unit
    expect(screen.getByText(/Старое назначение ORD-100193/i)).toBeDefined();
    expect(screen.getByText(/Текущий заказ: —/i)).toBeDefined();
    expect(screen.getByText(/Историческое назначение:/i)).toBeDefined();

    // Origin lineage
    expect(screen.getAllByText('SUP-001201').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(/Принято:/i).length).toBeGreaterThanOrEqual(1);

    // Non-warehouse units
    expect(screen.getByText('ZMU-28E9HXCS7C5TZ893')).toBeDefined();
    expect(screen.getByText('Недоступна — ожидается')).toBeDefined();
    expect(screen.getByText('ZMU-CQRB7WXP2G6T2YD9')).toBeDefined();
    expect(screen.getByText('Недоступна — брак')).toBeDefined();
    expect(screen.getByText('ZMU-C5MXPTQ7WH8WZYQP')).toBeDefined();
    expect(screen.getByText('Не на складе')).toBeDefined();
  });

  it('filters units by "Расхождения" and excludes normal expected/damaged/shipped units', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-BRFEA757ZAMUQYVW')).toBeDefined();
    });

    // Verify "Расхождения (1)" pill exists
    const issuesPill = screen.getByRole('button', { name: /Расхождения \(1\)/i });
    expect(issuesPill).toBeDefined();

    // Click "Расхождения (1)"
    fireEvent.click(issuesPill);

    // Stale unit must be present
    expect(screen.getAllByText('ZMU-XUJBQQ5ADSW4BWTX').length).toBe(2);

    // Normal units must be excluded from Расхождения
    expect(screen.queryByText('ZMU-BRFEA757ZAMUQYVW')).toBeNull(); // warehouse free
    expect(screen.queryByText('ZMU-WJEFXRQDGPYY6JF7')).toBeNull(); // warehouse live allocated
    expect(screen.queryByText('ZMU-28E9HXCS7C5TZ893')).toBeNull(); // expected
    expect(screen.queryByText('ZMU-CQRB7WXP2G6T2YD9')).toBeNull(); // damaged
    expect(screen.queryByText('ZMU-C5MXPTQ7WH8WZYQP')).toBeNull(); // shipped
  });

  it('filters units by local search input', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-BRFEA757ZAMUQYVW')).toBeDefined();
    });

    const searchInput = screen.getByPlaceholderText('Найти ZMU...');
    fireEvent.change(searchInput, { target: { value: 'BRFEA757' } });

    expect(screen.getByText('ZMU-BRFEA757ZAMUQYVW')).toBeDefined();
    expect(screen.queryByText('ZMU-WJEFXRQDGPYY6JF7')).toBeNull();
  });

  it('renders clean legacy empty state with "Без ZMU" badge for pure legacy variants', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockLegacyItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Legacy Simple T-Shirt')).toBeDefined();
    });

    const legacyBadge = screen.getByText('Без ZMU');
    expect(legacyBadge).toBeDefined();
    expect(legacyBadge.getAttribute('title')).toBe('Физические единицы этого остатка отдельно не отслеживаются.');

    expect(
      screen.getByText(/Физические единицы ZMU для этого остатка не ведутся/i)
    ).toBeDefined();
    expect(screen.queryByText('Найти ZMU...')).toBeNull();
  });

  it('switches between "Физические единицы" and "История" tabs', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Dev Wool Coat')).toBeDefined();
    });

    // Both tabs exist
    const unitsTab = screen.getByRole('button', { name: /Физические единицы/i });
    const historyTab = screen.getByRole('button', { name: /История/i });
    expect(unitsTab).toBeDefined();
    expect(historyTab).toBeDefined();

    // Default is units tab
    expect(screen.getByText('ZMU-BRFEA757ZAMUQYVW')).toBeDefined();

    // Switch to history tab
    fireEvent.click(historyTab);

    await waitFor(() => {
      expect(screen.getByText('История движений варианта')).toBeDefined();
    });
    expect(screen.getByText('Initial stock')).toBeDefined();
    expect(screen.getByText('Поступление')).toBeDefined();
  });

  it('opens nested ZMU detail when clicking a ZMU row or code, and auto-opens when highlightUnitCode is set', async () => {
    render(
      <MemoryRouter>
        <VariantInventoryDrawer
          item={mockWoolCoatItem}
          isOpen={true}
          onClose={() => {}}
          highlightUnitCode="ZMU-WJEFXRQDGPYY6JF7"
        />
      </MemoryRouter>
    );

    // Auto-opened due to highlightUnitCode
    await waitFor(() => {
      expect(screen.getByTestId('zmu-traceability-drawer')).toBeDefined();
    });
    expect(screen.getAllByText('ZMU-WJEFXRQDGPYY6JF7').length).toBeGreaterThanOrEqual(1);

    // Close nested drawer
    const backBtn = screen.getByRole('button', { name: /Назад к варианту/i });
    fireEvent.click(backBtn);

    await waitFor(() => {
      expect(screen.queryByTestId('zmu-traceability-drawer')).toBeNull();
    });

    // Clicking a ZMU code opens nested drawer again
    const unitBtn = screen.getByRole('button', { name: 'ZMU-BRFEA757ZAMUQYVW' });
    fireEvent.click(unitBtn);

    await waitFor(() => {
      expect(screen.getByTestId('zmu-traceability-drawer')).toBeDefined();
    });
    expect(screen.getAllByText('ZMU-BRFEA757ZAMUQYVW').length).toBeGreaterThanOrEqual(1);
  });

  it('renders reconciliation section with single contextual CTA and no duplicate in header', async () => {
    mockNavigate.mockClear();
    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Инвентаризации')).toBeDefined();
    });

    expect(screen.getByText('Инвентаризации ещё не проводились')).toBeDefined();
    // Contextual button in reconciliation section exists
    const startBtns = screen.getAllByRole('button', { name: 'Начать инвентаризацию' });
    expect(startBtns).toHaveLength(1);
    // No duplicate buttons in header
    expect(screen.queryByRole('button', { name: /Продолжить инвентаризацию/i })).toBeNull();

    // Physical units tab still renders full width
    expect(screen.getByRole('button', { name: /Физические единицы/i })).toBeDefined();
    expect(screen.getByText('ZMU-BRFEA757ZAMUQYVW')).toBeDefined();
  });

  it('opens confirmation modal first without creating session, and navigates to canonical path upon confirmation', async () => {
    mockNavigate.mockClear();
    const { startInventoryReconciliation } = await import('../../api/adminInventory');
    vi.mocked(startInventoryReconciliation).mockClear();

    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Dev Wool Coat')).toBeDefined();
    });

    // Click "Начать инвентаризацию" in reconciliation section
    const startBtn = screen.getByRole('button', { name: 'Начать инвентаризацию' });
    fireEvent.click(startBtn);

    // Modal opens
    await waitFor(() => {
      expect(screen.getByText('Начать инвентаризацию?')).toBeDefined();
    });

    // Session NOT created prior to confirmation
    expect(startInventoryReconciliation).not.toHaveBeenCalled();

    const modal = screen.getByRole('dialog');
    expect(within(modal).getByText('Dev Wool Coat · M · Graphite')).toBeDefined();
    expect(within(modal).getByText('Физических ZMU на складе:')).toBeDefined();
    expect(within(modal).getByText('4')).toBeDefined();

    // Mixed warning text
    expect(within(modal).getByText(/Проверяются только физические единицы ZMU/i)).toBeDefined();
    expect(within(modal).getByText(/Остаток без ZMU:/i)).toBeDefined();
    expect(within(modal).getByText(/25 шт./i)).toBeDefined();

    // Cancel closes modal without creating session
    const cancelBtn = within(modal).getByRole('button', { name: 'Отмена' });
    fireEvent.click(cancelBtn);

    await waitFor(() => {
      expect(screen.queryByText('Начать инвентаризацию?')).toBeNull();
    });
    expect(startInventoryReconciliation).not.toHaveBeenCalled();

    // Open again and confirm
    fireEvent.click(screen.getByRole('button', { name: 'Начать инвентаризацию' }));
    await waitFor(() => {
      expect(screen.getByText('Начать инвентаризацию?')).toBeDefined();
    });
    const confirmModal = screen.getByRole('dialog');
    const confirmBtn = within(confirmModal).getByRole('button', { name: 'Начать инвентаризацию' });
    fireEvent.click(confirmBtn);

    // Creates session and navigates to canonical route
    await waitFor(() => {
      expect(startInventoryReconciliation).toHaveBeenCalledWith(mockWoolCoatItem.productVariantId);
      expect(mockNavigate).toHaveBeenCalledWith('/inventory/reconciliation/reconcile-session-1');
    });
  });

  it('renders active session row with single "Продолжить" CTA and navigates to canonical path', async () => {
    mockNavigate.mockClear();
    const { getActiveInventoryReconciliation, listInventoryReconciliations } = await import('../../api/adminInventory');
    vi.mocked(getActiveInventoryReconciliation).mockResolvedValueOnce({
      id: 'session-act-1',
      variantId: mockWoolCoatItem.productVariantId!,
      status: 'in_progress',
      startedAt: new Date().toISOString(),
      startedBy: 'admin-1',
      expectedCount: 4,
      foundExpectedCount: 2,
      unexpectedCount: 0,
      problemsCount: 0,
    });
    vi.mocked(listInventoryReconciliations).mockResolvedValueOnce([
      {
        id: 'session-act-1',
        variantId: mockWoolCoatItem.productVariantId!,
        status: 'in_progress',
        startedAt: new Date().toISOString(),
        startedBy: 'admin-1',
        expectedCount: 4,
        foundExpectedCount: 2,
        unexpectedCount: 0,
        problemsCount: 0,
      },
    ]);

    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    // No duplicate CTA in header
    expect(screen.queryByRole('button', { name: /Продолжить инвентаризацию/i })).toBeNull();

    await waitFor(() => {
      expect(screen.getByText('В процессе')).toBeDefined();
    });

    expect(screen.getByText(/2 \/ 4 найдено/i)).toBeDefined();
    const continueBtn = screen.getByRole('button', { name: 'Продолжить' });
    expect(continueBtn).toBeDefined();

    // Click "Продолжить" navigates to canonical route
    fireEvent.click(continueBtn);
    expect(mockNavigate).toHaveBeenCalledWith('/inventory/reconciliation/session-act-1');
  });

  it('renders completed session row with "Открыть" CTA and navigates to canonical path', async () => {
    mockNavigate.mockClear();
    const { getActiveInventoryReconciliation, listInventoryReconciliations } = await import('../../api/adminInventory');
    vi.mocked(getActiveInventoryReconciliation).mockResolvedValueOnce(null);
    vi.mocked(listInventoryReconciliations).mockResolvedValueOnce([
      {
        id: 'session-done-1',
        variantId: mockWoolCoatItem.productVariantId!,
        status: 'completed',
        startedAt: '2026-09-02T12:00:00Z',
        completedAt: '2026-09-02T12:15:00Z',
        startedBy: 'admin-1',
        expectedCount: 4,
        foundExpectedCount: 4,
        unexpectedCount: 0,
        problemsCount: 0,
      },
    ]);

    render(
      <MemoryRouter>
        <VariantInventoryDrawer item={mockWoolCoatItem} isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Завершена')).toBeDefined();
    });

    expect(screen.getByText(/4 \/ 4 найдено/i)).toBeDefined();
    const openBtn = screen.getByRole('button', { name: 'Открыть' });
    expect(openBtn).toBeDefined();

    // Click "Открыть" navigates to canonical route
    fireEvent.click(openBtn);
    expect(mockNavigate).toHaveBeenCalledWith('/inventory/reconciliation/session-done-1');
  });
});
