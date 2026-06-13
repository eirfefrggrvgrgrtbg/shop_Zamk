import { request } from './client';
import type { SellerMe, UpdateSellerProfileRequest, SellerProduct, InventoryItem, SellerOrder, SellerReturn, SellerReview, SellerBalance, Payout, SellerWarning, SellerViolation, SellerFulfillment } from './types';

export const getSellerMe = async (): Promise<SellerMe> => {
  return request<SellerMe>('GET', '/seller/me');
};

export const getSellerProducts = async (): Promise<SellerProduct[]> => {
  return request<SellerProduct[]>('GET', '/seller/products');
};

export const createSellerProduct = async (input: any): Promise<SellerProduct> => {
  return request<SellerProduct>('POST', '/seller/products', { body: input });
};

export const getSellerProduct = async (id: string): Promise<SellerProduct> => {
  return request<SellerProduct>('GET', `/seller/products/${id}`);
};

export const updateSellerProduct = async (id: string, input: any): Promise<SellerProduct> => {
  return request<SellerProduct>('PATCH', `/seller/products/${id}`, { body: input });
};

// P0 fix: was /images, backend route is /images/upload
export const uploadSellerProductImage = async (productId: string, file: File): Promise<{ imageUrl: string }> => {
  const formData = new FormData();
  formData.append('image', file);
  return request<{ imageUrl: string }>('POST', `/seller/products/${productId}/images/upload`, { body: formData });
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerInventory = async (): Promise<{ items: InventoryItem[]; totalCount: number }> => {
  return request<{ items: InventoryItem[]; totalCount: number }>('GET', '/seller/inventory');
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerOrders = async (): Promise<{ items: SellerOrder[]; totalCount: number }> => {
  return request<{ items: SellerOrder[]; totalCount: number }>('GET', '/seller/orders');
};

export const getSellerOrder = async (id: string): Promise<SellerOrder> => {
  return request<SellerOrder>('GET', `/seller/orders/${id}`);
};

export const getSellerShipment = async (orderId: string): Promise<any> => {
  return request<any>('GET', `/seller/orders/${orderId}/shipment`);
};

export const getSellerFulfillments = async (params?: { limit?: number; offset?: number; status?: string }): Promise<{ items: SellerFulfillment[]; totalCount: number }> => {
  const query = new URLSearchParams();
  if (params?.limit) query.append('limit', params.limit.toString());
  if (params?.offset) query.append('offset', params.offset.toString());
  if (params?.status) query.append('status', params.status);
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request<{ items: SellerFulfillment[]; totalCount: number }>('GET', `/seller/fulfillments${qStr}`);
};

export const getSellerFulfillment = async (id: string): Promise<SellerFulfillment> => {
  return request<SellerFulfillment>('GET', `/seller/fulfillments/${id}`);
};

export const markSellerFulfillmentAssembling = async (id: string): Promise<void> => {
  return request<void>('POST', `/seller/fulfillments/${id}/mark-assembling`);
};

export const markSellerFulfillmentPacked = async (id: string): Promise<void> => {
  return request<void>('POST', `/seller/fulfillments/${id}/mark-packed`);
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerReturns = async (): Promise<{ items: SellerReturn[]; totalCount: number }> => {
  return request<{ items: SellerReturn[]; totalCount: number }>('GET', '/seller/returns');
};

export const getSellerReturn = async (id: string): Promise<SellerReturn> => {
  return request<SellerReturn>('GET', `/seller/returns/${id}`);
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerReviews = async (): Promise<{ items: SellerReview[]; totalCount: number }> => {
  return request<{ items: SellerReview[]; totalCount: number }>('GET', '/seller/reviews');
};

export const getSellerReview = async (id: string): Promise<SellerReview> => {
  return request<SellerReview>('GET', `/seller/reviews/${id}`);
};

export const getSellerBalance = async (): Promise<SellerBalance> => {
  return request<SellerBalance>('GET', '/seller/balance');
};

export const getSellerPayouts = async (): Promise<Payout[]> => {
  return request<Payout[]>('GET', '/seller/payouts');
};

export const requestSellerPayout = async (amountCents: number, comment?: string): Promise<Payout> => {
  return request<Payout>('POST', '/seller/payouts/request', { body: { amountCents, comment } });
};

export const updateSellerMe = async (req: UpdateSellerProfileRequest): Promise<SellerMe> => {
  return request<SellerMe>('PATCH', '/seller/me', { body: req });
};

export const uploadSellerLogo = async (file: File): Promise<{ logoUrl: string }> => {
  const formData = new FormData();
  formData.append('logo', file);
  return request<{ logoUrl: string }>('POST', '/seller/me/logo/upload', { body: formData });
};

export const getSellerWarnings = async (): Promise<SellerWarning[]> => {
  return request<SellerWarning[]>('GET', '/seller/warnings');
};

export const getSellerViolations = async (): Promise<SellerViolation[]> => {
  return request<SellerViolation[]>('GET', '/seller/violations');
};

export const getModerationHistory = async (productId: string): Promise<{ items: any[] }> => {
  return request<{ items: any[] }>('GET', `/seller/products/${productId}/moderation-history`);
};
