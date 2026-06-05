import {
  createAdminShipment as apiCreateAdminShipment,
  getAdminShipment as apiGetAdminShipment,
  getAdminShipments as apiGetAdminShipments,
  updateAdminShipmentStatus as apiUpdateAdminShipmentStatus,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminShipment } from '@zamk/api-client/src/types';

export interface AdminShipmentView {
  id: string;
  orderId: string;
  status: string;
  statusLabel: string;
  carrier?: string;
  trackingNumber?: string;
  trackingUrl?: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdAt?: string;
  updatedAt?: string;
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
  pending: 'Pending',
  assembling: 'Assembling',
  packed: 'Packed',
  shipped: 'Shipped',
  delivered: 'Delivered',
  failed: 'Failed',
  cancelled: 'Cancelled',
};

const shipmentStatuses = ['pending', 'assembling', 'packed', 'shipped', 'delivered', 'failed', 'cancelled'];

export const getShipmentStatuses = (): string[] => shipmentStatuses;

export const getShipmentStatusLabel = (status: string): string => {
  return shipmentStatusLabels[status] ?? status;
};

export const mapAdminShipment = (shipment: AdminShipment): AdminShipmentView => {
  return {
    id: shipment.id,
    orderId: shipment.orderId,
    status: shipment.status,
    statusLabel: getShipmentStatusLabel(shipment.status),
    carrier: shipment.carrier,
    trackingNumber: shipment.trackingNumber,
    trackingUrl: shipment.trackingUrl,
    shippedAt: shipment.shippedAt,
    deliveredAt: shipment.deliveredAt,
    createdAt: shipment.createdAt,
    updatedAt: shipment.updatedAt,
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

export const updateAdminShipmentStatus = async (id: string, input: ShipmentStatusInput): Promise<void> => {
  await apiUpdateAdminShipmentStatus(id, {
    status: input.status,
    carrier: input.carrier || undefined,
    trackingNumber: input.trackingNumber || undefined,
    trackingUrl: input.trackingUrl || undefined,
    comment: input.comment || undefined,
  });
};

export const getAdminShipmentErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'You do not have permission to manage shipments.';
    if (error.status === 400) return 'Shipment action was rejected by backend rules.';
    if (error.status === 404) return 'Shipment was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Network error. Check that the backend API is running and try again.';
  }
  return fallback;
};
