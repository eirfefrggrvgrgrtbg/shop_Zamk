import {
  createAdminInventoryAdjustment,
  createAdminInventoryReceipt,
  createAdminInventoryWriteOff,
  getAdminInventory as apiGetAdminInventory,
  getAdminInventoryItem as apiGetAdminInventoryItem,
  getAdminInventoryMovements as apiGetAdminInventoryMovements,
  getAdminInventoryUnitTraceability as apiGetAdminInventoryUnitTraceability,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import { request } from '@zamk/api-client/src/client';
import type {
  AdminInventoryItem,
  AdminInventoryMovement,
  AdminInventoryListResponse,
  PhysicalUnitContext,
  AggregateStock,
  PhysicalStock,
  LegacyStock,
  InventoryHealth,
  AdminInventoryPhysicalUnit,
  AdminInventoryAllocationInfo,
  AdminInventorySupplyLineage,
  AdminInventoryUnitTraceability,
  AdminInventoryUnitIdentity,
  AdminInventoryUnitCurrentState,
  AdminInventoryUnitTimelineEvent,
  AdminInventoryUnitContext,
} from '@zamk/api-client/src/types';

export type {
  PhysicalUnitContext,
  AggregateStock,
  PhysicalStock,
  LegacyStock,
  InventoryHealth,
  AdminInventoryPhysicalUnit,
  AdminInventoryAllocationInfo,
  AdminInventorySupplyLineage,
  AdminInventoryUnitTraceability,
  AdminInventoryUnitIdentity,
  AdminInventoryUnitCurrentState,
  AdminInventoryUnitTimelineEvent,
  AdminInventoryUnitContext,
  AdminInventoryMovement,
};

export interface AdminInventoryView {
  id: string;
  productId: string;
  productTitle: string;
  productVariantId: string;
  variant: string;
  sku?: string;
  sellerSku?: string;
  barcode?: string;
  size?: string;
  color?: string;
  mainImageUrl?: string;
  sellerId?: string;
  sellerName?: string;
  source: string;
  totalStock: number;
  reservedStock: number;
  availableStock: number;
  aggregate: AggregateStock;
  physical: PhysicalStock;
  legacy: LegacyStock;
  accountingMode: 'serialized' | 'mixed' | 'legacy';
  health: InventoryHealth;
  physicalUnits?: AdminInventoryPhysicalUnit[];
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
  const sku = item.variantInfo?.sku || getString(flexible, 'sku');
  const size = item.variantInfo?.size || getString(flexible, 'size');
  const color = item.variantInfo?.color || getString(flexible, 'color');
  const variantParts = [sku, size, color].filter(Boolean);

  const agg = item.aggregate || {
    total: item.totalStock,
    reserved: item.reservedStock,
    available: Math.max(item.totalStock - item.reservedStock, 0),
  };

  const phys = item.physical || {
    warehouse: 0,
    allocated: 0,
    picked: 0,
    free: 0,
    expected: 0,
    damaged: 0,
    writtenOff: 0,
    shipped: 0,
  };

  const leg = item.legacy || {
    onHand: agg.total - phys.warehouse,
    reserved: agg.reserved - phys.allocated,
    available: (agg.total - phys.warehouse) - (agg.reserved - phys.allocated),
  };

  const health = item.health || {
    status: 'healthy' as const,
    issues: [],
  };

  const accountingMode = (
    item.accountingMode === 'serialized' || item.accountingMode === 'mixed' || item.accountingMode === 'legacy'
      ? item.accountingMode
      : phys.warehouse > 0 && leg.onHand <= 0
      ? 'serialized'
      : phys.warehouse > 0 && leg.onHand > 0
      ? 'mixed'
      : 'legacy'
  );

  return {
    id: item.id,
    productId: item.productId,
    productTitle: item.product?.title || getString(flexible, 'productTitle') || item.productId,
    productVariantId: item.productVariantId,
    variant: item.variantInfo?.label || (variantParts.length > 0 ? variantParts.join(' / ') : item.productVariantId),
    sku,
    sellerSku: item.variantInfo?.sellerSku,
    barcode: item.variantInfo?.barcode,
    size,
    color,
    mainImageUrl: item.product?.mainImageUrl,
    sellerId: item.sellerId,
    sellerName: item.seller?.name || getString(flexible, 'sellerName'),
    source: getString(flexible, 'source') || 'seller',
    totalStock: agg.total,
    reservedStock: agg.reserved,
    availableStock: agg.available,
    aggregate: agg,
    physical: phys,
    legacy: leg,
    accountingMode,
    health,
    physicalUnits: item.physicalUnits,
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

export const getAdminInventory = async (params?: {
  q?: string;
  sellerId?: string;
  source?: string;
  accountingMode?: string;
  stockStatus?: string;
  lowStock?: boolean;
  limit?: number;
  offset?: number;
}): Promise<{ items: AdminInventoryView[]; totalCount: number; issuesCount: number; unitContext?: PhysicalUnitContext }> => {
  const response = await apiGetAdminInventory(params) as unknown as AdminInventoryListResponse;
  const items = response.items || [];
  return {
    items: items.map(mapInventoryItem),
    totalCount: response.totalCount || 0,
    issuesCount: response.issuesCount || 0,
    unitContext: response.unitContext,
  };
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

export const getAdminInventoryReservations = async (id: string): Promise<any[]> => {
  const resp = await request<{ items: any[] }>('GET', `/admin/inventory/${id}/reservations`);
  return resp.items || [];
};

export const getAdminInventoryUnitTraceability = async (unitCode: string): Promise<AdminInventoryUnitTraceability> => {
  return apiGetAdminInventoryUnitTraceability(unitCode);
};

export interface ReconciliationSession {
  id: string;
  variantId: string;
  status: 'in_progress' | 'review' | 'completed' | 'cancelled';
  startedBy: string;
  startedAt: string;
  completedAt?: string;
  completedBy?: string;
  notes?: string;

  variantTitle?: string;
  variantSize?: string;
  variantColor?: string;
  variantSKU?: string;
  variantBarcode?: string;
  accountingMode?: string;
  legacyOnHand?: number;

  expectedCount: number;
  foundExpectedCount: number;
  unexpectedCount: number;
  problemsCount: number;
}

export interface ReconciliationScanUnitContext {
  unitCode: string;
  productTitle: string;
  size?: string;
  color?: string;
  sku?: string;
  barcode?: string;
  status: string;
}

export interface ScanReconciliationResponse {
  classification: 'expected_found' | 'duplicate' | 'unexpected_found' | 'wrong_variant' | 'unknown_code';
  unit?: AdminInventoryPhysicalUnit;
  unitContext?: ReconciliationScanUnitContext;
  session: ReconciliationSession;
}

export const getReconciliationRoute = (sessionId: string): string => `/inventory/reconciliation/${sessionId}`;

export const startInventoryReconciliation = async (variantId: string): Promise<ReconciliationSession> => {
  return request('POST', '/admin/inventory/reconciliations', { body: { variantId } });
};

export const getInventoryReconciliation = async (sessionId: string): Promise<ReconciliationSession> => {
  return request('GET', `/admin/inventory/reconciliations/${sessionId}`);
};

export const scanInventoryReconciliation = async (sessionId: string, rawCode: string): Promise<ScanReconciliationResponse> => {
  return request('POST', `/admin/inventory/reconciliations/${sessionId}/scan`, { body: { rawCode } });
};

export const completeInventoryReconciliation = async (sessionId: string): Promise<{status: string}> => {
  return request('POST', `/admin/inventory/reconciliations/${sessionId}/complete`);
};

export const getActiveInventoryReconciliation = async (variantId: string): Promise<ReconciliationSession | null> => {
  return request('GET', '/admin/inventory/reconciliations/active' + '?' + new URLSearchParams({ variantId }).toString());
};

export const listInventoryReconciliations = async (variantId: string, limit = 10): Promise<ReconciliationSession[]> => {
  const resp = await request<{ items: ReconciliationSession[] }>('GET', `/admin/inventory/reconciliations?variantId=${variantId}&limit=${limit}`);
  return resp?.items || [];
};

export const moveInventoryReconciliationToReview = async (sessionId: string): Promise<{status: string}> => {
  return request('POST', `/admin/inventory/reconciliations/${sessionId}/review`);
};

export const cancelInventoryReconciliation = async (sessionId: string): Promise<{status: string}> => {
  return request('POST', `/admin/inventory/reconciliations/${sessionId}/cancel`);
};

export interface ReconciliationReviewItem {
  unitId: string;
  unitCode: string;
  snapshotStatus: string;
  currentStatus: string;
  classification: string;
  scannedAt?: string;
}

export interface ReconciliationReview {
  expectedFound: ReconciliationReviewItem[];
  missing: ReconciliationReviewItem[];
  unexpectedFound: ReconciliationReviewItem[];
  changedDuringCount: ReconciliationReviewItem[];
}

export interface ReconciliationResolutionAction {
  id: string;
  label: string;
  safetyLevel: 'NAVIGATION' | 'WORKFLOW_HANDOFF' | 'MUTATION_REQUIRES_CONFIRMATION' | 'BLOCKED';
  route?: string;
  blockedReason?: string;
  enabled: boolean;
}

export interface ReconciliationHistoricalContext {
  orderId?: string;
  orderNumber?: string;
  orderStatus?: string;
  fulfillmentId?: string;
  fulfillmentStatus?: string;
  shipmentId?: string;
  shipmentStatus?: string;
  returnId?: string;
  returnStatus?: string;
  supplyId?: string;
  supplyNumber?: string;
  supplyStatus?: string;
  allocationId?: string;
  pickedAt?: string;
  releasedAt?: string;
  releaseReason?: string;
}

export interface ReconciliationVariantContext {
  productTitle: string;
  size?: string;
  color?: string;
  sku?: string;
  barcode?: string;
}

export interface ReconciliationResolutionAudit {
  actionId: string;
  performedBy: string;
  performedAt: string;
  replacementUnitCode?: string;
}

export interface ReplacementCandidate {
  unitId: string;
  unitCode: string;
  variantId: string;
  status: string;
  createdAt: string;
}

export interface ReconciliationResolutionCase {
  unitId: string;
  unitCode: string;
  variantId: string;
  variant: ReconciliationVariantContext;
  caseType: string;
  title: string;
  severity: 'info' | 'warning' | 'high' | 'critical';
  explanation: string;
  allowedActions: ReconciliationResolutionAction[];
  replacementCandidates?: ReplacementCandidate[];
  historicalContext?: ReconciliationHistoricalContext;
  snapshotStatus?: string;
  currentStatus?: string;
  currentAllocationCtx?: string;
  lineageCtx?: string;
  blockedReason?: string;
  resolution?: ReconciliationResolutionAudit;
}

export interface ReconciliationResolutionPlan {
  sessionId: string;
  cases: ReconciliationResolutionCase[];
  resolutionsCount?: number;
  resolvedCasesCount?: number;
}

export const getReconciliationResolutionPlan = async (sessionId: string): Promise<ReconciliationResolutionPlan> => {
  return request('GET', `/admin/inventory/reconciliations/${sessionId}/resolution-plan`);
};

export interface ResolveReconciliationCaseInput {
  unitId?: string;
  unitCode?: string;
  actionId: string;
  replacementUnitId?: string;
  replacementUnitCode?: string;
  note?: string;
}

export const resolveInventoryReconciliationCase = async (
  sessionId: string,
  input: ResolveReconciliationCaseInput,
): Promise<ReconciliationResolutionPlan> => {
  return request('POST', `/admin/inventory/reconciliations/${sessionId}/resolve`, {
    body: input,
  });
};

export const resolveReconciliationCase = async (
  sessionId: string,
  unitId: string,
  actionId: string,
  replacementUnitId?: string,
  note?: string,
): Promise<ReconciliationResolutionPlan> => {
  return resolveInventoryReconciliationCase(sessionId, {
    unitId,
    actionId,
    replacementUnitId,
    note,
  });
};

export const getInventoryReconciliationReview = async (sessionId: string): Promise<ReconciliationReview> => {
  return request('GET', `/admin/inventory/reconciliations/${sessionId}/review`);
};
