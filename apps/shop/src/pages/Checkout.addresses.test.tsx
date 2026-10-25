/**
 * @vitest-environment jsdom
 */
// SHOP.ADDRESSES.1 — checkout with saved addresses: the main address is preselected, the order gets a snapshot of the
// chosen address, a new address can still be typed, and without saved addresses manual entry works as before.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Checkout } from './Checkout';
import { PromoProvider } from '../contexts/PromoContext';

const createOrder = vi.fn();
const createPayment = vi.fn();
const getAddresses = vi.fn();

vi.mock('@zamk/api-client/src/customer', () => ({
  validatePromo: vi.fn(),
  createOrder: (...a: unknown[]) => createOrder(...a),
  createPayment: (...a: unknown[]) => createPayment(...a),
  getAddresses: (...a: unknown[]) => getAddresses(...a),
  getOrder: () => Promise.resolve(null),
  getPaymentMethods: () => Promise.resolve({ methods: [{ method: 'tpay', mock: true }], default: 'tpay' }),
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
    totalPrice: 5900, isLoadingCart: false, clearCart: vi.fn().mockResolvedValue(undefined), refreshCart: vi.fn(),
  }),
}));

const home = { id: 'a1', label: 'Дом', recipientName: 'Анна Иванова', phone: '+7 999 000-11-22', city: 'Москва', street: 'Тверская', house: '1', apartment: '12', isDefault: true };
const office = { id: 'a2', label: 'Офис', recipientName: 'Борис Петров', phone: '+7 900 111-22-33', city: 'Казань', street: 'Баумана', house: '5', isDefault: false };

const open = () => render(<MemoryRouter><PromoProvider><Checkout /></PromoProvider></MemoryRouter>);

