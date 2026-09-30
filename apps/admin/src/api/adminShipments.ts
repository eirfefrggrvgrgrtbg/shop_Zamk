import {
  createAdminShipment as apiCreateAdminShipment,
  createAdminFulfillmentShipment as apiCreateAdminFulfillmentShipment,
  deliverAdminShipment as apiDeliverAdminShipment,
  getAdminShipment as apiGetAdminShipment,
  getAdminShipments as apiGetAdminShipments,
  updateAdminShipmentStatus as apiUpdateAdminShipmentStatus,
} from '@zamk/api-client/src/admin';
import type { AdminShipmentDeliveryResult } from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminShipment, AdminShipmentItem } from '@zamk/api-client/src/types';

export type { AdminShipmentDeliveryResult, AdminShipmentItem };

export interface AdminShipmentView {
  id: string;
  shipmentId: string;
  orderId: string;
  orderNumber?: string | null;
  orderStatus?: string;
  fulfillmentId?: string | null;
  fulfillmentStatus?: string;
  sellerId?: string;
  sellerName?: string | null;
  status: string;
  statusLabel: string;
  carrier?: string;
  trackingNumber?: string;
  trackingUrl?: string;
  deliveryMethodName?: string | null;
  itemsCount: number;
  unitsCount: number;
  packedAt?: string | null;
  shippedAt?: string;
  deliveredAt?: string;
  createdAt?: string;
  updatedAt?: string;
  customerName?: string | null;
  customerPhone?: string | null;
  deliveryAddress?: string | null;
  items?: AdminShipmentItem[];
}

export interface ShipmentCreateInput {
  orderId: string;
  carrier?: string;
  trackingNumber?: string;
  trackingUrl?: string;
}

export interface ShipmentStatusInput {
  status: string;
  carrier?: string;
  trackingNumber?: string;
  trackingUrl?: string;
  comment?: string;
}

const shipmentStatusLabels: Record<string, string> = {
  pending: 'Ожидает',
  assembling: 'В сборке',
  packed: 'Упакован',
  shipped: 'Отгружен',
  delivered: 'Доставлен',
  failed: 'Ошибка',
  cancelled: 'Отменен',
};

const shipmentWorkspaceStatusLabels: Record<string, string> = {
  pending: 'Ожидает',
  assembling: 'В сборке',
  packed: 'Готово к отправке',
  shipped: 'В пути',
  delivered: 'Доставлено',
  failed: 'Ошибка доставки',
  cancelled: 'Отправка отменена',
};

const shipmentStatuses = ['pending', 'assembling', 'packed', 'shipped', 'delivered', 'failed', 'cancelled'];
const genericEditableShipmentStatuses = ['pending', 'assembling', 'packed', 'failed', 'cancelled'];

export const getShipmentStatuses = (): string[] => shipmentStatuses;

export const isShipmentEligibleForDelivery = (status: string): boolean => {
  return status === 'shipped';
};

export const getGenericEditableShipmentStatuses = (currentStatus?: string): string[] => {
  if (currentStatus === 'shipped' || currentStatus === 'delivered') {
    return [currentStatus];
  }
  return genericEditableShipmentStatuses;
};

export const getShipmentStatusLabel = (status: string): string => {
  return shipmentStatusLabels[status] ?? status;
};

export const getShipmentWorkspaceStatusLabel = (status: string): string => {
  return shipmentWorkspaceStatusLabels[status] ?? getShipmentStatusLabel(status);
};

