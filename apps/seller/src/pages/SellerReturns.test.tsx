import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerReturns } from './SellerReturns';
import * as sellerApi from '@zamk/api-client/src/seller';
import type { SellerReturn } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerReturns: vi.fn(),
}));

describe('SellerReturns List View - Financial Truth', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders truthful "Финансовая корректировка" column, "—" when absent, and exact deduction when formed', async () => {
    const mockReturns: SellerReturn[] = [
      {
        returnItemId: 'item-1',
        returnId: 'ret-1',
        orderId: 'ord-1',
        orderNumber: 'ORD-1001',
        orderItemId: 'oi-1',
        status: 'needs_info',
        quantity: 1,
        productTitle: 'Dev Silk Dress',
        priceCents: 1299000,
        subtotalPriceCents: 1299000,
        restock: false,
        financialAdjustment: null, // absent
        createdAt: '2026-09-01T10:00:00Z',
        updatedAt: '2026-09-01T14:00:00Z',
        arrivedAtZamk: false,
        inspectionCompleted: false,
        physicalOutcome: 'needs_info',
        restockedQuantity: 0,
        damagedQuantity: 0,
        rejectedQuantity: 0,
        notReceivedQuantity: 0,
        processingStatus: 'needs_info',
      },
      {
        returnItemId: 'item-2',
        returnId: 'ret-2',
        orderId: 'ord-2',
        orderNumber: 'ORD-1002',
        orderItemId: 'oi-2',
        status: 'refunded',
        quantity: 1,
        productTitle: 'Dev Wool Coat',
        priceCents: 1500000,
        subtotalPriceCents: 1500000,
        restock: true,
        financialAdjustment: {
          deductionCents: 1350000, // 13 500 ₽ deduction (net earning)
          context: 'hold',
          adjustedAt: '2026-09-02T12:00:00Z',
          grossCents: 1500000,
          commissionCents: 150000,
          sellerEarningCents: 1350000,
        },
        createdAt: '2026-09-02T10:00:00Z',
        updatedAt: '2026-09-02T14:00:00Z',
        arrivedAtZamk: true,
        inspectionCompleted: true,
        physicalOutcome: 'restocked',
        restockedQuantity: 1,
        damagedQuantity: 0,
        rejectedQuantity: 0,
        notReceivedQuantity: 0,
        processingStatus: 'completed',
      },
    ];

    vi.mocked(sellerApi.getSellerReturns).mockResolvedValue({
      items: mockReturns,
      totalCount: 2,
    });

    render(
      <MemoryRouter>
        <SellerReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-1001/)).toBeTruthy();
    });

    // 1. Column header must be "Финансы", NEVER "Сумма возврата" and NEVER "Финансовая корректировка"
    expect(screen.getByText('Финансы')).toBeTruthy();
    expect(screen.queryByText('Сумма возврата')).toBeNull();
    expect(screen.queryByText('Финансовая корректировка')).toBeNull();

    // 2. Row 1 (absent adjustment): shows "—", NOT "0 ₽", and NOT subtotalPriceCents (12 990 ₽)
    expect(screen.getByText('—')).toBeTruthy();
    expect(screen.queryByText(/(?:^|\s)0\s?₽/)).toBeNull();

    // 3. Row 2 (formed adjustment): shows exact deduction −13 500 ₽ and label "Отмена дохода"
    expect(screen.getByText(/−13\s?500/)).toBeTruthy();
    expect(screen.getByText('Отмена дохода')).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();

    // 4. Gross subtotalPriceCents (15 000 ₽ or 12 990 ₽) must NOT be shown as financial adjustment
    expect(screen.queryByText(/12\s?990\s?₽/)).toBeNull();
    expect(screen.queryByText(/15\s?000\s?₽/)).toBeNull();
  });

  it('renders compensation badges when present without amounts', async () => {
    const mockReturns: SellerReturn[] = [
      {
        returnItemId: 'item-comp-1',
        returnId: 'ret-comp-1',
        orderId: 'ord-comp-1',
        orderNumber: 'ORD-2001',
        orderItemId: 'oi-comp-1',
        status: 'refunded',
        quantity: 1,
        productTitle: 'Dev Silk Dress',
        priceCents: 1000000,
        subtotalPriceCents: 1000000,
        restock: false,
        financialAdjustment: {
          deductionCents: 900000,
          context: 'available',
          adjustedAt: '2026-09-02T12:00:00Z',
          grossCents: 1000000,
          commissionCents: 100000,
          sellerEarningCents: 900000,
        },
        compensation: {
          status: 'credited',
          responsibleParty: 'zamk',
          reasonCode: 'zamk_warehouse_damage',
        },
        createdAt: '2026-09-02T10:00:00Z',
        updatedAt: '2026-09-02T14:00:00Z',
        arrivedAtZamk: true,
        inspectionCompleted: true,
        physicalOutcome: 'damaged',
        restockedQuantity: 0,
        damagedQuantity: 1,
        rejectedQuantity: 0,
        notReceivedQuantity: 0,
        processingStatus: 'completed',
      },
      {
        returnItemId: 'item-comp-2',
        returnId: 'ret-comp-2',
        orderId: 'ord-comp-2',
        orderNumber: 'ORD-2002',
        orderItemId: 'oi-comp-2',
        status: 'refunded',
        quantity: 1,
        productTitle: 'Dev Wool Coat',
        priceCents: 1500000,
        subtotalPriceCents: 1500000,
        restock: false,
        financialAdjustment: {
          deductionCents: 1350000,
          context: 'hold',
          adjustedAt: '2026-09-03T12:00:00Z',
          grossCents: 1500000,
          commissionCents: 150000,
          sellerEarningCents: 1350000,
        },
        compensation: {
          status: 'pending',
        },
        createdAt: '2026-09-03T10:00:00Z',
        updatedAt: '2026-09-03T14:00:00Z',
        arrivedAtZamk: true,
        inspectionCompleted: true,
        physicalOutcome: 'damaged',
        restockedQuantity: 0,
        damagedQuantity: 1,
        rejectedQuantity: 0,
        notReceivedQuantity: 0,
        processingStatus: 'completed',
      },
    ];

    vi.mocked(sellerApi.getSellerReturns).mockResolvedValue({
      items: mockReturns,
      totalCount: 2,
    });

    render(
      <MemoryRouter>
        <SellerReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-2001/)).toBeTruthy();
      expect(screen.getByText(/ORD-2002/)).toBeTruthy();
    });

    // Row 1: credited compensation badge
    expect(screen.getByText('Компенсация ZAMK')).toBeTruthy();

    // Row 2: pending responsibility badge
    expect(screen.getByText('Ответственность определяется')).toBeTruthy();
  });
});
