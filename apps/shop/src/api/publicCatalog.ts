import { getProducts, getProduct, getCategories, getBrands, getProductReviews, getProductRatingSummary } from '@zamk/api-client/src/public';
import type { Product as UIProduct, Brand as UIBrand, Category as UICategory, Review as UIReview } from '../lib/mock-data';

// Cache for brands to map brandId to brand name
let cachedBrands: Record<string, string> = {};

export async function fetchBrands(): Promise<UIBrand[]> {
  const brands = await getBrands();
  const mapped = brands.map(b => ({
    id: b.id,
    name: b.name,
    description: '',
    country: '',
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
    icon: '✦', // Default icon since backend doesn't provide one yet
    count: 0 // Mock count
  }));
}

export async function fetchProducts(params?: any): Promise<UIProduct[]> {
  // ensure brands are loaded for mapping
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {}); // best effort
  }

  const res = await getProducts(params);
  return res.items.map(p => ({
    id: p.id,
    name: p.title,
    brand: p.brandId ? (cachedBrands[p.brandId] || 'Unknown Brand') : 'Unknown Brand',
    brandId: p.brandId || '',
    price: p.priceCents / 100,
    oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
    image: p.mainImageUrl || 'https://wsrv.nl/?url=images.unsplash.com/photo-1591047139829-d91aecb6caea&w=800&output=webp',
    category: p.categoryId || 'all',
    sellerId: p.sellerId,
    rating: p.rating?.averageRating,
    reviewsCount: p.rating?.reviewCount,
    isNew: true, // Assuming new if recently created or mock
  }));
}

export async function fetchProductById(idOrSlug: string): Promise<UIProduct> {
  if (Object.keys(cachedBrands).length === 0) {
    await fetchBrands().catch(() => {});
  }
  
  const p = await getProduct(idOrSlug);
  
  return {
    id: p.id,
    name: p.title,
    brand: p.brandId ? (cachedBrands[p.brandId] || 'Unknown Brand') : 'Unknown Brand',
    brandId: p.brandId || '',
    price: p.priceCents / 100,
    oldPrice: p.oldPriceCents ? p.oldPriceCents / 100 : undefined,
    image: p.mainImageUrl || 'https://wsrv.nl/?url=images.unsplash.com/photo-1591047139829-d91aecb6caea&w=800&output=webp',
    images: p.images || (p.mainImageUrl ? [p.mainImageUrl] : []),
    category: p.categoryId || 'all',
    sellerId: p.sellerId,
    description: p.description || '',
    rating: p.rating?.averageRating,
    reviewsCount: p.rating?.reviewCount,
    // Provide some default options for UI to not break
    sizes: ['S', 'M', 'L'], 
  };
}

export async function fetchProductReviews(productId: string): Promise<UIReview[]> {
  const reviews = await getProductReviews(productId);
  return reviews.map(r => ({
    id: r.id,
    author: r.customerName || 'Аноним',
    rating: r.rating,
    text: r.content,
    date: new Date(r.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
  }));
}
