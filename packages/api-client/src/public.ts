import { request } from './client';
import type { ProductSummary, ProductDetail, Category, Brand, PublicReview, RatingSummary, AuctionEvent, AuctionLot, PublicDeliveryMethod } from './types';

export interface ProductListResponse {
  items: ProductSummary[];
  totalCount: number;
}

export const getProducts = async (params?: any): Promise<ProductListResponse> => {
  const res = await request<ProductListResponse>('GET', '/public/products', { params });
  return { ...res, items: res?.items || [] };
};

export const getDirectSaleProducts = async (params?: any): Promise<ProductListResponse> => {
  const res = await request<ProductListResponse>('GET', '/public/direct-sale', { params });
  return { ...res, items: res?.items || [] };
};

export const getProduct = async (idOrSlug: string): Promise<ProductDetail> => {
  const res = await request<any>('GET', `/public/products/${idOrSlug}`);
  if (res) {
    res.images = res.images || [];
    res.variants = res.variants || [];
  }
  return res;
};

export const getSimilarProducts = async (productId: string, limit = 8): Promise<ProductListResponse> => {
  const res = await request<ProductListResponse>('GET', `/public/products/${productId}/similar`, { params: { limit } });
  return { ...res, items: res?.items || [] };
};

export const getPopularProducts = async (limit = 12): Promise<ProductListResponse> => {
  const res = await request<ProductListResponse>('GET', '/public/products/popular', { params: { limit } });
  return { ...res, items: res?.items || [] };
};

export const getProductPreviewByToken = async (token: string): Promise<any> => {
  const res = await request<any>('GET', `/public/product-previews/${token}`);
  if (res) {
    res.images = res.images || [];
    res.variants = res.variants || [];
  }
  return res;
};

export const getCategories = async (): Promise<Category[]> => {
  const res = await request<any>('GET', '/public/categories');
  return res?.items || (Array.isArray(res) ? res : []);
};

export interface PublicColor {
  id: string;
  code: string;
  nameRu: string;
  hex?: string;
}

/** Canonical colors present on purchasable variants of storefront-eligible products. */
export const getPublicColors = async (): Promise<PublicColor[]> => {
  const res = await request<any>('GET', '/public/colors');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getDeliveryMethods = async (): Promise<PublicDeliveryMethod[]> => {
  const res = await request<any>('GET', '/public/delivery-methods');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getBrands = async (): Promise<Brand[]> => {
  const res = await request<any>('GET', '/public/brands');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getProductReviews = async (productId: string, params?: any): Promise<PublicReview[]> => {
  const res = await request<any>('GET', `/public/products/${productId}/reviews`, { params });
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getProductRatingSummary = async (productId: string): Promise<RatingSummary> => {
  return request<RatingSummary>('GET', `/public/products/${productId}/rating-summary`);
};

export const getPublicSeller = async (slugOrId: string, params?: any): Promise<import('./types').PublicSellerStorefrontResponse> => {
  const res = await request<import('./types').PublicSellerStorefrontResponse>('GET', `/public/sellers/${slugOrId}`, { params });
  if (res && res.items) {
    res.items = res.items || [];
  }
  return res;
};

export const getPublicBrand = async (slugOrId: string, params?: any): Promise<import('./types').PublicBrandStorefrontResponse> => {
  const res = await request<import('./types').PublicBrandStorefrontResponse>('GET', `/public/brands/${slugOrId}`, { params });
  if (res && res.items) {
    res.items = res.items || [];
  }
  return res;
};


// --- Auctions ---
export const getActiveAuctions = async (): Promise<AuctionEvent[]> => {
  const res = await request<any>('GET', '/public/auctions/active');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getHomepageAuctions = async (): Promise<AuctionEvent[]> => {
  const res = await request<any>('GET', '/public/auctions/homepage');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getNavHighlightAuctions = async (): Promise<AuctionEvent[]> => {
  const res = await request<any>('GET', '/public/auctions/nav-highlight');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getAuctionLots = async (id: string): Promise<AuctionLot[]> => {
  const res = await request<any>('GET', `/public/auctions/${id}/lots`);
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getAuctionLot = async (id: string): Promise<AuctionLot> => {
  return request<AuctionLot>('GET', `/public/auction-lots/${id}`);
};

// --- Editorial collections ---
export const getCollections = async (): Promise<import('./types').EditorialCollection[]> => {
  const res = await request<any>('GET', '/public/collections');
  return Array.isArray(res) ? res : res?.items || [];
};

export const getCollection = async (slug: string): Promise<import('./types').PublicCollectionDetail> => {
  const res = await request<import('./types').PublicCollectionDetail>('GET', `/public/collections/${encodeURIComponent(slug)}`);
  return { ...res, products: res?.products || [] };
};
