/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Cart } from './Cart';
import { diffCartPrices, readLastSeenCartPrices, saveLastSeenCartPrices } from '../lib/cartAvailability';
import type { UIContextCartItem } from '../contexts/CartContext';

const updateQuantity = vi.fn();
const removeItem = vi.fn();
let items: UIContextCartItem[] = [];

vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({
    items,
    // mirrors CartContext: only lines without an unavailable reason are payable
    totalPrice: items.reduce((s, i) => (i.unavailableReason ? s : s + (i.price || 0) * i.quantity), 0),
    isLoadingCart: false,
    addItem: vi.fn(),
    removeItem,
    updateQuantity,
    clearCart: vi.fn(),
    totalItems: items.reduce((s, i) => s + i.quantity, 0),
  }),
}));
vi.mock('../components/PromoCodeBox', () => ({ PromoCodeBox: () => null }));
vi.mock('../contexts/promoContextValue', () => ({
  usePromo: () => ({ appliedCode: null, discount: 0, isChecking: false }),
}));

const line = (over: Partial<UIContextCartItem>): UIContextCartItem => ({
  id: 'i1',
  productId: 'p1',
  productVariantId: 'v1',
  quantity: 2,
  title: 'Худи Осло',
  price: 1000,
  inStock: true,
  imageUrl: 'https://cdn.zamk.test/a.jpg',
  ...over,
});

const store = new Map<string, string>();
vi.stubGlobal('localStorage', {
  getItem: (k: string) => store.get(k) ?? null,
  setItem: (k: string, v: string) => void store.set(k, v),
  removeItem: (k: string) => void store.delete(k),
  clear: () => store.clear(),
});

const renderCart = () => render(<MemoryRouter><Cart /></MemoryRouter>);

