import { request } from './client';
import type { SellerMe, SellerProduct, InventoryItem, SellerOrder, SellerReturn, SellerReview, SellerBalance, Payout } from './types';

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

export const uploadSellerProductImage = async (productId: string, file: File): Promise<{ imageUrl: string }> => {
  const formData = new FormData();
  formData.append('image', file);
  return request<{ imageUrl: string }>('POST', `/seller/products/${productId}/images`, { body: formData });
};

export const getSellerInventory = async (): Promise<InventoryItem[]> => {
  return request<InventoryItem[]>('GET', '/seller/inventory');
};

export const getSellerOrders = async (): Promise<SellerOrder[]> => {
  return request<SellerOrder[]>('GET', '/seller/orders');
};

export const getSellerOrder = async (id: string): Promise<SellerOrder> => {
  return request<SellerOrder>('GET', `/seller/orders/${id}`);
};

export const getSellerShipment = async (orderId: string): Promise<any> => {
  return request<any>('GET', `/seller/orders/${orderId}/shipment`);
};



export const getSellerReturns = async (): Promise<SellerReturn[]> => {
  return request<SellerReturn[]>('GET', '/seller/returns');
};

export const getSellerReturn = async (id: string): Promise<SellerReturn> => {
  return request<SellerReturn>('GET', `/seller/returns/${id}`);
};

export const getSellerReviews = async (): Promise<SellerReview[]> => {
  return request<SellerReview[]>('GET', '/seller/reviews');
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

export const uploadSellerLogo = async (file: File): Promise<{ logoUrl: string }> => {
  const formData = new FormData();
  formData.append('logo', file);
  return request<{ logoUrl: string }>('POST', '/seller/me/logo', { body: formData });
};
