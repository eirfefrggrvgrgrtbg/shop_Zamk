import {
  createAdminInventoryAdjustment,
  createAdminInventoryReceipt,
  createAdminInventoryWriteOff,
  getAdminInventory as apiGetAdminInventory,
  getAdminInventoryItem as apiGetAdminInventoryItem,
  getAdminInventoryMovements as apiGetAdminInventoryMovements,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminInventoryItem, AdminInventoryMovement } from '@zamk/api-client/src/types';

export interface AdminInventoryView {
  id: string;
  productId: string;
  productTitle: string;
  productVariantId: string;
  variant: string;
  sku?: string;
  size?: string;
  color?: string;
  sellerId?: string;
  sellerName?: string;
  totalStock: number;
  reservedStock: number;
  availableStock: number;
  updatedAt?: string;
}

export interface AdminInventoryMovementView {
  id: string;
  type: string;
  quantity: number;
  reason?: string;
  actor?: string;
  referenceType?: string;
  referenceId?: string;
  createdAt: string;
}

export interface InventoryMutationInput {
  productVariantId: string;
  quantity: number;
  reason?: string;
}

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

const unwrapItems = <T>(response: ListResponse<T>): T[] => {
  if (Array.isArray(response)) {
    return response;
  }
  return response.items ?? [];
};

const getString = (source: Record<string, unknown>, key: string): string | undefined => {
  const value = source[key];
  return typeof value === 'string' ? value : undefined;
};

export const mapInventoryItem = (item: AdminInventoryItem): AdminInventoryView => {
  const flexible = item as unknown as Record<string, unknown>;
  const sku = getString(flexible, 'sku');
  const size = getString(flexible, 'size');
  const color = getString(flexible, 'color');
  const variantParts = [sku, size, color].filter(Boolean);

  return {
    id: item.id,
    productId: item.productId,
    productTitle: getString(flexible, 'productTitle') ?? item.productId,
    productVariantId: item.productVariantId,
    variant: variantParts.length > 0 ? variantParts.join(' / ') : item.productVariantId,
    sku,
    size,
    color,
    sellerId: item.sellerId,
    sellerName: getString(flexible, 'sellerName'),
    totalStock: item.totalStock,
    reservedStock: item.reservedStock,
    availableStock: item.availableStock,
    updatedAt: item.updatedAt,
  };
};

export const mapInventoryMovement = (movement: AdminInventoryMovement): AdminInventoryMovementView => {
  return {
    id: movement.id,
    type: movement.type,
    quantity: movement.quantity,
    reason: movement.reason,
    actor: movement.actorUserId,
    referenceType: movement.referenceType,
    referenceId: movement.referenceId,
    createdAt: movement.createdAt,
  };
};

export const getAdminInventory = async (): Promise<AdminInventoryView[]> => {
  const response = await apiGetAdminInventory() as unknown as ListResponse<AdminInventoryItem>;
  return unwrapItems(response).map(mapInventoryItem);
};

export const getAdminInventoryItem = async (id: string): Promise<AdminInventoryView> => {
  return mapInventoryItem(await apiGetAdminInventoryItem(id));
};

export const getAdminInventoryMovements = async (id: string): Promise<AdminInventoryMovementView[]> => {
  const response = await apiGetAdminInventoryMovements(id) as unknown as ListResponse<AdminInventoryMovement>;
  return unwrapItems(response).map(mapInventoryMovement);
};

export const receiveInventoryStock = async (input: InventoryMutationInput): Promise<void> => {
  await createAdminInventoryReceipt({
    productVariantId: input.productVariantId,
    quantity: input.quantity,
    reason: input.reason || undefined,
  });
};

export const adjustInventoryStock = async (input: InventoryMutationInput): Promise<void> => {
  await createAdminInventoryAdjustment({
    productVariantId: input.productVariantId,
    quantity: input.quantity,
    reason: input.reason || 'Admin adjustment',
  });
};

export const writeOffInventoryStock = async (input: InventoryMutationInput): Promise<void> => {
  await createAdminInventoryWriteOff({
    productVariantId: input.productVariantId,
    quantity: input.quantity,
    reason: input.reason || 'Admin write-off',
  });
};

export const getAdminInventoryErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для управления остатками.';
    if (error.status === 400 || error.code === 'validation_error') return 'Check the inventory request and try again.';
    if (error.code === 'invalid_adjustment' || error.code === 'invalid_write_off') return 'Inventory operation was rejected by backend stock rules.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  return fallback;
};
