import { request } from './client';
import type { PaginatedAdminUsersResponse, AdminSeller, AdminProduct, AdminOrder, AdminOrderDetail, AdminPayment, AdminShipment, AdminReturn, AdminSendReturnMessageRequest, ReturnConversationResponse, AdminRefund, AdminPayout, AdminReview, Category, Brand, AdminInventoryItem, AdminInventoryMovement, AdminInventoryListResponse, StaffMemberView, StaffRoleWithPermissions, AdminMeResponse, CreateStaffMemberRequest, CreateStaffMemberResponse, UpdateStaffRoleRequest, UpdateStaffStatusRequest, ResetStaffPasswordRequest, SellerDetail, SellerOverviewData, SellerStatusHistoryItem, SellerWarning, SellerViolation, CreateWarningRequest, CreateViolationRequest, AdminFulfillment, AdminDashboardSummary, PaginatedAdminProductsResponse, ModerationHistoryResponse, SellerNote, CreateSellerNoteRequest, SellerImprovementPlan, CreateSellerImprovementPlanRequest, SellerSupply, SupplyReceivingSession, RecordReceivingScanRequest, FinalizeReceivingRequest, RecordSerializedScanRequest, SerializedScanResponse, SerializedRecentScan, UndoSerializedScanResponse } from './types';

export const getAdminSellers = async (params?: {
  search?: string;
  status?: string[];
  store?: string;
  problems?: string;
  ratingMin?: number;
  ratingMax?: number;
  hasReviews?: boolean;
  performanceMin?: number;
  performanceMax?: number;
  performanceCategory?: string;
  salesGrossMin?: number;
  salesGrossMax?: number;
  ordersCountMin?: number;
  ordersCountMax?: number;
  hasWarnings?: boolean;
  hasViolations?: boolean;
  blocked?: boolean;
  sort?: string;
  direction?: string;
  limit?: number;
  page?: number;
}): Promise<{ items: AdminSeller[]; totalCount: number; page: number; limit: number; totalPages: number; statusCounts: Record<string, number> }> => {
  const query = new URLSearchParams();
  if (params?.search) query.append('search', params.search);
  
  if (params?.status && params.status.length > 0) {
    params.status.forEach(st => {
      if (st !== 'all') query.append('status', st);
    });
  }
  
  if (params?.store && params.store !== 'all') query.append('store', params.store);
  if (params?.problems && params.problems !== 'all') query.append('problems', params.problems);
  
  if (params?.ratingMin !== undefined) query.append('ratingMin', params.ratingMin.toString());
  if (params?.ratingMax !== undefined) query.append('ratingMax', params.ratingMax.toString());
  if (params?.hasReviews !== undefined) query.append('hasReviews', params.hasReviews.toString());
  
  if (params?.performanceMin !== undefined) query.append('performanceMin', params.performanceMin.toString());
  if (params?.performanceMax !== undefined) query.append('performanceMax', params.performanceMax.toString());
  if (params?.performanceCategory && params.performanceCategory !== 'all') query.append('performanceCategory', params.performanceCategory);
  
  if (params?.salesGrossMin !== undefined) query.append('salesGrossMin', params.salesGrossMin.toString());
  if (params?.salesGrossMax !== undefined) query.append('salesGrossMax', params.salesGrossMax.toString());
  if (params?.ordersCountMin !== undefined) query.append('ordersCountMin', params.ordersCountMin.toString());
  if (params?.ordersCountMax !== undefined) query.append('ordersCountMax', params.ordersCountMax.toString());
  
  if (params?.hasWarnings !== undefined) query.append('hasWarnings', params.hasWarnings.toString());
  if (params?.hasViolations !== undefined) query.append('hasViolations', params.hasViolations.toString());
  if (params?.blocked !== undefined) query.append('blocked', params.blocked.toString());

  if (params?.sort) query.append('sort', params.sort);
  if (params?.direction) query.append('direction', params.direction);
  if (params?.limit) query.append('limit', params.limit.toString());
  if (params?.page) {
    const offset = (params.page - 1) * (params.limit || 25);
    query.append('offset', offset.toString());
  }
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  const res = await request<any>('GET', `/admin/sellers${qStr}`);
  return { 
    ...res, 
    items: res?.items || [], 
    totalCount: res?.total || 0,
    page: res?.page || 1,
    limit: res?.limit || 25,
    totalPages: res?.totalPages || 0,
    statusCounts: res?.statusCounts || {}
  };
};

export const getAdminUsers = async (params?: { q?: string; role?: string; status?: string; limit?: number; offset?: number }): Promise<PaginatedAdminUsersResponse> => {
  const query = new URLSearchParams();
  if (params?.q) query.append('q', params.q);
  if (params?.role) query.append('role', params.role);
  if (params?.status) query.append('status', params.status);
  if (params?.limit) query.append('limit', params.limit.toString());
  if (params?.offset) query.append('offset', params.offset.toString());
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request<PaginatedAdminUsersResponse>('GET', `/admin/users${qStr}`);
};


