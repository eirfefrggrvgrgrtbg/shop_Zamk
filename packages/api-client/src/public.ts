import { request } from './client';
import { ProductSummary, ProductDetail, Category, Brand, PublicReview, RatingSummary } from './types';

export const getProducts = async (params?: any): Promise<ProductSummary[]> => {
  return request<ProductSummary[]>('GET', '/products', { params });
};

export const getProduct = async (idOrSlug: string): Promise<ProductDetail> => {
  return request<ProductDetail>('GET', `/products/${idOrSlug}`);
};

export const getCategories = async (): Promise<Category[]> => {
  return request<Category[]>('GET', '/categories');
};

export const getBrands = async (): Promise<Brand[]> => {
  return request<Brand[]>('GET', '/brands');
};

export const getProductReviews = async (productId: string, params?: any): Promise<PublicReview[]> => {
  return request<PublicReview[]>('GET', `/products/${productId}/reviews`, { params });
};

export const getProductRatingSummary = async (productId: string): Promise<RatingSummary> => {
  return request<RatingSummary>('GET', `/products/${productId}/rating-summary`);
};
