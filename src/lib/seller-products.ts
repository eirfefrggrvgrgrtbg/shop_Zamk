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

export const SELLER_PRODUCTS_STORAGE_KEY = 'zamk_seller_products';

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

export const initialSellerProducts: SellerProduct[] = [
  {
    id: 'seller-product-anorak',
    title: 'Анорак ледяной линии',
    sku: 'ZMK-ANR-001',
    category: 'Верхняя одежда',
    brand: 'ZAMK Selected',
    price: 14900,
    oldPrice: 17900,
    cost: 7100,
    status: 'published',
    issue: 'no_issue',
    views: 12800,
    orders: 84,
    rating: 4.8,
    returns: 4,
    revenue: 1251600,
    adsSpend: 18500,
    ctr: 14.2,
    conversion: 4.6,
    quality: 92,
    mainPhoto: 'АН',
    photos: ['front', 'model', 'detail', 'size'],
    description: 'Лёгкий анорак с плотной фактурой, высоким воротом и регулируемым низом.',
    material: 'Нейлон, хлопок',
    color: 'Ледяной серый',
    season: 'Весна',
    sizes: [
      { size: 'S', stock: 5 },
      { size: 'M', stock: 8 },
      { size: 'L', stock: 5 },
    ],
    updatedAt: 'Сегодня, 09:20',
  },
  {
    id: 'seller-product-top',
    title: 'Топ базовый асимметричный',
    sku: 'ZMK-TOP-014',
    category: 'Топы',
    brand: 'ZAMK Selected',
    price: 12500,
    cost: 5200,
    status: 'needs_changes',
    issue: 'weak_card',
    views: 9100,
    orders: 19,
    rating: 4.3,
    returns: 7,
    revenue: 237500,
    adsSpend: 14200,
    ctr: 7.1,
    conversion: 1.8,
    quality: 58,
    mainPhoto: 'ТО',
    photos: ['front', 'detail'],
    description: 'Асимметричный базовый топ для многослойных образов.',
    material: 'Хлопок, эластан',
    color: 'Чёрный',
    season: 'Лето',
    sizes: [
      { size: 'XS', stock: 9 },
      { size: 'S', stock: 12 },
      { size: 'M', stock: 10 },
    ],
    updatedAt: 'Вчера, 18:10',
  },
  {
    id: 'seller-product-slipons',
    title: 'Слипоны мягкой формы',
    sku: 'ZMK-SLP-039',
    category: 'Обувь',
    brand: 'ZAMK Selected',
    price: 8400,
    oldPrice: 9200,
    cost: 3900,
    status: 'low_stock',
    issue: 'low_stock',
    views: 6200,
    orders: 36,
    rating: 4.7,
    returns: 2,
    revenue: 302400,
    adsSpend: 7200,
    ctr: 11.4,
    conversion: 4.9,
    quality: 84,
    mainPhoto: 'СЛ',
    photos: ['front', 'side', 'sole'],
    description: 'Мягкие слипоны на гибкой подошве для ежедневной носки.',
    material: 'Кожа, текстиль',
    color: 'Чёрный',
    season: 'Демисезон',
    sizes: [
      { size: '38', stock: 0 },
      { size: '39', stock: 2 },
      { size: '40', stock: 3 },
    ],
    updatedAt: 'Сегодня, 08:35',
  },
  {
    id: 'seller-product-belt',
    title: 'Ремень с матовой пряжкой',
    sku: 'ZMK-BLT-008',
    category: 'Аксессуары',
    brand: 'ZAMK Selected',
    price: 5800,
    cost: 2400,
    status: 'published',
    issue: 'ads_waste',
    views: 4100,
    orders: 8,
    rating: 4.1,
    returns: 5,
    revenue: 46400,
    adsSpend: 6100,
    ctr: 5.2,
    conversion: 1.1,
    quality: 61,
    mainPhoto: 'РМ',
    photos: ['front', 'buckle'],
    description: 'Кожаный ремень с матовой металлической пряжкой.',
    material: 'Кожа',
    color: 'Графит',
    season: 'Всесезон',
    sizes: [
      { size: '90', stock: 14 },
      { size: '100', stock: 16 },
      { size: '110', stock: 14 },
    ],
    updatedAt: '23 апреля, 16:40',
  },
];

export function loadSellerProducts() {
  try {
    const raw = window.localStorage.getItem(SELLER_PRODUCTS_STORAGE_KEY);
    if (!raw) return initialSellerProducts;
    const parsed = JSON.parse(raw) as SellerProduct[];
    return Array.isArray(parsed) && parsed.length > 0 ? parsed : initialSellerProducts;
  } catch {
    return initialSellerProducts;
  }
}

export function saveSellerProducts(products: SellerProduct[]) {
  window.localStorage.setItem(SELLER_PRODUCTS_STORAGE_KEY, JSON.stringify(products));
}

export function upsertSellerProduct(product: SellerProduct) {
  const products = loadSellerProducts();
  const exists = products.some((item) => item.id === product.id);
  const nextProducts = exists ? products.map((item) => (item.id === product.id ? product : item)) : [product, ...products];
  saveSellerProducts(nextProducts);
  return nextProducts;
}