import type { CreateAdminSellerRequest, CreateAdminSellerResponse } from './types';

// Backend automatically generates a temporary password and returns it via temporaryPassword
export const createAdminSeller = async (data: CreateAdminSellerRequest): Promise<CreateAdminSellerResponse> => {
  return request<CreateAdminSellerResponse>('POST', '/admin/sellers', { body: data });
};

export const updateAdminSellerStatus = async (id: string, status: string, reason?: string): Promise<void> => {
  return request<void>('PATCH', `/admin/sellers/${id}/status`, { body: { status, reason } });
};

// ---------------------------------------------------------
// WAREHOUSE RECEIVING
// ---------------------------------------------------------

export const lookupSupplyByCode = (qrToken: string): Promise<SellerSupply> => {
  return request<SellerSupply>('GET', `/admin/receiving/lookup?qr_token=${encodeURIComponent(qrToken)}`);
};

export const markSupplyArrived = (supplyId: string): Promise<void> => {
  return request<void>('POST', `/admin/receiving/${supplyId}/arrive`);
};

export const startSupplyReceivingSession = async (qrToken: string): Promise<SupplyReceivingSession> => {
  return request<SupplyReceivingSession>('POST', `/admin/receiving/sessions?qr_token=${encodeURIComponent(qrToken)}`);
};

export const recordSupplyReceivingScan = async (sessionId: string, input: RecordReceivingScanRequest): Promise<void> => {
  return request<void>('POST', `/admin/receiving/sessions/${sessionId}/scan`, { body: input });
};

export const recordSerializedReceivingScan = async (
  sessionId: string,
  input: RecordSerializedScanRequest
): Promise<SerializedScanResponse> => {
  return request<SerializedScanResponse>('POST', `/admin/receiving/sessions/${sessionId}/scan-unit`, { body: input });
};

export const getSerializedReceivingScans = async (
  sessionId: string,
  limit: number = 10
): Promise<SerializedRecentScan[]> => {
  return request<SerializedRecentScan[]>('GET', `/admin/receiving/sessions/${sessionId}/scans?limit=${limit}`);
};

export const undoSerializedReceivingScan = async (
  sessionId: string,
  scanId: string
): Promise<UndoSerializedScanResponse> => {
  return request<UndoSerializedScanResponse>('POST', `/admin/receiving/sessions/${sessionId}/scans/${scanId}/undo`);
};

export const finalizeSupplyReceivingSession = async (sessionId: string, input: FinalizeReceivingRequest): Promise<void> => {
  return request<void>('POST', `/admin/receiving/sessions/${sessionId}/finalize`, { body: input });
};

