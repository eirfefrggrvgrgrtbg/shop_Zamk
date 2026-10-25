// FBO.SHIPPING.DOCS.1 — the internal shipping card printed from the dispatch screen.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useNavigate } from 'react-router-dom';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { AdminDispatchDetail } from './AdminDispatchDetail';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminPickingApi from '../api/adminPicking';
import { SHIPPING_CARD_DISCLAIMER } from '../components/shipments/ShippingCard';

vi.mock('../api/adminPicking', () => ({
  getAdminDispatchContext: vi.fn(),
  dispatchFulfillment: vi.fn(),
  getDispatchErrorMessage: vi.fn((e: any) => e?.message || 'error'),
}));

const parcel1: adminPickingApi.DispatchContext = {
  id: 'ful-1', fulfillmentId: 'ful-1', orderId: 'ord-1', orderNumber: 'ORD-100500', status: 'packed', shipmentId: 'ship-1111',
  deliveryAddress: 'г. Москва, ул. Тверская, д. 1, кв. 12', recipientName: 'Анна Иванова', recipientPhone: '+7 999 000-11-22',
  customerName: 'Анна Иванова', customerPhone: '+7 999 000-11-22', deliveryMethodName: 'Курьер ZAMK',
  deliveryPostalCode: '101000', deliveryCourierComment: 'Домофон 12, позвонить за час',
  parcelNumber: 1, parcelsTotal: 2, carrier: 'СДЭК', trackingNumber: 'TRK-A-0001',
  items: [{ orderItemId: 'i1', productTitle: 'Худи', quantity: 1, allocationMode: 'legacy' }],
};
const parcel2: adminPickingApi.DispatchContext = {
  ...parcel1, id: 'ful-2', fulfillmentId: 'ful-2', shipmentId: 'ship-2222', parcelNumber: 2, carrier: 'Курьер ZAMK', trackingNumber: 'TRK-B-0002',
};
const oldOrder: adminPickingApi.DispatchContext = {
  id: 'ful-9', fulfillmentId: 'ful-9', orderId: 'ord-9', orderNumber: 'ORD-100001', status: 'packed',
  deliveryAddress: 'г. Тула, ул. Старая, д. 3', recipientName: 'Борис Петров', recipientPhone: '+7 900 111-22-33',
  items: [{ orderItemId: 'i9', productTitle: 'Худи', quantity: 1, allocationMode: 'legacy' }],
};

const auth = (perm = 'warehouse.dispatch') => vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
  hasPermission: (p: string) => p === perm, user: null, isAuthenticated: true, isLoading: false,
  hasAnyPermission: () => false, staff: null, login: vi.fn(), logout: vi.fn(),
} as never);

function Nav() {
  const go = useNavigate();
  return <button onClick={() => go('/fulfillment/dispatch/ful-2')}>next</button>;
}
const page = (path = '/fulfillment/dispatch/ful-1') => render(
  <MemoryRouter initialEntries={[path]}>
    <Nav />
    <Routes><Route path="/fulfillment/dispatch/:id" element={<AdminDispatchDetail />} /></Routes>
  </MemoryRouter>,
);
const card = () => screen.getByTestId('shipping-card');
const text = (id: string) => within(card()).getByTestId(id).textContent;

