/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
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
        grossCents: 1500000,
        commissionCents: 0,
        sellerEarningCents: 1500000,
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
    expect(screen.getByText('Из доступного баланса')).toBeTruthy();
    expect(screen.getByText(/−15\s?000/)).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
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
        grossCents: 500000,
        commissionCents: 0,
        sellerEarningCents: 500000,
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
    expect(screen.getByText('После выплаты')).toBeTruthy();
    expect(screen.getByText(/−5\s?000/)).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
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
        grossCents: 600000,
        commissionCents: 0,
        sellerEarningCents: 600000,
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
    expect(screen.getByText('Из замороженной суммы')).toBeTruthy();
    expect(screen.getByText(/−6\s?000/)).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();

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
    expect(screen.queryByRole('button', { name: /почему такая сумма\?/i })).toBeNull();
  });
});

describe('Seller Return Finance Explanation (SA.5.2B)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('Case A: Formed HOLD — toggle collapsed by default, expands with exact kopecks, hold copy, collapses on re-click', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-hold-1',
      returnId: 'ret-hold-1',
      orderId: 'ord-hold-1',
      orderNumber: 'ORD-900201',
      orderItemId: 'oi-hold-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Evening Dress',
      priceCents: 1299000,
      subtotalPriceCents: 1299000,
      restock: true,
      financialAdjustment: {
        deductionCents: 1182090,
        context: 'hold',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 1299000,
        commissionCents: 116910,
        sellerEarningCents: 1182090,
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
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
      <MemoryRouter initialEntries={['/returns/ret-hold-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900201/)).toBeTruthy();
    });

    // Headline and context label
    expect(screen.getByText(/−11\s?821/)).toBeTruthy();
    expect(screen.getByText('Из замороженной суммы')).toBeTruthy();

    // Toggle button is present and collapsed by default
    const toggleButton = screen.getByRole('button', { name: /почему изменилась сумма\?/i });
    expect(toggleButton).toBeTruthy();
    expect(toggleButton.getAttribute('aria-expanded')).toBe('false');
    expect(toggleButton.getAttribute('aria-controls')).toBe('finance-explanation-item-hold-1');

    // Explanation details NOT visible initially
    expect(screen.queryByText('Стоимость возвращённого товара')).toBeNull();

    // Click to expand
    fireEvent.click(toggleButton);
    expect(toggleButton.getAttribute('aria-expanded')).toBe('true');

    // Explanation details visible with exact kopecks
    expect(screen.getByText('Стоимость возвращённого товара')).toBeTruthy();
    expect(screen.getByText(/12\s?990,00\s?₽/)).toBeTruthy();

    expect(screen.getByText('Комиссия ZAMK по исходной продаже')).toBeTruthy();
    expect(screen.getByText(/−1\s?169,10\s?₽/)).toBeTruthy();

    expect(screen.getByText('Доход продавца по исходной продаже')).toBeTruthy();
    expect(screen.getAllByText(/11\s?820,90\s?₽/).length).toBe(2);

    expect(screen.getAllByText('Отмена дохода по продаже').length).toBe(2);
    expect(screen.getByText(/−\s*11\s?820,90\s?₽/)).toBeTruthy();

    // Hold context copy
    expect(
      screen.getByText(/Доход по этой продаже ещё находился на удержании, поэтому отмена уменьшила замороженную сумму\./)
    ).toBeTruthy();

    // Ensure NO visible Сторно
    expect(screen.queryByText(/Сторно/i)).toBeNull();

    // Click to collapse
    fireEvent.click(toggleButton);
    expect(toggleButton.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByText('Стоимость возвращённого товара')).toBeNull();
  });

  it('Case B: Formed AVAILABLE — expands with available copy and no hold/post_payout copy', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-avail-1',
      returnId: 'ret-avail-1',
      orderId: 'ord-avail-1',
      orderNumber: 'ORD-900301',
      orderItemId: 'oi-avail-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Available Dress',
      priceCents: 1299000,
      subtotalPriceCents: 1299000,
      restock: true,
      financialAdjustment: {
        deductionCents: 1182090,
        context: 'available',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 1299000,
        commissionCents: 116910,
        sellerEarningCents: 1182090,
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
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
      <MemoryRouter initialEntries={['/returns/ret-avail-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900301/)).toBeTruthy();
    });

    const toggleButton = screen.getByRole('button', { name: /почему изменилась сумма\?/i });
    fireEvent.click(toggleButton);

    expect(screen.getByText('Отмена уменьшила текущий доступный баланс продавца.')).toBeTruthy();
    expect(screen.queryByText(/14-дневном удержании/)).toBeNull();
    expect(screen.queryByText(/выплачен на ваш расчётный счёт/)).toBeNull();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
  });

  it('Case C: Formed POST_PAYOUT — expands with post_payout copy and strictly no debt words', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-post-1',
      returnId: 'ret-post-1',
      orderId: 'ord-post-1',
      orderNumber: 'ORD-900401',
      orderItemId: 'oi-post-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Post Payout Dress',
      priceCents: 1299000,
      subtotalPriceCents: 1299000,
      restock: true,
      financialAdjustment: {
        deductionCents: 1182090,
        context: 'post_payout',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 1299000,
        commissionCents: 116910,
        sellerEarningCents: 1182090,
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
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
      <MemoryRouter initialEntries={['/returns/ret-post-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900401/)).toBeTruthy();
    });

    const toggleButton = screen.getByRole('button', { name: /почему изменилась сумма\?/i });
    fireEvent.click(toggleButton);

    const postPayoutNote = screen.getByText(
      'Доход по этой продаже уже был выплачен. Отмена отражена в текущем балансе и будет учтена при последующих расчётах.'
    );
    expect(postPayoutNote).toBeTruthy();
    expect(screen.getByText(/Отмена отражена в текущем балансе/)).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();

    // Verify absence of debt words
    const explanationEl = document.getElementById('finance-explanation-item-post-1');
    expect(explanationEl?.textContent).not.toMatch(/долг|задолженност|к погашению/i);

  });

  it('Case D: Absence state — no disclosure toggle button rendered', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-absent-1',
      returnId: 'ret-absent-1',
      orderId: 'ord-absent-1',
      orderNumber: 'ORD-900501',
      orderItemId: 'oi-absent-1',
      status: 'requested',
      quantity: 1,
      productTitle: 'Pending Dress',
      priceCents: 1000000,
      subtotalPriceCents: 1000000,
      restock: false,
      financialAdjustment: null,
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
      arrivedAtZamk: false,
      inspectionCompleted: false,
      physicalOutcome: 'requested',
      restockedQuantity: 0,
      damagedQuantity: 0,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'requested',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-absent-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900501/)).toBeTruthy();
    });

    expect(screen.getByText('Финансовая корректировка не сформирована')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /почему такая сумма\?/i })).toBeNull();
  });

  it('Case E: Exact kopecks formatting — preserves 2 decimals on all breakdown rows', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-kopecks-1',
      returnId: 'ret-kopecks-1',
      orderId: 'ord-kopecks-1',
      orderNumber: 'ORD-900601',
      orderItemId: 'oi-kopecks-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Kopecks Dress',
      priceCents: 1299050,
      subtotalPriceCents: 1299050,
      restock: true,
      financialAdjustment: {
        deductionCents: 1182136,
        context: 'available',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 1299050,
        commissionCents: 116914,
        sellerEarningCents: 1182136,
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
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
      <MemoryRouter initialEntries={['/returns/ret-kopecks-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900601/)).toBeTruthy();
    });

    const toggleButton = screen.getByRole('button', { name: /почему изменилась сумма\?/i });
    fireEvent.click(toggleButton);

    expect(screen.getByText(/12\s?990,50\s?₽/)).toBeTruthy();
    expect(screen.getByText(/−1\s?169,14\s?₽/)).toBeTruthy();
    expect(screen.getAllByText(/11\s?821,36\s?₽/).length).toBe(2);
    expect(screen.getByText(/−\s*11\s?821,36\s?₽/)).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
  });

  it('Case F: Awkward partial-return values — renders backend-provided cents directly without frontend recalculation', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-partial-1',
      returnId: 'ret-partial-1',
      orderId: 'ord-partial-1',
      orderNumber: 'ORD-900701',
      orderItemId: 'oi-partial-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Partial Item 3 of 3',
      priceCents: 233334,
      subtotalPriceCents: 233334,
      restock: true,
      financialAdjustment: {
        deductionCents: 233334,
        context: 'post_payout',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 233334,
        commissionCents: 0,
        sellerEarningCents: 233334,
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
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
      <MemoryRouter initialEntries={['/returns/ret-partial-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900701/)).toBeTruthy();
    });

    const toggleButton = screen.getByRole('button', { name: /почему изменилась сумма\?/i });
    fireEvent.click(toggleButton);

    // Exact 2 333,34 ₽ rendered directly
    expect(screen.getAllByText(/2\s?333,34\s?₽/).length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(/−2\s?333,34\s?₽/)).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
  });

  it('Case G: Contract — strictly requires grossCents, commissionCents, and sellerEarningCents in financialAdjustment type', () => {
    const adj: SellerReturn['financialAdjustment'] = {
      deductionCents: 1000,
      context: 'hold',
      adjustedAt: '2026-09-08T00:00:00Z',
      grossCents: 1200,
      commissionCents: 200,
      sellerEarningCents: 1000,
    };
    expect(adj?.grossCents).toBe(1200);
    expect(adj?.commissionCents).toBe(200);
    expect(adj?.sellerEarningCents).toBe(1000);
  });

  it('Case H: Credited ZAMK Compensation — renders status badge and explanation without invented amount cents or net cents', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-zamk-comp-1',
      returnId: 'ret-zamk-comp-1',
      orderId: 'ord-zamk-comp-1',
      orderNumber: 'ORD-900801',
      orderItemId: 'oi-zamk-comp-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Damaged Dress',
      priceCents: 1000000,
      subtotalPriceCents: 1000000,
      restock: false,
      financialAdjustment: {
        deductionCents: 900000,
        context: 'available',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 1000000,
        commissionCents: 100000,
        sellerEarningCents: 900000,
      },
      compensation: {
        status: 'credited',
        responsibleParty: 'zamk',
        reasonCode: 'zamk_warehouse_damage',
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      physicalOutcome: 'damaged',
      restockedQuantity: 0,
      damagedQuantity: 1,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-zamk-comp-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900801/)).toBeTruthy();
    });

    // Compensation status card
    expect(screen.getByText('Компенсация ZAMK')).toBeTruthy();
    expect(screen.getByText('Учтено при расчёте компенсации')).toBeTruthy();
    expect(
      screen.getByText(/По этому возврату зафиксирована потеря товара по ответственности ZAMK\. Этот возврат учтён в общем расчёте компенсации продавцу\./)
    ).toBeTruthy();
    expect(screen.getByText(/Этот возврат учтён в общем расчёте компенсации продавцу\./)).toBeTruthy();
    expect(screen.queryByText(/Компенсация начислена на ваш баланс/i)).toBeNull();
    expect(screen.queryByText(/возмещено в полном объёме/i)).toBeNull();

    // Truth invariant: No invented compensation money cents or net cents
    expect(screen.queryByText(/Итого к выплате/i)).toBeNull();
    expect(screen.queryByText(/Чистый результат/i)).toBeNull();
    expect(screen.queryByText(/\+9\s?000/)).toBeNull();
    expect(screen.queryByText(/\+10\s?000/)).toBeNull();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
  });

  it('Case I: Credited Carrier Compensation — renders carrier explanation', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-carrier-comp-1',
      returnId: 'ret-carrier-comp-1',
      orderId: 'ord-carrier-comp-1',
      orderNumber: 'ORD-900802',
      orderItemId: 'oi-carrier-comp-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Carrier Damaged Shoes',
      priceCents: 500000,
      subtotalPriceCents: 500000,
      restock: false,
      financialAdjustment: {
        deductionCents: 450000,
        context: 'hold',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 500000,
        commissionCents: 50000,
        sellerEarningCents: 450000,
      },
      compensation: {
        status: 'credited',
        responsibleParty: 'carrier',
        reasonCode: 'carrier_damage',
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      physicalOutcome: 'damaged',
      restockedQuantity: 0,
      damagedQuantity: 1,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-carrier-comp-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900802/)).toBeTruthy();
    });

    expect(screen.getByText('Компенсация ZAMK')).toBeTruthy();
    expect(screen.getByText('Учтено при расчёте компенсации')).toBeTruthy();
    expect(
      screen.getByText(/По этому возврату зафиксировано повреждение при доставке\. Этот возврат учтён в общем расчёте компенсации продавцу\./)
    ).toBeTruthy();
    expect(screen.getByText(/Этот возврат учтён в общем расчёте компенсации продавцу\./)).toBeTruthy();
    expect(screen.queryByText(/Компенсация начислена на ваш баланс/i)).toBeNull();
    expect(screen.queryByText(/возмещено в полном объёме/i)).toBeNull();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
  });

  it('Case J: Pending Responsibility — renders pending banner without financial promises', async () => {
    const mockReturn: SellerReturn = {
      returnItemId: 'item-pending-comp-1',
      returnId: 'ret-pending-comp-1',
      orderId: 'ord-pending-comp-1',
      orderNumber: 'ORD-900803',
      orderItemId: 'oi-pending-comp-1',
      status: 'completed',
      quantity: 1,
      productTitle: 'Pending Investigation Jacket',
      priceCents: 800000,
      subtotalPriceCents: 800000,
      restock: false,
      financialAdjustment: {
        deductionCents: 720000,
        context: 'available',
        adjustedAt: '2026-09-08T10:00:00Z',
        grossCents: 800000,
        commissionCents: 80000,
        sellerEarningCents: 720000,
      },
      compensation: {
        status: 'pending',
      },
      createdAt: '2026-09-08T09:00:00Z',
      updatedAt: '2026-09-08T10:00:00Z',
      arrivedAtZamk: true,
      inspectionCompleted: true,
      physicalOutcome: 'damaged',
      restockedQuantity: 0,
      damagedQuantity: 1,
      rejectedQuantity: 0,
      notReceivedQuantity: 0,
      processingStatus: 'completed',
    };

    vi.mocked(sellerApi.getSellerReturn).mockResolvedValue({ items: [mockReturn] } as any);

    render(
      <MemoryRouter initialEntries={['/returns/ret-pending-comp-1']}>
        <Routes>
          <Route path="/returns/:id" element={<SellerReturnDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/ORD-900803/)).toBeTruthy();
    });

    expect(screen.getByText('Финансовая ответственность определяется')).toBeTruthy();
    expect(screen.getByText('На рассмотрении')).toBeTruthy();
    expect(
      screen.getByText(/Возврат обработан, но ответственность за повреждение ещё определяется\./)
    ).toBeTruthy();
    expect(screen.queryByText(/Сторно/i)).toBeNull();
  });
});
