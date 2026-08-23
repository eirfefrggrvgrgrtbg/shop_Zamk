import { getProducts, getDirectSaleProducts, getProduct, getCategories, getBrands, getProductReviews, getPublicSeller, getProductPreviewByToken } from '@zamk/api-client/src/public';
import type { ProductSummary } from '@zamk/api-client/src/types';
import type { Product as UIProduct, Brand as UIBrand, Category as UICategory, Review as UIReview } from '../types/catalog';

export const PRODUCT_PLACEHOLDER_IMAGE =
  'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="900" height="1200" viewBox="0 0 900 1200"><rect width="900" height="1200" fill="%23f1f5f9"/><rect x="140" y="220" width="620" height="760" rx="42" fill="none" stroke="%23cbd5e1" stroke-width="12" stroke-dasharray="28 24"/><text x="450" y="610" text-anchor="middle" font-family="Arial,sans-serif" font-size="38" fill="%2364758b">Нет изображения</text></svg>';

// Cache for brands to map brandId to brand name
let cachedBrands: Record<string, string> = {};

export function mapProductSummaryToCatalog(
  p: ProductSummary & { title?: string; priceCents?: number; oldPriceCents?: number; mainImageUrl?: string },
  brandName?: string
): UIProduct {
  const priceCents = p.priceCents ?? 0;
  const oldPriceCents = p.oldPriceCents;
  const price = priceCents / 100;
  const oldPrice = oldPriceCents ? oldPriceCents / 100 : undefined;

  return {
    id: p.id,
    name: p.title,
    brand: brandName || (p.brandId ? (cachedBrands[p.brandId] || 'Бренд не указан') : 'Бренд не указан'),
    brandId: p.brandId || '',
    price,
    oldPrice,
    discountPrice: oldPrice && oldPrice > price ? price : undefined,
    image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
    images: p.mainImageUrl ? [{ url: p.mainImageUrl }] : [],
    category: p.categoryId || 'Категория не указана',
    sellerId: p.sellerId,
    sellerSlug: p.sellerSlug,
    sellerName: p.sellerName,
    rating: p.rating?.average,
    reviewsCount: p.rating?.count,
    isNew: false,
  };
}

export async function mapFavoritesToCatalog(items: ProductSummary[]): Promise<UIProduct[]> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }
  return items.map((p) => mapProductSummaryToCatalog(p));
}

export async function fetchBrands(): Promise<UIBrand[]> {
  const brands = await getBrands();
  const mapped = brands.map(b => ({
    id: b.id,
    name: b.name,
    description: 'Описание бренда пока не указано.',
    country: 'Страна не указана',
    image: b.logoUrl || ''
  }));

  cachedBrands = {};
  mapped.forEach(b => { cachedBrands[b.id] = b.name; });
  return mapped;
}

export async function fetchCategories(): Promise<UICategory[]> {
  const cats = await getCategories();
  return cats.map(c => ({
    id: c.id,
    slug: c.slug,
    name: c.name,
    icon: '✦',
  }));
}

export async function fetchProducts(params?: any): Promise<{ items: UIProduct[], totalCount: number }> {
  // ensure brands are loaded for mapping
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {}); // best effort
  }

  const res = await getProducts(params);
  return {
    items: res.items.map(p => ({
      id: p.id,
      name: p.title,
      brand: p.brandId ? (cachedBrands[p.brandId] || 'Бренд не указан') : 'Бренд не указан',
      brandId: p.brandId || '',
      price: p.priceCents / 100,
      oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
      image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
      category: p.categoryId || 'Категория не указана',
      sellerId: p.sellerId,
      sellerSlug: p.sellerSlug,
      sellerName: p.sellerName,
      rating: p.rating?.average,
      reviewsCount: p.rating?.count,
      isNew: false,
    })),
    totalCount: res.totalCount
  };
}

export async function fetchDirectSaleProducts(params?: any): Promise<{ items: UIProduct[], totalCount: number }> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }

  const res = await getDirectSaleProducts(params);
  return {
    items: res.items.map(p => ({
      id: p.id,
      name: p.title,
      brand: p.brandId ? (cachedBrands[p.brandId] || 'Бренд не указан') : 'Бренд не указан',
      brandId: p.brandId || '',
      price: p.priceCents / 100,
      oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
      image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
      category: p.categoryId || 'Категория не указана',
      sellerId: p.sellerId,
      rating: p.rating?.average,
      reviewsCount: p.rating?.count,
      isNew: false,
    })),
    totalCount: res.totalCount
  };
}

