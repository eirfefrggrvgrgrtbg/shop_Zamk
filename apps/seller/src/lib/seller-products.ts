export type SellerProductStatus = 'draft' | 'pending_moderation' | 'in_review' | 'approved' | 'published' | 'rejected' | 'hidden' | 'blocked' | 'out_of_stock' | 'archived';

export type SellerProductIssue = 'low_stock' | 'weak_card' | 'ads_waste' | 'no_issue';

export interface SellerProductSize {
  size: string;
  stock: number;
  /** False when the SKU is switched off; undefined when the backend did not say. */
  active?: boolean;
}

export interface SellerProduct {
  id: string;
  title: string;
  sku: string;
  category: string;
  brand: string;
  price: number;
  oldPrice?: number;
  cost: number;
  status: SellerProductStatus;
  issue: SellerProductIssue;
  views: number;
  orders: number;
  rating: number;
  returns: number;
  revenue: number;
  adsSpend: number;
  ctr: number;
  conversion: number;
  quality: number;
  mainPhoto: string;
  photos: string[];
  description: string;
  material: string;
  color: string;
  season: string;
  sizes: SellerProductSize[];
  updatedAt: string;
  rejectionReason?: string;
  actualVisibility?: boolean;
  storefrontUrl?: string;
  visibilityReasons?: string[];
}

export const statusLabels: Record<SellerProductStatus, string> = {
  draft: 'Черновик',
  pending_moderation: 'На модерации',
  in_review: 'На проверке',
  approved: 'Одобрен',
  published: 'Опубликован',
  rejected: 'Нужны исправления',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
  out_of_stock: 'Нет в наличии',
  archived: 'В архиве',
};

export const issueLabels: Record<SellerProductIssue, string> = {
  low_stock: 'Мало остатков',
  weak_card: 'Слабая карточка',
  ads_waste: 'Реклама не окупается',
  no_issue: 'Без проблем',
};

export function getStorefrontStatusInfo(product: {
  status: SellerProductStatus;
  actualVisibility?: boolean;
  visibilityReasons?: string[];
  sizes?: Array<{ stock: number }>;
}): { label: string; toneClass: string } {
  // 1. Explicit lifecycle statuses that dictate storefront state
  if (
    product.status === 'draft' ||
    product.status === 'pending_moderation' ||
    product.status === 'in_review' ||
    product.status === 'rejected' ||
    product.status === 'approved'
  ) {
    return {
      label: 'Не опубликован',
      toneClass: 'text-gray-600 dark:text-gray-400 font-medium',
    };
  }

  if (product.status === 'hidden') {
    return {
      label: 'Скрыт из каталога',
      toneClass: 'text-amber-600 dark:text-amber-400 font-medium',
    };
  }

  if (product.status === 'blocked') {
    return {
      label: 'Заблокирован',
      toneClass: 'text-red-600 dark:text-red-400 font-medium',
    };
  }

  if (product.status === 'out_of_stock') {
    return {
      label: 'Нет в наличии',
      toneClass: 'text-orange-600 dark:text-orange-400 font-medium',
    };
  }

  if (product.status === 'archived') {
    return {
      label: 'Снят с продажи',
      toneClass: 'text-gray-500 dark:text-gray-400 font-medium',
    };
  }

  // 2. Published products
  if (product.status === 'published') {
    if (product.actualVisibility) {
      return {
        label: 'В продаже',
        toneClass: 'text-emerald-600 dark:text-emerald-400 font-semibold',
      };
    }

    // Published but actualVisibility is false -> derive truthful reason
    if (product.visibilityReasons?.includes('low_stock')) {
      const totalStock = product.sizes?.reduce((sum, s) => sum + (s.stock || 0), 0) ?? 0;
      return {
        label: totalStock <= 0 ? 'Нет в наличии' : 'Недостаточно остатков',
        toneClass: 'text-orange-600 dark:text-orange-400 font-medium',
      };
    }

    if (product.visibilityReasons?.includes('product_hidden')) {
      return {
        label: 'Скрыт из каталога',
        toneClass: 'text-amber-600 dark:text-amber-400 font-medium',
      };
    }

    if (product.visibilityReasons?.includes('product_blocked')) {
      return {
        label: 'Заблокирован',
        toneClass: 'text-red-600 dark:text-red-400 font-medium',
      };
    }

    if (product.visibilityReasons?.includes('seller_inactive')) {
      return {
        label: 'Магазин неактивен',
        toneClass: 'text-amber-600 dark:text-amber-400 font-medium',
      };
    }

    if (product.visibilityReasons?.includes('no_active_variants')) {
      return {
        label: 'Нет активных вариантов',
        toneClass: 'text-amber-600 dark:text-amber-400 font-medium',
      };
    }

    if (product.visibilityReasons?.includes('invalid_price')) {
      return {
        label: 'Не указана цена',
        toneClass: 'text-amber-600 dark:text-amber-400 font-medium',
      };
    }

    return {
      label: 'Скрыт из каталога',
      toneClass: 'text-amber-600 dark:text-amber-400 font-medium',
    };
  }

  return {
    label: 'Не опубликован',
    toneClass: 'text-gray-600 dark:text-gray-400 font-medium',
  };
}

