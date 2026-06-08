import {
  getAdminOrder as apiGetAdminOrder,
  getAdminOrders as apiGetAdminOrders,
  updateAdminOrderStatus as apiUpdateAdminOrderStatus,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminOrder, OrderItem } from '@zamk/api-client/src/types';

export interface AdminOrderView {
  id: string;
  status: string;
  statusLabel: string;
  customerName?: string;
  customerPhone?: string;
  customerEmail?: string;
  deliveryAddress?: string;
  totalAmount: number;
  totalPriceCents: number;
  currency: string;
  items: OrderItem[];
  createdAt?: string;
  updatedAt?: string;
}

export interface OrderStatusUpdateInput {
  status: string;
  comment?: string;
}

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

const orderStatusLabels: Record<string, string> = {
  awaiting_payment: 'Ожидает оплаты',
  paid: 'Оплачен',
  assembling: 'Собирается',
  packed: 'Упакован',
  shipped: 'Отгружен',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
  failed: 'Ошибка',
};

const unwrapItems = <T>(response: ListResponse<T>): T[] => {
  if (Array.isArray(response)) {
    return response;
  }
  return response.items ?? [];
};

export const mapAdminOrder = (order: AdminOrder): AdminOrderView => {
  return {
    id: order.id,
    status: order.status,
    statusLabel: orderStatusLabels[order.status] ?? order.status,
    customerName: order.customerName,
    customerPhone: order.customerPhone,
    customerEmail: order.customerEmail,
    deliveryAddress: order.deliveryAddress,
    totalAmount: order.totalPriceCents / 100,
    totalPriceCents: order.totalPriceCents,
    currency: order.currency || 'RUB',
    items: order.items ?? [],
    createdAt: order.createdAt,
    updatedAt: order.updatedAt,
  };
};

export const getAdminOrders = async (): Promise<AdminOrderView[]> => {
  const response = await apiGetAdminOrders() as unknown as ListResponse<AdminOrder>;
  return unwrapItems(response).map(mapAdminOrder);
};

export const getAdminOrder = async (id: string): Promise<AdminOrderView> => {
  return mapAdminOrder(await apiGetAdminOrder(id));
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
