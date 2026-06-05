import { request } from './client';
import type { AdminSeller, AdminProduct, ModerationProduct, AdminOrder, AdminPayment, AdminShipment, AdminReturn, AdminRefund, AdminPayout, AdminReview, Category, Brand, AdminInventoryItem, AdminInventoryMovement } from './types';

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

export const getAdminInventory = async (): Promise<{ items: AdminInventoryItem[]; totalCount: number }> => {
  return request<{ items: AdminInventoryItem[]; totalCount: number }>('GET', '/admin/inventory');
};

export const getAdminInventoryItem = async (id: string): Promise<AdminInventoryItem> => {
  return request<AdminInventoryItem>('GET', `/admin/inventory/${id}`);
};

export const getAdminInventoryMovements = async (id: string): Promise<{ items: AdminInventoryMovement[]; totalCount: number }> => {
  return request<{ items: AdminInventoryMovement[]; totalCount: number }>('GET', `/admin/inventory/${id}/movements`);
};

export const createAdminInventoryReceipt = async (data: { productVariantId: string; quantity: number; reason?: string }): Promise<AdminInventoryItem> => {
  return request<AdminInventoryItem>('POST', '/admin/inventory/receipts', { body: data });
};

export const createAdminInventoryAdjustment = async (data: { productVariantId: string; quantity: number; reason: string }): Promise<AdminInventoryItem> => {
  return request<AdminInventoryItem>('POST', '/admin/inventory/adjustments', { body: data });
};

export const createAdminInventoryWriteOff = async (data: { productVariantId: string; quantity: number; reason: string }): Promise<AdminInventoryItem> => {
  return request<AdminInventoryItem>('POST', '/admin/inventory/write-offs', { body: data });
};

export const getAdminOrders = async (): Promise<{ items: AdminOrder[]; totalCount: number }> => {
  return request<{ items: AdminOrder[]; totalCount: number }>('GET', '/admin/orders');
};

export const getAdminOrder = async (id: string): Promise<AdminOrder> => {
  return request<AdminOrder>('GET', `/admin/orders/${id}`);
};

export const updateAdminOrderStatus = async (id: string, data: { status: string; comment?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/orders/${id}/status`, { body: data });
};

export const getAdminPayments = async (): Promise<{ items: AdminPayment[]; totalCount: number }> => {
  return request<{ items: AdminPayment[]; totalCount: number }>('GET', '/admin/payments');
};

export const getAdminPayment = async (id: string): Promise<AdminPayment> => {
  return request<AdminPayment>('GET', `/admin/payments/${id}`);
};

export const getAdminShipments = async (): Promise<AdminShipment[]> => {
  return request<AdminShipment[]>('GET', '/admin/shipments');
};

export const getAdminShipment = async (id: string): Promise<AdminShipment> => {
  return request<AdminShipment>('GET', `/admin/shipments/${id}`);
};

export const createAdminShipment = async (orderId: string, data: { carrier?: string; trackingNumber?: string; trackingUrl?: string }): Promise<AdminShipment> => {
  return request<AdminShipment>('POST', `/admin/orders/${orderId}/shipment`, { body: data });
};

export const updateAdminShipmentStatus = async (id: string, data: { status: string; carrier?: string; trackingNumber?: string; trackingUrl?: string; comment?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/shipments/${id}/status`, { body: data });
};

export const getAdminReturns = async (): Promise<{ items: AdminReturn[]; totalCount: number }> => {
  return request<{ items: AdminReturn[]; totalCount: number }>('GET', '/admin/returns');
};

export const getAdminReturn = async (id: string): Promise<AdminReturn> => {
  return request<AdminReturn>('GET', `/admin/returns/${id}`);
};

export const updateAdminReturnStatus = async (id: string, data: { status: string; adminComment?: string; itemRestock?: Array<{ returnItemId: string; restock: boolean }> }): Promise<void> => {
  return request<void>('PATCH', `/admin/returns/${id}/status`, { body: data });
};

export const createAdminRefundForReturn = async (returnId: string, data: { reason?: string }): Promise<AdminRefund> => {
  return request<AdminRefund>('POST', `/admin/returns/${returnId}/refund`, { body: data });
};

export const getAdminRefunds = async (): Promise<{ items: AdminRefund[]; totalCount: number }> => {
  return request<{ items: AdminRefund[]; totalCount: number }>('GET', '/admin/refunds');
};

export const getAdminRefund = async (id: string): Promise<AdminRefund> => {
  return request<AdminRefund>('GET', `/admin/refunds/${id}`);
};

export const getAdminPayouts = async (): Promise<{ items: AdminPayout[]; totalCount: number }> => {
  return request<{ items: AdminPayout[]; totalCount: number }>('GET', '/admin/payouts');
};

export const getAdminPayout = async (id: string): Promise<{ id?: string; payout?: AdminPayout } | AdminPayout> => {
  return request<{ id?: string; payout?: AdminPayout } | AdminPayout>('GET', `/admin/payouts/${id}`);
};

export const updateAdminPayoutStatus = async (id: string, data: { status: string; comment?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/payouts/${id}/status`, { body: data });
};

export const getAdminReviews = async (): Promise<{ items: AdminReview[]; totalCount: number }> => {
  return request<{ items: AdminReview[]; totalCount: number }>('GET', '/admin/reviews');
};

export const getAdminReview = async (id: string): Promise<AdminReview> => {
  return request<AdminReview>('GET', `/admin/reviews/${id}`);
};

export const moderateAdminReview = async (id: string, action: 'approve' | 'reject' | 'hide' | 'block', comment?: string): Promise<void> => {
  return request<void>('POST', `/admin/reviews/${id}/${action}`, { body: { comment } });
};

export const uploadAdminProductImage = async (productId: string, file: File): Promise<{ imageUrl: string }> => {
  const formData = new FormData();
  formData.append('image', file);
  return request<{ imageUrl: string }>('POST', `/admin/products/${productId}/images/upload`, { body: formData });
};
