import {
  getAdminPayment as apiGetAdminPayment,
  getAdminPayments as apiGetAdminPayments,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminPayment } from '@zamk/api-client/src/types';

export interface AdminPaymentView {
  id: string;
  orderId: string;
  provider: string;
  providerPaymentId?: string;
  status: string;
  amount: number;
  amountCents: number;
  currency: string;
  createdAt?: string;
  paidAt?: string;
  failedAt?: string;
  cancelledAt?: string;
}

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

const unwrapItems = <T>(response: ListResponse<T>): T[] => {
  if (Array.isArray(response)) {
    return response;
  }
  return response.items ?? [];
};

export const mapAdminPayment = (payment: AdminPayment): AdminPaymentView => {
  return {
    id: payment.id,
    orderId: payment.orderId,
    provider: payment.provider,
    providerPaymentId: payment.providerPaymentId,
    status: payment.status,
    amount: payment.amountCents / 100,
    amountCents: payment.amountCents,
    currency: payment.currency || 'RUB',
    createdAt: payment.createdAt,
    paidAt: payment.paidAt,
    failedAt: payment.failedAt,
    cancelledAt: payment.cancelledAt,
  };
};

export const getAdminPayments = async (): Promise<AdminPaymentView[]> => {
  const response = await apiGetAdminPayments() as unknown as ListResponse<AdminPayment>;
  return unwrapItems(response).map(mapAdminPayment);
};

export const getAdminPayment = async (id: string): Promise<AdminPaymentView> => {
  return mapAdminPayment(await apiGetAdminPayment(id));
};

export const getAdminPaymentErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'You do not have permission to view payments.';
    if (error.status === 404) return 'Payment was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Network error. Check that the backend API is running and try again.';
  }
  return fallback;
};