export const getAdminCategories = async (): Promise<Category[]> => {
  const res = await request<any>('GET', '/admin/categories');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const createAdminCategory = async (data: any): Promise<Category> => {
  return request<Category>('POST', '/admin/categories', { body: data });
};

export const getAdminBrands = async (): Promise<Brand[]> => {
  const res = await request<any>('GET', '/admin/brands');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const createAdminBrand = async (data: any): Promise<Brand> => {
  return request<Brand>('POST', '/admin/brands', { body: data });
};

// P0 fix: was /logo, backend route is /logo/upload
export const uploadAdminBrandLogo = async (brandId: string, file: File): Promise<{ logoUrl: string }> => {
  const formData = new FormData();
  formData.append('logo', file);
  return request<{ logoUrl: string }>('POST', `/admin/brands/${brandId}/logo/upload`, { body: formData });
};

export interface GetAdminProductsParams {
  page?: number;
  limit?: number;
  offset?: number;
  q?: string;
  status?: string;
  statuses?: string;
  sellerId?: string;
  categoryId?: string;
  categoryIds?: string;
  brandId?: string;
  brandIds?: string;
  submittedPeriod?: string;
  noMainImage?: boolean;
  noDescription?: boolean;
  noBrand?: boolean;
  noVariants?: boolean;
  noPrice?: boolean;
  duplicateSku?: boolean;
  noStock?: boolean;
  resubmitted?: boolean;
  hasProblems?: boolean;
  sortBy?: string;
  sortOrder?: string;
  source?: string;
  signal?: AbortSignal;
}

export const getAdminProducts = async (
  pageOrParams?: number | GetAdminProductsParams,
  limitParam?: number,
  filtersParam?: {
    q?: string;
    status?: string;
    sellerId?: string;
    source?: string;
  }
): Promise<PaginatedAdminProductsResponse> => {
  let params: GetAdminProductsParams = {};

  if (typeof pageOrParams === 'number') {
    params = {
      page: pageOrParams,
      limit: limitParam ?? 20,
      ...filtersParam,
    };
  } else if (pageOrParams && typeof pageOrParams === 'object') {
    params = pageOrParams;
  }

  const query = new URLSearchParams();
  if (params.page !== undefined) query.append('page', params.page.toString());
  if (params.limit !== undefined) query.append('limit', params.limit.toString());
  if (params.offset !== undefined) query.append('offset', params.offset.toString());
  if (params.q) query.append('q', params.q);
  if (params.status) query.append('status', params.status);
  if (params.statuses) query.append('statuses', params.statuses);
  if (params.sellerId) query.append('sellerId', params.sellerId);
  if (params.categoryId) query.append('categoryId', params.categoryId);
  if (params.categoryIds) query.append('categoryIds', params.categoryIds);
  if (params.brandId) query.append('brandId', params.brandId);
  if (params.brandIds) query.append('brandIds', params.brandIds);
  if (params.submittedPeriod) query.append('submittedPeriod', params.submittedPeriod);
  if (params.noMainImage) query.append('noMainImage', 'true');
  if (params.noDescription) query.append('noDescription', 'true');
  if (params.noBrand) query.append('noBrand', 'true');
  if (params.noVariants) query.append('noVariants', 'true');
  if (params.noPrice) query.append('noPrice', 'true');
  if (params.duplicateSku) query.append('duplicateSku', 'true');
  if (params.noStock) query.append('noStock', 'true');
  if (params.resubmitted) query.append('resubmitted', 'true');
  if (params.hasProblems) query.append('hasProblems', 'true');
  if (params.sortBy) query.append('sortBy', params.sortBy);
  if (params.sortOrder) query.append('sortOrder', params.sortOrder);
  if (params.source) query.append('source', params.source);

  const queryString = query.toString() ? `?${query.toString()}` : '';
  const res = await request<any>('GET', `/admin/products${queryString}`, { signal: params.signal });
  return {
    items: res?.items || (Array.isArray(res) ? res : []),
    totalCount: res?.totalCount ?? res?.total ?? (res?.items?.length || 0),
  };
};

export const getAdminProduct = async (id: string): Promise<AdminProduct> => {
  const res = await request<any>('GET', `/admin/products/${id}`);
  if (res) {
    res.images = res.images || [];
    res.variants = res.variants || [];
  }
  return res;
};

export interface ProductPreviewLinkResponse {
  pageUrl: string;
  catalogCardUrl: string;
  expiresAt: string;
}

export const createProductPreviewLink = async (productId: string): Promise<ProductPreviewLinkResponse> => {
  return request<ProductPreviewLinkResponse>('POST', `/admin/products/${productId}/preview-link`);
};

export const getAdminProductModerationHistory = async (productId: string): Promise<ModerationHistoryResponse> => {
  return request<ModerationHistoryResponse>('GET', `/admin/products/${productId}/moderation-logs`);
};

export const getModerationProducts = async (params?: {
  q?: string;
  status?: string;
  sellerId?: string;
  categoryId?: string;
  categoryIds?: string;
  brandId?: string;
  brandIds?: string;
  submittedPeriod?: string;
  submittedFrom?: string;
  submittedTo?: string;
  noMainImage?: boolean;
  noDescription?: boolean;
  noBrand?: boolean;
  noVariants?: boolean;
  noPrice?: boolean;
  duplicateSku?: boolean;
  noStock?: boolean;
  resubmitted?: boolean;
  hasProblems?: boolean;
  sortBy?: string;
  sortOrder?: string;
  limit?: number;
  offset?: number;
  signal?: AbortSignal;
}): Promise<{ items: any[]; totalCount: number }> => {
  const query = new URLSearchParams();
  if (params?.q) query.append('q', params.q);
  if (params?.status) query.append('status', params.status);
  if (params?.sellerId) query.append('sellerId', params.sellerId);
  if (params?.categoryId) query.append('categoryId', params.categoryId);
  if (params?.categoryIds) query.append('categoryIds', params.categoryIds);
  if (params?.brandId) query.append('brandId', params.brandId);
  if (params?.brandIds) query.append('brandIds', params.brandIds);
  if (params?.submittedPeriod) query.append('submittedPeriod', params.submittedPeriod);
  if (params?.submittedFrom) query.append('submittedFrom', params.submittedFrom);
  if (params?.submittedTo) query.append('submittedTo', params.submittedTo);
  if (params?.noMainImage) query.append('noMainImage', 'true');
  if (params?.noDescription) query.append('noDescription', 'true');
  if (params?.noBrand) query.append('noBrand', 'true');
  if (params?.noVariants) query.append('noVariants', 'true');
  if (params?.noPrice) query.append('noPrice', 'true');
  if (params?.duplicateSku) query.append('duplicateSku', 'true');
  if (params?.noStock) query.append('noStock', 'true');
  if (params?.resubmitted) query.append('resubmitted', 'true');
  if (params?.hasProblems) query.append('hasProblems', 'true');
  if (params?.sortBy) query.append('sortBy', params.sortBy);
  if (params?.sortOrder) query.append('sortOrder', params.sortOrder);
  if (params?.limit) query.append('limit', String(params.limit));
  if (params?.offset) query.append('offset', String(params.offset));

  const queryString = query.toString() ? `?${query.toString()}` : '';
  const res = await request<any>('GET', `/admin/moderation/products${queryString}`, { signal: params?.signal });
  return {
    items: res?.items || (Array.isArray(res) ? res : []),
    totalCount: res?.totalCount ?? (res?.items?.length || 0),
  };
};

export const adminApproveProduct = async (id: string, comment?: string, expectedUpdatedAt?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/approve`, { body: { comment, expectedUpdatedAt } });
};

export const adminStartProductReview = async (id: string, expectedUpdatedAt?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/start-review`, {
    body: { expectedUpdatedAt },
  });
};

export const adminRejectProduct = async (id: string, comment: string, expectedUpdatedAt?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/reject`, { body: { comment, expectedUpdatedAt } });
};

export const adminPublishProduct = async (id: string, comment?: string, expectedUpdatedAt?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/publish`, { body: { comment, expectedUpdatedAt } });
};

