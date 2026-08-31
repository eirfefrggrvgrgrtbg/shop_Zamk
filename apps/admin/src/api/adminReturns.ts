import {
  approveAdminReturn as apiApproveAdminReturn,
  getAdminReturn as apiGetAdminReturn,
  getAdminReturns as apiGetAdminReturns,
  rejectAdminReturn as apiRejectAdminReturn,
  getAdminReturnMessages as getAdminReturnMessagesApi,
  getAdminReturnRefundQuote as apiGetAdminReturnRefundQuote,
  createAdminRefundForReturn as apiCreateAdminRefundForReturn,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type {
  AdminReturn,
  AdminReturnItem,
  AdminReturnEvidence,
  ReturnShipment,
  ReturnShipmentStatus,
  ReturnShipmentMethod,
  AdminReturnRefundQuote,
  AdminReturnRefundQuoteItem,
  AdminRefund,
} from '@zamk/api-client/src/types';
import { formatReturnShipmentStatus, formatReturnShipmentMethod } from '@zamk/api-client/src/types';

export type {
  AdminReturn,
  AdminReturnItem,
  AdminReturnEvidence,
  ReturnShipment,
  ReturnShipmentStatus,
  ReturnShipmentMethod,
  AdminReturnRefundQuote,
  AdminReturnRefundQuoteItem,
  AdminRefund,
};
export { formatReturnShipmentStatus, formatReturnShipmentMethod };

export const RETURN_REASON_LABELS: Record<string, string> = {
  defective: 'Товар неисправен',
  damaged: 'Товар повреждён',
  wrong_item: 'Получен не тот товар',
  not_as_described: 'Не соответствует описанию',
  incomplete: 'Не хватает части комплекта',
  size_fit: 'Не подошёл размер / посадка',
  changed_mind: 'Передумал',
  other: 'Другое',
};

export const getReturnReasonLabel = (reason?: string): string => {
  if (!reason) return '—';
  return RETURN_REASON_LABELS[reason] ?? reason;
};

export const RETURN_STATUS_LABELS: Record<string, string> = {
  requested: 'Новая заявка',
  needs_info: 'Ожидает ответа покупателя',
  approved: 'Возврат одобрен',
  rejected: 'Отклонена',
  receiving: 'Приёмка на складе',
  item_received: 'Товар принят',
  refunded: 'Деньги возвращены',
  completed: 'Завершена',
  cancelled: 'Отменена',
};

export const getReturnStatusLabel = (status: string): string => {
  return RETURN_STATUS_LABELS[status] ?? status;
};

export const getStatusBadgeClass = (status: string): string => {
  switch (status) {
    case 'requested':
      return 'bg-amber-50 text-amber-800 border border-amber-200';
    case 'needs_info':
      return 'bg-yellow-50 text-yellow-800 border border-yellow-200';
    case 'approved':
      return 'bg-blue-50 text-blue-800 border border-blue-200';
    case 'receiving':
      return 'bg-purple-50 text-purple-800 border border-purple-200';
    case 'item_received':
      return 'bg-teal-50 text-teal-800 border border-teal-200';
    case 'refunded':
    case 'completed':
      return 'bg-green-50 text-green-800 border border-green-200';
    case 'rejected':
    case 'cancelled':
      return 'bg-red-50 text-red-800 border border-red-200';
    default:
      return 'bg-gray-50 text-gray-800 border border-gray-200';
  }
};

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

const unwrapItems = <T>(response: ListResponse<T>): T[] => Array.isArray(response) ? response : response.items ?? [];

export const getAdminReturns = async (): Promise<AdminReturn[]> => {
  const response = await apiGetAdminReturns() as unknown as ListResponse<AdminReturn>;
  return unwrapItems(response);
};

export const getAdminReturn = async (id: string): Promise<AdminReturn> => {
  return apiGetAdminReturn(id);
};

export const approveAdminReturn = async (id: string): Promise<void> => {
  await apiApproveAdminReturn(id);
};

export const rejectAdminReturn = async (id: string, reason: string): Promise<void> => {
  await apiRejectAdminReturn(id, reason);
};

export const getAdminReturnRefundQuote = async (returnId: string): Promise<AdminReturnRefundQuote> => {
  return await apiGetAdminReturnRefundQuote(returnId);
};

export const createAdminRefundForReturn = async (returnId: string, reason?: string): Promise<AdminRefund> => {
  return await apiCreateAdminRefundForReturn(returnId, { reason });
};

export const getAdminReturnErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для управления возвратами.';
    if (error.code === 'rejection_reason_required' || error.message?.includes('rejection reason')) {
      return 'Укажите причину отказа в комментарии.';
    }
    if (error.code === 'refund_allocation_invariant') {
      return 'Несогласованное состояние резервирования: количество единиц не соответствует заказу.';
    }
    if (error.code === 'refund_exceeds_paid') {
      return 'Сумма возврата превышает оплаченную сумму.';
    }
    if (error.code === 'payment_not_found') {
      return 'Не найдена успешная оплата по заказу.';
    }
    if (error.code === 'ambiguous_payment') {
      return 'Неоднозначная оплата: обнаружено несколько успешных платежей по заказу.';
    }
    if (error.code === 'refund_no_eligible_items') {
      return 'Нет принятых позиций, подлежащих возврату средств.';
    }
    if (error.code === 'return_not_received') {
      return 'Возврат средств доступен только после приёмки товара на складе.';
    }
    if (error.code === 'return_already_refunded') {
      return 'Возврат средств уже выполнен.';
    }
    if (error.status === 400 || error.code === 'invalid_transition' || error.code === 'validation_error') {
      return error.message || 'Действие отклонено сервером.';
    }
    if (error.status === 404) return 'Возврат не найден.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
};

import type { ReturnConversationResponse } from '@zamk/api-client/src/types';

export const getReturnMessages = async (id: string): Promise<ReturnConversationResponse> => {
  return await getAdminReturnMessagesApi(id);
};
