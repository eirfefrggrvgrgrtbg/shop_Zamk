import { request } from './client';
import type { AdminSeller, AdminProduct, ModerationProduct, AdminOrder, AdminPayment, AdminShipment, AdminReturn, AdminRefund, AdminPayout, AdminReview, Category, Brand } from './types';

export const getAdminSellers = async (): Promise<AdminSeller[]> => {
  return request<AdminSeller[]>('GET', '/admin/sellers');
};

export const createAdminSeller = async (data: any): Promise<{ seller: AdminSeller; temporaryPassword?: string }> => {
  return request<{ seller: AdminSeller; temporaryPassword?: string }>('POST', '/admin/sellers', { body: data });
};

export const updateAdminSellerStatus = async (id: string, status: string): Promise<AdminSeller> => {
  return request<AdminSeller>('PATCH', `/admin/sellers/${id}/status`, { body: { status } });
};

export const getAdminCategories = async (): Promise<Category[]> => {
  return request<Category[]>('GET', '/admin/categories');
};

export const createAdminCategory = async (data: any): Promise<Category> => {
  return request<Category>('POST', '/admin/categories', { body: data });
};

export const getAdminBrands = async (): Promise<Brand[]> => {
  return request<Brand[]>('GET', '/admin/brands');
};

export const createAdminBrand = async (data: any): Promise<Brand> => {
  return request<Brand>('POST', '/admin/brands', { body: data });
};

export const uploadAdminBrandLogo = async (brandId: string, file: File): Promise<{ logoUrl: string }> => {
  const formData = new FormData();
  formData.append('logo', file);
  return request<{ logoUrl: string }>('POST', `/admin/brands/${brandId}/logo`, { body: formData });
};

export const getAdminProducts = async (): Promise<AdminProduct[]> => {
  return request<AdminProduct[]>('GET', '/admin/products');
};

export const getModerationProducts = async (): Promise<ModerationProduct[]> => {
  return request<ModerationProduct[]>('GET', '/admin/moderation/products');
};

export const adminApproveProduct = async (id: string, comment?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/approve`, { body: { comment } });
};

export const adminRejectProduct = async (id: string, comment: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/reject`, { body: { comment } });
};

export const adminPublishProduct = async (id: string, comment?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/publish`, { body: { comment } });
};

export const adminHideProduct = async (id: string, comment?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/hide`, { body: { comment } });
};

export const adminBlockProduct = async (id: string, comment?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/block`, { body: { comment } });
};

export const getAdminInventory = async (): Promise<any[]> => {
  return request<any[]>('GET', '/admin/inventory');
};

export const getAdminOrders = async (): Promise<AdminOrder[]> => {
  return request<AdminOrder[]>('GET', '/admin/orders');
};

export const getAdminPayments = async (): Promise<AdminPayment[]> => {
  return request<AdminPayment[]>('GET', '/admin/payments');
};

export const getAdminShipments = async (): Promise<AdminShipment[]> => {
  return request<AdminShipment[]>('GET', '/admin/shipments');
};

export const getAdminReturns = async (): Promise<AdminReturn[]> => {
  return request<AdminReturn[]>('GET', '/admin/returns');
};

export const getAdminRefunds = async (): Promise<AdminRefund[]> => {
  return request<AdminRefund[]>('GET', '/admin/refunds');
};

export const getAdminPayouts = async (): Promise<AdminPayout[]> => {
  return request<AdminPayout[]>('GET', '/admin/payouts');
};

export const getAdminReviews = async (): Promise<AdminReview[]> => {
  return request<AdminReview[]>('GET', '/admin/reviews');
};

export const uploadAdminProductImage = async (productId: string, file: File): Promise<{ imageUrl: string }> => {
  const formData = new FormData();
  formData.append('image', file);
  return request<{ imageUrl: string }>('POST', `/admin/products/${productId}/images/upload`, { body: formData });
};
