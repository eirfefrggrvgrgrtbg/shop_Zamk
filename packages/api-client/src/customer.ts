import { request } from './client';
import type { Cart, Order, ReturnRequest, ReturnResponse, ReviewCreateRequest, CustomerFulfillment } from './types';

export const getCart = async (): Promise<Cart> => {
  const res = await request<Cart>('GET', '/customer/cart');
  if (res && !res.items) res.items = [];
  return res;
};

export const addToCart = async (input: { productId: string; productVariantId: string; quantity: number }): Promise<Cart> => {
  return request<Cart>('POST', '/customer/cart/items', { body: input });
};

export const updateCartItem = async (itemId: string, quantity: number): Promise<Cart> => {
  return request<Cart>('PATCH', `/customer/cart/items/${itemId}`, { body: { quantity } });
};

export const removeFromCart = async (itemId: string): Promise<Cart> => {
  return request<Cart>('DELETE', `/customer/cart/items/${itemId}`);
};

export const clearCart = async (): Promise<void> => {
  return request<void>('DELETE', '/customer/cart');
};

export const createOrder = async (input: { customerName: string; customerPhone: string; customerEmail: string; deliveryAddress: string; deliveryMethodId: string }, idempotencyKey?: string): Promise<Order> => {
  const headers: Record<string, string> = {};
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey;
  }
  return request<Order>('POST', '/customer/orders', { body: input, headers });
};

export const getOrders = async (): Promise<Order[]> => {
  const res = await request<any>('GET', '/customer/orders');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const getOrder = async (orderId: string): Promise<Order> => {
  return request<Order>('GET', `/customer/orders/${orderId}`);
};

export const getCustomerOrderFulfillments = async (orderId: string): Promise<CustomerFulfillment[]> => {
  const res = await request<any>('GET', `/customer/orders/${orderId}/fulfillments`);
  return res?.items || (Array.isArray(res) ? res : []);
};

// P0 fix: was /pay, backend route is /payment
export const createPayment = async (orderId: string, method?: string): Promise<{
  paymentId: string;
  paymentNumber: string;
  provider: string;
  paymentMethod: string;
  status: string;
  amountCents: number;
  currency: string;
  paymentUrl: string;
  integrationMode: string;
}> => {
  return request('POST', `/customer/orders/${orderId}/payment`, { body: { method } });
};

// P0 fix: path is /customer/orders/{orderId}/returns, body requires items[]
export const createReturn = async (orderId: string, input: ReturnRequest): Promise<ReturnResponse> => {
  return request<ReturnResponse>('POST', `/customer/orders/${orderId}/returns`, { body: input });
};

export const createReview = async (orderId: string, orderItemId: string, input: ReviewCreateRequest): Promise<any> => {
  return request('POST', `/customer/orders/${orderId}/items/${orderItemId}/review`, { body: input });
};

export const getCustomerReturns = async (): Promise<any> => {
  const res = await request<any>('GET', '/customer/returns');
  return { ...res, items: res?.items || [] };
};

export const getCustomerReturn = async (returnId: string): Promise<any> => {
  return request('GET', `/customer/returns/${returnId}`);
};

export const getCustomerReviews = async (): Promise<any> => {
  const res = await request<any>('GET', '/customer/reviews');
  return { ...res, items: res?.items || [] };
};

export const getFavorites = async (): Promise<any[]> => {
  const res = await request<any>('GET', '/customer/favorites');
  return Array.isArray(res) ? res : (res?.items || []);
};

export const addFavorite = async (productId: string): Promise<any> => {
  return request('POST', `/customer/favorites/${productId}`);
};

export const removeFavorite = async (productId: string): Promise<any> => {
  return request('DELETE', `/customer/favorites/${productId}`);
};

export const getProfile = async (): Promise<any> => {
  return request('GET', '/customer/profile');
};

export const updateProfile = async (input: { firstName: string; lastName: string; middleName?: string; phone: string }): Promise<void> => {
  return request<void>('PATCH', '/customer/profile', { body: input });
};

export const getAddresses = async (): Promise<any> => {
  return request('GET', '/customer/addresses');
};

export const createAddress = async (data: any): Promise<any> => {
  return request('POST', '/customer/addresses', { body: data });
};

export const updateAddress = async (id: string, data: any): Promise<any> => {
  return request('PATCH', `/customer/addresses/${id}`, { body: data });
};

export const deleteAddress = async (id: string): Promise<any> => {
  return request('DELETE', `/customer/addresses/${id}`);
};

export const setDefaultAddress = async (id: string): Promise<any> => {
  return request('POST', `/customer/addresses/${id}/default`);
};

// --- Auctions ---
export interface PlaceBidRequest {
  amountCents: number;
  idempotencyKey?: string;
}

export const placeBid = async (lotId: string, data: PlaceBidRequest): Promise<any> => {
  return request<any>('POST', `/customer/auction-lots/${lotId}/bid`, { body: data });
};

export const getAuctionWins = async (): Promise<any[]> => {
  const res = await request<any>('GET', '/customer/auction-wins');
  return res?.items || (Array.isArray(res) ? res : []);
};

export const createOrderForLot = async (lotId: string): Promise<{ OrderID: string; AmountCents: number }> => {
  return request<{ OrderID: string; AmountCents: number }>('POST', `/customer/auction-lots/${lotId}/create-order`);
};

export const getCustomerNotifications = async (limit = 20, offset = 0): Promise<import('./types').PaginatedNotifications> => {
  const res = await request<import('./types').PaginatedNotifications>('GET', `/customer/notifications?limit=${limit}&offset=${offset}`);
  if (res && !res.items) res.items = [];
  return res;
};

export const getCustomerUnreadNotificationsCount = async (): Promise<import('./types').UnreadCountResponse> => {
  return request<import('./types').UnreadCountResponse>('GET', '/customer/notifications/unread-count');
};

export const markCustomerNotificationRead = async (id: string): Promise<void> => {
  return request<void>('POST', `/customer/notifications/${id}/read`);
};

export const markAllCustomerNotificationsRead = async (): Promise<void> => {
  return request<void>('POST', '/customer/notifications/read-all');
};


export async function uploadReturnEvidence(file: File): Promise<{ id: string; url: string }> {
  const formData = new FormData();
  formData.append('file', file);
  return request<{ id: string; url: string }>('POST', '/customer/returns/evidence/upload', { body: formData });
}

export async function deleteReturnEvidence(evidenceId: string): Promise<void> {
  return request<void>('DELETE', `/customer/returns/evidence/${evidenceId}`);
}