describe('CART.C2 — недоступные позиции в корзине Shop', () => {
  beforeEach(() => {
    localStorage.clear();
    updateQuantity.mockReset();
    removeItem.mockReset();
  });
  afterEach(cleanup);

  it.each([
    ['product_unpublished', 'Товар снят с продажи'],
    ['variant_inactive', 'Этот вариант больше не продаётся'],
    ['out_of_stock', 'Товар закончился'],
  ] as const)('показывает причину %s и оставляет строку видимой', (reason, text) => {
    items = [line({ unavailableReason: reason, inStock: false, availableQuantity: 0 })];
    renderCart();
    expect(screen.getByTestId('cart-item-unavailable-reason').textContent).toBe(text);
    expect(screen.getByText('Худи Осло')).toBeDefined();
  });

  it('недостаточный остаток: причина, «Уменьшить до N» и свободный минус', () => {
    items = [line({ unavailableReason: 'insufficient_stock', availableQuantity: 1 })];
    renderCart();
    expect(screen.getByTestId('cart-item-unavailable-reason').textContent).toBe('В наличии только 1 шт.');
    fireEvent.click(screen.getByText('Уменьшить до 1 шт.'));
    expect(updateQuantity).toHaveBeenCalledWith('i1', 1);
    expect((screen.getByLabelText('Уменьшить количество Худи Осло') as HTMLButtonElement).disabled).toBe(false);
    expect((screen.getByLabelText('Увеличить количество Худи Осло') as HTMLButtonElement).disabled).toBe(true);
  });

  it('снятая позиция удаляется только явным действием', () => {
    items = [line({ unavailableReason: 'product_unpublished', inStock: false })];
    renderCart();
    expect(removeItem).not.toHaveBeenCalled();
    fireEvent.click(screen.getByLabelText('Удалить Худи Осло из корзины'));
    expect(removeItem).toHaveBeenCalledWith('i1');
  });

  it('блокирует оформление и не включает недоступное в сумму', () => {
    items = [
      line({ id: 'ok', productVariantId: 'vok', title: 'Доступное', price: 1000, quantity: 1 }),
      line({ id: 'bad', productVariantId: 'vbad', title: 'Снятое', price: 5000, quantity: 1, unavailableReason: 'variant_inactive', inStock: false }),
    ];
    renderCart();
    expect(screen.getByTestId('cart-unavailable-hint')).toBeDefined();
    const checkout = screen.getByText('Оформить заказ').closest('button') as HTMLButtonElement;
    expect(checkout.disabled).toBe(true);
    expect(screen.queryByRole('link', { name: /оформить заказ/i })).toBeNull();
    expect(screen.getAllByText(/1\s?000/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/6\s?000/)).toBeNull();
  });

  it('полностью доступная корзина ведёт к оформлению', () => {
    items = [line({})];
    renderCart();
    expect(screen.queryByTestId('cart-unavailable-hint')).toBeNull();
    expect(screen.getByRole('link', { name: /оформить заказ/i }).getAttribute('href')).toBe('/checkout');
  });

  it('уведомляет об изменении цены, не сохраняя старую как действующую', () => {
    localStorage.setItem('zamk.cart.lastPrices.v1', JSON.stringify({ v1: 900 }));
    items = [line({ price: 1000 })];
    renderCart();
    expect(screen.getByTestId('cart-item-price-changed').textContent).toMatch(/было.*900.*стало.*1\s?000/);
  });

  it('первое открытие корзины не показывает ложное изменение цены и запоминает цены', () => {
    items = [line({ price: 1000 })];
    renderCart();
    expect(screen.queryByTestId('cart-item-price-changed')).toBeNull();
    expect(JSON.parse(store.get('zamk.cart.lastPrices.v1') || '{}')).toEqual({ v1: 1000 });
  });

  it('та же цена — без уведомления', () => {
    localStorage.setItem('zamk.cart.lastPrices.v1', JSON.stringify({ v1: 1000 }));
    items = [line({ price: 1000 })];
    renderCart();
    expect(screen.queryByTestId('cart-item-price-changed')).toBeNull();
  });

  it('объясняет, почему итог меньше, и показывает цену недоступной позиции', () => {
    items = [
      line({ id: 'ok', productVariantId: 'vok', title: 'Доступное', price: 1000, quantity: 1 }),
      line({ id: 'bad', productVariantId: 'vbad', title: 'Снятое', price: 5000, quantity: 1, unavailableReason: 'product_unpublished', inStock: false }),
    ];
    renderCart();
    expect(screen.getByTestId('cart-total-excludes-hint').textContent).toContain('1');
    expect(screen.getByText('Сумма доступных товаров')).toBeDefined();
    expect(screen.getAllByText(/5\s?000/).length).toBeGreaterThan(0); // цена недоступной позиции видна
  });
});

describe('CART.C2 — хранение цен для уведомлений', () => {
  beforeEach(() => store.clear());

  it('хранит только variantId → число', () => {
    saveLastSeenCartPrices([
      { productVariantId: 'v1', price: 100 },
      { productVariantId: 'v2', price: undefined },
    ]);
    expect(JSON.parse(store.get('zamk.cart.lastPrices.v1')!)).toEqual({ v1: 100 });
  });

  it('удалённая позиция не оставляет записи и не даёт ложного уведомления при возврате', () => {
    saveLastSeenCartPrices([{ productVariantId: 'v1', price: 100 }]);
    saveLastSeenCartPrices([]);
    expect(readLastSeenCartPrices()).toEqual({});
    expect(diffCartPrices(readLastSeenCartPrices(), [{ productVariantId: 'v1', price: 150 }])).toEqual({});
  });

  it('diff: уведомление только при реальной смене цены', () => {
    expect(diffCartPrices({ v1: 100 }, [{ productVariantId: 'v1', price: 100 }])).toEqual({});
    expect(diffCartPrices({ v1: 100 }, [{ productVariantId: 'v1', price: 120 }])).toEqual({ v1: { from: 100, to: 120 } });
    expect(diffCartPrices({}, [{ productVariantId: 'v1', price: 120 }])).toEqual({});
  });

  it('повреждённое или недоступное хранилище не ломает чтение', () => {
    store.set('zamk.cart.lastPrices.v1', '{oops');
    expect(readLastSeenCartPrices()).toEqual({});
    store.set('zamk.cart.lastPrices.v1', JSON.stringify({ v1: 'x', v2: 5 }));
    expect(readLastSeenCartPrices()).toEqual({ v2: 5 });
  });
});
