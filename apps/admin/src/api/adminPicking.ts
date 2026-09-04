import { getAccessToken } from '@zamk/api-client/src/tokenStore';
import { ApiError } from '@zamk/api-client/src/errors';
import { API_URL } from '../lib/api';
import { getAdminFulfillments } from './adminOrders';

export interface PickingAllocatedUnit {
  inventoryUnitId: string;
  unitCode: string;
  pickedAt?: string | null;
}

export interface CompatibleUnit {
  inventoryUnitId: string;
  unitCode: string;
  productVariantId: string;
  availability: 'allocated_to_current_item' | 'free';
  pickedAt?: string | null;
}

export interface PickingItem {
  orderItemId: string;
  title: string;
  productVariantId: string;
  variantSize?: string | null;
  variantColor?: string | null;
  imageUrl?: string | null;
  sku?: string | null;
  barcode?: string | null;
  quantity: number;
  pickedQuantity: number;
  remainingQuantity: number;
  allocationMode: 'serialized' | 'legacy';
  allocatedUnits: PickingAllocatedUnit[];
  compatibleUnitsCount?: number;
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
  substituted?: boolean;
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
  orderStatus?: string;
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

export const getCompatibleUnits = async (
  fulfillmentId: string,
  orderItemId: string
): Promise<CompatibleUnit[]> => {
  const token = getAccessToken();
  const res = await fetch(
    `${API_URL}/admin/fulfillments/${fulfillmentId}/picking/compatible-units?orderItemId=${encodeURIComponent(orderItemId)}`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );
  if (!res.ok) {
    let data: any = {};
    try {
      data = await res.json();
    } catch (_) {}
    throw new ApiError(data.message || 'Не удалось загрузить подходящие единицы', data.error || 'get_compatible_units_failed', res.status);
  }
  return res.json();
};

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

export const scanPickingCode = async (
  fulfillmentId: string,
  code: string,
  orderItemId?: string
): Promise<PickingScanResult> => {
  const body: { code: string; orderItemId?: string } = { code: code.trim() };
  if (orderItemId) {
    body.orderItemId = orderItemId;
  }
  const response = await fetch(`${API_URL}/admin/fulfillments/${fulfillmentId}/picking/scan`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
    body: JSON.stringify(body),
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
        orderStatus: po.orderStatus,
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

  // Filter only actionable picking work (eligible fulfillment & order status, unpicked units remain)
  const actionableItems = queueItems.filter(
    (i) =>
      (i.status === 'paid' || i.status === 'assembling') &&
      (!i.orderStatus || i.orderStatus === 'paid' || i.orderStatus === 'assembling') &&
      !i.isComplete &&
      i.remainingQuantity > 0
  );

  // Sort: assembling first, then oldest actionable order first (FIFO)
  return actionableItems.sort((a, b) => {
    if (a.status === 'assembling' && b.status !== 'assembling') return -1;
    if (a.status !== 'assembling' && b.status === 'assembling') return 1;
    return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
  });
};

export const getPickingErrorMessage = (error: unknown, fallback = 'Произошла ошибка при сканировании'): string => {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'unit_variant_mismatch':
        return 'Эта единица относится к другому варианту товара.';
      case 'unit_allocated_to_other_order':
        return 'Эта единица уже предназначена для другого заказа. Возьмите другую.';
      case 'unit_allocated_to_other_order_item':
        return 'Эта единица назначена другой позиции заказа.';
      case 'order_item_required_for_substitution':
        return 'Для выбора свободной единицы сначала откройте конкретную позицию сборки.';
      case 'unit_not_in_warehouse':
        return 'Эта единица сейчас недоступна для сборки.';
      case 'no_unpicked_allocation_for_variant':
        return 'Все единицы этого варианта уже собраны.';
      case 'unit_not_allocated_to_fulfillment':
        return 'Эта единица не относится к текущей сборке';
      case 'cannot_pick_serialized_with_barcode':
        return 'Для этой позиции нужно отсканировать конкретную ZMU';
      case 'already_picked':
        return 'Эта единица уже была отобрана';
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

export interface PackResult {
  fulfillmentId: string;
  orderId: string;
  fulfillmentStatus: string;
  orderStatus: string;
  packedAt: string;
}

export const packFulfillment = async (fulfillmentId: string): Promise<PackResult> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${fulfillmentId}/pack`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Не удалось упаковать заказ', data.error?.code, response.status);
  }
  return data;
};

export const getPackingErrorMessage = (error: unknown, fallback = 'Произошла ошибка при упаковке'): string => {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'packing_not_allowed':
        return 'Упаковка недоступна для текущего статуса заказа или сборки';
      case 'fulfillment_not_fully_picked':
        return 'Нельзя завершить упаковку: не все позиции сборки укомплектованы';
      case 'fulfillment_not_found':
        return 'Сборка не найдена';
      default:
        if (error.status === 403) return 'Недостаточно прав для выполнения упаковки';
        if (error.status === 404) return 'Сборка не найдена';
        return error.message || fallback;
    }
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  return fallback;
};

export interface DispatchResult {
  fulfillmentId: string;
  orderId: string;
  shipmentId: string;
  fulfillmentStatus: string;
  orderStatus: string;
  shipmentStatus: string;
  shippedAt: string;
}

export const dispatchFulfillment = async (fulfillmentId: string): Promise<DispatchResult> => {
  const response = await fetch(`${API_URL}/admin/fulfillments/${fulfillmentId}/dispatch`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  const data = await response.json();
  if (!response.ok) {
    throw new ApiError(data.error?.message || 'Не удалось отгрузить заказ', data.error?.code, response.status);
  }
  return data;
};

export const getDispatchErrorMessage = (error: unknown, fallback = 'Произошла ошибка при отгрузке'): string => {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'dispatch_not_allowed':
        return 'Отгрузка недоступна для текущего статуса сборки или заказа (требуется статус «Упакован»)';
      case 'fulfillment_not_fully_picked':
        return 'Нельзя отгрузить: не все позиции сборки укомплектованы';
      case 'inventory_unit_state_conflict':
        return 'Конфликт состояния физических единиц (товар не находится на складе)';
      case 'insufficient_total_stock':
        return 'Недостаточно остатков на складе для списания';
      case 'insufficient_reserved_stock':
        return 'Недостаточно зарезервированного остатка для списания';
      case 'shipment_contradictory_state':
        return 'Отгрузка уже находится в противоречивом или завершенном статусе';
      case 'fulfillment_not_found':
        return 'Сборка не найдена';
      default:
        if (error.status === 403) return 'Недостаточно прав для выполнения отгрузки';
        if (error.status === 404) return 'Сборка не найдена';
        return error.message || fallback;
    }
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  return fallback;
};
