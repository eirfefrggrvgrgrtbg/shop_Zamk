import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminReturns } from './AdminReturns';
import * as adminReturnsApi from '../api/adminReturns';
import type {
  AdminReturn,
  ReturnResponsibilityAllocation,
  AdminReturnRefundQuote,
} from '../api/adminReturns';
import { ApiError } from '@zamk/api-client/src/errors';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: () => ({
    user: { id: 'admin-1', role: 'admin', email: 'admin@zamk.local' },
    staff: {
      id: 'staff-1',
      permissions: ['returns.read', 'returns.update_status', 'warehouse.returns', 'refunds.create'],
    },
    permissions: ['returns.read', 'returns.update_status', 'warehouse.returns', 'refunds.create'],
    isAuthenticated: true,
    isLoading: false,
    error: null,
    hasPermission: () => true,
    hasAnyPermission: () => true,
    isOwner: () => true,
    isCoOwner: () => false,
  }),
}));

vi.mock('../api/adminTimeline', () => ({
  getAdminReturnTimeline: () =>
    Promise.resolve({ entityType: 'return', entityId: '', canonicalIdentifier: '', events: [] }),
}));

describe('Admin Returns Financial Responsibility UI (SA.5.3B)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  const baseReturn: AdminReturn = {
    id: 'ret-100198-id',
    orderId: 'ord-100198-id',
    orderNumber: 'ORD-100198',
    status: 'item_received',
    reason: 'defective',
    customerName: 'Test Buyer',
    customerEmail: 'buyer@test.com',
    createdAt: '2026-09-01T12:00:00Z',
    items: [
      {
        id: 'ri-1',
        orderItemId: 'oi-1',
        productTitle: 'Худи ZAMK Red L',
        quantity: 1,
        priceCents: 1182100,
        subtotalPriceCents: 1182100,
      },
    ],
  };

  const pendingAllocation: ReturnResponsibilityAllocation = {
    id: 'alloc-1',
    returnItemId: 'ri-1',
    quantity: 1,
    status: 'pending',
    legacyDisposition: 'damaged',
    createdAt: '2026-09-01T13:00:00Z',
    updatedAt: '2026-09-01T13:00:00Z',
  };

  const resolvedAllocationZamk: ReturnResponsibilityAllocation = {
    id: 'alloc-1',
    returnItemId: 'ri-1',
    quantity: 1,
    status: 'resolved',
    responsibleParty: 'zamk',
    reasonCode: 'zamk_warehouse_damage',
    legacyDisposition: 'damaged',
    decisionSource: 'employee',
    actorId: 'admin-1',
    decidedAt: '2026-09-01T14:30:00Z',
    internalNote: 'Акт повреждения №123 составлен',
    createdAt: '2026-09-01T13:00:00Z',
    updatedAt: '2026-09-01T14:30:00Z',
  };

  const emptyQuote: AdminReturnRefundQuote = {
    returnId: 'ret-100198-id',
    orderNumber: 'ORD-100198',
    currency: 'RUB',
    items: [],
    productsRefundCents: 0,
    deliveryRefundCents: 0,
    totalRefundCents: 0,
    alreadyRefundedCents: 0,
    succeededRefundedCents: 0,
    pendingRefundCents: 0,
    remainingRefundableCents: 0,
    canRefund: false,
    blockingReason: null,
    latestRefundStatus: null,
  };

  function setupPageMock(returnOverride: Partial<AdminReturn> = {}) {
    const ret: AdminReturn = {
      ...baseReturn,
      ...returnOverride,
    };
    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([ret]);
    vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(ret);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockResolvedValue(emptyQuote);
    vi.spyOn(adminReturnsApi, 'getAdminReturnResponsibilityAllocations').mockResolvedValue(
      ret.responsibilityAllocations || []
    );
    return ret;
  }

  // A. Pending damaged allocation renders required elements
  it('A: renders pending damaged allocation with "Финансовая ответственность" and "Требуется решение"', async () => {
    setupPageMock({
      responsibilityAllocations: [pendingAllocation],
    });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-responsibility-card')).toBeDefined();
    });

    const card = screen.getByTestId('return-responsibility-card');
    expect(card.textContent).toContain('Финансовая ответственность');
    expect(screen.getByTestId('responsibility-status-badge').textContent).toContain('Требуется решение');
    expect(screen.getByTestId('physical-fact-display').textContent).toContain('Повреждено: 1 шт.');

    // 3 options exposed
    expect(screen.getByTestId('choice-zamk')).toBeDefined();
    expect(screen.getByTestId('choice-carrier')).toBeDefined();
    expect(screen.getByTestId('choice-seller')).toBeDefined();

    // Plain Russian explanation
    expect(card.textContent).toContain('Склад зафиксировал повреждение товара при приёмке');

    // Confirm button present
    expect(screen.getByTestId('btn-submit-responsibility')).toBeDefined();
  });

  // B. Choosing ZAMK + confirm calls typed API with exact payload (No status, no legacyDisposition)
  it('B: choosing ZAMK + confirm calls typed API with exact { responsibleParty: "zamk", reasonCode: "zamk_warehouse_damage" }', async () => {
    setupPageMock({
      responsibilityAllocations: [pendingAllocation],
    });

    const updateSpy = vi
      .spyOn(adminReturnsApi, 'updateReturnResponsibility')
      .mockResolvedValue(resolvedAllocationZamk);

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('choice-zamk')).toBeDefined();
    });

    // Select ZAMK
    const radioZamk = screen.getByTestId('choice-zamk').querySelector('input[type="radio"]')!;
    fireEvent.click(radioZamk);

    // Add optional note
    const noteInput = screen.getByTestId('input-internal-note');
    fireEvent.change(noteInput, { target: { value: 'Акт №441' } });

    // Click confirm button in form -> opens modal
    fireEvent.click(screen.getByTestId('btn-submit-responsibility'));

    // Modal is open
    expect(screen.getByTestId('responsibility-confirm-modal')).toBeDefined();
    expect(updateSpy).not.toHaveBeenCalled();

    // Confirm in modal
    fireEvent.click(screen.getByTestId('btn-modal-confirm'));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledTimes(1);
    });

    const [calledReturnId, calledAllocId, calledPayload] = updateSpy.mock.calls[0];
    expect(calledReturnId).toBe('ret-100198-id');
    expect(calledAllocId).toBe('alloc-1');
    expect(calledPayload).toEqual({
      responsibleParty: 'zamk',
      reasonCode: 'zamk_warehouse_damage',
      internalNote: 'Акт №441',
    });

    // Explicit contract verification: NO status, NO legacyDisposition
    const payloadObj = calledPayload as unknown as Record<string, unknown>;
    expect(payloadObj.status).toBeUndefined();
    expect(payloadObj.legacyDisposition).toBeUndefined();
    expect(payloadObj.decisionSource).toBeUndefined();
    expect(payloadObj.actorId).toBeUndefined();
    expect(payloadObj.decidedAt).toBeUndefined();
  });

  // C. Carrier maps to carrier / carrier_damage
  it('C: choosing CARRIER maps to carrier / carrier_damage', async () => {
    setupPageMock({
      responsibilityAllocations: [pendingAllocation],
    });

    const updateSpy = vi
      .spyOn(adminReturnsApi, 'updateReturnResponsibility')
      .mockResolvedValue({
        ...pendingAllocation,
        status: 'resolved',
        responsibleParty: 'carrier',
        reasonCode: 'carrier_damage',
      });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('choice-carrier')).toBeDefined();
    });

    const radioCarrier = screen.getByTestId('choice-carrier').querySelector('input[type="radio"]')!;
    fireEvent.click(radioCarrier);

    fireEvent.click(screen.getByTestId('btn-submit-responsibility'));
    fireEvent.click(screen.getByTestId('btn-modal-confirm'));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledTimes(1);
    });

    expect(updateSpy).toHaveBeenCalledWith('ret-100198-id', 'alloc-1', {
      responsibleParty: 'carrier',
      reasonCode: 'carrier_damage',
    });
  });

  // D. Seller maps to seller / seller_product_defect
  it('D: choosing SELLER maps to seller / seller_product_defect', async () => {
    setupPageMock({
      responsibilityAllocations: [pendingAllocation],
    });

    const updateSpy = vi
      .spyOn(adminReturnsApi, 'updateReturnResponsibility')
      .mockResolvedValue({
        ...pendingAllocation,
        status: 'resolved',
        responsibleParty: 'seller',
        reasonCode: 'seller_product_defect',
      });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('choice-seller')).toBeDefined();
    });

    const radioSeller = screen.getByTestId('choice-seller').querySelector('input[type="radio"]')!;
    fireEvent.click(radioSeller);

    fireEvent.click(screen.getByTestId('btn-submit-responsibility'));
    fireEvent.click(screen.getByTestId('btn-modal-confirm'));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledTimes(1);
    });

    expect(updateSpy).toHaveBeenCalledWith('ret-100198-id', 'alloc-1', {
      responsibleParty: 'seller',
      reasonCode: 'seller_product_defect',
    });
  });

  // E. No API call occurs before explicit confirmation
  it('E: no API call occurs before explicit modal confirmation', async () => {
    setupPageMock({
      responsibilityAllocations: [pendingAllocation],
    });

    const updateSpy = vi.spyOn(adminReturnsApi, 'updateReturnResponsibility');

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('choice-zamk')).toBeDefined();
    });

    // 1. Selecting radio -> no API call
    fireEvent.click(screen.getByTestId('choice-zamk').querySelector('input[type="radio"]')!);
    expect(updateSpy).not.toHaveBeenCalled();

    // 2. Clicking submit opens modal -> no API call yet
    fireEvent.click(screen.getByTestId('btn-submit-responsibility'));
    expect(screen.getByTestId('responsibility-confirm-modal')).toBeDefined();
    expect(updateSpy).not.toHaveBeenCalled();

    // 3. Canceling modal -> no API call
    fireEvent.click(screen.getByTestId('btn-modal-cancel'));
    expect(screen.queryByTestId('responsibility-confirm-modal')).toBeNull();
    expect(updateSpy).not.toHaveBeenCalled();
  });

  // F. Successful response renders resolved human-readable state
  it('F: renders resolved allocation with human-readable party, reason, timestamp and note', async () => {
    setupPageMock({
      responsibilityAllocations: [resolvedAllocationZamk],
    });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-responsibility-card')).toBeDefined();
    });

    expect(screen.getByTestId('responsibility-status-badge').textContent).toContain('Решение принято');
    expect(screen.getByTestId('resolved-party').textContent).toContain('ZAMK');
    expect(screen.getByTestId('resolved-reason').textContent).toContain('Повреждение на стороне ZAMK');
    expect(screen.getByTestId('resolved-internal-note').textContent).toContain('Акт повреждения №123 составлен');
    expect(screen.getByTestId('physical-fact-display').textContent).toContain('Повреждено: 1 шт.');
  });

  // G. Resolved allocation exposes "Изменить решение"
  it('G: resolved allocation exposes "Изменить решение" button', async () => {
    setupPageMock({
      responsibilityAllocations: [resolvedAllocationZamk],
    });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('btn-change-responsibility')).toBeDefined();
    });

    expect(screen.getByTestId('btn-change-responsibility').textContent).toContain('Изменить решение');
  });

  // H. Correction reuses same API and does not perform frontend finance math
  it('H: clicking "Изменить решение" reopens decision UI and submits correction via same API without frontend math', async () => {
    setupPageMock({
      responsibilityAllocations: [resolvedAllocationZamk],
    });

    const updateSpy = vi
      .spyOn(adminReturnsApi, 'updateReturnResponsibility')
      .mockImplementation(async () => {
        return {
          ...resolvedAllocationZamk,
          responsibleParty: 'seller',
          reasonCode: 'seller_product_defect',
        };
      });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('btn-change-responsibility')).toBeDefined();
    });

    // Form was hidden before clicking
    expect(screen.queryByTestId('choice-seller')).toBeNull();

    // Click "Изменить решение"
    fireEvent.click(screen.getByTestId('btn-change-responsibility'));

    // Form reopened
    expect(screen.getByTestId('choice-seller')).toBeDefined();

    // Switch to seller
    fireEvent.click(screen.getByTestId('choice-seller').querySelector('input[type="radio"]')!);

    // Submit -> opens modal
    fireEvent.click(screen.getByTestId('btn-submit-responsibility'));

    // Modal contains correction advisory
    expect(
      screen.getByText(/Предыдущие финансовые проводки не перезаписываются/i)
    ).toBeDefined();

    // Confirm
    fireEvent.click(screen.getByTestId('btn-modal-confirm'));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledTimes(1);
    });

    expect(updateSpy).toHaveBeenCalledWith('ret-100198-id', 'alloc-1', {
      responsibleParty: 'seller',
      reasonCode: 'seller_product_defect',
      internalNote: 'Акт повреждения №123 составлен',
    });
  });

  // I. Clean/restocked/non-eligible allocation does not expose active responsibility decision controls
  it('I: clean/restocked/non-eligible allocation does not expose active decision controls', async () => {
    const cleanAllocation: ReturnResponsibilityAllocation = {
      id: 'alloc-clean',
      returnItemId: 'ri-1',
      quantity: 1,
      status: 'not_required',
      legacyDisposition: 'accepted',
      createdAt: '2026-09-01T13:00:00Z',
      updatedAt: '2026-09-01T13:00:00Z',
    };

    setupPageMock({
      responsibilityAllocations: [cleanAllocation],
    });

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('return-responsibility-card')).toBeDefined();
    });

    // Badge shows "Не требуется"
    expect(screen.getByTestId('responsibility-status-badge').textContent).toContain('Не требуется');

    // No active choice controls
    expect(screen.queryByTestId('choice-zamk')).toBeNull();
    expect(screen.queryByTestId('btn-submit-responsibility')).toBeNull();
    expect(screen.queryByTestId('btn-change-responsibility')).toBeNull();
  });

  // J. API error does not leave fake resolved state
  it('J: API error does not leave fake resolved state and displays error banner', async () => {
    setupPageMock({
      responsibilityAllocations: [pendingAllocation],
    });

    vi.spyOn(adminReturnsApi, 'updateReturnResponsibility').mockRejectedValue(
      new ApiError('Invalid transition for this allocation', 'domain_error', 400)
    );

    render(
      <MemoryRouter initialEntries={['/returns?id=ret-100198-id']}>
        <AdminReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('choice-zamk')).toBeDefined();
    });

    fireEvent.click(screen.getByTestId('choice-zamk').querySelector('input[type="radio"]')!);
    fireEvent.click(screen.getByTestId('btn-submit-responsibility'));
    fireEvent.click(screen.getByTestId('btn-modal-confirm'));

    await waitFor(() => {
      expect(screen.getByTestId('responsibility-error-banner')).toBeDefined();
    });

    expect(screen.getByTestId('responsibility-error-banner').textContent).toContain(
      'Invalid transition for this allocation'
    );

    // Remains in pending status, does not become fake resolved
    expect(screen.getByTestId('responsibility-status-badge').textContent).toContain('Требуется решение');
  });
});
