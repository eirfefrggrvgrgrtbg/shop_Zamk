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
  paymentNumber: string;
  status: string;
  paymentMethod?: string;
  integrationMode?: string;
  amount: number;
  amountCents: number;
  currency: string;
  refundState: string;
  paidAmountCents: number;
  refundedAmountCents: number;
  netAmountCents: number;
  availableToRefundCents: number;
  customer?: {
    id: string;
    name: string;
    email: string;
    phone: string;
  };
  attemptNumber?: number;
  attemptsCount?: number;
  problems?: {
    code: string;
    severity: string;
  }[];
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
    id: payment.paymentId,
    orderId: payment.orderId,
    provider: payment.provider || '',
    providerPaymentId: payment.providerPaymentId || undefined,
    paymentNumber: payment.paymentNumber,
    status: payment.status,
    paymentMethod: payment.paymentMethod || undefined,
    integrationMode: payment.integrationMode || undefined,
    amount: payment.amountCents / 100,
    amountCents: payment.amountCents,
    currency: payment.currency || 'RUB',
    refundState: payment.refundState,
    paidAmountCents: payment.paidAmountCents,
    refundedAmountCents: payment.succeededRefundedAmountCents,
    netAmountCents: payment.netAmountCents,
    availableToRefundCents: payment.availableToRefundCents,
    customer: payment.customer || undefined,
    attemptNumber: payment.attemptNumber,
    attemptsCount: payment.attemptsCount,
    problems: payment.problems,
    createdAt: payment.createdAt,
    paidAt: payment.paidAt || undefined,
    failedAt: payment.failedAt || undefined,
    cancelledAt: payment.cancelledAt || undefined,
  };
};

export type PaymentSort = 'createdAt' | 'updatedAt' | 'amount' | 'paymentNumber' | 'status';

export interface AdminPaymentQueryParams {
  q?: string;
  status?: string;
  provider?: string;
  paymentMethod?: string;
  integrationMode?: string;
  refundState?: string;
  hasProblem?: boolean | string;
  problemCode?: string;
  amountFromCents?: string;
  amountToCents?: string;
  dateFrom?: string;
  dateTo?: string;
  sort?: PaymentSort | string;
  direction?: 'asc' | 'desc' | string;
  limit?: number;
  offset?: number;
  signal?: AbortSignal;
}

export const getAdminPayments = async (
  paramsObj: AdminPaymentQueryParams
): Promise<{ items: AdminPaymentView[]; totalCount: number }> => {
  let query = '';
  const params = new URLSearchParams();
  if (paramsObj.q) params.set('q', paramsObj.q);
  if (paramsObj.status) params.set('status', paramsObj.status);
  if (paramsObj.provider) params.set('provider', paramsObj.provider);
  if (paramsObj.paymentMethod) params.set('paymentMethod', paramsObj.paymentMethod);
  if (paramsObj.integrationMode) params.set('integrationMode', paramsObj.integrationMode);
  if (paramsObj.refundState) params.set('refundState', paramsObj.refundState);
  if (paramsObj.hasProblem !== undefined && paramsObj.hasProblem !== '') params.set('hasProblem', String(paramsObj.hasProblem));
  if (paramsObj.problemCode) params.set('problemCode', paramsObj.problemCode);
  if (paramsObj.amountFromCents) params.set('amountFromCents', paramsObj.amountFromCents);
  if (paramsObj.amountToCents) params.set('amountToCents', paramsObj.amountToCents);
  if (paramsObj.dateFrom) params.set('dateFrom', paramsObj.dateFrom);
  if (paramsObj.dateTo) params.set('dateTo', paramsObj.dateTo);
  if (paramsObj.sort) params.set('sort', paramsObj.sort);
  if (paramsObj.direction) params.set('direction', paramsObj.direction);
  if (paramsObj.limit !== undefined) params.set('limit', String(paramsObj.limit));
  if (paramsObj.offset !== undefined) params.set('offset', String(paramsObj.offset));

  if (params.toString()) query = '?' + params.toString();

  const response = await apiGetAdminPayments(query, paramsObj.signal) as unknown as ListResponse<AdminPayment>;
  const items = unwrapItems(response).map(mapAdminPayment);
  const totalCount = !Array.isArray(response) && typeof response === 'object' && response !== null && 'totalCount' in response
    ? (response as any).totalCount
    : items.length;
  
  return { items, totalCount };
};

export const getAdminPayment = async (id: string): Promise<AdminPaymentView> => {
  return mapAdminPayment(await apiGetAdminPayment(id));
};

export const getAdminPaymentErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для просмотра платежей.';
    if (error.status === 404) return 'Payment was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  return fallback;
};
