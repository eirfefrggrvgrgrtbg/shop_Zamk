import { getAccessToken } from '@zamk/api-client/src/tokenStore';
import { ApiError } from '@zamk/api-client/src/errors';
import { API_URL } from '../lib/api';
import { getAdminFulfillments } from './adminOrders';

export interface PickingAllocatedUnit {
  inventoryUnitId: string;
  unitCode: string;
  pickedAt?: string | null;
}

export interface PickingItem {
  orderItemId: string;
  title: string;
  productVariantId: string;
  quantity: number;
  pickedQuantity: number;
  remainingQuantity: number;
  allocationMode: 'serialized' | 'legacy';
  allocatedUnits: PickingAllocatedUnit[];
}

export interface PickingOrder {
  orderId: string;
  orderNumber?: string | null;
  orderStatus: string;
  fulfillmentId: string;
  fulfillmentStatus: string;
  items: PickingItem[];
}

export interface PickingScanDetail {
  code: string;
  type: 'serialized' | 'legacy';
  orderItemId: string;
  newlyPicked: boolean;
  alreadyPicked: boolean;
  alreadyComplete: boolean;
}

export interface PickingItemState {
  quantity: number;
  pickedQuantity: number;
  remainingQuantity: number;
  allocationMode: 'serialized' | 'legacy';
}

export interface PickingProgress {
  totalQuantity: number;
  pickedQuantity: number;
  remainingQuantity: number;
  isComplete: boolean;
}

export interface PickingScanResult {
  fulfillmentId: string;
  orderId: string;
  scanResult: PickingScanDetail;
  item: PickingItemState;
  fulfillmentProgress: PickingProgress;
}

export interface PickingQueueItem {
  fulfillmentId: string;
  orderId: string;
  orderNumber?: string | null;
  status: string;
  sellerName?: string | null;
  customerName?: string | null;
  createdAt: string;
  itemPositionsCount: number;
  totalQuantity: number;
  pickedQuantity: number;
  remainingQuantity: number;
  progressPercent: number;
  isComplete: boolean;
}

export const getAdminPickingOrder = async (fulfillmentId: string): Promise<PickingOrder> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${fulfillmentId}/picking`, {
    headers: {
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Не удалось загрузить данные сборки', data.error?.code, response.status);
  }
  return data;
};

export const scanPickingCode = async (fulfillmentId: string, code: string): Promise<PickingScanResult> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${fulfillmentId}/picking/scan`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
    body: JSON.stringify({ code: code.trim() }),
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Ошибка сканирования', data.error?.code, response.status);
  }
  return data;
};

export const getAdminPickingQueue = async (): Promise<PickingQueueItem[]> => {
  // Fetch eligible fulfillments: paid and assembling (do not swallow failures)
  const [paidFulfillments, assemblingFulfillments] = await Promise.all([
    getAdminFulfillments({ status: 'paid' }),
    getAdminFulfillments({ status: 'assembling' }),
  ]);

  const combined = [...paidFulfillments, ...assemblingFulfillments];
  // Deduplicate by ID
  const uniqueMap = new Map<string, typeof combined[0]>();
  for (const f of combined) {
    uniqueMap.set(f.id, f);
  }
  const uniqueFulfillments = Array.from(uniqueMap.values());

  // Fetch canonical picking progress for each eligible fulfillment
  const queueItems: PickingQueueItem[] = await Promise.all(
    uniqueFulfillments.map(async (f) => {
      const po = await getAdminPickingOrder(f.id);
      const totalQty = po.items.reduce((sum, it) => sum + it.quantity, 0);
      const pickedQty = po.items.reduce((sum, it) => sum + it.pickedQuantity, 0);
      const remQty = Math.max(0, totalQty - pickedQty);
      const percent = totalQty > 0 ? Math.round((pickedQty / totalQty) * 100) : 0;

      return {
        fulfillmentId: f.id,
        orderId: f.orderId,
        orderNumber: f.orderNumber || po.orderNumber,
        status: f.status,
        sellerName: f.sellerName,
        customerName: f.customerName,
        createdAt: f.createdAt,
        itemPositionsCount: po.items.length,
        totalQuantity: totalQty,
        pickedQuantity: pickedQty,
        remainingQuantity: remQty,
        progressPercent: percent,
        isComplete: totalQty > 0 && pickedQty === totalQty,
      };
    })
  );

  // Sort: assembling first, then by createdAt descending
  return queueItems.sort((a, b) => {
    if (a.status === 'assembling' && b.status !== 'assembling') return -1;
    if (a.status !== 'assembling' && b.status === 'assembling') return 1;
    return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
  });
};

export const getPickingErrorMessage = (error: unknown, fallback = 'Произошла ошибка при сканировании'): string => {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'unit_allocated_to_other_order':
        return 'Эта единица зарезервирована для другого заказа';
      case 'unit_not_allocated_to_fulfillment':
        return 'Эта единица не назначена этому заказу';
      case 'unit_not_in_warehouse':
        return 'Эта единица сейчас не находится на складе';
      case 'cannot_pick_serialized_with_barcode':
        return 'Для этого товара нужно отсканировать конкретный ZMU';
      case 'ambiguous_picking_code':
        return 'Штрихкод соответствует нескольким позициям заказа';
      case 'picking_code_not_found':
        return 'Код не найден';
      case 'picking_not_allowed':
        return 'Этот заказ сейчас нельзя собирать';
      default:
        if (error.status === 403) return 'Недостаточно прав для выполнения сборки';
        if (error.status === 404) return 'Код не найден';
        return error.message || fallback;
    }
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  return fallback;
};
