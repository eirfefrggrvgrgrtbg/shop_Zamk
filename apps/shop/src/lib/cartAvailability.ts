import type { UIContextCartItem } from '../contexts/CartContext';

/** Русское объяснение, почему позиция корзины сейчас недоступна; null — позиция доступна. */
export function cartUnavailableMessage(item: Pick<UIContextCartItem, 'unavailableReason' | 'availableQuantity' | 'inStock'>): string | null {
  switch (item.unavailableReason) {
    case 'product_unpublished':
      return 'Товар снят с продажи';
    case 'variant_inactive':
      return 'Этот вариант больше не продаётся';
    case 'out_of_stock':
      return 'Товар закончился';
    case 'insufficient_stock':
      return `В наличии только ${item.availableQuantity ?? 0} шт.`;
    default:
      // Older API responses carry only inStock.
      return item.inStock === false ? 'Товар закончился' : null;
  }
}

/** Можно ли уменьшить количество до доступного (позиция жива, но остатка меньше, чем в корзине). */
export function cartCanShrinkToAvailable(item: Pick<UIContextCartItem, 'unavailableReason' | 'availableQuantity'>): boolean {
  return item.unavailableReason === 'insufficient_stock' && (item.availableQuantity ?? 0) > 0;
}

const PRICE_KEY = 'zamk.cart.lastPrices.v1';

export type CartPriceChanges = Record<string, { from: number; to: number }>;
type PriceItem = Pick<UIContextCartItem, 'productVariantId' | 'price'>;

/**
 * Цены, которые покупатель видел при прошлом открытии корзины: только variantId → число,
 * никаких персональных данных. Пусто при первом открытии или если хранилище недоступно.
 */
export function readLastSeenCartPrices(): Record<string, number> {
  try {
    const raw = JSON.parse(localStorage.getItem(PRICE_KEY) || '{}') as unknown;
    if (!raw || typeof raw !== 'object') return {};
    return Object.fromEntries(Object.entries(raw as Record<string, unknown>).filter(([, v]) => typeof v === 'number')) as Record<string, number>;
  } catch {
    return {};
  }
}

/** Запоминает актуальные серверные цены. Записи об удалённых позициях отбрасываются. */
export function saveLastSeenCartPrices(items: PriceItem[]): void {
  const next: Record<string, number> = {};
  for (const it of items) if (typeof it.price === 'number') next[it.productVariantId] = it.price;
  try {
    localStorage.setItem(PRICE_KEY, JSON.stringify(next));
  } catch {
    /* хранилище недоступно: уведомления не будет, цена по-прежнему приходит с сервера */
  }
}

/** Чистая функция: что изменилось относительно ранее виденных цен. Нет baseline — нет уведомления. */
export function diffCartPrices(lastSeen: Record<string, number>, items: PriceItem[]): CartPriceChanges {
  const changes: CartPriceChanges = {};
  for (const it of items) {
    const was = lastSeen[it.productVariantId];
    if (typeof it.price === 'number' && typeof was === 'number' && was !== it.price) {
      changes[it.productVariantId] = { from: was, to: it.price };
    }
  }
  return changes;
}
