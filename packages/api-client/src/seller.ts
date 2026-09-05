import { request } from './client';
import type { SellerMe, UpdateSellerProfileRequest, SellerProduct, SellerInventoryItem, SellerOrder, SellerReturn, SellerReturnDetailResponse, SellerReview, SellerBalance, PayoutBatchListResponse, LedgerListResponse, SellerWarning, SellerViolation, SellerSupply, CreateSupplyRequest, SellerSupplyUnitLabelsResponse } from './types';

export const getSellerMe = async (): Promise<SellerMe> => {
  return request<SellerMe>('GET', '/seller/me');
};

export const getSellerProducts = async (): Promise<SellerProduct[]> => {
  const res = await request<any>('GET', '/seller/products');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const createSellerProduct = async (input: any): Promise<SellerProduct> => {
  return request<SellerProduct>('POST', '/seller/products', { body: input });
};

export const getSellerProduct = async (id: string): Promise<SellerProduct> => {
  const res = await request<any>('GET', `/seller/products/${id}`);
  if (res) {
    res.images = res.images || [];
    res.variants = res.variants || [];
  }
  return res;
};

export const updateSellerProduct = async (id: string, input: any): Promise<SellerProduct> => {
  return request<SellerProduct>('PATCH', `/seller/products/${id}`, { body: input });
};

// P0 fix: was /images, backend route is /images/upload
export const uploadSellerProductImage = async (productId: string, file: File): Promise<{ id: string, imageUrl: string }> => {
  const formData = new FormData();
  formData.append('image', file);
  return request<{ id: string, imageUrl: string }>('POST', `/seller/products/${productId}/images/upload`, { body: formData });
};

export const deleteSellerProductImage = async (productId: string, imageId: string): Promise<void> => {
  return request<void>('DELETE', `/seller/products/${productId}/images/${imageId}`);
};

export const reorderSellerProductImages = async (productId: string, imageIds: string[]): Promise<void> => {
  return request<void>('PUT', `/seller/products/${productId}/images/reorder`, { body: { imageIds } });
};
export const getSellerInventory = async (): Promise<{ items: SellerInventoryItem[]; totalCount: number }> => {
  const res = await request<any>('GET', '/seller/inventory');
  return { ...res, items: res?.items || [] };
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerOrders = async (): Promise<{ items: SellerOrder[]; totalCount: number }> => {
  const res = await request<any>('GET', '/seller/orders');
  return { ...res, items: res?.items || [] };
};

export const getSellerOrderSummary = async (): Promise<any> => {
  return request<any>('GET', '/seller/orders/summary');
};

export const getSellerOrder = async (id: string): Promise<SellerOrder> => {
  return request<SellerOrder>('GET', `/seller/orders/${id}`);
};

export const getSellerShipment = async (orderId: string): Promise<any> => {
  return request<any>('GET', `/seller/orders/${orderId}/shipment`);
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerReturns = async (): Promise<{ items: SellerReturn[]; totalCount: number }> => {
  const res = await request<any>('GET', '/seller/returns');
  return { ...res, items: res?.items || [] };
};

export const getSellerReturn = async (id: string): Promise<SellerReturnDetailResponse> => {
  return request<SellerReturnDetailResponse>('GET', `/seller/returns/${id}`);
};

// P0 fix: backend returns { items, totalCount } not bare array
export const getSellerReviews = async (): Promise<{ items: SellerReview[]; totalCount: number }> => {
  const res = await request<any>('GET', '/seller/reviews');
  return { ...res, items: res?.items || [] };
};

export const getSellerReview = async (id: string): Promise<SellerReview> => {
  return request<SellerReview>('GET', `/seller/reviews/${id}`);
};

export const getSellerBalance = async (): Promise<SellerBalance> => {
  return request<SellerBalance>('GET', '/seller/balance');
};

export const getSellerLedger = async (limit = 50, offset = 0): Promise<LedgerListResponse> => {
  return request<LedgerListResponse>('GET', `/seller/payouts/ledger?limit=${limit}&offset=${offset}`);
};

export const getSellerPayouts = async (limit = 50, offset = 0): Promise<PayoutBatchListResponse> => {
  return request<PayoutBatchListResponse>('GET', `/seller/payouts?limit=${limit}&offset=${offset}`);
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
  const res = await request<any>('GET', '/seller/warnings');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getSellerViolations = async (): Promise<SellerViolation[]> => {
  const res = await request<any>('GET', '/seller/violations');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getSellerSupplies = async (): Promise<SellerSupply[]> => {
  const res = await request<any>('GET', '/seller/supplies');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const createSellerSupply = async (input: CreateSupplyRequest): Promise<SellerSupply> => {
  return request<SellerSupply>('POST', '/seller/supplies', { body: input });
};

export const getSellerSupply = async (id: string): Promise<SellerSupply> => {
  return request<SellerSupply>('GET', `/seller/supplies/${id}`);
};

export const getSellerSupplyUnitLabels = async (id: string): Promise<SellerSupplyUnitLabelsResponse> => {
  return request<SellerSupplyUnitLabelsResponse>('GET', `/seller/supplies/${id}/unit-labels`);
};

export const shipSellerSupply = async (id: string): Promise<SellerSupply> => {
  return request<SellerSupply>('POST', `/seller/supplies/${id}/ship`);
};

export const getModerationHistory = async (productId: string): Promise<{ items: any[] }> => {
  return request<{ items: any[] }>('GET', `/seller/products/${productId}/moderation-history`);
};

export const submitSellerProductModeration = async (productId: string, comment?: string): Promise<void> => {
  return request<void>('POST', `/seller/products/${productId}/submit-moderation`, { body: { comment } });
};

export const completeSellerOnboarding = async (): Promise<void> => {
  return request<void>('POST', '/seller/onboarding/complete');
};

export const getSellerNotifications = async (limit = 20, offset = 0): Promise<import('./types').PaginatedNotifications> => {
  return request<import('./types').PaginatedNotifications>('GET', `/seller/notifications?limit=${limit}&offset=${offset}`);
};

export const getSellerUnreadNotificationsCount = async (): Promise<import('./types').UnreadCountResponse> => {
  return request<import('./types').UnreadCountResponse>('GET', '/seller/notifications/unread-count');
};

export const markSellerNotificationRead = async (id: string): Promise<void> => {
  return request<void>('POST', `/seller/notifications/${id}/read`);
};

export const markAllSellerNotificationsRead = async (): Promise<void> => {
  return request<void>('POST', '/seller/notifications/read-all');
};


export interface SellerCategory {
  id: string;
  name: string;
  slug: string;
  parentId?: string;
  sortOrder: number;
}

export interface SellerColor {
  id: string;
  code: string;
  nameRu: string;
  hexValue: string;
}

export interface SellerMaterial {
  id: string;
  code: string;
  nameRu: string;
}

export interface SellerSizeSystem {
  id: string;
  code: string;
  nameRu: string;
}

export interface SellerSizeValue {
  id: string;
  sizeSystemId: string;
  value: string;
  sortOrder: number;
}

export interface SellerDictionaryValue {
  id: string;
  dictionaryId: string;
  code: string;
  nameRu: string;
}

export interface SellerCategorySchema {
  id: string;
  name: string;
  slug: string;
  sizeChartRequired: boolean;
  attributes: {
    id: string;
    code: string;
    nameRu: string;
    valueType: string;
    valueSource: string;
    scope: string;
    required: boolean;
    filterable: boolean;
    variantAxis: boolean;
    sortOrder: number;
    dictionaryId?: string;
  }[];
  allowedSizeSystems: SellerSizeSystem[];
  sizeChartFields: {
    code: string;
    name: string;
    unit: string;
    isRequired: boolean;
    sortOrder: number;
  }[];
}

export const getSellerCategories = async (): Promise<SellerCategory[]> => {
  const res = await request<{ items: SellerCategory[] }>('GET', '/seller/reference/categories');
  return res.items || [];
};

export const getSellerCategorySchema = async (id: string): Promise<SellerCategorySchema> => {
  return request<SellerCategorySchema>('GET', `/seller/reference/categories/${id}/schema`);
};

export const getSellerColors = async (): Promise<SellerColor[]> => {
  return request<SellerColor[]>('GET', '/seller/reference/colors');
};

export const getSellerMaterials = async (): Promise<SellerMaterial[]> => {
  return request<SellerMaterial[]>('GET', '/seller/reference/materials');
};

export const getSellerSizeSystems = async (): Promise<SellerSizeSystem[]> => {
  return request<SellerSizeSystem[]>('GET', '/seller/reference/size-systems');
};

export const getSellerSizeValues = async (systemId: string): Promise<SellerSizeValue[]> => {
  return request<SellerSizeValue[]>('GET', `/seller/reference/size-systems/${systemId}/values`);
};

export const getSellerDictionaryValues = async (dictId: string): Promise<SellerDictionaryValue[]> => {
  return request<SellerDictionaryValue[]>('GET', `/seller/reference/dictionaries/${dictId}/values`);
};

export const updateSellerProductPrices = async (id: string, payload: any): Promise<void> => {
  return request<void>('PATCH', `/seller/products/${id}/prices`, { body: payload });
};

export const generateSellerSKUs = async (count: number): Promise<{ skus: string[] }> => {
  return request<{ skus: string[] }>('POST', '/seller/products/generate-skus', { body: { count } });
};

export const cropSellerProductImage = async (productId: string, imageId: string, crop: { cropX: number; cropY: number; cropWidth: number; cropHeight: number }): Promise<{ id: string, imageUrl: string, isMain: boolean, renditionUrl?: string }> => {
  return request<{ id: string, imageUrl: string, isMain: boolean }>('POST', `/seller/products/${productId}/images/${imageId}/crop`, { body: crop });
};

export const setMainSellerProductImage = async (productId: string, imageId: string): Promise<void> => {
  return request<void>('POST', `/seller/products/${productId}/images/${imageId}/main`);
};
