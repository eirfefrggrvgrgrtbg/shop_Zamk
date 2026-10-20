import { getProducts, getDirectSaleProducts, getProduct, getCategories, getBrands, getProductReviews, getPublicSeller, getProductPreviewByToken, getSimilarProducts, getPopularProducts, getCollections, getCollection } from '@zamk/api-client/src/public';
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
    brand: brandName || (p.brandId ? (cachedBrands[p.brandId] || p.sellerName || 'Бренд не указан') : (p.sellerName || 'Бренд не указан')),
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
  const activeBrands = (brands || []).filter(b => (b as any).isActive !== false);

  const seenIds = new Set<string>();
  const seenNames = new Set<string>();
  const mapped: UIBrand[] = [];

  for (const b of activeBrands) {
    if (!b || !b.id || !b.name) continue;
    const normName = b.name.trim().toLowerCase();
    if (seenIds.has(b.id) || seenNames.has(normName)) {
      continue;
    }
    seenIds.add(b.id);
    seenNames.add(normName);
    mapped.push({
      id: b.id,
      name: b.name.trim(),
      slug: b.slug || b.id,
      description: b.description || '',
      country: '',
      image: b.logoUrl || '',
      isActive: true,
    });
  }

  mapped.sort((a, b) => a.name.localeCompare(b.name, 'ru', { sensitivity: 'base' }));

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

export interface FetchProductsOptions {
  isCustomer?: boolean;
}

export async function fetchProducts(params?: any, options?: FetchProductsOptions): Promise<{ items: UIProduct[], totalCount: number }> {
  // ensure brands are loaded for mapping
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {}); // best effort
  }

  const isDefaultSort = !params?.sort || params.sort === 'default' || params.sort === '';
  const useCustomerCatalog = Boolean(options?.isCustomer && isDefaultSort);

  let res;
  if (useCustomerCatalog) {
    try {
      const { getCustomerCatalog } = await import('@zamk/api-client/src/customer');
      const cleanParams = { ...params };
      if (cleanParams.sort === 'default') {
        delete cleanParams.sort;
      }
      res = await getCustomerCatalog(cleanParams);
    } catch (err) {
      // Fail-open fallback: if customer catalog fails, fallback once to public products with identical params
      console.warn('Customer catalog fetch failed, falling back to public products:', err);
      const cleanParams = { ...params };
      if (cleanParams.sort === 'default') {
        delete cleanParams.sort;
      }
      res = await getProducts(cleanParams);
    }
  } else {
    const cleanParams = { ...params };
    if (cleanParams.sort === 'default') {
      delete cleanParams.sort;
    }
    res = await getProducts(cleanParams);
  }

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

export async function fetchRecentlyViewed(limit = 12): Promise<{ items: UIProduct[], totalCount: number }> {
  const { getCustomerRecentlyViewed } = await import('@zamk/api-client/src/customer');
  try {
    const res = await getCustomerRecentlyViewed(limit);
    const items = await mapFavoritesToCatalog(res.items);
    return { items, totalCount: res.totalCount };
  } catch (error) {
    // If not authenticated or other error, degrade gracefully
    return { items: [], totalCount: 0 };
  }
}

export async function fetchForYouProducts(limit = 12): Promise<{ items: UIProduct[], totalCount: number }> {
  const { getCustomerForYou } = await import('@zamk/api-client/src/customer');
  try {
    const res = await getCustomerForYou(limit);
    const items = await mapFavoritesToCatalog(res.items);
    return { items, totalCount: res.totalCount };
  } catch (error) {
    return { items: [], totalCount: 0 };
  }
}

export interface UIHomeRecommendationBlock {
  type: 'for_you' | 'popular' | 'new';
  title: string;
  items: UIProduct[];
}

export interface UIHomeRecommendationsResponse {
  blocks: UIHomeRecommendationBlock[];
}

export async function fetchHomeRecommendations(): Promise<UIHomeRecommendationsResponse> {
  const { getCustomerHomeRecommendations } = await import('@zamk/api-client/src/customer');
  try {
    const res = await getCustomerHomeRecommendations();
    const blocks: UIHomeRecommendationBlock[] = [];
    for (const b of (res.blocks || [])) {
      const items = await mapFavoritesToCatalog(b.items || []);
      blocks.push({
        type: b.type,
        title: b.title,
        items,
      });
    }
    return { blocks };
  } catch (error) {
    return { blocks: [] };
  }
}

/** Public (guest-safe) popular products: 30-day paid-order ranking, storefront-eligible only. */
export async function fetchPopularProducts(limit = 12): Promise<{ items: UIProduct[], totalCount: number }> {
  try {
    const res = await getPopularProducts(limit);
    const items = await mapFavoritesToCatalog(res.items);
    return { items, totalCount: res.totalCount };
  } catch {
    return { items: [], totalCount: 0 };
  }
}

