export type SellerProductStatus = 'draft' | 'pending_moderation' | 'in_review' | 'approved' | 'published' | 'rejected' | 'hidden' | 'blocked' | 'out_of_stock' | 'archived';

export type SellerProductIssue = 'low_stock' | 'weak_card' | 'ads_waste' | 'no_issue';

export interface SellerProductSize {
  size: string;
  stock: number;
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