export const updateAdminProduct = async (id: string, data: any): Promise<AdminProduct> => {
  return request<AdminProduct>('PATCH', `/admin/products/${id}`, { body: data });
};

export const adminHideProduct = async (id: string, comment?: string, expectedUpdatedAt?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/hide`, { body: { comment, expectedUpdatedAt } });
};

export const adminBlockProduct = async (id: string, comment?: string, expectedUpdatedAt?: string): Promise<void> => {
  return request<void>('POST', `/admin/moderation/products/${id}/block`, { body: { comment, expectedUpdatedAt } });
};

export const getAdminInventory = async (params?: { q?: string; sellerId?: string; source?: string; lowStock?: boolean; limit?: number; offset?: number }): Promise<AdminInventoryListResponse> => {
  const query = new URLSearchParams();
  if (params?.q) query.append('q', params.q);
  if (params?.sellerId) query.append('sellerId', params.sellerId);
  if (params?.source) query.append('source', params.source);
  if (params?.lowStock) query.append('lowStock', 'true');
  if (params?.limit) query.append('limit', params.limit.toString());
  if (params?.offset) query.append('offset', params.offset.toString());
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request<AdminInventoryListResponse>('GET', `/admin/inventory${qStr}`);
};

export const getAdminInventoryItem = async (id: string): Promise<AdminInventoryItem> => {
  return request<AdminInventoryItem>('GET', `/admin/inventory/${id}`);
};

export const getAdminInventoryMovements = async (id: string): Promise<{ items: AdminInventoryMovement[]; totalCount: number }> => {
  const res = await request<any>('GET', `/admin/inventory/${id}/movements`);
  return { ...res, items: res?.items || [] };
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

export const getAdminOrders = async (params?: { q?: string; status?: string; paymentStatus?: string; fulfillmentStatus?: string; sourceType?: string; sellerId?: string; limit?: number; offset?: number }): Promise<{ items: AdminOrder[]; totalCount: number }> => {
  const query = new URLSearchParams();
  if (params?.q) query.append('q', params.q);
  if (params?.status) query.append('status', params.status);
  if (params?.paymentStatus) query.append('paymentStatus', params.paymentStatus);
  if (params?.fulfillmentStatus) query.append('fulfillmentStatus', params.fulfillmentStatus);
  if (params?.sourceType) query.append('sourceType', params.sourceType);
  if (params?.sellerId) query.append('sellerId', params.sellerId);
  if (params?.limit) query.append('limit', params.limit.toString());
  if (params?.offset) query.append('offset', params.offset.toString());
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request<{ items: AdminOrder[]; totalCount: number }>('GET', `/admin/orders${qStr}`);
};

export const getAdminOrder = async (id: string): Promise<AdminOrderDetail> => {
  return request<AdminOrderDetail>('GET', `/admin/orders/${id}`);
};

export const updateAdminOrderStatus = async (id: string, data: { status: string; comment?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/orders/${id}/status`, { body: data });
};

export const getAdminOrderFulfillments = async (orderId: string): Promise<{ items: AdminFulfillment[]; totalCount: number }> => {
  const res = await request<any>('GET', `/admin/orders/${orderId}/fulfillments`);
  if (Array.isArray(res)) {
    return { items: res, totalCount: res.length };
  }
  return { ...res, items: res?.items || [] };
};

export const getAdminFulfillments = async (params?: { limit?: number; offset?: number; status?: string }): Promise<{ items: AdminFulfillment[]; totalCount: number }> => {
  const query = new URLSearchParams();
  if (params?.limit) query.append('limit', params.limit.toString());
  if (params?.offset) query.append('offset', params.offset.toString());
  if (params?.status) query.append('status', params.status);
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request<{ items: AdminFulfillment[]; totalCount: number }>('GET', `/admin/order-fulfillments${qStr}`);
};

