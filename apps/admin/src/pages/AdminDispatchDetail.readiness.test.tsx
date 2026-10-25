// FBO.DISPATCH.CHECKLIST.1 — the «Готовность к отгрузке» block of the dispatch screen.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useNavigate } from 'react-router-dom';
import { AdminDispatchDetail } from './AdminDispatchDetail';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';
import { ApiError } from '@zamk/api-client/src/errors';
import { READINESS_TITLES } from '../components/shipments/DispatchReadinessCard';

vi.mock('../api/adminPicking', async (orig) => {
  const actual = await orig<typeof import('../api/adminPicking')>();
  return { ...actual, getAdminDispatchContext: vi.fn(), dispatchFulfillment: vi.fn() };
});

type Check = adminPickingApi.ReadinessCheck;
const ALL: Check[] = [
  { code: 'ORDER_STATE', severity: 'blocking', ok: true },
  { code: 'NOT_ALREADY_SHIPPED', severity: 'blocking', ok: true },
  { code: 'PICKING_COMPLETE', severity: 'blocking', ok: true },
  { code: 'PACKING_COMPLETE', severity: 'blocking', ok: true },
  { code: 'COMPOSITION_CONSISTENT', severity: 'blocking', ok: true },
  { code: 'UNITS_AND_STOCK', severity: 'blocking', ok: true },
  { code: 'SHIPMENT_STATE', severity: 'blocking', ok: true },
  { code: 'RECIPIENT_PRESENT', severity: 'blocking', ok: true },
  { code: 'PHONE_PRESENT', severity: 'blocking', ok: true },
  { code: 'ADDRESS_PRESENT', severity: 'blocking', ok: true },
  { code: 'DELIVERY_METHOD', severity: 'warning', ok: true },
  { code: 'POSTAL_CODE', severity: 'info', ok: false, detail: 'Индекс не указан (не обязателен).' },
  { code: 'COURIER_COMMENT', severity: 'info', ok: false, detail: 'Комментарий курьеру не указан (не обязателен).' },
  { code: 'CARRIER_AT_DISPATCH', severity: 'info', ok: false, detail: 'Служба доставки и трек-номер появятся при отгрузке.' },
];
const readiness = (patch: Record<string, Partial<Check>> = {}): adminPickingApi.DispatchReadiness => {
  const checks = ALL.map((c) => ({ ...c, ...(patch[c.code] ?? {}) }));
  const blockingCount = checks.filter((c) => c.severity === 'blocking' && !c.ok).length;
  return { ready: blockingCount === 0, blockingCount, warningCount: checks.filter((c) => c.severity === 'warning' && !c.ok).length, checks };
};

const ctx = (id: string, rd: adminPickingApi.DispatchReadiness, over: Partial<adminPickingApi.DispatchContext> = {}): adminPickingApi.DispatchContext => ({
  id, fulfillmentId: id, orderId: 'ord-1', orderNumber: 'ORD-100500', status: 'packed', packedAt: '2026-09-15T00:00:00Z',
  deliveryAddress: 'г. Москва, ул. Тверская, д. 1', recipientName: 'Анна Иванова', recipientPhone: '+7 999 000-11-22',
  customerName: 'Анна Иванова', customerPhone: '+7 999 000-11-22', parcelNumber: 1, parcelsTotal: 2, readiness: rd,
  items: [{ orderItemId: 'i1', productTitle: 'Худи', quantity: 1, allocationMode: 'legacy' }], ...over,
});

const auth = (perm = 'warehouse.dispatch') => vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
  hasPermission: (p: string) => p === perm, user: null, isAuthenticated: true, isLoading: false,
  hasAnyPermission: () => false, staff: null, login: vi.fn(), logout: vi.fn(),
} as never);

function Nav() {
  const go = useNavigate();
  return <button onClick={() => go('/fulfillment/dispatch/ful-2')}>next</button>;
}
const page = () => render(
  <MemoryRouter initialEntries={['/fulfillment/dispatch/ful-1']}>
    <Nav />
    <Routes><Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} /></Routes>
  </MemoryRouter>,
);
const block = () => screen.getByTestId('dispatch-readiness');
const row = (code: string) => within(block()).getByTestId(`readiness-check-${code}`);