describe('Checkout — saved addresses', () => {
  beforeEach(() => {
    for (const f of [createOrder, createPayment, getAddresses]) f.mockReset();
    sessionStorage.clear();
    createOrder.mockResolvedValue({ id: 'order-1', orderNumber: 'ORD-1' });
    createPayment.mockResolvedValue({ integrationMode: 'real', paymentUrl: '' });
    window.scrollTo = vi.fn() as unknown as typeof window.scrollTo;
  });
  afterEach(() => cleanup());

  it('the main address is preselected and shown; the order gets its snapshot', async () => {
    getAddresses.mockResolvedValue([office, home]);
    open();
    const select = (await screen.findByTestId('saved-address-select')) as HTMLSelectElement;
    await waitFor(() => expect(select.value).toBe('a1'));
    expect(screen.getByTestId('saved-address-summary').textContent).toContain('г. Москва, ул. Тверская, д. 1, кв. 12');
    expect(screen.getByTestId('saved-address-summary').textContent).toContain('Анна Иванова');
    expect(select.textContent).toContain('основной');
    fireEvent.change(screen.getByPlaceholderText('Эл. почта для получения чека'), { target: { value: 'anna@example.com' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalledTimes(1));
    const [input] = createOrder.mock.calls[0];
    expect(input.deliveryAddress).toBe('г. Москва, ул. Тверская, д. 1, кв. 12');
    expect(input.customerName).toBe('Анна Иванова');
    expect(input.customerPhone).toBe('+7 999 000-11-22');
    expect(input.deliveryMethodId).toBe('dm-1');
  });

  it('another saved address can be chosen: its data goes into the order', async () => {
    getAddresses.mockResolvedValue([home, office]);
    open();
    const select = (await screen.findByTestId('saved-address-select')) as HTMLSelectElement;
    await waitFor(() => expect(select.value).toBe('a1'));
    fireEvent.change(select, { target: { value: 'a2' } });
    expect(screen.getByTestId('saved-address-summary').textContent).toContain('г. Казань, ул. Баумана, д. 5');
    fireEvent.change(screen.getByPlaceholderText('Эл. почта для получения чека'), { target: { value: 'anna@example.com' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalled());
    expect(createOrder.mock.calls[0][0].deliveryAddress).toBe('г. Казань, ул. Баумана, д. 5');
    expect(createOrder.mock.calls[0][0].customerName).toBe('Борис Петров');
  });

  it('«new address» switches to typing; the typed address is used', async () => {
    getAddresses.mockResolvedValue([home]);
    open();
    const select = (await screen.findByTestId('saved-address-select')) as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'new' } });
    fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
    fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Санкт-Петербург, Невский 1' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalled());
    expect(createOrder.mock.calls[0][0].deliveryAddress).toBe('Санкт-Петербург, Невский 1');
  });

  it('no saved addresses: manual entry works and there is no selector', async () => {
    getAddresses.mockResolvedValue([]);
    open();
    fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
    fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Москва, Тверская 1' } });
    expect(screen.queryByTestId('saved-address-select')).toBeNull();
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalled());
    expect(createOrder.mock.calls[0][0].deliveryAddress).toBe('Москва, Тверская 1');
  });

  it('the address list could not be loaded: manual entry still works', async () => {
    getAddresses.mockRejectedValue(new Error('network'));
    open();
    expect(await screen.findByPlaceholderText('Полный адрес доставки')).toBeDefined();
    expect(screen.queryByTestId('saved-address-select')).toBeNull();
  });

  it('a saved address brings its postal code and courier comment: shown before the order, sent with it', async () => {
    getAddresses.mockResolvedValue([{ ...home, postalCode: '101000', comment: 'Домофон 12, <b>позвонить</b>' }]);
    open();
    const summary = await screen.findByTestId('saved-address-summary');
    expect(screen.getByTestId('saved-address-postal').textContent).toContain('101000');
    expect(screen.getByTestId('saved-address-comment').textContent).toContain('Домофон 12, <b>позвонить</b>');
    expect(summary.querySelector('b')).toBeNull(); // shown as text, never as markup
    fireEvent.change(screen.getByPlaceholderText('Эл. почта для получения чека'), { target: { value: 'anna@example.com' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalled());
    const [input] = createOrder.mock.calls[0];
    expect(input.deliveryPostalCode).toBe('101000');
    expect(input.deliveryCourierComment).toBe('Домофон 12, <b>позвонить</b>');
  });

  it('a saved address without extras sends none', async () => {
    getAddresses.mockResolvedValue([home]);
    open();
    await screen.findByTestId('saved-address-summary');
    expect(screen.queryByTestId('saved-address-postal')).toBeNull();
    expect(screen.queryByTestId('saved-address-comment')).toBeNull();
    fireEvent.change(screen.getByPlaceholderText('Эл. почта для получения чека'), { target: { value: 'anna@example.com' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalled());
    expect(createOrder.mock.calls[0][0].deliveryPostalCode).toBeUndefined();
    expect(createOrder.mock.calls[0][0].deliveryCourierComment).toBeUndefined();
  });

  it('manual entry: optional postal code and comment are sent trimmed; blank ones are not sent', async () => {
    getAddresses.mockResolvedValue([]);
    open();
    fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
    fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Москва, Тверская 1' } });
    fireEvent.change(screen.getByTestId('checkout-postal-code'), { target: { value: ' 101000 ' } });
    fireEvent.change(screen.getByTestId('checkout-courier-comment'), { target: { value: '  Оставить у консьержа ' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    await waitFor(() => expect(createOrder).toHaveBeenCalled());
    expect(createOrder.mock.calls[0][0].deliveryPostalCode).toBe('101000');
    expect(createOrder.mock.calls[0][0].deliveryCourierComment).toBe('Оставить у консьержа');
  });

  it('a bad postal code is refused in the form before anything is sent', async () => {
    getAddresses.mockResolvedValue([]);
    open();
    fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
    fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Москва, Тверская 1' } });
    fireEvent.change(screen.getByTestId('checkout-postal-code'), { target: { value: '10<1>' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    expect((await screen.findByRole('alert')).textContent).toMatch(/Индекс/);
    expect(createOrder).not.toHaveBeenCalled();
  });

  it('the server refuses the delivery details: its message is shown, no order is kept', async () => {
    getAddresses.mockResolvedValue([]);
    createOrder.mockRejectedValue(Object.assign(new Error('x'), { code: 'validation_error', rawMessage: 'Комментарий для курьера слишком длинный (не более 500 символов)' }));
    open();
    fireEvent.change(await screen.findByPlaceholderText('Телефон'), { target: { value: '+79990001122' } });
    fireEvent.change(screen.getByPlaceholderText('Полный адрес доставки'), { target: { value: 'Москва, Тверская 1' } });
    await screen.findByText(/Курьер/);
    fireEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));
    expect((await screen.findByRole('alert')).textContent).toContain('слишком длинный');
    expect(createPayment).not.toHaveBeenCalled();
  });
});