export const mapAdminShipment = (shipment: AdminShipment): AdminShipmentView => {
  return {
    id: shipment.id,
    shipmentId: shipment.shipmentId || shipment.id,
    orderId: shipment.orderId,
    orderNumber: shipment.orderNumber,
    orderStatus: shipment.orderStatus,
    fulfillmentId: shipment.fulfillmentId,
    fulfillmentStatus: shipment.fulfillmentStatus,
    sellerId: shipment.sellerId,
    sellerName: shipment.sellerName,
    status: shipment.status,
    statusLabel: getShipmentStatusLabel(shipment.status),
    carrier: shipment.carrier,
    trackingNumber: shipment.trackingNumber,
    trackingUrl: shipment.trackingUrl,
    deliveryMethodName: shipment.deliveryMethodName,
    itemsCount: shipment.itemsCount ?? (shipment.items ? shipment.items.length : 0),
    unitsCount:
      shipment.unitsCount ??
      (shipment.items ? shipment.items.reduce((sum, item) => sum + item.quantity, 0) : 0),
    packedAt: shipment.packedAt,
    shippedAt: shipment.shippedAt,
    deliveredAt: shipment.deliveredAt,
    createdAt: shipment.createdAt,
    updatedAt: shipment.updatedAt,
    customerName: shipment.customerName,
    customerPhone: shipment.customerPhone,
    deliveryAddress: shipment.deliveryAddress,
    items: shipment.items,
  };
};

export const getAdminShipments = async (): Promise<AdminShipmentView[]> => {
  const response = await apiGetAdminShipments();
  const items = Array.isArray(response) ? response : [];
  return items.map(mapAdminShipment);
};

export const getAdminShipment = async (id: string): Promise<AdminShipmentView> => {
  return mapAdminShipment(await apiGetAdminShipment(id));
};

export const createAdminShipment = async (input: ShipmentCreateInput): Promise<AdminShipmentView> => {
  const shipment = await apiCreateAdminShipment(input.orderId, {
    carrier: input.carrier || undefined,
    trackingNumber: input.trackingNumber || undefined,
    trackingUrl: input.trackingUrl || undefined,
  });
  return mapAdminShipment(shipment);
};

export const createAdminFulfillmentShipment = async (fulfillmentId: string, input: Omit<ShipmentCreateInput, 'orderId'>): Promise<AdminShipmentView> => {
  const shipment = await apiCreateAdminFulfillmentShipment(fulfillmentId, {
    carrier: input.carrier || undefined,
    trackingNumber: input.trackingNumber || undefined,
    trackingUrl: input.trackingUrl || undefined,
  });
  return mapAdminShipment(shipment);
};

export const updateAdminShipmentStatus = async (id: string, input: ShipmentStatusInput): Promise<void> => {
  await apiUpdateAdminShipmentStatus(id, {
    status: input.status,
    carrier: input.carrier || undefined,
    trackingNumber: input.trackingNumber || undefined,
    trackingUrl: input.trackingUrl || undefined,
    comment: input.comment || undefined,
  });
};

export const deliverAdminShipment = async (id: string, data?: { comment?: string }): Promise<AdminShipmentDeliveryResult> => {
  return apiDeliverAdminShipment(id, data);
};

export const getDeliveryErrorMessage = (error: unknown, fallback = 'Произошла ошибка при подтверждении доставки'): string => {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'shipment_not_found':
        return 'Отправление не найдено';
      case 'shipment_not_linked_to_fulfillment':
        return 'Отправление не привязано к сборке';
      case 'shipment_already_delivered':
        return 'Отправление уже отмечено как доставленное';
      case 'delivery_not_allowed':
        return 'Доставка недоступна: отправление должно быть в статусе «Отгружен»';
      case 'fulfillment_not_shipped':
        return 'Связанная сборка не находится в статусе «Отгружен»';
      case 'contradictory_shipment_state':
        return 'Состояние отправления противоречит статусу заказа или сборки';
      case 'order_cancelled':
        return 'Заказ отменен';
      case 'fulfillment_not_found':
        return 'Сборка заказа не найдена';
      default:
        if (error.status === 403) return 'Недостаточно прав для подтверждения доставки';
        if (error.status === 404) return 'Отправление не найдено';
        if (error.status === 409) return error.message || 'Конфликт состояния при подтверждении доставки';
        if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
        return error.message || fallback;
    }
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  return fallback;
};

export const getAdminShipmentErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для управления отгрузками.';
    if (error.status === 400) return 'Shipment action was rejected by backend rules.';
    if (error.status === 404) return 'Shipment was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  return fallback;
};