export async function fetchSimilarProducts(productId: string, limit = 8): Promise<{ items: UIProduct[], totalCount: number }> {
  try {
    const res = await getSimilarProducts(productId, limit);
    const items = await mapFavoritesToCatalog(res.items);
    return { items, totalCount: res.totalCount };
  } catch (error) {
    return { items: [], totalCount: 0 };
  }
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

export interface PublicBrandStorefront {
  brand: {
    id: string;
    name: string;
    slug: string;
    description?: string;
    logoUrl?: string;
    coverUrl?: string;
  };
  items: UIProduct[];
  totalCount: number;
}

export async function fetchPublicBrand(slugOrId: string, params?: any): Promise<PublicBrandStorefront> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }

  let res: any;
  try {
    const { getPublicBrand } = await import('@zamk/api-client/src/public');
    res = await getPublicBrand(slugOrId, params);
  } catch (err: any) {
    // If brand endpoint gave 404 or failed, fallback to getPublicSeller
    res = await getPublicSeller(slugOrId, params);
  }

  const brandData = res.brand || res.seller || {};
  return {
    brand: {
      id: brandData.id,
      name: brandData.name || brandData.brandName || 'Бренд',
      slug: brandData.slug || slugOrId,
      description: brandData.description || undefined,
      logoUrl: brandData.logoUrl || undefined,
      coverUrl: brandData.coverUrl || undefined,
    },
    items: (res.items || []).map((p: any) => ({
      id: p.id,
      name: p.title,
      brand: (p.brandId && cachedBrands[p.brandId]) || p.brand || brandData.name || brandData.brandName || 'Бренд не указан',
      brandId: p.brandId || brandData.id || '',
      price: p.priceCents / 100,
      oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
      discountPrice: p.oldPriceCents && p.oldPriceCents > p.priceCents ? p.priceCents / 100 : undefined,
      image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
      images: p.mainImageUrl ? [{ url: p.mainImageUrl }] : [],
      category: p.categoryId || 'Категория не указана',
      sellerId: p.sellerId,
      sellerSlug: p.sellerSlug,
      sellerName: p.sellerName,
      rating: p.rating?.average,
      reviewsCount: p.rating?.count,
      isNew: false,
    })),
    totalCount: res.totalCount ?? (res.items ? res.items.length : 0),
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
      brand: p.brandId ? (cachedBrands[p.brandId] || p.sellerName || 'Бренд не указан') : (p.sellerName || 'Бренд не указан'),
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
    brand: p.brandId ? (cachedBrands[p.brandId] || p.sellerName || 'Бренд не указан') : (p.sellerName || 'Бренд не указан'),
    brandId: p.brandId || '',
    price: p.priceCents / 100,
    oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
    image: p.mainImageUrl || PRODUCT_PLACEHOLDER_IMAGE,
    images: (p.images && p.images.length > 0) ? p.images.map((img: any) => ({ url: img.renditionUrl || img.renditionURL || img.imageUrl || img.imageURL || img.url, colorId: img.colorId })) : (p.mainImageUrl ? [{ url: p.mainImageUrl }] : []),
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
    colors: p.variants?.filter(v => v.colorName || v.color).map(v => ({
      name: (v.colorName || v.color)!,
      hex: v.colorHex || undefined,
    })) || [],
    variants: p.variants?.map(v => ({
      id: v.id,
      size: v.size,
      color: v.colorName || v.color,
      inStock: v.inStock ?? v.isActive,
      isActive: v.isActive,
      sellerSku: v.sellerSku,
      price: v.priceCents ? v.priceCents / 100 : undefined,
      colorName: v.colorName || v.color,
      colorHex: v.colorHex || undefined,
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

  const images = (p.images && p.images.length > 0) ? p.images.map((img: any) => ({ url: img.renditionUrl || img.renditionURL || img.imageUrl || img.imageURL || img.url, colorId: img.colorId })) : (p.mainImageUrl ? [{ url: p.mainImageUrl }] : [{ url: PRODUCT_PLACEHOLDER_IMAGE }]);

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
    colors: p.variants?.filter((v: any) => v.colorName || v.color).map((v: any) => ({
      name: v.colorName || v.color,
      hex: v.colorHex || undefined,
    })) || [],
    variants: p.variants?.map((v: any) => ({
      id: v.id,
      size: v.size,
      color: v.colorName || v.color,
      inStock: v.inStock ?? v.isActive,
      isActive: v.isActive,
      sellerSku: v.sellerSku,
      price: v.priceCents ? v.priceCents / 100 : undefined,
      colorName: v.colorName || v.color,
      colorHex: v.colorHex || undefined,
      colorId: v.colorId,
      sizeValueId: v.sizeValueId
    })) || [],
    isPreview: true,
  };
}


export interface UICollectionSummary {
  id: string;
  slug: string;
  title: string;
  description?: string;
  coverImageUrl?: string;
}

export interface UICollectionDetail extends UICollectionSummary {
  products: UIProduct[];
}

export async function fetchCollections(): Promise<UICollectionSummary[]> {
  const items = await getCollections();
  return items.map((c) => ({
    id: c.id,
    slug: c.slug,
    title: c.title,
    description: c.description || undefined,
    coverImageUrl: c.coverImageUrl || undefined,
  }));
}

/** Returns null when the collection is not published (404 from the API). */
export async function fetchCollectionBySlug(slug: string): Promise<UICollectionDetail | null> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }
  let res;
  try {
    res = await getCollection(slug);
  } catch (e) {
    if ((e as { status?: number })?.status === 404) return null;
    throw e;
  }
  return {
    id: res.id,
    slug: res.slug,
    title: res.title,
    description: res.description || undefined,
    coverImageUrl: res.coverImageUrl || undefined,
    products: res.products.map((p) =>
      mapProductSummaryToCatalog(
        { ...p, slug: '', createdAt: '', status: 'published' },
        (p.brandId && cachedBrands[p.brandId]) || p.sellerName || undefined,
      ),
    ),
  };
}
