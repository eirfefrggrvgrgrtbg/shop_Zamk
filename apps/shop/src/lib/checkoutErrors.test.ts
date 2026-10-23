import { describe, it, expect } from 'vitest';
import { cartFixableMessage, isCartFixableCheckoutError, isCheckoutKeyConflict, isPaymentInProgressError, isPaymentMethodUnavailableError, serverMessageOr } from './checkoutErrors';

describe('checkout error classification (CHECKOUT.E2E.1)', () => {
  it('stock, unavailable items and an empty cart are fixed in the cart', () => {
    for (const code of ['insufficient_stock', 'item_unavailable', 'empty_cart']) {
      expect(isCartFixableCheckoutError({ code })).toBe(true);
    }
    for (const code of ['internal_error', 'invalid_order', 'conflict', 'invalid_delivery_method', undefined]) {
      expect(isCartFixableCheckoutError({ code })).toBe(false);
    }
    expect(isCartFixableCheckoutError(null)).toBe(false);
  });

  it('detects the idempotency conflict by code (the old message check never matched)', () => {
    expect(isCheckoutKeyConflict({ code: 'conflict', message: 'Данные заказа изменились после отправки.' })).toBe(true);
    expect(isCheckoutKeyConflict({ code: 'x', rawMessage: 'idempotency key conflict' })).toBe(true);
    expect(isCheckoutKeyConflict({ code: 'insufficient_stock' })).toBe(false);
  });

  it('shows the server message for cart refusals, never internal or English text', () => {
    expect(cartFixableMessage({ code: 'insufficient_stock', rawMessage: 'Недостаточно товара на складе. Вернитесь в корзину и измените количество.' }))
      .toBe('Недостаточно товара на складе. Вернитесь в корзину и измените количество.');
    expect(cartFixableMessage({ code: 'item_unavailable', rawMessage: 'product variant not found' }))
      .toBe('Корзина изменилась: проверьте наличие и количество товаров в корзине.');
    expect(cartFixableMessage({})).toMatch(/Корзина изменилась/);
  });
});

describe('payment start refusals (PAY.P1)', () => {
  it('classifies by code', () => {
    expect(isPaymentInProgressError({ code: 'payment_in_progress' })).toBe(true);
    expect(isPaymentInProgressError({ code: 'internal_error' })).toBe(false);
    expect(isPaymentMethodUnavailableError({ code: 'payment_method_unavailable' })).toBe(true);
    expect(isPaymentMethodUnavailableError(null)).toBe(false);
  });

  it('uses the Russian server message, never English internals', () => {
    expect(serverMessageOr({ rawMessage: 'Оплата заказа ещё не завершена.' }, 'x')).toBe('Оплата заказа ещё не завершена.');
    expect(serverMessageOr({ rawMessage: 'another order is currently in payment' }, 'Запасной текст')).toBe('Запасной текст');
  });
});