/**
 * Who has to act for a product to become purchasable. The seller never receives warehouse actions: "warehouse" only
 * tells them that stock comes from a supply to the ZAMK warehouse.
 */
export type AvailabilityOwner = 'seller' | 'moderation' | 'warehouse' | 'info';

export const availabilityOwnerLabels: Record<AvailabilityOwner, string> = {
  seller: 'Действие продавца',
  moderation: 'Решение модерации ZAMK',
  warehouse: 'Склад ZAMK',
  info: 'Информация',
};

export interface AvailabilityReason {
  code: string;
  text: string;
  owner: AvailabilityOwner;
}

export interface ProductAvailability {
  /** available: a buyer can order now (the backend says so); unavailable: the backend says not; unknown: no data. */
  state: 'available' | 'unavailable' | 'unknown';
  label: string;
  reasons: AvailabilityReason[];
  /** Only for an available product: some active SKUs are sold out. */
  skuTotal: number;
  skuSellable: number;
  partial: boolean;
}

const REASON_ORDER = ['seller_inactive', 'product_blocked', 'product_hidden', 'product_archived', 'moderation_required', 'no_active_variants', 'invalid_price', 'low_stock'];

function describeReason(code: string, product: Pick<SellerProduct, 'status' | 'sizes'>): AvailabilityReason {
  switch (code) {
    case 'seller_inactive':
      return { code, text: 'Магазин сейчас не активен', owner: 'info' };
    case 'product_blocked':
      return { code, text: 'Товар заблокирован ZAMK', owner: 'moderation' };
    case 'product_hidden':
      return { code, text: 'Товар скрыт из каталога', owner: 'info' };
    case 'product_archived':
      return { code, text: 'Товар в архиве', owner: 'seller' };
    case 'moderation_required':
      switch (product.status) {
        case 'draft':
          return { code, text: 'Не отправлен на модерацию', owner: 'seller' };
        case 'pending_moderation':
        case 'in_review':
          return { code, text: 'Ожидает проверки ZAMK', owner: 'moderation' };
        case 'rejected':
          return { code, text: 'Нужны исправления после модерации', owner: 'seller' };
        case 'approved':
          return { code, text: 'Одобрен, но ещё не опубликован', owner: 'info' };
        default:
          return { code, text: 'Товар не опубликован', owner: 'info' };
      }
    case 'no_active_variants':
      return { code, text: 'Нет активных вариантов (SKU)', owner: 'seller' };
    case 'invalid_price':
      return { code, text: 'Не указана цена', owner: 'seller' };
    case 'low_stock': {
      const free = (product.sizes || []).reduce((sum, s) => sum + (s.active === false ? 0 : s.stock || 0), 0);
      return {
        code,
        text: free <= 0 ? 'Нет доступного остатка на складе ZAMK' : 'Остатка на складе ZAMK недостаточно для показа в каталоге',
        owner: 'warehouse',
      };
    }
    default:
      return { code, text: 'Недоступен для покупки', owner: 'info' };
  }
}

/**
 * Presents the backend's own verdict (actualVisibility + visibilityReasons, the same rules the Shop catalog uses).
 * It never decides eligibility itself: without the verdict the state is "unknown", not a guess.
 */
export function getProductAvailability(product: Pick<SellerProduct, 'status' | 'sizes' | 'actualVisibility' | 'visibilityReasons'>): ProductAvailability {
  const skus = (product.sizes || []).filter((s) => s.active !== false);
  const skuTotal = skus.length;
  const skuSellable = skus.filter((s) => (s.stock || 0) > 0).length;

  if (product.actualVisibility === undefined) {
    return { state: 'unknown', label: 'Нет данных', reasons: [], skuTotal, skuSellable, partial: false };
  }
  if (product.actualVisibility) {
    return { state: 'available', label: 'Можно купить', reasons: [], skuTotal, skuSellable, partial: skuTotal > 1 && skuSellable > 0 && skuSellable < skuTotal };
  }
  const codes = (product.visibilityReasons || []).slice().sort((a, b) => REASON_ORDER.indexOf(a) - REASON_ORDER.indexOf(b));
  return {
    state: 'unavailable',
    label: 'Нельзя купить',
    reasons: codes.map((code) => describeReason(code, product)),
    skuTotal,
    skuSellable,
    partial: false,
  };
}

/** Per-SKU state inside a product: only facts the DTO carries. */
export function getSkuState(size: SellerProductSize): { label: string; tone: 'good' | 'warning' | 'neutral' } {
  if (size.active === false) return { label: 'Выключен', tone: 'neutral' };
  if ((size.stock || 0) > 0) return { label: 'Есть в наличии', tone: 'good' };
  return { label: 'Нет остатка', tone: 'warning' };
}
