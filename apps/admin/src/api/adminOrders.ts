import {
  getAdminOrder as apiGetAdminOrder,
  getAdminOrders as apiGetAdminOrders,
  cancelAdminOrder as apiCancelAdminOrder,
  getAdminOrderFulfillments as apiGetAdminOrderFulfillments,
} from '@zamk/api-client/src/admin';
import { getAccessToken } from '@zamk/api-client/src/tokenStore';
import { ApiError } from '@zamk/api-client/src/errors';
import type {
  AdminOrder,
  AdminOrderDetail,
  AdminFulfillment,
  OrderItem,
  OrderTimelineEvent,
} from '@zamk/api-client';
import { API_URL } from '../lib/api';

export interface AdminOrderView {
  id: string;
  orderNumber?: string;
  status: string;
  statusLabel: string;
  paymentStatus: string;
  paymentStatusLabel: string;
  fulfillmentStatus?: string;
  fulfillmentsCount: number;
  itemPositionsCount: number;
  unitsCount: number;
  sourceType: string;
  customerName?: string;
  customerPhone?: string;
  customerEmail?: string;
  deliveryAddress?: string;
  deliveryMethodName?: string;
  deliveryPriceCents?: number;
  deliveryEstimatedDaysMin?: number;
  deliveryEstimatedDaysMax?: number;
  totalAmount: number;
  totalPriceCents: number;
  currency: string;
  items: OrderItem[];
  fulfillments?: AdminFulfillment[];
  timeline?: OrderTimelineEvent[];
  createdAt?: string;
  updatedAt?: string;
}

export interface CancelAdminOrderInput {
  reason?: string;
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

export const paymentStatusLabels: Record<string, string> = {
  paid: 'Оплачен',
  succeeded: 'Оплачен',
  pending: 'Ожидает оплаты',
  created: 'Ожидает оплаты',
  awaiting_payment: 'Ожидает оплаты',
  failed: 'Ошибка оплаты',
  cancelled: 'Отменен',
};

export const getPaymentStatusLabel = (paymentStatus?: string): string => {
  if (paymentStatus && paymentStatusLabels[paymentStatus]) {
    return paymentStatusLabels[paymentStatus];
  }
  return 'Ожидает оплаты';
};

const unwrapItems = <T>(response: ListResponse<T> | T[]): T[] => {
  if (Array.isArray(response)) {
    return response;
  }
  return response.items ?? [];
};

export const mapAdminOrder = (order: AdminOrderDetail | AdminOrder): AdminOrderView => {
  const fulfillmentsCount = (order as any).fulfillmentsCount ?? 0;
  const rawFulfillmentStatus = order.fulfillmentStatus || '';
  const fulfillmentStatus = fulfillmentsCount > 0 ? (rawFulfillmentStatus || 'paid') : '';

  const rawPaymentStatus = (order as any).paymentStatus;
  const paymentStatus = rawPaymentStatus || 'pending';

  return {
    id: order.id,
    orderNumber: order.orderNumber,
    status: order.status,
    statusLabel: orderStatusLabels[order.status] ?? order.status,
    paymentStatus: paymentStatus,
    paymentStatusLabel: getPaymentStatusLabel(paymentStatus),
    fulfillmentStatus: fulfillmentStatus,
    fulfillmentsCount: fulfillmentsCount,
    itemPositionsCount: (order as any).itemPositionsCount || 0,
    unitsCount: (order as any).unitsCount || 0,
    sourceType: order.sourceType || 'normal',
    customerName: order.customerName,
    customerPhone: (order as AdminOrderDetail).customerPhone,
    customerEmail: order.customerEmail,
    deliveryAddress: (order as AdminOrderDetail).deliveryAddress,
    deliveryMethodName: (order as AdminOrderDetail).deliveryMethodName,
    deliveryPriceCents: (order as AdminOrderDetail).deliveryPriceCents,
    deliveryEstimatedDaysMin: (order as AdminOrderDetail).deliveryEstimatedDaysMin,
    deliveryEstimatedDaysMax: (order as AdminOrderDetail).deliveryEstimatedDaysMax,
    totalAmount: order.totalPriceCents / 100,
    totalPriceCents: order.totalPriceCents,
    currency: order.currency || 'RUB',
    items: (order as AdminOrderDetail).items ?? [],
    fulfillments: (order as AdminOrderDetail).fulfillments,
    timeline: (order as AdminOrderDetail).timeline ?? [],
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

export const cancelAdminOrder = async (id: string, input?: CancelAdminOrderInput): Promise<void> => {
  await apiCancelAdminOrder(id, input);
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

export const getAdminFulfillment = async (id: string): Promise<AdminFulfillment> => {
  const response = await fetch(`${API_URL}/admin/order-fulfillments/${id}`, {
    headers: {
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new ApiError(data.error?.message || 'Не удалось загрузить сборку', data.error?.code, response.status);
  }
  return response.json();
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