export const getAdminFulfillment = async (id: string): Promise<AdminFulfillment> => {
  return request<AdminFulfillment>('GET', `/admin/order-fulfillments/${id}`);
};

export const getAdminPayments = async (query: string = '', signal?: AbortSignal): Promise<{ items: AdminPayment[]; totalCount: number }> => {
  const res = await request<any>('GET', `/admin/payments${query}`, { signal });
  return { ...res, items: res?.items || [] };
};

export const getAdminPayment = async (id: string): Promise<AdminPayment> => {
  return request<AdminPayment>('GET', `/admin/payments/${id}`);
};

export const getAdminPaymentDetail = async (id: string, signal?: AbortSignal): Promise<import('./types').AdminPaymentDetail> => {
  return request<import('./types').AdminPaymentDetail>('GET', `/admin/payments/${id}`, { signal });
};

export const getAdminShipments = async (): Promise<AdminShipment[]> => {
  const res = await request<any>('GET', '/admin/shipments');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getAdminShipment = async (id: string): Promise<AdminShipment> => {
  return request<AdminShipment>('GET', `/admin/shipments/${id}`);
};

export const createAdminShipment = async (orderId: string, data: { carrier?: string; trackingNumber?: string; trackingUrl?: string }): Promise<AdminShipment> => {
  return request<AdminShipment>('POST', `/admin/orders/${orderId}/shipment`, { body: data });
};

export const createAdminFulfillmentShipment = async (fulfillmentId: string, data: { carrier?: string; trackingNumber?: string; trackingUrl?: string }): Promise<AdminShipment> => {
  return request<AdminShipment>('POST', `/admin/fulfillments/${fulfillmentId}/shipment`, { body: data });
};

export const updateAdminShipmentStatus = async (id: string, data: { status: string; carrier?: string; trackingNumber?: string; trackingUrl?: string; comment?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/shipments/${id}/status`, { body: data });
};

export interface AdminShipmentDeliveryResult {
  shipmentId: string;
  fulfillmentId: string;
  orderId: string;
  shipmentStatus: string;
  fulfillmentStatus: string;
  orderStatus: string;
  deliveredAt: string;
}

export const deliverAdminShipment = async (id: string, data?: { comment?: string }): Promise<AdminShipmentDeliveryResult> => {
  const res = await request<any>('POST', `/admin/shipments/${id}/deliver`, { body: data || {} });
  return {
    shipmentId: res.shipment_id || res.shipmentId,
    fulfillmentId: res.fulfillment_id || res.fulfillmentId,
    orderId: res.order_id || res.orderId,
    shipmentStatus: res.shipment_status || res.shipmentStatus,
    fulfillmentStatus: res.fulfillment_status || res.fulfillmentStatus,
    orderStatus: res.order_status || res.orderStatus,
    deliveredAt: res.delivered_at || res.deliveredAt,
  };
};

export const getAdminReturns = async (): Promise<{ items: AdminReturn[]; totalCount: number }> => {
  const res = await request<any>('GET', '/admin/returns');
  return { ...res, items: res?.items || [] };
};

export const getAdminReturn = async (id: string): Promise<AdminReturn> => {
  return request<AdminReturn>('GET', `/admin/returns/${id}`);
};

export const updateAdminReturnStatus = async (id: string, data: { status: string; adminComment?: string; itemRestock?: Array<{ returnItemId: string; restock: boolean }> }): Promise<void> => {
  return request<void>('PATCH', `/admin/returns/${id}/status`, { body: data });
};

export const approveAdminReturn = async (id: string): Promise<void> => {
  return request<void>('PATCH', `/admin/returns/${id}/status`, { body: { status: 'approved' } });
};

export const rejectAdminReturn = async (id: string, reason: string): Promise<void> => {
  return request<void>('PATCH', `/admin/returns/${id}/status`, { body: { status: 'rejected', adminComment: reason } });
};

export const createAdminRefundForReturn = async (returnId: string, data: { reason?: string }): Promise<AdminRefund> => {
  return request<AdminRefund>('POST', `/admin/returns/${returnId}/refund`, { body: data });
};

export const getAdminRefunds = async (): Promise<{ items: AdminRefund[]; totalCount: number }> => {
  const res = await request<any>('GET', '/admin/refunds');
  return { ...res, items: res?.items || [] };
};

export const getAdminRefund = async (id: string): Promise<AdminRefund> => {
  return request<AdminRefund>('GET', `/admin/refunds/${id}`);
};

export const getAdminPayouts = async (params?: { q?: string; sellerId?: string; status?: string; limit?: number; offset?: number }): Promise<{ items: AdminPayout[]; totalCount: number }> => {
  const query = new URLSearchParams();
  if (params?.q) query.set('q', params.q);
  if (params?.sellerId) query.set('sellerId', params.sellerId);
  if (params?.status) query.set('status', params.status);
  if (params?.limit) query.set('limit', params.limit.toString());
  if (params?.offset) query.set('offset', params.offset.toString());
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request<{ items: AdminPayout[]; totalCount: number }>('GET', `/admin/payouts${qStr}`);
};

export const getAdminPayout = async (id: string): Promise<{ id?: string; payout?: AdminPayout } | AdminPayout> => {
  return request<{ id?: string; payout?: AdminPayout } | AdminPayout>('GET', `/admin/payouts/${id}`);
};

export const updateAdminPayoutStatus = async (id: string, data: { status: string; comment?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/payouts/${id}/status`, { body: data });
};

export const getAdminReviews = async (): Promise<{ items: AdminReview[]; totalCount: number }> => {
  const res = await request<any>('GET', '/admin/reviews');
  return { ...res, items: res?.items || [] };
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

// ---- Staff Management (Phase C) ----

export const getAdminMe = async (): Promise<AdminMeResponse> =>
  request<AdminMeResponse>('GET', '/admin/me');

export const listStaffRoles = async (): Promise<{ items: StaffRoleWithPermissions[] }> =>
  request<{ items: StaffRoleWithPermissions[] }>('GET', '/admin/staff/roles');

export const listStaffMembers = async (): Promise<{ items: StaffMemberView[] }> =>
  request<{ items: StaffMemberView[] }>('GET', '/admin/staff/members');

export const createStaffMember = async (data: CreateStaffMemberRequest): Promise<CreateStaffMemberResponse> =>
  request<CreateStaffMemberResponse>('POST', '/admin/staff/members', { body: data });

export const updateStaffRole = async (userId: string, data: UpdateStaffRoleRequest): Promise<void> =>
  request<void>('PATCH', `/admin/staff/members/${userId}/role`, { body: data });

export const updateStaffStatus = async (userId: string, data: UpdateStaffStatusRequest): Promise<void> =>
  request<void>('PATCH', `/admin/staff/members/${userId}/status`, { body: data });

export const resetStaffPassword = async (userId: string, data: ResetStaffPasswordRequest): Promise<void> =>
  request<void>('POST', `/admin/staff/members/${userId}/reset-password`, { body: data });

export const listAuditLogs = async (params?: { limit?: number; offset?: number; q?: string; action?: string; entityType?: string; actorUserId?: string; entityId?: string; dateFrom?: string; dateTo?: string }): Promise<{ items: any[]; totalCount: number }> => {
  const query = new URLSearchParams();
  if (params?.limit) query.set('limit', params.limit.toString());
  if (params?.offset) query.set('offset', params.offset.toString());
  if (params?.q) query.set('q', params.q);
  if (params?.action) query.set('action', params.action);
  if (params?.entityType) query.set('entityType', params.entityType);
  if (params?.actorUserId) query.set('actorUserId', params.actorUserId);
  if (params?.entityId) query.set('entityId', params.entityId);
  if (params?.dateFrom) query.set('dateFrom', params.dateFrom);
  if (params?.dateTo) query.set('dateTo', params.dateTo);
  
  const qStr = query.toString() ? `?${query.toString()}` : '';
  return request('GET', `/admin/audit-logs${qStr}`);
};

export const getAdminReportsSummary = async (): Promise<any> => {
  return request('GET', '/admin/reports/summary');
};

// ---- Phase E: Seller Management ----

export const getAdminSellerDetail = (id: string) =>
  request<SellerDetail>('GET', `/admin/sellers/${id}`);

export const getAdminSellerOverview = (id: string, period?: string) =>
  request<SellerOverviewData>('GET', `/admin/sellers/${id}/overview${period ? `?period=${period}` : ''}`);

export const verifyAdminSeller = (id: string) =>
  request<{ sellerId: string; status: string }>('POST', `/admin/sellers/${id}/verify`);

export const getSellerStatusHistory = (id: string) =>
  request<{ items: SellerStatusHistoryItem[] }>('GET', `/admin/sellers/${id}/status-history`);

export const listSellerWarnings = (id: string) =>
  request<{ items: SellerWarning[] }>('GET', `/admin/sellers/${id}/warnings`);

export const createSellerWarning = (id: string, data: CreateWarningRequest) =>
  request<SellerWarning>('POST', `/admin/sellers/${id}/warnings`, { body: data });

export const resolveSellerWarning = (sellerId: string, warningId: string, note?: string) =>
  request<void>('PATCH', `/admin/sellers/${sellerId}/warnings/${warningId}/resolve`, { body: { resolutionNote: note } });

export const cancelSellerWarning = (sellerId: string, warningId: string) =>
  request<void>('PATCH', `/admin/sellers/${sellerId}/warnings/${warningId}/cancel`, { body: {} });

export const listSellerViolations = (id: string) =>
  request<{ items: SellerViolation[] }>('GET', `/admin/sellers/${id}/violations`);

export const createSellerViolation = (id: string, data: CreateViolationRequest) =>
  request<SellerViolation>('POST', `/admin/sellers/${id}/violations`, { body: data });

export const resolveSellerViolation = (sellerId: string, violationId: string, note?: string) =>
  request<void>('PATCH', `/admin/sellers/${sellerId}/violations/${violationId}/resolve`, { body: { resolutionNote: note } });

export const cancelSellerViolation = (sellerId: string, violationId: string) =>
  request<void>('PATCH', `/admin/sellers/${sellerId}/violations/${violationId}/cancel`, { body: {} });

// --- Auctions ---
import type { AdminAuction, AdminAuctionLot, AdminAuctionBid } from './types';

export const getAdminAuctions = async (): Promise<{ items: AdminAuction[]; totalCount: number }> => {
  const res = await request<any>('GET', '/admin/auctions');
  return { ...res, items: res?.items || [] };
};

export const updateAuctionLotStatus = async (id: string, status: string, adminNote?: string): Promise<void> => {
  return request<void>('PATCH', `/admin/auction-lots/${id}/status`, { body: { status, adminNote } });
};

export const getAdminAuction = async (id: string): Promise<AdminAuction> => {
  return request<AdminAuction>('GET', `/admin/auctions/${id}`);
};

export const createAdminAuction = async (data: Partial<AdminAuction>): Promise<AdminAuction> => {
  return request<AdminAuction>('POST', '/admin/auctions', { body: data });
};

export const updateAdminAuction = async (id: string, data: Partial<AdminAuction>): Promise<void> => {
  return request<void>('PATCH', `/admin/auctions/${id}`, { body: data });
};

export const publishAdminAuction = async (id: string): Promise<void> => {
  return request<void>('POST', `/admin/auctions/${id}/publish`);
};

export const pauseAdminAuction = async (id: string): Promise<void> => {
  return request<void>('POST', `/admin/auctions/${id}/pause`);
};

export const resumeAdminAuction = async (id: string): Promise<void> => {
  return request<void>('POST', `/admin/auctions/${id}/resume`);
};

export const cancelAdminAuction = async (id: string): Promise<void> => {
  return request<void>('POST', `/admin/auctions/${id}/cancel`);
};

export const finalizeAdminAuction = async (id: string): Promise<void> => {
  return request<void>('POST', `/admin/auctions/${id}/finalize`);
};

export const getAdminLots = async (auctionId: string): Promise<{ items: AdminAuctionLot[]; totalCount: number }> => {
  const res = await request<any>('GET', `/admin/auctions/${auctionId}/lots`);
  return { ...res, items: res?.items || [] };
};

export const createAdminLot = async (auctionId: string, data: Partial<AdminAuctionLot>): Promise<AdminAuctionLot> => {
  return request<AdminAuctionLot>('POST', `/admin/auctions/${auctionId}/lots`, { body: data });
};

export const expireUnpaidAuctions = async (limit?: number): Promise<{ checkedCount: number; expiredCount: number }> => {
  const query = limit ? `?limit=${limit}` : '';
  return request<{ checkedCount: number; expiredCount: number }>('POST', `/admin/auctions/expire-unpaid${query}`);
};

export const updateAdminLot = async (lotId: string, data: Partial<AdminAuctionLot>): Promise<void> => {
  return request<void>('PATCH', `/admin/auction-lots/${lotId}`, { body: data });
};

export const getAdminLotBids = async (lotId: string): Promise<{ items: AdminAuctionBid[]; totalCount: number }> => {
  const res = await request<any>('GET', `/admin/auction-lots/${lotId}/bids`);
  return { ...res, items: res?.items || [] };
};

export const markLotUnpaid = async (lotId: string): Promise<void> => {
  return request<void>('POST', `/admin/auction-lots/${lotId}/mark-unpaid-review`);
};

export const moveLotToDirectSale = async (lotId: string): Promise<void> => {
  return request<void>('POST', `/admin/auction-lots/${lotId}/move-to-direct-sale`);
};

export const getAdminNotifications = async (limit = 20, offset = 0): Promise<import('./types').PaginatedNotifications> => {
  return request<import('./types').PaginatedNotifications>('GET', `/admin/notifications?limit=${limit}&offset=${offset}`);
};

export const getAdminUnreadNotificationsCount = async (): Promise<import('./types').UnreadCountResponse> => {
  return request<import('./types').UnreadCountResponse>('GET', '/admin/notifications/unread-count');
};

export const markAdminNotificationRead = async (id: string): Promise<void> => {
  return request<void>('POST', `/admin/notifications/${id}/read`);
};

export const markAllAdminNotificationsRead = async (): Promise<void> => {
  return request<void>('POST', '/admin/notifications/read-all');
};

export const getDashboardSummary = async (): Promise<AdminDashboardSummary> => {
  return request<AdminDashboardSummary>('GET', '/admin/dashboard/summary');
};

export const resetAdminSellerOwnerPassword = async (id: string): Promise<{ temporaryPassword: string }> => {
  return request<{ temporaryPassword: string }>('POST', `/admin/sellers/${id}/reset-owner-password`);
};

export const updateAdminOrderFulfillmentStatus = async (orderId: string, data: { status: string; reason?: string }): Promise<void> => {
  return request<void>('PATCH', `/admin/orders/${orderId}/fulfillment-status`, { body: data });
};

export const getAdminPayoutSummary = async (): Promise<any> => {
  return await request<any>('GET', '/admin/payouts/summary');
};

export const getAdminSellerBalances = async (params?: { limit?: number; offset?: number }): Promise<any> => {
  const query = new URLSearchParams();
  if (params?.limit) query.set('limit', params.limit.toString());
  if (params?.offset) query.set('offset', params.offset.toString());
  return await request<any>('GET', `/admin/seller-balances?${query.toString()}`);
};


export const listSellerNotes = (id: string) =>
  request<{ items: SellerNote[] }>('GET', `/admin/sellers/${id}/notes`);

export const createSellerNote = (id: string, data: CreateSellerNoteRequest) =>
  request<SellerNote>('POST', `/admin/sellers/${id}/notes`, { body: data });

export const listImprovementPlans = (id: string) =>
  request<{ items: SellerImprovementPlan[] }>('GET', `/admin/sellers/${id}/improvement-plans`);

export const createImprovementPlan = (id: string, data: CreateSellerImprovementPlanRequest) =>
  request<SellerImprovementPlan>('POST', `/admin/sellers/${id}/improvement-plans`, { body: data });

export const updateImprovementPlanStatus = (id: string, planId: string, status: string) =>
  request<void>('PATCH', `/admin/sellers/${id}/improvement-plans/${planId}/status`, { body: { status } });

export const getAdminSellerCommissionHistory = (id: string) =>
  request<any[]>('GET', `/admin/sellers/${id}/commission`);

export const setAdminSellerCommission = (id: string, data: { rateBps: number; reason: string }) =>
  request<void>('POST', `/admin/sellers/${id}/commission`, { body: data });

export interface ResolvedPhysicalUnit {
  inventoryUnitId: string;
  unitCode: string;
  unitStatus: string;
  recommendedAction: string;
  product: {
    title: string;
  };
  variant: {
    color?: string;
    size?: string;
    sellerSku?: string;
    barcode?: string;
  };
  origin: {
    supplyId: string;
    supplyNumber: string;
    supplyStatus: string;
    supplyItemId: string;
    boxNumber?: string;
    sellerName?: string;
  };
  receivingState: {
    activeReceivingSessionId?: string;
  };
}

export const resolvePhysicalUnit = (unitCode: string): Promise<ResolvedPhysicalUnit> => {
  return request<ResolvedPhysicalUnit>('GET', `/admin/receiving/free-scan?unitCode=${encodeURIComponent(unitCode)}`);
};

export interface ProcessFoundUnitRequest {
  unitCode: string;
  condition?: 'ok' | 'damaged';
}

export interface ProcessFoundUnitResponse {
  unitCode: string;
  inventoryUnitId: string;
  supplyId: string;
  supplyNumber: string;
  receivingSessionId?: string;
  condition?: string;
  sessionExpected: number;
  sessionScanned: number;
  sessionOk: number;
  sessionDamaged: number;
  sessionRemaining: number;
  unitStatus: string;
  recommendedNextAction: string;
  productTitle?: string;
  colorName?: string;
  sizeName?: string;
  sellerSku?: string;
  variantBarcode?: string;
  sellerName?: string;
  boxNumber?: string;
}

export const processFoundUnit = (data: ProcessFoundUnitRequest): Promise<ProcessFoundUnitResponse> => {
  return request<ProcessFoundUnitResponse>('POST', '/admin/receiving/free-scan/process', { body: data });
};


export const sendAdminReturnMessage = async (returnId: string, data: AdminSendReturnMessageRequest): Promise<void> => {
  await request('POST', `/admin/returns/${returnId}/messages`, { body: data });
};

export const getAdminReturnMessages = async (returnId: string): Promise<ReturnConversationResponse> => {
  return request<ReturnConversationResponse>('GET', `/admin/returns/${returnId}/messages`);
};

export const uploadAdminReturnMessageAttachment = async (returnId: string, file: File): Promise<import('./types').UploadReturnMessageAttachmentResponse> => {
  const formData = new FormData();
  formData.append('file', file);
  return request<import('./types').UploadReturnMessageAttachmentResponse>('POST', `/admin/returns/${returnId}/messages/attachments`, { body: formData});
};

export const getAdminGlobalSearch = async (query: string): Promise<import('./types').GlobalSearchResponse> => {
  return request<import('./types').GlobalSearchResponse>('GET', `/admin/search?q=${encodeURIComponent(query)}`);
};
