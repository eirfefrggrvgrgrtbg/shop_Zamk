/**
 * @vitest-environment jsdom
 */
// PAY.P1 — a second order while the first payment is unfinished is refused with the way to «Мои заказы»; an
// unavailable payment method keeps the order and lets the buyer pay it with another method.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Checkout } from './Checkout';
import { PromoProvider } from '../contexts/PromoContext';

const createOrder = vi.fn();
const createPayment = vi.fn();
const clearCart = vi.fn();
const getPaymentMethods = vi.fn();
const refreshCart = vi.fn();

vi.mock('@zamk/api-client/src/customer', () => ({
  validatePromo: vi.fn(),
  createOrder: (...a: unknown[]) => createOrder(...a),
  createPayment: (...a: unknown[]) => createPayment(...a),
  getPaymentMethods: (...a: unknown[]) => getPaymentMethods(...a),
  getAddresses: () => Promise.resolve([]),
  getOrder: () => Promise.resolve(null),
}));
vi.mock('@zamk/api-client/src/public', () => ({
  getDeliveryMethods: () => Promise.resolve([{ id: 'dm-1', name: 'Курьер', priceCents: 50000 }]),
}));
vi.mock('../lib/behavior', () => ({ trackCheckoutStarted: vi.fn(), getOrRenewSession: () => null, getOrCreateVisitorId: () => null }));
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ isAuthenticated: true, isInitializing: false, user: { id: 'u1', name: 'Анна Тестова', email: 'anna@example.com' }, openAuthModal: vi.fn() }),
}));
vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({
    items: [{ id: 'ci-1', productId: 'p1', productVariantId: 'v1', title: 'Худи', price: 5900, quantity: 1, inStock: true }],
    totalPrice: 5900,
    isLoadingCart: false,
    clearCart: (...a: unknown[]) => clearCart(...a),
    refreshCart: (...a: unknown[]) => refreshCart(...a),
  }),
}));

const apiError = (code: string, message: string, rawMessage?: string) => Object.assign(new Error(message), { code, rawMessage });

async function submit() {
  render(<MemoryRouter><PromoProvider><Checkout /></PromoProvider></MemoryRouter>);
  fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
  fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Москва, Тверская 1' } });
  await screen.findByText(/Курьер/);
  fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
}

describe('Checkout (PAY.P1)', () => {
  beforeEach(() => {
    for (const f of [createOrder, createPayment, clearCart, refreshCart]) f.mockReset();
    sessionStorage.clear();
    getPaymentMethods.mockReset();
    getPaymentMethods.mockResolvedValue({ methods: [{ method: 'card', mock: false }, { method: 'tpay', mock: true }], default: 'card' });
    clearCart.mockResolvedValue(undefined);
    refreshCart.mockResolvedValue(undefined);
    window.scrollTo = vi.fn() as unknown as typeof window.scrollTo;
  });
  afterEach(() => cleanup());

  it('another order in payment: the server message, a link to «Мои заказы», no payment, the cart is kept', async () => {
    const msg = 'Оплата заказа ORD-100041 ещё не завершена. Завершите её или отмените на странице оплаты, затем оформите новый заказ. Заказ доступен в разделе «Мои заказы».';
    createOrder.mockRejectedValue(apiError('payment_in_progress', msg, msg));
    await submit();
    expect((await screen.findByRole('alert')).textContent).toContain('ORD-100041');
    expect(screen.getByTestId('checkout-to-orders').getAttribute('href')).toBe('/orders');
    expect(createPayment).not.toHaveBeenCalled();
    expect(clearCart).not.toHaveBeenCalled();
    expect(screen.queryByTestId('checkout-back-to-cart')).toBeNull();
  });

  it('card unavailable: clear message, the order is kept and the next submit pays it with T-Pay (no second order)', async () => {
    createOrder.mockResolvedValue({ id: 'order-1', orderNumber: 'ORD-100042' });
    const msg = 'Этот способ оплаты сейчас недоступен. Выберите другой способ оплаты.';
    createPayment.mockRejectedValueOnce(apiError('payment_method_unavailable', msg, msg));
    createPayment.mockResolvedValueOnce({ integrationMode: 'real', paymentUrl: '' });
    await submit();
    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain('способ оплаты сейчас недоступен');
    expect(alert.textContent).toContain('Заказ сохранён');
    expect(createPayment).toHaveBeenLastCalledWith('order-1', 'card');

    fireEvent.click(screen.getByRole('button', { name: /T-Pay/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    expect((await screen.findByTestId('checkout-order-number')).textContent).toContain('ORD-100042');
    expect(createOrder).toHaveBeenCalledTimes(1);
    expect(createPayment).toHaveBeenLastCalledWith('order-1', 'tpay');
  });

  it('card not configured: only T-Pay is offered and preselected, the order is paid with it', async () => {
    getPaymentMethods.mockResolvedValue({ methods: [{ method: 'tpay', mock: true }], default: 'tpay' });
    createOrder.mockResolvedValue({ id: 'order-2', orderNumber: 'ORD-100043' });
    createPayment.mockResolvedValue({ integrationMode: 'real', paymentUrl: '' });
    await submit();
    await screen.findByTestId('checkout-order-number');
    expect(screen.queryByRole('button', { name: 'Карта онлайн' })).toBeNull();
    expect(createPayment).toHaveBeenCalledWith('order-2', 'tpay');
  });

  it('no available methods: clear message and CreateOrder is never sent', async () => {
    getPaymentMethods.mockResolvedValue({ methods: [], default: '' });
    await submit();
    expect((await screen.findAllByRole('alert')).map(a => a.textContent).join(' ')).toContain('нет доступных способов оплаты');
    expect(createOrder).not.toHaveBeenCalled();
    expect(createPayment).not.toHaveBeenCalled();
  });

  it('methods request failed: nothing is preselected and CreateOrder is never sent', async () => {
    getPaymentMethods.mockRejectedValue(new Error('network'));
    await submit();
    expect((await screen.findAllByRole('alert')).map(a => a.textContent).join(' ')).toContain('Не удалось получить способы оплаты');
    expect(createOrder).not.toHaveBeenCalled();
  });
});
