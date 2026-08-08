import {
  getAdminOrder as apiGetAdminOrder,
  getAdminOrders as apiGetAdminOrders,
  updateAdminOrderStatus as apiUpdateAdminOrderStatus,
  getAdminOrderFulfillments as apiGetAdminOrderFulfillments,
} from '@zamk/api-client/src/admin';
import { getAccessToken } from '@zamk/api-client/src/tokenStore';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminOrder, AdminOrderDetail, OrderItem, AdminFulfillment } from '@zamk/api-client/src/types';
import { API_URL } from '../lib/api';

export interface AdminOrderView {
  id: string;
  orderNumber?: string;
  status: string;
  statusLabel: string;
  fulfillmentStatus: string;
  fulfillmentsCount: number;
  itemPositionsCount: number;
  unitsCount: number;
  sourceType: string;
  customerName?: string;
  customerPhone?: string;
  customerEmail?: string;
  deliveryAddress?: string;
  totalAmount: number;
  totalPriceCents: number;
  currency: string;
  items: OrderItem[];
  fulfillments?: AdminFulfillment[];
  createdAt?: string;
  updatedAt?: string;
}

export interface OrderStatusUpdateInput {
  status: string;
  comment?: string;
}

interface ListResponse<T> {
  items: T[];
  totalCount: number;
}

const orderStatusLabels: Record<string, string> = {
  created: 'Создан',
  awaiting_payment: 'Ожидает оплаты',
  paid: 'Оплачен',
  assembling: 'Собирается',
  packed: 'Упакован',
  shipped: 'В пути',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
};

const unwrapItems = <T>(response: ListResponse<T> | T[]): T[] => {
  if (Array.isArray(response)) {
    return response;
  }
  return response.items ?? [];
};

export const mapAdminOrder = (order: AdminOrderDetail | AdminOrder): AdminOrderView => {
  return {
    id: order.id,
    orderNumber: order.orderNumber,
    status: order.status,
    statusLabel: orderStatusLabels[order.status] ?? order.status,
    fulfillmentStatus: order.fulfillmentStatus || 'pending',
    fulfillmentsCount: (order as any).fulfillmentsCount || 0,
    itemPositionsCount: (order as any).itemPositionsCount || 0,
    unitsCount: (order as any).unitsCount || 0,
    sourceType: order.sourceType || 'normal',
    customerName: order.customerName,
    customerPhone: (order as AdminOrderDetail).customerPhone,
    customerEmail: order.customerEmail,
    deliveryAddress: (order as AdminOrderDetail).deliveryAddress,
    totalAmount: order.totalPriceCents / 100,
    totalPriceCents: order.totalPriceCents,
    currency: order.currency || 'RUB',
    items: (order as AdminOrderDetail).items ?? [],
    fulfillments: (order as AdminOrderDetail).fulfillments,
    createdAt: order.createdAt,
    updatedAt: order.updatedAt,
  };
};

export const getAdminOrders = async (params?: Parameters<typeof apiGetAdminOrders>[0]): Promise<{ items: AdminOrderView[]; totalCount: number }> => {
  const response = await apiGetAdminOrders(params);
  return {
    items: response.items.map(mapAdminOrder),
    totalCount: response.totalCount,
  };
};

export const getAdminOrder = async (id: string): Promise<AdminOrderView> => {
  return mapAdminOrder(await apiGetAdminOrder(id));
};

export const getAdminOrderFulfillments = async (id: string): Promise<AdminFulfillment[]> => {
  const response = await apiGetAdminOrderFulfillments(id) as unknown as ListResponse<AdminFulfillment>;
  return unwrapItems(response);
};

export const getAllowedOrderStatusTargets = (status: string): string[] => {
  switch (status) {
    case 'awaiting_payment':
      return ['cancelled'];
    case 'paid':
      return ['assembling', 'cancelled'];
    case 'assembling':
      return ['packed', 'cancelled'];
    case 'packed':
      return ['shipped', 'cancelled'];
    case 'shipped':
      return ['delivered'];
    default:
      return [];
  }
};

export const getOrderStatusLabel = (status: string): string => {
  return orderStatusLabels[status] ?? status;
};

export const updateAdminOrderStatus = async (id: string, input: OrderStatusUpdateInput): Promise<void> => {
  if (input.status === 'paid') {
    throw new Error('Администратор не может вручную установить статус оплаты.');
  }
  await apiUpdateAdminOrderStatus(id, input);
};

export const getAdminFulfillments = async (params?: { status?: string }): Promise<AdminFulfillment[]> => {
  const query = params?.status ? `?status=${encodeURIComponent(params.status)}` : '';
  const response = await fetch(`${API_URL}/admin/order-fulfillments${query}`, {
    headers: {
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new ApiError(data.error?.message || 'Failed to fetch fulfillments', data.error?.code, response.status);
  }
  const data = await response.json();
  return data.items || data;
};

export const resolveReceivingCode = async (code: string): Promise<AdminFulfillment> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/resolve-receiving-code`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
    body: JSON.stringify({ code }),
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Сборка не найдена', data.error?.code, response.status);
  }
  return data;
};

export const startReceiving = async (id: string): Promise<any> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${id}/receiving/start`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Ошибка начала приёмки', data.error?.code, response.status);
  }
  return data;
};

export const scanItem = async (id: string, payload: { barcode: string; expectedVersion: number; idempotencyKey: string }): Promise<any> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${id}/receiving/scan-item`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
    body: JSON.stringify(payload),
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Ошибка сканирования товара', data.error?.code, response.status);
  }
  return data;
};

export const confirmReceiving = async (id: string, payload: any): Promise<any> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${id}/receiving/confirm`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
    body: JSON.stringify(payload),
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Ошибка подтверждения приёмки', data.error?.code, response.status);
  }
  return data;
};

export const recordDiscrepancy = async (id: string, payload: any): Promise<any> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${id}/receiving/discrepancy`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
    body: JSON.stringify(payload),
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Ошибка записи расхождения', data.error?.code, response.status);
  }
  return data;
};

export const getAdminOrderErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для управления заказами.';
    if (error.status === 400 || error.code === 'validation_error' || error.code === 'invalid_status') {
      return 'Backend отклонил изменение статуса заказа.';
    }
    if (error.status === 404) return 'Заказ не найден.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  if (error instanceof Error && error.message.includes('paid')) {
    return 'Администратор не может вручную установить статус оплаты. Этот переход выполняет платёжный webhook.';
  }
  return fallback;
};
