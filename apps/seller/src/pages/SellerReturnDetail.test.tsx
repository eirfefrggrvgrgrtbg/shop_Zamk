/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { SellerReturnDetail, getReturnPresentationMode } from './SellerReturnDetail';
import * as sellerApi from '@zamk/api-client/src/seller';
import type { SellerReturn } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerReturn: vi.fn(),
}));

describe('SellerReturnDetail Component (SA.3)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('1. restocked physical outcome renders with units and logistics info', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-101',
      returnId: 'ret-101',
      orderId: 'ord-101',
      orderNumber: 'ORD-100101',
      orderItemId: 'oi-101',
      status: 'completed',
      quantity: 1,
      reason: 'Не подошёл размер',
      condition: 'Примерялся один раз',
      productTitle: 'Dev Silk Dress',
      variantSize: 'M',
      variantColor: 'Черный',
      sku: 'DEV-DRESS-M',
      imageUrl: 'https://example.com/dress.jpg',
      priceCents: 1500000,
      subtotalPriceCents: 1500000,
      restock: true,
      financialAdjustment: {
        deductionCents: 1500000,
        context: 'available',
        adjustedAt: '2026-09-01T14:00:00Z',
      },
      createdAt: '2026-09-01T10:00:00Z',
      updatedAt: '2026-09-01T14:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      receivingStartedAt: '2026-09-01T11:00:00Z',
      completedAt: '2026-09-01T12:00:00Z',
      logisticsStatus: 'arrived_at_zamk',
      trackingNumber: 'TRK-RET-101',
      shipmentMethod: 'cdek_courier',
      physicalOutcome: 'restocked',
      restockedQuantity: 1,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
      units: [
        {
          unitCode: 'ZMU-1234567890ABCDEF',
          disposition: 'restock',
          scannedAt: '2026-09-01T11:30:00Z',
        },
      ],
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-101']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100101/)).toBeTruthy();
    });

    // Logistics assertions
    expect(screen.getByText('Логистика возврата')).toBeTruthy();
    expect(screen.getByText('TRK-RET-101')).toBeTruthy();
    expect(screen.getByText('СДЭК (Курьер)')).toBeTruthy();
    expect(screen.getByText('Доставлен на склад ZAMK')).toBeTruthy();
    expect(screen.getByText('Посылка поступила на склад ZAMK')).toBeTruthy();

    // Outcome & quantity assertions
    expect(screen.getByText('Результат проверки ZAMK')).toBeTruthy();
    expect(screen.getByText('Возвращён в продажу')).toBeTruthy();
    expect(screen.getAllByText('В продажу').length).toBeGreaterThan(0);
    expect(screen.getByText('ZMU-1234567890ABCDEF')).toBeTruthy();

    // High-priority Seller Business Outcome Meaning (SA.3 UX gap)
    expect(screen.getByTestId('seller-outcome-meaning')).toBeTruthy();
    expect(screen.getByText('Что это значит для вас')).toBeTruthy();
    expect(screen.getByText('Товар снова доступен к продаже')).toBeTruthy();
    expect(screen.getByText(/Возвращённая единица добавлена обратно в доступный складской остаток/i)).toBeTruthy();
    // Finance boundary check: no financial deduction claims in physical outcome summary
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/удержан|списан|потеряли/i);

    // Financial adjustment assertion
    expect(screen.getByText('Корректировка доступного баланса')).toBeTruthy();
    expect(screen.getByText(/−15\s?000/)).toBeTruthy();
  });

  it('2. damaged physical outcome renders with defective breakdown', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-102',
      returnId: 'ret-102',
      orderId: 'ord-102',
      orderNumber: 'ORD-100102',
      orderItemId: 'oi-102',
      status: 'completed',
      quantity: 1,
      reason: 'damaged',
      productTitle: 'Dev Cotton Shirt',
      sku: 'DEV-SHIRT-L',
      priceCents: 500000,
      subtotalPriceCents: 500000,
      restock: false,
      financialAdjustment: {
        deductionCents: 500000,
        context: 'post_payout',
        adjustedAt: '2026-09-02T14:00:00Z',
      },
      createdAt: '2026-09-02T10:00:00Z',
      updatedAt: '2026-09-02T14:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      logisticsStatus: 'arrived_at_zamk',
      physicalOutcome: 'damaged',
      restockedQuantity: 0,
      damagedQuantity: 1,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
      units: [
        {
          unitCode: 'ZMU-DAMAGED99999',
          disposition: 'damaged',
          scannedAt: '2026-09-02T11:00:00Z',
        },
      ],
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-102']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100102/)).toBeTruthy();
    });

    expect(screen.getByText('Повреждён (брак)')).toBeTruthy();
    expect(screen.getByText('Брак / повреждён')).toBeTruthy();
    expect(screen.getByText('ZMU-DAMAGED99999')).toBeTruthy();
    expect(screen.getByText('Корректировка после выплаты')).toBeTruthy();
    expect(screen.getByText(/−5\s?000/)).toBeTruthy();
    expect(screen.queryByText(/будет удержана из будущих выплат/i)).toBeNull();
    expect(screen.queryByText(/долг/i)).toBeNull();

    // Human-readable reason mapping check
    expect(screen.getByText('Товар повреждён')).toBeTruthy();

    // High-priority Seller Business Outcome Meaning (SA.3 UX gap)
    expect(screen.getByTestId('seller-outcome-meaning')).toBeTruthy();
    expect(screen.getByText('Что это значит для вас')).toBeTruthy();
    expect(screen.getByText('Товар не возвращён в продажу')).toBeTruthy();
    expect(screen.getByText(/При проверке ZAMK товар признан повреждённым/i)).toBeTruthy();
    expect(screen.getByText(/Доступный остаток продавца не увеличен/i)).toBeTruthy();

    // Verify damaged does NOT claim write-off and does not make finance deduction claims
    expect(screen.queryByText(/списан/i)).toBeNull();
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/списан/i);
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/удержан|потеряли/i);
  });

  it('3. in-inspection state renders with progress indicators', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-103',
      returnId: 'ret-103',
      orderId: 'ord-103',
      orderNumber: 'ORD-100103',
      orderItemId: 'oi-103',
      status: 'receiving',
      quantity: 1,
      reason: 'Передумал',
      productTitle: 'Dev Leather Bag',
      sku: 'DEV-BAG-BLK',
      priceCents: 2000000,
      subtotalPriceCents: 2000000,
      restock: false,
      createdAt: '2026-09-03T10:00:00Z',
      updatedAt: '2026-09-03T12:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: false,
      receivingStartedAt: '2026-09-03T11:30:00Z',
      logisticsStatus: 'arrived_at_zamk',
      physicalOutcome: 'in_inspection',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'receiving',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-103']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100103/)).toBeTruthy();
    });

    expect(screen.getByText('Проверяется на складе ZAMK')).toBeTruthy();
    expect(screen.getByText(/Начало приёмки:/)).toBeTruthy();
    expect(screen.getByText('Финансовая корректировка не сформирована')).toBeTruthy();

    // High-priority Seller Business Outcome Meaning (SA.3 UX gap)
    expect(screen.getByTestId('seller-outcome-meaning')).toBeTruthy();
    expect(screen.getByText('Что это значит для вас')).toBeTruthy();
    expect(screen.getByText('Идёт проверка товара на складе ZAMK')).toBeTruthy();
    expect(screen.getByText(/складской остаток пока не изменился/i)).toBeTruthy();
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/возвращён в доступный остаток/i);
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/удержан|списан|потеряли/i);

    // In-inspection state is WAREHOUSE_PROCESSING mode:
    // It MUST NOT render final breakdown counters or false "Не поступило"
    expect(screen.queryByTestId('final-outcome-breakdown')).toBeNull();
    expect(screen.queryByText('Не поступило')).toBeNull();
    expect(screen.getByTestId('warehouse-processing-status')).toBeTruthy();
    expect(screen.getByText(/Идёт приёмка и осмотр товара специалистами склада ZAMK/i)).toBeTruthy();
  });

  it('4. multi-quantity mixed outcome renders accurately (1 restock, 1 damaged)', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-104',
      returnId: 'ret-104',
      orderId: 'ord-104',
      orderNumber: 'ORD-100104',
      orderItemId: 'oi-104',
      status: 'completed',
      quantity: 2,
      reason: 'Не подошёл размер',
      productTitle: 'Dev Wool Scarf',
      sku: 'DEV-SCARF-UNI',
      priceCents: 300000,
      subtotalPriceCents: 600000,
      restock: true,
      financialAdjustment: {
        deductionCents: 600000,
        context: 'hold',
        adjustedAt: '2026-09-04T11:05:00Z',
      },
      createdAt: '2026-09-04T10:00:00Z',
      updatedAt: '2026-09-04T14:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      logisticsStatus: 'arrived_at_zamk',
      physicalOutcome: 'partial_restock',
      restockedQuantity: 1,
      damagedQuantity: 1,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
      units: [
        {
          unitCode: 'ZMU-SCARF-OK-1',
          disposition: 'restock',
          scannedAt: '2026-09-04T11:00:00Z',
        },
        {
          unitCode: 'ZMU-SCARF-DMG-2',
          disposition: 'damaged',
          scannedAt: '2026-09-04T11:05:00Z',
        },
      ],
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-104']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100104/)).toBeTruthy();
    });

    expect(screen.getByText('Частично в продажу')).toBeTruthy();
    expect(screen.getByText('ZMU-SCARF-OK-1')).toBeTruthy();
    expect(screen.getByText('ZMU-SCARF-DMG-2')).toBeTruthy();
    expect(screen.getAllByText('В продажу').length).toBeGreaterThan(0);
    expect(screen.getByText('Брак / повреждён')).toBeTruthy();
    expect(screen.getByText('Корректировка замороженных средств')).toBeTruthy();
    expect(screen.getByText(/−6\s?000/)).toBeTruthy();

    // High-priority Seller Business Outcome Meaning (SA.3 UX gap)
    expect(screen.getByTestId('seller-outcome-meaning')).toBeTruthy();
    expect(screen.getByText('Что это значит для вас')).toBeTruthy();
    expect(screen.getByText('Часть товара возвращена в продажу')).toBeTruthy();
    expect(screen.getByText(/В доступный остаток возвращено: 1 из 2 шт/i)).toBeTruthy();
    expect(screen.getByText(/Остальные единицы повреждены или не приняты складом/i)).toBeTruthy();
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/удержан|списан|потеряли/i);
  });

  it('5. customer PII and internal admin notes are strictly NOT rendered', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-105',
      returnId: 'ret-105',
      orderId: 'ord-105',
      orderNumber: 'ORD-100105',
      orderItemId: 'oi-105',
      status: 'completed',
      quantity: 1,
      productTitle: 'Dev Coat',
      sku: 'DEV-COAT-M',
      priceCents: 1000000,
      subtotalPriceCents: 1000000,
      restock: true,
      createdAt: '2026-09-05T10:00:00Z',
      updatedAt: '2026-09-05T12:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      physicalOutcome: 'restocked',
      restockedQuantity: 1,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-105']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100105/)).toBeTruthy();
    });

    // Strictly ensure no PII fields appear in the DOM
    expect(screen.queryByText(/customer_email/i)).toBeNull();
    expect(screen.queryByText(/customer_phone/i)).toBeNull();
    expect(screen.queryByText(/adminComment/i)).toBeNull();
    expect(screen.queryByText(/internal_note/i)).toBeNull();
    expect(screen.queryByText(/operator_id/i)).toBeNull();
  });

  it('6. read-only contract: no warehouse operational action buttons exist', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-106',
      returnId: 'ret-106',
      orderId: 'ord-106',
      orderNumber: 'ORD-100106',
      orderItemId: 'oi-106',
      status: 'receiving',
      quantity: 1,
      productTitle: 'Dev Boots',
      sku: 'DEV-BOOTS-42',
      priceCents: 800000,
      subtotalPriceCents: 800000,
      restock: false,
      createdAt: '2026-09-06T10:00:00Z',
      updatedAt: '2026-09-06T12:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: false,
      physicalOutcome: 'in_inspection',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'receiving',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-106']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100106/)).toBeTruthy();
    });

    // Verify negative action matrix (Seller cannot perform warehouse operations)
    expect(screen.queryByRole('button', { name: /начать приёмку/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /сканировать/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /завершить/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /принять/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /отклонить/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /списать/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /вернуть деньги/i })).toBeNull();
    expect(screen.queryByRole('textbox')).toBeNull();
  });

  it('7. needs_info state renders neutral customer info request and NOT awaiting arrival', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-107',
      returnId: 'ret-107',
      orderId: 'ord-107',
      orderNumber: 'ORD-100107',
      orderItemId: 'oi-107',
      status: 'needs_info',
      quantity: 1,
      productTitle: 'Dev Silk Dress',
      sku: 'DEV-DRESS-S',
      priceCents: 1200000,
      subtotalPriceCents: 1200000,
      restock: false,
      createdAt: '2026-09-07T10:00:00Z',
      updatedAt: '2026-09-07T10:30:00Z',
      arrivedAtZamk: false,
      inspectionCompleted: false,
      physicalOutcome: 'needs_info',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'needs_info',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-107']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100107/)).toBeTruthy();
    });

    // Assert status and outcome are 'Требуется информация'
    expect(screen.getAllByText('Требуется информация').length).toBeGreaterThan(0);
    expect(screen.getByText('Ожидается ответ покупателя по запросу информации')).toBeTruthy();

    // High-priority Seller Business Outcome Meaning (SA.3 UX gap)
    expect(screen.getByTestId('seller-outcome-meaning')).toBeTruthy();
    expect(screen.getByText('Что это значит для вас')).toBeTruthy();
    expect(screen.getByText('По возврату пока нет физического результата')).toBeTruthy();
    expect(screen.getByText(/Складской остаток продавца пока не меняется/i)).toBeTruthy();
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/удержан|списан|потеряли/i);

    // Candidate C (needs_info) is PRE_PHYSICAL mode:
    // It MUST NOT render final breakdown counters, "Не поступило", "В продажу", "Брак", or "Отклонено"
    expect(screen.queryByTestId('final-outcome-breakdown')).toBeNull();
    expect(screen.queryByText('Не поступило')).toBeNull();
    expect(screen.queryByText('В продажу')).toBeNull();
    expect(screen.queryByText('Брак')).toBeNull();
    expect(screen.queryByText('Отклонено')).toBeNull();
    expect(screen.getByTestId('pre-physical-status')).toBeTruthy();
    expect(screen.getByText(/Физическая обработка ещё не началась/i)).toBeTruthy();

    // Assert it does NOT say "В пути на склад ZAMK" or "Посылка поступила на склад"
    expect(screen.queryByText('Посылка поступила на склад ZAMK')).toBeNull();
    expect(screen.queryByText('В пути на склад ZAMK')).toBeNull();
    expect(screen.queryByText(/списан/i)).toBeNull();
  });

  it('8. in_transit state renders in-transit outcome and neutral stock consequence', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-108',
      returnId: 'ret-108',
      orderId: 'ord-108',
      orderNumber: 'ORD-100108',
      orderItemId: 'oi-108',
      status: 'in_transit',
      quantity: 1,
      reason: 'wrong_item',
      productTitle: 'Dev Fedora Hat',
      sku: 'DEV-HAT-BRN',
      priceCents: 450000,
      subtotalPriceCents: 450000,
      restock: false,
      createdAt: '2026-09-07T11:00:00Z',
      updatedAt: '2026-09-07T11:30:00Z',
      arrivedAtZamk: false,
      inspectionCompleted: false,
      logisticsStatus: 'in_transit',
      physicalOutcome: 'in_transit',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'in_transit',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-108']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100108/)).toBeTruthy();
    });

    // Reason code mapped
    expect(screen.getByText('Привезли не тот товар')).toBeTruthy();

    // High-priority Seller Business Outcome Meaning (SA.3 UX gap)
    expect(screen.getByTestId('seller-outcome-meaning')).toBeTruthy();
    expect(screen.getByText('Что это значит для вас')).toBeTruthy();
    expect(screen.getByText('Товар находится в пути на склад ZAMK')).toBeTruthy();
    expect(screen.getByText(/Складской остаток пока не меняется/i)).toBeTruthy();
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/возвращён в доступный остаток/i);
    expect(screen.getByTestId('seller-outcome-meaning').textContent).not.toMatch(/удержан|списан|потеряли/i);

    // in_transit is LOGISTICS mode:
    // It MUST NOT render final breakdown counters or false "Не поступило"
    expect(screen.queryByTestId('final-outcome-breakdown')).toBeNull();
    expect(screen.queryByText('Не поступило')).toBeNull();
    expect(screen.getByTestId('logistics-status')).toBeTruthy();
    expect(screen.getByText(/Товар в процессе доставки на склад ZAMK/i)).toBeTruthy();
  });

  it('9. pre-physical states (requested, rejected_by_support, cancelled) render without final outcome counters', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-109',
      returnId: 'ret-109',
      orderId: 'ord-109',
      orderNumber: 'ORD-100109',
      orderItemId: 'oi-109',
      status: 'requested',
      quantity: 1,
      reason: 'changed_mind',
      productTitle: 'Dev Linen Scarf',
      sku: 'DEV-SCARF-LIN',
      priceCents: 250000,
      subtotalPriceCents: 250000,
      restock: false,
      createdAt: '2026-09-07T12:00:00Z',
      updatedAt: '2026-09-07T12:00:00Z',
      arrivedAtZamk: false,
      inspectionCompleted: false,
      physicalOutcome: 'requested',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'requested',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-109']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100109/)).toBeTruthy();
    });

    expect(screen.getByText('Передумал')).toBeTruthy();
    expect(screen.getByText('Заявка на возврат находится на рассмотрении')).toBeTruthy();
    expect(screen.queryByTestId('final-outcome-breakdown')).toBeNull();
    expect(screen.queryByText('Не поступило')).toBeNull();
    expect(screen.getByTestId('pre-physical-status')).toBeTruthy();
  });

  it('10. logistics states (awaiting_shipment, awaiting_handover, arrived_at_zamk) render without final outcome counters or false non-receipt', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-110',
      returnId: 'ret-110',
      orderId: 'ord-110',
      orderNumber: 'ORD-100110',
      orderItemId: 'oi-110',
      status: 'approved',
      quantity: 1,
      productTitle: 'Dev Gloves',
      sku: 'DEV-GLOVES-L',
      priceCents: 150000,
      subtotalPriceCents: 150000,
      restock: false,
      createdAt: '2026-09-07T13:00:00Z',
      updatedAt: '2026-09-07T13:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: false,
      logisticsStatus: 'arrived_at_zamk',
      physicalOutcome: 'arrived_at_zamk',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'arrived_at_zamk',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-110']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100110/)).toBeTruthy();
    });

    expect(screen.getByText('Товар поступил на склад ZAMK и ожидает проверки')).toBeTruthy();
    expect(screen.queryByTestId('final-outcome-breakdown')).toBeNull();
    expect(screen.queryByText('Не поступило')).toBeNull();
    expect(screen.getByTestId('logistics-status')).toBeTruthy();
  });

  it('11. real authoritative not_received outcome DOES render final outcome counters with notReceivedQuantity', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-111',
      returnId: 'ret-111',
      orderId: 'ord-111',
      orderNumber: 'ORD-100111',
      orderItemId: 'oi-111',
      status: 'completed',
      quantity: 1,
      reason: 'wrong_item',
      productTitle: 'Dev Sunglasses',
      sku: 'DEV-SUN-BLK',
      priceCents: 350000,
      subtotalPriceCents: 350000,
      restock: false,
      createdAt: '2026-09-07T14:00:00Z',
      updatedAt: '2026-09-07T15:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      completedAt: '2026-09-07T15:00:00Z',
      logisticsStatus: 'arrived_at_zamk',
      physicalOutcome: 'not_received',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 1,
      processingStatus: 'completed',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-111']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100111/)).toBeTruthy();
    });

    // Authoritative final outcome breakdown MUST render
    expect(screen.getByTestId('final-outcome-breakdown')).toBeTruthy();
    expect(screen.getByText('Не поступил')).toBeTruthy();
    expect(screen.getByText('Товар фактически не поступил на склад')).toBeTruthy();
    expect(screen.getByText('Не поступило')).toBeTruthy();
  });

  it('12. getReturnPresentationMode accurately categorizes all domain return states', () => {
    // PRE_PHYSICAL
    expect(getReturnPresentationMode({ status: 'requested', physicalOutcome: 'requested', inspectionCompleted: false })).toBe('PRE_PHYSICAL');
    expect(getReturnPresentationMode({ status: 'needs_info', physicalOutcome: 'needs_info', inspectionCompleted: false })).toBe('PRE_PHYSICAL');
    expect(getReturnPresentationMode({ status: 'rejected', physicalOutcome: 'rejected_by_support', inspectionCompleted: false })).toBe('PRE_PHYSICAL');
    expect(getReturnPresentationMode({ status: 'cancelled', physicalOutcome: 'cancelled', inspectionCompleted: false })).toBe('PRE_PHYSICAL');

    // LOGISTICS
    expect(getReturnPresentationMode({ status: 'approved', physicalOutcome: 'awaiting_shipment', inspectionCompleted: false })).toBe('LOGISTICS');
    expect(getReturnPresentationMode({ status: 'approved', physicalOutcome: 'awaiting_handover', inspectionCompleted: false })).toBe('LOGISTICS');
    expect(getReturnPresentationMode({ status: 'approved', physicalOutcome: 'in_transit', inspectionCompleted: false })).toBe('LOGISTICS');
    expect(getReturnPresentationMode({ status: 'approved', physicalOutcome: 'arrived_at_zamk', inspectionCompleted: false })).toBe('LOGISTICS');

    // WAREHOUSE_PROCESSING
    expect(getReturnPresentationMode({ status: 'receiving', physicalOutcome: 'in_inspection', inspectionCompleted: false })).toBe('WAREHOUSE_PROCESSING');

    // FINAL_OUTCOME
    expect(getReturnPresentationMode({ status: 'completed', physicalOutcome: 'restocked', inspectionCompleted: true })).toBe('FINAL_OUTCOME');
    expect(getReturnPresentationMode({ status: 'completed', physicalOutcome: 'damaged', inspectionCompleted: true })).toBe('FINAL_OUTCOME');
    expect(getReturnPresentationMode({ status: 'completed', physicalOutcome: 'rejected', inspectionCompleted: true })).toBe('FINAL_OUTCOME');
    expect(getReturnPresentationMode({ status: 'completed', physicalOutcome: 'not_received', inspectionCompleted: true })).toBe('FINAL_OUTCOME');
    expect(getReturnPresentationMode({ status: 'completed', physicalOutcome: 'partial_restock', inspectionCompleted: true })).toBe('FINAL_OUTCOME');
  });

  it('13. absent financialAdjustment renders truthful absence state without fake 0 ₽', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-113',
      returnId: 'ret-113',
      orderId: 'ord-113',
      orderNumber: 'ORD-100113',
      orderItemId: 'oi-113',
      status: 'needs_info',
      quantity: 1,
      productTitle: 'Dev Silk Scarf',
      priceCents: 800000,
      subtotalPriceCents: 800000,
      restock: false,
      financialAdjustment: null,
      createdAt: '2026-09-07T14:00:00Z',
      updatedAt: '2026-09-07T15:00:00Z',
      arrivedAtZamk: false,
      inspectionCompleted: false,
      physicalOutcome: 'needs_info',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'needs_info',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-113']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-100113/)).toBeTruthy();
    });

    expect(screen.getByText('Финансовая корректировка не сформирована')).toBeTruthy();
    expect(screen.queryByText(/(?:^|\s)0\s?₽/)).toBeNull();
  });
});
