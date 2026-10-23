// CHECKOUT.E2E.1 — how the checkout reacts to a refused order creation.

type ErrorLike = { code?: string; message?: string; rawMessage?: string } | null | undefined;

/** Order refusals the buyer fixes in the cart (stock ran out, an item became unavailable, the cart is empty). */
const CART_FIXABLE = new Set(['insufficient_stock', 'item_unavailable', 'empty_cart']);

export function isCartFixableCheckoutError(error: unknown): boolean {
  const e = error as ErrorLike;
  return !!e && typeof e.code === 'string' && CART_FIXABLE.has(e.code);
}

/** The same idempotency key was sent with different order data (the buyer changed something after submitting). */
export function isCheckoutKeyConflict(error: unknown): boolean {
  const e = error as ErrorLike;
  if (!e) return false;
  if (e.code === 'conflict') return true;
  return typeof e.rawMessage === 'string' && e.rawMessage.toLowerCase().includes('idempotency key conflict');
}

/** Buyer-facing text for a cart-fixable refusal: the server's own (Russian, actionable) message when present. */
export function cartFixableMessage(error: unknown): string {
  const e = error as ErrorLike;
  const raw = typeof e?.rawMessage === 'string' ? e.rawMessage.trim() : '';
  if (raw && /[А-Яа-яЁё]/.test(raw)) return raw;
  return 'Корзина изменилась: проверьте наличие и количество товаров в корзине.';
}

// PAY.P1 — refusals when the payment is started.

/** A customer without paid orders pays one order at a time: another order's payment is still in progress. */
export function isPaymentInProgressError(error: unknown): boolean {
  const e = error as ErrorLike;
  return !!e && e.code === 'payment_in_progress';
}

/** The chosen payment method is not available (for example, the card terminal is not configured). */
export function isPaymentMethodUnavailableError(error: unknown): boolean {
  const e = error as ErrorLike;
  return !!e && e.code === 'payment_method_unavailable';
}

/** The server's Russian message when present, otherwise the given fallback. */
export function serverMessageOr(error: unknown, fallback: string): string {
  const e = error as ErrorLike;
  const raw = typeof e?.rawMessage === 'string' ? e.rawMessage.trim() : '';
  return raw && /[А-Яа-яЁё]/.test(raw) ? raw : fallback;
}
