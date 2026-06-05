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

export const createPayment = async (orderId: string): Promise<{ paymentUrl: string }> => {
  return request<{ paymentUrl: string }>('POST', `/customer/orders/${orderId}/pay`);
};

export const createReturn = async (input: ReturnRequest): Promise<ReturnResponse> => {
  return request<ReturnResponse>('POST', '/customer/returns', { body: input });
};

export const createReview = async (input: ReviewCreateRequest): Promise<any> => {
  return request('POST', '/customer/reviews', { body: input });
};
