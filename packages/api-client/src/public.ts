import { request } from './client';
import type { ProductSummary, ProductDetail, Category, Brand, PublicReview, RatingSummary, AuctionEvent, AuctionLot } from './types';

export interface ProductListResponse {
  items: ProductSummary[];
  totalCount: number;
}

export const getProducts = async (params?: any): Promise<ProductListResponse> => {
  return request<ProductListResponse>('GET', '/public/products', { params });
};

export const getDirectSaleProducts = async (params?: any): Promise<ProductListResponse> => {
  return request<ProductListResponse>('GET', '/public/direct-sale', { params });
};

export const getProduct = async (idOrSlug: string): Promise<ProductDetail> => {
  return request<ProductDetail>('GET', `/public/products/${idOrSlug}`);
};

export const getCategories = async (): Promise<Category[]> => {
  return request<Category[]>('GET', '/public/categories');
};

export const getBrands = async (): Promise<Brand[]> => {
  return request<Brand[]>('GET', '/public/brands');
};

export const getProductReviews = async (productId: string, params?: any): Promise<PublicReview[]> => {
  return request<PublicReview[]>('GET', `/public/products/${productId}/reviews`, { params });
};

export const getProductRatingSummary = async (productId: string): Promise<RatingSummary> => {
  return request<RatingSummary>('GET', `/public/products/${productId}/rating-summary`);
};

export const getPublicSeller = async (slugOrId: string, params?: any): Promise<any> => {
  return request<any>('GET', `/public/sellers/${slugOrId}`, { params });
};

// --- Auctions ---
export const getActiveAuctions = async (): Promise<AuctionEvent[]> => {
  return request<AuctionEvent[]>('GET', '/public/auctions/active');
};

export const getHomepageAuctions = async (): Promise<AuctionEvent[]> => {
  return request<AuctionEvent[]>('GET', '/public/auctions/homepage');
};

export const getNavHighlightAuctions = async (): Promise<AuctionEvent[]> => {
  return request<AuctionEvent[]>('GET', '/public/auctions/nav-highlight');
};

export const getAuctionLots = async (id: string): Promise<AuctionLot[]> => {
  return request<AuctionLot[]>('GET', `/public/auctions/${id}/lots`);
};

export const getAuctionLot = async (id: string): Promise<AuctionLot> => {
  return request<AuctionLot>('GET', `/public/auction-lots/${id}`);
};
