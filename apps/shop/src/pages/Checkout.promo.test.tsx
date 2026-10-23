/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Checkout } from './Checkout';
import { PromoProvider } from '../contexts/PromoContext';
import { formatPrice } from '../lib/utils';

const validatePromo = vi.fn();
const createOrder = vi.fn();
const createPayment = vi.fn();
const clearCart = vi.fn();

vi.mock('@zamk/api-client/src/customer', () => ({
  validatePromo: (c: string) => validatePromo(c),
  createOrder: (...a: unknown[]) => createOrder(...a),
  createPayment: (...a: unknown[]) => createPayment(...a),
  getPaymentMethods: () => Promise.resolve({ methods: [{ method: 'card', mock: false }, { method: 'tpay', mock: true }], default: 'card' }),
  getAddresses: () => Promise.resolve([]),
  getOrder: () => Promise.resolve(null),
}));
vi.mock('@zamk/api-client/src/public', () => ({
  getDeliveryMethods: () => Promise.resolve([{ id: 'dm-1', name: 'Курьер', priceCents: 30000 }]),
}));
vi.mock('../lib/behavior', () => ({
  trackCheckoutStarted: vi.fn(),
  getOrRenewSession: () => null,
  getOrCreateVisitorId: () => null,
}));
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    isInitializing: false,
    user: { id: 'u1', name: 'Иван Петров', email: 'ivan@example.com' },
    openAuthModal: vi.fn(),
  }),
}));

const cartItems = [
  { id: 'ci-1', productId: 'p1', productVariantId: 'v1', title: 'Худи', price: 5000, quantity: 2, inStock: true },
];
vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({
    items: cartItems,
    totalPrice: 10000,
    isLoadingCart: false,
    clearCart: (...a: unknown[]) => clearCart(...a),
  }),
}));

// getByText does not match a string containing a non-breaking space, so compare textContent directly.
const totalNode = (rub: number) =>
  screen.getByText((_, el) => !!el && el.children.length === 0 && el.textContent === formatPrice(rub));

const apiError = (code: string, rawMessage: string) => Object.assign(new Error(rawMessage), { code, rawMessage });

function renderCheckout() {
  return render(
    <MemoryRouter>
      <PromoProvider>
        <Checkout />
      </PromoProvider>
    </MemoryRouter>,
  );
}

async function fillContacts() {
  fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
  fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Москва, Тверская 1' } });
}

async function applyCode(code: string) {
  fireEvent.change(screen.getByLabelText('Промокод'), { target: { value: code } });
  fireEvent.click(screen.getByRole('button', { name: 'Применить' }));
}

describe('Checkout promo code', () => {
  beforeEach(() => {
    validatePromo.mockReset();
    createOrder.mockReset();
    createPayment.mockReset();
    clearCart.mockReset();
    sessionStorage.clear();
    createOrder.mockResolvedValue({ id: 'order-1' });
    createPayment.mockResolvedValue({ integrationMode: 'real', paymentUrl: '' });
    clearCart.mockResolvedValue(undefined);
  });
  afterEach(() => cleanup());

  it('shows the confirmed discount and updated total, and sends the code to CreateOrder', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 100000 });
    renderCheckout();
    await fillContacts();
    await screen.findByText(/Курьер/);
    await applyCode('save10');

    expect((await screen.findByTestId('checkout-promo-discount')).textContent).toContain(formatPrice(1000));
    // 10000 - 1000 + 300 delivery
    await waitFor(() => expect(totalNode(9300)).toBeTruthy());

    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalledTimes(1));
    expect(createOrder.mock.calls[0][0]).toMatchObject({ promoCode: 'SAVE10', deliveryMethodId: 'dm-1' });
  });

  it('regression: checkout without a promo code sends no promoCode', async () => {
    renderCheckout();
    await fillContacts();
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalledTimes(1));
    expect(createOrder.mock.calls[0][0].promoCode).toBeUndefined();
    expect(validatePromo).not.toHaveBeenCalled();
  });

  it('a double click creates one order (no double redemption)', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 100000 });
    let resolveOrder!: (v: unknown) => void;
    createOrder.mockReturnValue(new Promise((r) => { resolveOrder = r; }));
    renderCheckout();
    await fillContacts();
    await screen.findByText(/Курьер/);
    await applyCode('SAVE10');
    await screen.findByTestId('checkout-promo-discount');

    const button = screen.getByRole('button', { name: 'Оформить заказ' });
    fireEvent.click(button);
    fireEvent.click(button);
    fireEvent.click(button);
    expect(createOrder).toHaveBeenCalledTimes(1);
    await act(async () => { resolveOrder({ id: 'order-1' }); });
  });

  it('when the promo stops working at order time: shows the reason, drops the discount, creates no order', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 100000 });
    createOrder.mockRejectedValue(apiError('invalid_order', 'promo_global_limit'));
    renderCheckout();
    await fillContacts();
    await screen.findByText(/Курьер/);
    await applyCode('SAVE10');
    await screen.findByTestId('checkout-promo-discount');

    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));

    await waitFor(() => expect(screen.queryByTestId('checkout-promo-discount')).toBeNull());
    expect(screen.getAllByText(/Промокод исчерпан/).length).toBeGreaterThan(0);
    // Actual full amount is shown: 10000 + 300 delivery.
    expect(totalNode(10300)).toBeTruthy();
    expect(createPayment).not.toHaveBeenCalled();
    expect(createOrder).toHaveBeenCalledTimes(1);

    // Next submit is a deliberate new action and goes without the promo.
    createOrder.mockResolvedValue({ id: 'order-2' });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Оформить заказ' })).toHaveProperty('disabled', false));
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalledTimes(2));
    expect(createOrder.mock.calls[1][0].promoCode).toBeUndefined();
  });

  it('shows an API error for an invalid code and keeps the full price', async () => {
    validatePromo.mockRejectedValue(apiError('invalid_promo', 'promo_expired'));
    renderCheckout();
    await screen.findByText(/Курьер/);
    await applyCode('OLD');
    expect((await screen.findByRole('alert')).textContent).toContain('истёк');
    expect(screen.queryByTestId('checkout-promo-discount')).toBeNull();
    expect(totalNode(10300)).toBeTruthy();
  });

  it('shows a network error without applying the code', async () => {
    validatePromo.mockRejectedValue(apiError('NETWORK_ERROR', 'down'));
    renderCheckout();
    await screen.findByText(/Курьер/);
    await applyCode('SAVE10');
    expect((await screen.findByRole('alert')).textContent).toContain('Проверьте соединение');
    expect(screen.queryByTestId('checkout-promo-discount')).toBeNull();
  });

  it('removing the promo restores the full total', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 100000 });
    renderCheckout();
    await screen.findByText(/Курьер/);
    await applyCode('SAVE10');
    await screen.findByTestId('checkout-promo-discount');
    fireEvent.click(screen.getByRole('button', { name: 'Удалить промокод SAVE10' }));
    expect(screen.queryByTestId('checkout-promo-discount')).toBeNull();
    expect(totalNode(10300)).toBeTruthy();
  });
});