describe('AdminDispatchDetail — shipping card', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    auth();
    window.print = vi.fn();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockImplementation(async (id: string) => ({ 'ful-1': parcel1, 'ful-2': parcel2, 'ful-9': oldOrder } as Record<string, adminPickingApi.DispatchContext>)[id]);
  });
  afterEach(() => cleanup());

  it('A. the print button appears after the context loaded and opens the browser print dialog — nothing else', async () => {
    page();
    const btn = await screen.findByTestId('print-shipping-card');
    expect(btn.textContent).toContain('Распечатать карточку отправления');
    expect(window.print).not.toHaveBeenCalled(); // never automatic
    fireEvent.click(btn);
    expect(window.print).toHaveBeenCalledTimes(1);
  });

  it('B + C. the card has the recipient, phone, address, postal code, comment, order, parcel, shipment, carrier, track and the disclaimer', async () => {
    page();
    await screen.findByTestId('print-shipping-card');
    expect(card().textContent).toContain('ZAMK');
    expect(card().textContent).toContain('Внутренняя карточка отправления');
    expect(text('sc-order')).toBe('ORD-100500');
    expect(text('sc-parcel')).toBe('1 из 2');
    expect(text('sc-shipment')).toBe('ship-1111');
    expect(text('sc-recipient')).toBe('Анна Иванова');
    expect(text('sc-phone')).toBe('+7 999 000-11-22');
    expect(text('sc-address')).toBe('г. Москва, ул. Тверская, д. 1, кв. 12');
    expect(text('sc-postal')).toBe('101000');
    expect(text('sc-comment')).toBe('Домофон 12, позвонить за час');
    expect(text('sc-carrier')).toBe('СДЭК');
    expect(text('sc-tracking')).toBe('TRK-A-0001');
    expect(text('sc-disclaimer')).toBe(SHIPPING_CARD_DISCLAIMER);
    expect(SHIPPING_CARD_DISCLAIMER).toBe('Внутренний документ ZAMK. Не является транспортной накладной или этикеткой перевозчика.');
    expect(card().textContent).toMatch(/не наклеивать на посылку/);
  });

  it('D. an old order without postal code, comment, carrier and track prints without those rows and without invented values', async () => {
    page('/fulfillment/dispatch/ful-9');
    await screen.findByTestId('print-shipping-card');
    expect(text('sc-recipient')).toBe('Борис Петров');
    expect(text('sc-address')).toBe('г. Тула, ул. Старая, д. 3');
    for (const id of ['sc-postal', 'sc-comment', 'sc-carrier', 'sc-tracking', 'sc-method']) expect(within(card()).queryByTestId(id)).toBeNull();
    expect(text('sc-shipment')).toBe('ещё не создано'); // no invented shipment or waybill number
    expect(text('sc-parcel')).toBe('—');
  });

  it('E. another parcel shows its own card only: the previous parcel is never printable', async () => {
    page();
    await screen.findByTestId('print-shipping-card');
    expect(text('sc-parcel')).toBe('1 из 2');
    fireEvent.click(screen.getByText('next'));
    await waitFor(() => expect(text('sc-parcel')).toBe('2 из 2'));
    expect(text('sc-shipment')).toBe('ship-2222');
    expect(text('sc-carrier')).toBe('Курьер ZAMK');
    expect(text('sc-tracking')).toBe('TRK-B-0002');
    expect(text('sc-address')).toBe('г. Москва, ул. Тверская, д. 1, кв. 12'); // the order's snapshot is shared
    expect(document.querySelectorAll('[data-testid=shipping-card]')).toHaveLength(1);
    expect(card().getAttribute('data-fulfillment-id')).toBe('ful-2');
    expect(card().textContent).not.toContain('TRK-A-0001');
  });

  it('E2. while the next parcel loads, and if loading it fails, no card and no print button remain', async () => {
    page();
    await screen.findByTestId('print-shipping-card');
    let release: (v: adminPickingApi.DispatchContext) => void = () => undefined;
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockImplementationOnce(() => new Promise((res) => { release = res; }));
    fireEvent.click(screen.getByText('next'));
    await waitFor(() => expect(screen.queryByTestId('shipping-card')).toBeNull());
    expect(screen.queryByTestId('print-shipping-card')).toBeNull();
    release(parcel2);
    await screen.findByTestId('print-shipping-card');
    expect(text('sc-parcel')).toBe('2 из 2');

    cleanup();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockRejectedValue(new Error('Нет доступа'));
    page();
    await screen.findByText(/Нет доступа/);
    expect(screen.queryByTestId('shipping-card')).toBeNull();
    expect(screen.queryByTestId('print-shipping-card')).toBeNull();
  });

  it('F. the card is a direct child of <body>, outside the app root, and the print stylesheet hides the app and shows only the card', async () => {
    page();
    await screen.findByTestId('print-shipping-card');
    expect(card().parentElement).toBe(document.body);
    const css = readFileSync(resolve(__dirname, '../styles/index.css'), 'utf8');
    const printBlock = css.slice(css.indexOf('@media print'));
    expect(css).toMatch(/@page zamk-card\s*\{\s*size:\s*A4/);
    expect(printBlock).toMatch(/html\.zamk-print-card #root\s*\{\s*display:\s*none\s*!important/);
    // no global rule hides the app or restyles pages in print: every #root / html / body rule needs the card class
    const globalRule = printBlock.split('\n').filter((l) => /^\s*(#root|html|body)\b/.test(l) && !/zamk-print-card/.test(l));
    expect(globalRule).toEqual([]);
    expect(printBlock).toMatch(/#zamk-shipping-card\s*\{[^}]*display:\s*block\s*!important/);
    expect(printBlock).toMatch(/overflow-wrap:\s*anywhere/);
    expect(css).toMatch(/#zamk-shipping-card\s*\{\s*display:\s*none;/); // never visible on screen
    expect(printBlock).toMatch(/background:\s*#fff\s*!important/); // white paper
    expect(printBlock).toMatch(/color:\s*#000\s*!important/); // black text
  });

  it('G. no card and no print button when the context cannot be loaded', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockRejectedValue(new Error('Нет доступа'));
    page();
    await screen.findByText(/Нет доступа/);
    expect(screen.queryByTestId('print-shipping-card')).toBeNull();
    expect(screen.queryByTestId('shipping-card')).toBeNull();
  });

  it('H. printing changes nothing: no dispatch call, no request other than reading the context, no storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    page();
    fireEvent.click(await screen.findByTestId('print-shipping-card'));
    fireEvent.click(screen.getByTestId('print-shipping-card'));
    expect(adminPickingApi.dispatchFulfillment).not.toHaveBeenCalled();
    expect(adminPickingApi.getAdminDispatchContext).toHaveBeenCalledTimes(1); // the read that loaded the screen
    expect(setItem).not.toHaveBeenCalled();
    expect(card().textContent).not.toMatch(/https?:\/\//); // no link or query string carries the data
  });

  it('the card is removed with the screen (nothing stays after leaving)', async () => {
    const { unmount } = page();
    await screen.findByTestId('shipping-card');
    expect(document.documentElement.classList.contains('zamk-print-card')).toBe(true);
    unmount();
    expect(document.querySelector('[data-testid=shipping-card]')).toBeNull();
    expect(document.documentElement.classList.contains('zamk-print-card')).toBe(false); // other pages print as usual
  });

  it('the print switch is on only while a loaded card exists: not while loading, not after a failure, not after a parcel change', async () => {
    const on = () => document.documentElement.classList.contains('zamk-print-card');
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockImplementation(() => new Promise(() => undefined));
    page();
    await new Promise((r) => setTimeout(r, 30));
    expect(on()).toBe(false); // loading
    cleanup();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockRejectedValue(new Error('Нет доступа'));
    page();
    await screen.findByText(/Нет доступа/);
    expect(on()).toBe(false); // failed
    cleanup();
    vi.mocked(adminPickingApi.getAdminDispatchContext).mockImplementation(async (id: string) => ({ 'ful-1': parcel1, 'ful-2': parcel2 } as Record<string, adminPickingApi.DispatchContext>)[id]);
    page();
    await screen.findByTestId('shipping-card');
    expect(on()).toBe(true);
    fireEvent.click(screen.getByText('next'));
    await waitFor(() => expect(text('sc-parcel')).toBe('2 из 2'));
    expect(on()).toBe(true);
    expect(document.querySelectorAll('[data-testid=shipping-card]')).toHaveLength(1);
    cleanup();
    expect(on()).toBe(false);
  });
});
