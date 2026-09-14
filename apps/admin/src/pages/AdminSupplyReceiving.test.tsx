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

    const input = screen.getByPlaceholderText(/Номер SUP-\.\.\./i) as HTMLInputElement;
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
    const lookupInput = screen.getByPlaceholderText(/Номер SUP-\.\.\./i);
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

    const lookupInput = screen.getByPlaceholderText(/Номер SUP-\.\.\./i);
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