describe('AdminDispatchDetail — readiness checklist', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    auth();
    window.print = vi.fn();
  });
  afterEach(() => cleanup());

  it('a ready parcel: every check listed with its Russian title, «Готова к отгрузке», dispatch offered', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', readiness()));
    page();
    await screen.findByTestId('dispatch-readiness');
    expect(screen.getByTestId('readiness-overall').textContent).toContain('Готова к отгрузке');
    expect(block().getAttribute('data-ready')).toBe('true');
    for (const c of ALL) {
      expect(row(c.code).textContent).toContain(READINESS_TITLES[c.code]);
    }
    expect(within(block()).getAllByTestId(/readiness-check-/)).toHaveLength(14);
    expect((screen.getByTestId('dispatch-open-confirm') as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByText('Обязательные условия')).toBeDefined();
    expect(screen.getByText(/Предупреждения/)).toBeDefined();
    expect(screen.getByText('Справочно')).toBeDefined();
  });

  it('missing optional data and carrier data are information only: still ready, nothing red', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', readiness()));
    page();
    await screen.findByTestId('dispatch-readiness');
    for (const code of ['POSTAL_CODE', 'COURIER_COMMENT', 'CARRIER_AT_DISPATCH']) {
      expect(row(code).getAttribute('data-severity')).toBe('info');
      expect(row(code).getAttribute('data-ok')).toBe('false');
    }
    expect(screen.getByTestId('readiness-overall').textContent).not.toContain('препятств');
    expect(row('POSTAL_CODE').textContent).toContain('не обязателен');
    expect((screen.getByTestId('dispatch-open-confirm') as HTMLButtonElement).disabled).toBe(false);
  });

  it('a warning does not block: the method is missing, the parcel is still ready and dispatchable', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', readiness({ DELIVERY_METHOD: { ok: false, detail: 'В заказе не указан способ доставки.' } })));
    page();
    await screen.findByTestId('dispatch-readiness');
    expect(row('DELIVERY_METHOD').getAttribute('data-severity')).toBe('warning');
    expect(row('DELIVERY_METHOD').textContent).toContain('не указан способ доставки');
    expect(screen.getByTestId('readiness-overall').textContent).toContain('Готова к отгрузке');
    expect((screen.getByTestId('dispatch-open-confirm') as HTMLButtonElement).disabled).toBe(false);
  });

  it('blockers: «Есть препятствия (N)», each reason shown, the dispatch button is disabled with an explanation', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', readiness({
      PHONE_PRESENT: { ok: false, detail: 'В заказе не указан телефон получателя.' },
      UNITS_AND_STOCK: { ok: false, detail: 'На складе недостаточно остатка или резерва для списания при отгрузке.' },
    })));
    page();
    await screen.findByTestId('dispatch-readiness');
    expect(screen.getByTestId('readiness-overall').textContent).toContain('Есть препятствия (2)');
    expect(block().getAttribute('data-ready')).toBe('false');
    expect(row('PHONE_PRESENT').textContent).toContain('не указан телефон получателя');
    expect(row('UNITS_AND_STOCK').textContent).toContain('недостаточно остатка');
    expect(row('PHONE_PRESENT').getAttribute('data-ok')).toBe('false');
    expect(row('ADDRESS_PRESENT').getAttribute('data-ok')).toBe('true');
    const btn = screen.getByTestId('dispatch-open-confirm') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(screen.getByText(/Отгрузка недоступна, пока не устранены препятствия/)).toBeDefined();
    fireEvent.click(btn);
    expect(screen.queryByText('Подтвердить отгрузку?')).toBeNull();
    expect(adminPickingApi.dispatchFulfillment).not.toHaveBeenCalled();
  });

  it('a successful dispatch: the screen refreshes to «shipped» and the checklist says the parcel is already shipped', async () => {
    const shipped = ctx('ful-1', readiness({ ORDER_STATE: { ok: false, detail: 'Заказ уже отгружен: отгрузка невозможна.' }, NOT_ALREADY_SHIPPED: { ok: false, detail: 'Посылка уже отгружена: повторная отгрузка невозможна.' } }), { status: 'shipped', shipmentStatus: 'shipped' });
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValueOnce(ctx('ful-1', readiness())).mockResolvedValue(shipped);
    vi.mocked(adminPickingApi.dispatchFulfillment).mockResolvedValue({ fulfillmentId: 'ful-1', orderId: 'ord-1', shipmentId: 'ship-1', fulfillmentStatus: 'shipped', orderStatus: 'shipped', shipmentStatus: 'shipped', shippedAt: '2026-09-15T00:00:00Z' });
    page();
    fireEvent.click(await screen.findByTestId('dispatch-open-confirm'));
    const buttons = await screen.findAllByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(row('NOT_ALREADY_SHIPPED').getAttribute('data-ok')).toBe('false'));
    expect(row('NOT_ALREADY_SHIPPED').textContent).toContain('уже отгружена');
    expect(screen.getByText('Отгрузка со склада выполнена')).toBeDefined();
    expect(screen.queryByTestId('dispatch-open-confirm')).toBeNull(); // no second dispatch is offered
  });

  it('the state changed after the screen loaded: the backend refuses, the reason is shown and the checklist is refreshed', async () => {
    const fresh = ctx('ful-1', readiness({ ADDRESS_PRESENT: { ok: false, detail: 'В заказе не указан адрес доставки.' } }));
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValueOnce(ctx('ful-1', readiness())).mockResolvedValue(fresh);
    vi.mocked(adminPickingApi.dispatchFulfillment).mockRejectedValue(new ApiError('x', 'dispatch_delivery_data_missing', 409));
    page();
    fireEvent.click(await screen.findByTestId('dispatch-open-confirm'));
    const buttons = await screen.findAllByRole('button', { name: /Подтвердить отгрузку/i });
    fireEvent.click(buttons[buttons.length - 1]);
    expect(await screen.findByText(/не указан получатель, телефон или адрес доставки/)).toBeDefined();
    await waitFor(() => expect(row('ADDRESS_PRESENT').getAttribute('data-ok')).toBe('false'));
    expect(screen.getByTestId('readiness-overall').textContent).toContain('Есть препятствия (1)');
    expect((screen.getByTestId('dispatch-open-confirm') as HTMLButtonElement).disabled).toBe(true);
    expect(adminPickingApi.dispatchFulfillment).toHaveBeenCalledTimes(1);
  });

  it('two parcels: each shows its own checklist; the readiness of one never carries over to the other', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockImplementation(async (id: string) =>
      id === 'ful-1' ? ctx('ful-1', readiness()) : ctx('ful-2', readiness({ PICKING_COMPLETE: { ok: false, detail: 'Собрано 0 из 1 ед.' }, PACKING_COMPLETE: { ok: false, detail: 'Сборка не начата, упаковка не выполнена.' } }), { status: 'paid', parcelNumber: 2 }));
    page();
    await screen.findByTestId('dispatch-readiness');
    expect(screen.getByTestId('readiness-overall').textContent).toContain('Готова к отгрузке');
    fireEvent.click(screen.getByText('next'));
    await waitFor(() => expect(screen.getByTestId('readiness-overall').textContent).toContain('Есть препятствия (2)'));
    expect(row('PICKING_COMPLETE').textContent).toContain('Собрано 0 из 1 ед.');
    expect(row('PACKING_COMPLETE').textContent).toContain('Сборка не начата');
    expect(screen.queryByTestId('dispatch-open-confirm')).toBeNull(); // not packed: no dispatch card at all
    expect(document.querySelectorAll('[data-testid=dispatch-readiness]')).toHaveLength(1);
  });

  it('loading and error states: no checklist while loading or when the context is refused (no data without the right)', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockImplementation(() => new Promise(() => undefined));
    page();
    await new Promise((r) => setTimeout(r, 30));
    expect(screen.queryByTestId('dispatch-readiness')).toBeNull();
    cleanup();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockRejectedValue(new ApiError('Нет доступа', 'forbidden', 403));
    page();
    await screen.findByText(/Нет доступа/);
    expect(screen.queryByTestId('dispatch-readiness')).toBeNull();
    expect(screen.queryByTestId('shipping-card')).toBeNull();
  });

  it('without warehouse.dispatch the checklist is read-only: no dispatch action', async () => {
    auth('fulfillment.read');
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', readiness()));
    page();
    await screen.findByTestId('dispatch-readiness');
    expect(screen.queryByTestId('dispatch-open-confirm')).toBeNull();
    expect(screen.getByText(/требуется право warehouse.dispatch/)).toBeDefined();
  });

  it('the print button keeps working next to the checklist, and printing changes nothing', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', readiness()));
    page();
    fireEvent.click(await screen.findByTestId('print-shipping-card'));
    expect(window.print).toHaveBeenCalledTimes(1);
    expect(adminPickingApi.dispatchFulfillment).not.toHaveBeenCalled();
    expect(screen.getByTestId('shipping-card')).toBeDefined();
  });

  it('an older backend without readiness: the screen works as before (no block, dispatch offered)', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockResolvedValue(ctx('ful-1', undefined as never, { readiness: undefined }));
    page();
    await screen.findByTestId('dispatch-open-confirm');
    expect(screen.queryByTestId('dispatch-readiness')).toBeNull();
  });
});
