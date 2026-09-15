import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AdminSupplyReceiving } from './AdminSupplyReceiving';
import * as adminApi from '@zamk/api-client/src/admin';

vi.mock('@zamk/api-client/src/admin', () => ({
  lookupSupplyByCode: vi.fn(),
  markSupplyArrived: vi.fn(),
  startSupplyReceivingSession: vi.fn(),
  recordSupplyReceivingScan: vi.fn(),
  recordSerializedReceivingScan: vi.fn(),
  getSerializedReceivingScans: vi.fn().mockResolvedValue([]),
  undoSerializedReceivingScan: vi.fn(),
  finalizeSupplyReceivingSession: vi.fn(),
  getSupplyReceivingQueue: vi.fn().mockResolvedValue([]),
}));

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: () => ({
    hasPermission: () => true,
    user: null,
    isAuthenticated: true,
    isLoading: false,
    staff: null,
  }),
}));

vi.mock('../utils/audio', () => ({
  playBeepSound: vi.fn(),
}));

describe('AdminSupplyReceiving Layout-Independent Normalization (SCN.1C)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('normalizes Russian-layout supply QR/code during lookup', async () => {
    vi.mocked(adminApi.lookupSupplyByCode).mockResolvedValueOnce({
      id: 'sup-1',
      supplyNumber: 'SUP-2026-00001',
      status: 'arrived_at_zamk',
      totalExpectedItems: 1,
      totalExpectedBoxes: 1,
      sellerName: 'Test Seller',
      items: [],
      boxes: [],
    } as any);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/SUP-номер|Номер SUP/i) as HTMLInputElement;
    // SUP-2026-00001 in RU layout -> ЫГЗ-2026-00001
    fireEvent.change(input, { target: { value: 'ЫГЗ-2026-00001' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(adminApi.lookupSupplyByCode).toHaveBeenCalledWith('SUP-2026-00001');
    });
  });

  it('I & K: submits canonical normalized ZMU from RU-layout input during serialized receiving', async () => {
    // Setup active session directly
    vi.mocked(adminApi.lookupSupplyByCode).mockResolvedValueOnce({
      id: 'sup-1',
      supplyNumber: 'SUP-2026-00001',
      status: 'arrived_at_zamk',
      totalExpectedItems: 1,
      totalExpectedBoxes: 1,
      sellerName: 'Test Seller',
      items: [],
      boxes: [],
    } as any);

    vi.mocked(adminApi.startSupplyReceivingSession).mockResolvedValueOnce({
      id: 'sess-1',
      supplyId: 'sup-1',
      status: 'active',
      receivingMode: 'serialized',
      items: [
        {
          id: 'item-1',
          variantId: 'var-1',
          sku: 'SKU-001',
          barcode: 'ZMK-001',
          productTitle: 'Sneakers',
          expectedQuantity: 1,
          scannedQuantity: 0,
          damagedQuantity: 0,
        },
      ],
    } as any);

    vi.mocked(adminApi.recordSerializedReceivingScan).mockResolvedValueOnce({
      scanId: 'scan-1',
      unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
      productVariantId: 'var-1',
      productTitle: 'Sneakers',
      condition: 'ok',
      scannedAt: new Date().toISOString(),
    } as any);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    // 1. Lookup supply
    const lookupInput = screen.getByPlaceholderText(/SUP-номер|Номер SUP/i);
    fireEvent.change(lookupInput, { target: { value: 'SUP-2026-00001' } });
    fireEvent.submit(lookupInput.closest('form')!);

    await waitFor(() => {
      expect(screen.getByText('Начать приёмку')).toBeDefined();
    });

    // 2. Start session
    fireEvent.click(screen.getByText('Начать приёмку'));

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Сканируйте ZMU товара\.\.\./i)).toBeDefined();
    });

    // 3. Scan ZMU with Russian keyboard layout:
    const scanInput = screen.getByPlaceholderText(/Сканируйте ZMU товара\.\.\./i);
    // Physical scanner keystrokes in RU layout
    fireEvent.change(scanInput, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });
    // Hardware scanner sends Enter key automatically
    fireEvent.submit(scanInput.closest('form')!);

    await waitFor(() => {
      expect(adminApi.recordSerializedReceivingScan).toHaveBeenCalledWith('sess-1', {
        unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
        condition: 'ok',
      });
    });
  });

  it('L: clicking arrow submit button normalizes identical to Enter', async () => {
    vi.mocked(adminApi.lookupSupplyByCode).mockResolvedValueOnce({
      id: 'sup-1',
      supplyNumber: 'SUP-2026-00001',
      status: 'arrived_at_zamk',
      totalExpectedItems: 1,
      totalExpectedBoxes: 1,
      sellerName: 'Test Seller',
      items: [],
      boxes: [],
    } as any);

    vi.mocked(adminApi.startSupplyReceivingSession).mockResolvedValueOnce({
      id: 'sess-1',
      supplyId: 'sup-1',
      status: 'active',
      receivingMode: 'serialized',
      items: [
        {
          id: 'item-1',
          variantId: 'var-1',
          sku: 'SKU-001',
          barcode: 'ZMK-001',
          productTitle: 'Sneakers',
          expectedQuantity: 1,
          scannedQuantity: 0,
          damagedQuantity: 0,
        },
      ],
    } as any);

    vi.mocked(adminApi.recordSerializedReceivingScan).mockResolvedValueOnce({
      scanId: 'scan-1',
      unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
      productVariantId: 'var-1',
      productTitle: 'Sneakers',
      condition: 'ok',
      scannedAt: new Date().toISOString(),
    } as any);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    const lookupInput = screen.getByPlaceholderText(/SUP-номер|Номер SUP/i);
    fireEvent.change(lookupInput, { target: { value: 'SUP-2026-00001' } });
    fireEvent.submit(lookupInput.closest('form')!);

    await waitFor(() => {
      expect(screen.getByText('Начать приёмку')).toBeDefined();
    });

    fireEvent.click(screen.getByText('Начать приёмку'));

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Сканируйте ZMU товара\.\.\./i)).toBeDefined();
    });

    const scanInput = screen.getByPlaceholderText(/Сканируйте ZMU товара\.\.\./i);
    fireEvent.change(scanInput, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });

    // Click submit button in scan form
    const submitBtn = scanInput.closest('form')!.querySelector('button[type="submit"]')!;
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(adminApi.recordSerializedReceivingScan).toHaveBeenCalledWith('sess-1', {
        unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
        condition: 'ok',
      });
    });
  });
});

