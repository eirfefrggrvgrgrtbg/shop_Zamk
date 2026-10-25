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
    if (error.status === 403) return 'Недостаточно прав для просмотра возмещений.';
    if (error.status === 404) return 'Refund was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  return fallback;
};

// BANK.REFUND.2: bank refund operations.
export {
  getAdminBankRefunds,
  getAdminBankRefund,
  submitAdminBankRefund,
  reconcileAdminBankRefund,
  addAdminBankRefundNote,
} from '@zamk/api-client/src/admin';
export type {
  AdminBankRefund,
  AdminBankRefundDetail,
  AdminBankRefundEvent,
  AdminBankRefundList,
  AdminBankRefundStage,
} from '@zamk/api-client/src/types';

const bankRefundErrorMessages: Record<string, string> = {
  already_submitted: 'Возврат уже передан в банк. Повторная отправка запрещена.',
  refund_provider_disabled: 'Отправка в банк выключена: провайдер возвратов не подключён.',
  payment_not_linked: 'Нет успешной оплаты с банковским идентификатором платежа.',
  legacy_refund: 'Историческая запись не отправляется в банк автоматически.',
  refund_under_review: 'Возврат на ручной сверке: автоматические действия запрещены.',
  provider_unavailable: 'Банк не ответил. Состояние возврата не изменилось, попробуйте сверку позже.',
  validation_error: 'Комментарий должен содержать от 1 до 2000 символов.',
};

// Russian operator message for a bank refund action error; the backend's own Russian reason wins for 409/422.
export const getBankRefundErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 401) return 'Сессия истекла. Войдите снова.';
    if (error.status === 403) return 'Недостаточно прав для этого действия с возвратом.';
    if (error.status === 404) return 'Возврат не найден.';
    if (error.status === 429) return 'Слишком много операций подряд. Повторите через минуту.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
    if ((error.code === 'refund_not_submittable' || error.code === 'refund_not_reconcilable' || error.code === 'refund_not_sendable')
      && error.rawMessage) {
      return error.rawMessage;
    }
    if (error.code && bankRefundErrorMessages[error.code]) return bankRefundErrorMessages[error.code];
  }
  return fallback;
};
