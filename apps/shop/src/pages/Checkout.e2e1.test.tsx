/**
 * @vitest-environment jsdom
 */
// CHECKOUT.E2E.1 — confirmation with the order number; a cart-fixable refusal shows a clear message, the way back to
// the cart and reloads the cart; an unexpected failure never shows internal text.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Checkout } from './Checkout';
import { PromoProvider } from '../contexts/PromoContext';

const createOrder = vi.fn();
const createPayment = vi.fn();
const clearCart = vi.fn();
const refreshCart = vi.fn();

vi.mock('@zamk/api-client/src/customer', () => ({
  validatePromo: vi.fn(),
  createOrder: (...a: unknown[]) => createOrder(...a),
  createPayment: (...a: unknown[]) => createPayment(...a),
  getPaymentMethods: () => Promise.resolve({ methods: [{ method: 'card', mock: false }, { method: 'tpay', mock: true }], default: 'card' }),
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

describe('Checkout (CHECKOUT.E2E.1)', () => {
  beforeEach(() => {
    for (const f of [createOrder, createPayment, clearCart, refreshCart]) f.mockReset();
    sessionStorage.clear();
    clearCart.mockResolvedValue(undefined);
    refreshCart.mockResolvedValue(undefined);
    window.scrollTo = vi.fn() as unknown as typeof window.scrollTo;
  });
  afterEach(() => cleanup());

  it('shows the order number on the confirmation and no false e-mail promise', async () => {
    createOrder.mockResolvedValue({ id: 'order-1', orderNumber: 'ORD-100042' });
    createPayment.mockResolvedValue({ integrationMode: 'real', paymentUrl: '' });
    await submit();
    expect((await screen.findByTestId('checkout-order-number')).textContent).toContain('ORD-100042');
    expect(document.body.textContent).not.toContain('отправлено на вашу почту');
    expect(screen.getByRole('link', { name: 'Мои заказы' }).getAttribute('href')).toBe('/orders');
    expect(window.scrollTo).toHaveBeenCalled();
  });

  it('out of stock: clear message, link back to the cart, the cart is reloaded, no payment attempted', async () => {
    createOrder.mockRejectedValue(apiError('insufficient_stock', 'Недостаточно товара на складе', 'Недостаточно товара на складе. Вернитесь в корзину и измените количество.'));
    await submit();
    expect((await screen.findByRole('alert')).textContent).toContain('Вернитесь в корзину и измените количество.');
    expect(screen.getByTestId('checkout-back-to-cart').getAttribute('href')).toBe('/cart');
    await waitFor(() => expect(refreshCart).toHaveBeenCalledTimes(1));
    expect(createPayment).not.toHaveBeenCalled();
  });

  it('an unavailable item behaves the same', async () => {
    createOrder.mockRejectedValue(apiError('item_unavailable', 'Один из товаров больше недоступен.', 'Один из товаров больше недоступен. Вернитесь в корзину и удалите его.'));
    await submit();
    expect((await screen.findByRole('alert')).textContent).toContain('больше недоступен');
    expect(screen.getByTestId('checkout-back-to-cart')).toBeTruthy();
  });

  it('a server failure shows the safe client message and no cart link', async () => {
    createOrder.mockRejectedValue(apiError('internal_error', 'Произошла ошибка на сервере. Попробуйте позже', 'Не удалось оформить заказ. Попробуйте ещё раз.'));
    await submit();
    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain('Произошла ошибка на сервере');
    expect(screen.queryByTestId('checkout-back-to-cart')).toBeNull();
    expect(refreshCart).not.toHaveBeenCalled();
  });
});
