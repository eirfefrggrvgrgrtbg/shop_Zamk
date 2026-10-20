export const PROMO_STORAGE_KEY = 'zamk_promo_code';

const PROMO_REASON_MESSAGES: Record<string, string> = {
  promo_not_found: 'Промокод не найден. Проверьте написание.',
  promo_inactive: 'Этот промокод сейчас не действует.',
  promo_not_started: 'Срок действия промокода ещё не начался.',
  promo_expired: 'Срок действия промокода истёк.',
  promo_min_subtotal: 'Сумма товаров меньше минимальной для этого промокода.',
  promo_min_quantity: 'В корзине недостаточно подходящих товаров для этого промокода.',
  promo_min_distinct_products: 'Для этого промокода нужно больше разных товаров в корзине.',
  promo_global_limit: 'Промокод исчерпан.',
  promo_customer_limit: 'Вы уже использовали этот промокод.',
  promo_first_order_only: 'Промокод действует только на первый заказ.',
  promo_repeat_customer_required: 'Промокод действует только для повторных покупателей.',
  promo_not_applicable: 'Промокод не подходит к товарам в корзине.',
  promo_budget_exhausted: 'Промокод больше недоступен.',
  promo_full_discount_not_allowed: 'Промокод нельзя применить: скидка не может покрывать всю стоимость товара.',
};

export const PROMO_GENERIC_MESSAGE = 'Не удалось применить промокод.';
export const PROMO_NETWORK_MESSAGE = 'Не удалось проверить промокод. Проверьте соединение и попробуйте ещё раз.';

type ErrorLike = { code?: string; status?: number; rawMessage?: string; message?: string };

/** True when the error means the promo itself was rejected (not a transport failure). */
export function isPromoRejection(error: unknown): boolean {
  const e = error as ErrorLike | null;
  if (!e) return false;
  if (e.code === 'invalid_promo') return true;
  return e.code === 'invalid_order' && typeof e.rawMessage === 'string' && e.rawMessage.startsWith('promo_');
}

export function isNetworkError(error: unknown): boolean {
  const e = error as ErrorLike | null;
  return !!e && (e.code === 'NETWORK_ERROR' || e.code === 'TIMEOUT_ERROR');
}

export function promoErrorMessage(error: unknown): string {
  if (isNetworkError(error)) return PROMO_NETWORK_MESSAGE;
  const e = error as ErrorLike | null;
  const reason = e?.rawMessage;
  if (reason && PROMO_REASON_MESSAGES[reason]) return PROMO_REASON_MESSAGES[reason];
  if (reason === 'empty cart' || reason === 'cart is empty') return 'В корзине нет товаров.';
  return PROMO_GENERIC_MESSAGE;
}

export function normalizePromoCode(raw: string): string {
  return raw.trim().toUpperCase();
}

/** Stable signature of the cart contents; a quote is valid only for the signature it was computed for. */
export function cartSignature(items: Array<{ productVariantId: string; quantity: number }>): string {
  return items
    .map((i) => `${i.productVariantId}:${i.quantity}`)
    .sort()
    .join('|');
}
