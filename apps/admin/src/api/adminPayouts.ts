import {
  getAdminPayout as apiGetAdminPayout,
  getAdminPayouts as apiGetAdminPayouts,
  updateAdminPayoutStatus as apiUpdateAdminPayoutStatus,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminPayout } from '@zamk/api-client/src/types';

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };

export interface AdminPayoutView {
  id: string;
  sellerId: string;
  sellerName?: string;
  status: string;
  statusLabel: string;
  amount: number;
  amountCents: number;
  currency: string;
  requestedAt?: string;
  approvedAt?: string;
  rejectedAt?: string;
  paidAt?: string;
  adminUserId?: string;
  comment?: string;
}

const labels: Record<string, string> = {
  requested: 'Запрошена',
  approved: 'Одобрена',
  rejected: 'Отклонена',
  paid: 'Выплачена',
  cancelled: 'Отменена',
};

const unwrapItems = <T>(response: ListResponse<T>): T[] => Array.isArray(response) ? response : response.items ?? [];

const unwrapPayout = (response: { payout?: AdminPayout } | AdminPayout): AdminPayout => {
  return 'payout' in response && response.payout ? response.payout : response as AdminPayout;
};

export const mapAdminPayout = (payout: AdminPayout): AdminPayoutView => ({
  id: payout.id,
  sellerId: payout.sellerId,
  sellerName: payout.sellerName,
  status: payout.status,
  statusLabel: labels[payout.status] ?? payout.status,
  amount: payout.amountCents / 100,
  amountCents: payout.amountCents,
  currency: payout.currency || 'RUB',
  requestedAt: payout.requestedAt,
  approvedAt: payout.approvedAt,
  rejectedAt: payout.rejectedAt,
  paidAt: payout.paidAt,
  adminUserId: payout.adminUserId,
  comment: payout.comment,
});

export const getPayoutStatusTargets = (status: string): string[] => {
  switch (status) {
    case 'requested':
      return ['approved', 'rejected'];
    case 'approved':
      return ['paid', 'cancelled'];
    default:
      return [];
  }
};

export const getPayoutStatusLabel = (status: string): string => labels[status] ?? status;

export const getAdminPayouts = async (): Promise<AdminPayoutView[]> => {
  const response = await apiGetAdminPayouts() as unknown as ListResponse<AdminPayout>;
  return unwrapItems(response).map(mapAdminPayout);
};

export const getAdminPayout = async (id: string): Promise<AdminPayoutView> => {
  return mapAdminPayout(unwrapPayout(await apiGetAdminPayout(id)));
};

export const updateAdminPayoutStatus = async (id: string, status: string, comment?: string): Promise<void> => {
  await apiUpdateAdminPayoutStatus(id, { status, comment: comment || undefined });
};

export const getAdminPayoutErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Недостаточно прав для управления выплатами.';
    if (error.status === 400 || error.code === 'invalid_status_transition') return 'Backend отклонил действие по выплате.';
    if (error.status === 404) return 'Выплата не найдена.';
    if (error.code === 'NETWORK_ERROR') return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
  }
  return fallback;
};
