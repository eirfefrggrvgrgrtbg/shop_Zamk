import {
  createAdminRefundForReturn,
  getAdminReturn as apiGetAdminReturn,
  getAdminReturns as apiGetAdminReturns,
  updateAdminReturnStatus as apiUpdateAdminReturnStatus,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminRefund, AdminReturn, AdminReturnItem } from '@zamk/api-client/src/types';

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

export interface AdminReturnView {
  id: string;
  orderId: string;
  userId?: string;
  customerName?: string;
  status: string;
  statusLabel: string;
  reason?: string;
  comment?: string;
  adminComment?: string;
  items: AdminReturnItem[];
  createdAt?: string;
  approvedAt?: string;
  rejectedAt?: string;
  completedAt?: string;
}

export interface ReturnStatusInput {
  status: string;
  adminComment?: string;
  itemRestock?: Array<{ returnItemId: string; restock: boolean }>;
}

const labels: Record<string, string> = {
  requested: 'Requested',
  approved: 'Approved',
  rejected: 'Rejected',
  item_received: 'Item received',
  refunded: 'Refunded',
  completed: 'Completed',
  cancelled: 'Cancelled',
};

const unwrapItems = <T>(response: ListResponse<T>): T[] => Array.isArray(response) ? response : response.items ?? [];

export const mapAdminReturn = (ret: AdminReturn): AdminReturnView => {
  const flexible = ret as unknown as Record<string, unknown>;
  return {
    id: ret.id,
    orderId: ret.orderId,
    userId: ret.userId,
    customerName: typeof flexible.customerName === 'string' ? flexible.customerName : undefined,
    status: ret.status,
    statusLabel: labels[ret.status] ?? ret.status,
    reason: ret.reason,
    comment: ret.comment,
    adminComment: ret.adminComment,
    items: ret.items ?? [],
    createdAt: ret.createdAt,
    approvedAt: ret.approvedAt,
    rejectedAt: ret.rejectedAt,
    completedAt: ret.completedAt,
  };
};

export const getReturnStatusTargets = (status: string): string[] => {
  switch (status) {
    case 'requested':
      return ['approved', 'rejected', 'cancelled'];
    case 'approved':
      return ['item_received', 'cancelled'];
    case 'item_received':
      return ['completed'];
    case 'refunded':
      return ['completed'];
    default:
      return [];
  }
};

export const getReturnStatusLabel = (status: string): string => labels[status] ?? status;

export const getAdminReturns = async (): Promise<AdminReturnView[]> => {
  const response = await apiGetAdminReturns() as unknown as ListResponse<AdminReturn>;
  return unwrapItems(response).map(mapAdminReturn);
};

export const getAdminReturn = async (id: string): Promise<AdminReturnView> => {
  return mapAdminReturn(await apiGetAdminReturn(id));
};

export const updateAdminReturnStatus = async (id: string, input: ReturnStatusInput): Promise<void> => {
  await apiUpdateAdminReturnStatus(id, input);
};

export const createAdminRefund = async (returnId: string, reason?: string): Promise<AdminRefund> => {
  return createAdminRefundForReturn(returnId, { reason: reason || undefined });
};

export const getAdminReturnErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'You do not have permission to manage returns.';
    if (error.status === 400 || error.code === 'invalid_transition' || error.code === 'validation_error') return 'Return action was rejected by backend rules.';
    if (error.status === 404) return 'Return was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Network error. Check that the backend API is running and try again.';
  }
  return fallback;
};
