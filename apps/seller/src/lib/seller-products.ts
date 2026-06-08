export type SellerProductStatus = 'published' | 'draft' | 'moderation' | 'needs_changes' | 'low_stock' | 'paused';

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
}

export const statusLabels: Record<SellerProductStatus, string> = {
  published: 'Опубликован',
  draft: 'Черновик',
  moderation: 'На модерации',
  needs_changes: 'Нужны правки',
  low_stock: 'Заканчивается',
  paused: 'Снят с продажи',
};

export const issueLabels: Record<SellerProductIssue, string> = {
  low_stock: 'Мало остатков',
  weak_card: 'Слабая карточка',
  ads_waste: 'Реклама не окупается',
  no_issue: 'Без проблем',
};
