import {
  getAdminRefund as apiGetAdminRefund,
  getAdminRefunds as apiGetAdminRefunds,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminRefund } from '@zamk/api-client/src/types';

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

export interface AdminRefundView {
  id: string;
  returnId?: string;
  paymentId?: string;
  orderId: string;
  status: string;
  amount: number;
  amountCents: number;
  currency: string;
  provider?: string;
  providerRefundId?: string;
  reason?: string;
  createdAt?: string;
  processedAt?: string;
  failedAt?: string;
}

const unwrapItems = <T>(response: ListResponse<T>): T[] => Array.isArray(response) ? response : response.items ?? [];

export const mapAdminRefund = (refund: AdminRefund): AdminRefundView => ({
  id: refund.id,
  returnId: refund.returnId,
  paymentId: refund.paymentId,
  orderId: refund.orderId,
  status: refund.status,
  amount: refund.amountCents / 100,
  amountCents: refund.amountCents,
  currency: refund.currency || 'RUB',
  provider: refund.provider,
  providerRefundId: refund.providerRefundId,
  reason: refund.reason,
  createdAt: refund.createdAt,
  processedAt: refund.processedAt,
  failedAt: refund.failedAt,
});

export const getAdminRefunds = async (): Promise<AdminRefundView[]> => {
  const response = await apiGetAdminRefunds() as unknown as ListResponse<AdminRefund>;
  return unwrapItems(response).map(mapAdminRefund);
};

export const getAdminRefund = async (id: string): Promise<AdminRefundView> => {
  return mapAdminRefund(await apiGetAdminRefund(id));
};

export const getAdminRefundErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'You do not have permission to view refunds.';
    if (error.status === 404) return 'Refund was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Network error. Check that the backend API is running and try again.';
  }
  return fallback;
};