export async function fetchPublicSeller(slugOrId: string, params?: any): Promise<{ seller: any, items: UIProduct[], totalCount: number }> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }

  const res = await getPublicSeller(slugOrId, params);

  return {
    seller: res.seller,
    items: res.items.map((p: any) => ({
      id: p.id,
      name: p.title,
      brand: p.brandId ? (cachedBrands[p.brandId] || 'Бренд не указан') : 'Бренд не указан',
      brandId: p.brandId || '',
      price: p.priceCents / 100,
      oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
      image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
      category: p.categoryId || 'Категория не указана',
      sellerId: p.sellerId,
      sellerSlug: p.sellerSlug,
      sellerName: p.sellerName,
      rating: p.rating?.average,
      reviewsCount: p.rating?.count,
      isNew: false,
    })),
    totalCount: res.totalCount
  };
}

export async function fetchProductById(idOrSlug: string): Promise<UIProduct> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }

  const p = await getProduct(idOrSlug);

  return {
    id: p.id,
    name: p.title,
    brand: p.brandId ? (cachedBrands[p.brandId] || 'Бренд не указан') : 'Бренд не указан',
    brandId: p.brandId || '',
    price: p.priceCents / 100,
    oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
    image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
    images: (p.images && p.images.length > 0) ? p.images.map((img: any) => ({ url: img.imageUrl || img.imageURL || img.url, colorId: img.colorId })) : (p.mainImageUrl ? [{ url: p.mainImageUrl }] : []),
    category: p.categoryId || 'Категория не указана',
    sellerId: p.sellerId,
    sellerSlug: p.sellerSlug,
    sellerName: p.sellerName,
    description: p.description || '',
    materialComposition: (p as any).materialComposition,
    careInstructions: (p as any).careInstructions,
    sizeChart: (p as any).sizeChart,
    rating: p.rating?.average,
    reviewsCount: p.rating?.count,
    sizes: p.variants?.map(v => v.size).filter(Boolean) as string[] || [],
    variants: p.variants?.map(v => ({
      id: v.id,
      size: v.size,
      color: v.color,
      inStock: v.inStock ?? v.isActive,
      isActive: v.isActive,
      sellerSku: v.sellerSku,
      price: v.priceCents ? v.priceCents / 100 : undefined,
      colorName: (v as any).colorName,
      colorHex: (v as any).colorHex,
      colorId: v.colorId,
      sizeValueId: v.sizeValueId
    }))
  };
}

export async function fetchProductReviews(productId: string): Promise<UIReview[]> {
  const reviews = await getProductReviews(productId);
  return reviews.map(r => ({
    id: r.id,
    author: r.customerName || 'Аноним',
    rating: r.rating,
    text: r.comment || '',
    date: new Date(r.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
  }));
}

export async function fetchProductPreviewByToken(token: string): Promise<UIProduct> {
  const p = await getProductPreviewByToken(token);

  const images = (p.images && p.images.length > 0) ? p.images.map((img: any) => ({ url: img.imageUrl || img.imageURL || img.url, colorId: img.colorId })) : (p.mainImageUrl ? [{ url: p.mainImageUrl }] : [{ url: PRODUCT_PLACEHOLDER_IMAGE }]);

  return {
    id: p.id,
    name: p.title,
    brand: p.brand?.name || '',
    brandId: p.brand?.id || '',
    price: p.priceCents ? p.priceCents / 100 : 0,
    image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
    images: images,
    category: p.category?.name || '',
    sellerId: p.sellerId,
    sellerSlug: p.sellerSlug,
    sellerName: p.sellerName,
    description: p.description || '',
    materialComposition: (p as any).materialComposition,
    careInstructions: (p as any).careInstructions,
    sizeChart: (p as any).sizeChart,
    materials: p.characteristics?.['Материал'] || '',
    rating: p.rating,
    reviewsCount: p.reviewsCount,
    sizes: p.variants?.map((v: any) => v.size).filter(Boolean) || [],
    colors: p.variants?.filter((v: any) => v.color).map((v: any) => ({ name: v.color, hex: '#000000' })) || [],
    variants: p.variants?.map((v: any) => ({
      id: v.id,
      size: v.size,
      color: v.color,
      inStock: v.inStock ?? v.isActive,
      isActive: v.isActive,
      sellerSku: v.sellerSku,
      price: v.priceCents ? v.priceCents / 100 : undefined,
      colorName: (v as any).colorName,
      colorHex: (v as any).colorHex,
      colorId: v.colorId,
      sizeValueId: v.sizeValueId
    })) || [],
    isPreview: true,
  };
}
