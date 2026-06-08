import { request } from './client';
import type { Cart, Order, ReturnRequest, ReturnResponse, ReviewCreateRequest } from './types';

export const getCart = async (): Promise<Cart> => {
  return request<Cart>('GET', '/customer/cart');
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

export const createOrder = async (input: { customerName: string; customerPhone: string; customerEmail: string; deliveryAddress: string }): Promise<Order> => {
  return request<Order>('POST', '/customer/orders', { body: input });
};

export const getOrders = async (): Promise<Order[]> => {
  return request<Order[]>('GET', '/customer/orders');
};

export const getOrder = async (orderId: string): Promise<Order> => {
  return request<Order>('GET', `/customer/orders/${orderId}`);
};

// P0 fix: was /pay, backend route is /payment
export const createPayment = async (orderId: string): Promise<{ paymentUrl: string }> => {
  return request<{ paymentUrl: string }>('POST', `/customer/orders/${orderId}/payment`);
};

// P0 fix: path is /customer/orders/{orderId}/returns, body requires items[]
export const createReturn = async (orderId: string, input: ReturnRequest): Promise<ReturnResponse> => {
  return request<ReturnResponse>('POST', `/customer/orders/${orderId}/returns`, { body: input });
};

// P0 fix: path is /customer/orders/{orderId}/items/{orderItemId}/review, field is comment not content
export const createReview = async (orderId: string, orderItemId: string, input: ReviewCreateRequest): Promise<any> => {
  return request('POST', `/customer/orders/${orderId}/items/${orderItemId}/review`, { body: input });
};
