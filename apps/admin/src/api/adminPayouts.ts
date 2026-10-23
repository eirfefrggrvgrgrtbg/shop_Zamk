import {
  getAdminPayout as apiGetAdminPayout,
  getAdminPayouts as apiGetAdminPayouts,
  processAdminPayoutBatch as apiProcessAdminPayoutBatch,
  holdAdminPayoutBatch as apiHoldAdminPayoutBatch,
  updateAdminPayoutStatus as apiUpdateAdminPayoutStatus,
  getAdminPayoutSummary as apiGetAdminPayoutSummary,
  getAdminSellerBalances as apiGetAdminSellerBalances,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type {
  AdminPayout,
  AdminPayoutDetail,
  AdminPayoutEntry,
  AdminPayoutTotals,
  AdminPayoutSummary,
} from '@zamk/api-client/src/types';

export type {
  AdminPayout,
  AdminPayoutDetail,
  AdminPayoutEntry,
  AdminPayoutTotals,
  AdminPayoutSummary,
};

export const payoutStatusLabels: Record<string, string> = {
  scheduled: 'Запланирована',
  processing: 'В обработке',
  paid: 'Выплачена',
  held: 'Заблокирована',
  failed: 'Ошибка',
};

export const getPayoutStatusLabel = (status: string): string =>
  payoutStatusLabels[status] ?? status;

export interface GetAdminPayoutsParams {
  q?: string;
  sellerId?: string;
  status?: string;
  limit?: number;
  offset?: number;
}

export interface GetAdminPayoutsResponse {
  items: AdminPayout[];
  totalCount: number;
}

export const getAdminPayouts = async (
  params?: GetAdminPayoutsParams,
): Promise<GetAdminPayoutsResponse> => {
  const response = await apiGetAdminPayouts(params);
  return {
    items: response?.items ?? [],
    totalCount: response?.totalCount ?? 0,
  };
};

export const getAdminPayout = async (id: string): Promise<AdminPayoutDetail> => {
  return await apiGetAdminPayout(id);
};

export const processAdminPayoutBatch = async (id: string): Promise<void> => {
  await apiProcessAdminPayoutBatch(id);
};

export const holdAdminPayoutBatch = async (id: string): Promise<void> => {
  await apiHoldAdminPayoutBatch(id);
};

export const updateAdminPayoutStatus = async (
  id: string,
  status: string,
  comment?: string,
): Promise<void> => {
  await apiUpdateAdminPayoutStatus(id, { status, comment: comment || undefined });
};

export const getAdminPayoutErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для управления выплатами.';
    if (error.status === 409 || error.code === 'already_paid')
      return 'Выплата уже была подтверждена ранее.';
    if (error.code === 'open_return_not_covered')
      return 'Подтверждение заблокировано: обнаружен открытый возврат товара, претендующий на средства этой выплаты.';
    if (error.code === 'not_processable')
      return 'Выплата не может быть подтверждена в её текущем статусе.';
    if (error.code === 'not_holdable')
      return 'Выплата не может быть заблокирована в её текущем статусе.';
    if (error.code === 'invalid_status_transition')
      return 'Недопустимый переход статуса выплаты.';
    if (error.status === 404) return 'Выплата не найдена.';
    if (error.code === 'NETWORK_ERROR')
      return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
    if (error.message && typeof error.message === 'string' && error.message.length < 200) {
      return error.message;
    }
  }
  return fallback;
};

export const getAdminPayoutSummary = async (): Promise<AdminPayoutSummary> => {
  return await apiGetAdminPayoutSummary();
};

export const getAdminSellerBalances = async (params?: { limit?: number; offset?: number }) => {
  return await apiGetAdminSellerBalances(params);
};