describe('AdminSupplyReceiving Queue (WH.3)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders receiving queue items with status badges and KPIs', async () => {
    vi.mocked(adminApi.getSupplyReceivingQueue).mockResolvedValueOnce([
      {
        supplyId: 'sup-1',
        supplyNumber: 'SUP-2026-00001',
        status: 'arrived_at_zamk',
        sellerId: 'seller-1',
        sellerName: 'Nike Store',
        expectedUnitsCount: 10,
        acceptedUnitsCount: 0,
        remainingUnitsCount: 10,
        cargoPlacesCount: 2,
        arrivedAt: '2026-03-30T10:00:00Z',
      },
      {
        supplyId: 'sup-2',
        supplyNumber: 'SUP-2026-00002',
        status: 'receiving',
        sellerId: 'seller-2',
        sellerName: 'Adidas Shop',
        expectedUnitsCount: 15,
        acceptedUnitsCount: 5,
        remainingUnitsCount: 10,
        cargoPlacesCount: 3,
        receivingStartedAt: '2026-03-30T11:00:00Z',
        activeReceivingSessionId: 'sess-2',
      },
    ]);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    // Header & Queue title
    expect(screen.getByText('Ожидают приёмки')).toBeDefined();

    // Await queue render
    await waitFor(() => {
      expect(screen.getByText('SUP-2026-00001')).toBeDefined();
      expect(screen.getByText('SUP-2026-00002')).toBeDefined();
    });

    // Verify badges
    expect(screen.getByText('Ожидает приёмки')).toBeDefined();
    expect(screen.getAllByText('Приёмка начата').length).toBeGreaterThanOrEqual(1);

    // Verify sellers
    expect(screen.getByText('Nike Store')).toBeDefined();
    expect(screen.getByText('Adidas Shop')).toBeDefined();

    // Verify action buttons
    expect(screen.getByText('Начать приёмку')).toBeDefined();
    expect(screen.getByText('Продолжить приёмку')).toBeDefined();
  });

  it('clicking "Начать приёмку" on arrived supply starts session and enters scanning screen', async () => {
    vi.mocked(adminApi.getSupplyReceivingQueue).mockResolvedValueOnce([
      {
        supplyId: 'sup-1',
        supplyNumber: 'SUP-2026-00001',
        status: 'arrived_at_zamk',
        sellerId: 'seller-1',
        sellerName: 'Nike Store',
        expectedUnitsCount: 5,
        acceptedUnitsCount: 0,
        remainingUnitsCount: 5,
        cargoPlacesCount: 1,
      },
    ]);

    vi.mocked(adminApi.lookupSupplyByCode).mockResolvedValueOnce({
      id: 'sup-1',
      supplyNumber: 'SUP-2026-00001',
      status: 'arrived_at_zamk',
      totalExpectedItems: 5,
      totalExpectedBoxes: 1,
      sellerName: 'Nike Store',
      items: [],
      boxes: [],
    } as any);

    vi.mocked(adminApi.startSupplyReceivingSession).mockResolvedValueOnce({
      id: 'sess-1',
      supplyId: 'sup-1',
      status: 'active',
      receivingMode: 'serialized',
      items: [
        {
          id: 'item-1',
          variantId: 'var-1',
          sku: 'SKU-001',
          barcode: 'ZMK-001',
          productTitle: 'Sneakers',
          expectedQuantity: 5,
          scannedQuantity: 0,
          damagedQuantity: 0,
        },
      ],
    } as any);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Начать приёмку')).toBeDefined();
    });

    fireEvent.click(screen.getByText('Начать приёмку'));

    await waitFor(() => {
      expect(adminApi.lookupSupplyByCode).toHaveBeenCalledWith('SUP-2026-00001');
      expect(adminApi.startSupplyReceivingSession).toHaveBeenCalledWith('SUP-2026-00001');
      expect(screen.getByPlaceholderText(/Сканируйте ZMU товара\.\.\./i)).toBeDefined();
    });

    // Test back button
    const backBtn = screen.getAllByText('К очереди приёмки')[0];
    expect(backBtn).toBeDefined();
    fireEvent.click(backBtn);

    await waitFor(() => {
      expect(screen.getByText('Ожидают приёмки')).toBeDefined();
    });
  });

  it('clicking "Продолжить приёмку" on receiving supply resumes session and enters scanning screen', async () => {
    vi.mocked(adminApi.getSupplyReceivingQueue).mockResolvedValueOnce([
      {
        supplyId: 'sup-2',
        supplyNumber: 'SUP-2026-00002',
        status: 'receiving',
        sellerId: 'seller-2',
        sellerName: 'Adidas Shop',
        expectedUnitsCount: 15,
        acceptedUnitsCount: 5,
        remainingUnitsCount: 10,
        cargoPlacesCount: 3,
        activeReceivingSessionId: 'sess-2',
      },
    ]);

    vi.mocked(adminApi.lookupSupplyByCode).mockResolvedValueOnce({
      id: 'sup-2',
      supplyNumber: 'SUP-2026-00002',
      status: 'receiving',
      totalExpectedItems: 15,
      totalExpectedBoxes: 3,
      sellerName: 'Adidas Shop',
      items: [],
      boxes: [],
    } as any);

    vi.mocked(adminApi.startSupplyReceivingSession).mockResolvedValueOnce({
      id: 'sess-2',
      supplyId: 'sup-2',
      status: 'active',
      receivingMode: 'serialized',
      items: [
        {
          id: 'item-2',
          variantId: 'var-2',
          sku: 'SKU-002',
          barcode: 'ZMK-002',
          productTitle: 'Cap',
          expectedQuantity: 15,
          scannedQuantity: 5,
          damagedQuantity: 0,
        },
      ],
    } as any);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Продолжить приёмку')).toBeDefined();
    });

    fireEvent.click(screen.getByText('Продолжить приёмку'));

    await waitFor(() => {
      expect(adminApi.lookupSupplyByCode).toHaveBeenCalledWith('SUP-2026-00002');
      expect(adminApi.startSupplyReceivingSession).toHaveBeenCalledWith('SUP-2026-00002');
      expect(screen.getByPlaceholderText(/Сканируйте ZMU товара\.\.\./i)).toBeDefined();
    });
  });

  it('renders empty state when queue is empty', async () => {
    vi.mocked(adminApi.getSupplyReceivingQueue).mockResolvedValueOnce([]);

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Нет поставок, ожидающих приёмки')).toBeDefined();
    });
  });

  it('renders queue error with retry button and keeps search usable', async () => {
    vi.mocked(adminApi.getSupplyReceivingQueue).mockRejectedValueOnce(
      new Error('Не удалось загрузить очередь приёмки')
    );

    render(
      <MemoryRouter initialEntries={['/supply-receiving']}>
        <AdminSupplyReceiving />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Не удалось загрузить очередь приёмки')).toBeDefined();
      expect(screen.getByText('Повторить')).toBeDefined();
    });

    // Search remains usable
    const searchInput = screen.getByPlaceholderText(/SUP-номер|Номер SUP/i);
    expect(searchInput).toBeDefined();

    // Click retry
    vi.mocked(adminApi.getSupplyReceivingQueue).mockResolvedValueOnce([]);
    fireEvent.click(screen.getByText('Повторить'));

    await waitFor(() => {
      expect(adminApi.getSupplyReceivingQueue).toHaveBeenCalledTimes(2);
    });
  });
});
