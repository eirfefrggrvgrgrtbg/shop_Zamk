import type { CustomerCreateReviewPayload } from '@zamk/api-client/src/customer';

export const REVIEW_STATUS_LABELS: Record<string, string> = {
  pending_moderation: 'На модерации',
  approved: 'Одобрен',
  published: 'Опубликован',
  rejected: 'Отклонён',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
};

export const REVIEW_STATUS_STYLES: Record<string, string> = {
  pending_moderation: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300 border border-amber-200 dark:border-amber-800/40',
  approved: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/40',
  published: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/40',
  rejected: 'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-300 border border-rose-200 dark:border-rose-800/40',
  hidden: 'bg-gray-100 text-gray-700 dark:bg-white/10 dark:text-white/60 border border-gray-200 dark:border-white/10',
  blocked: 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300 border border-red-200 dark:border-red-800/40',
};

export function shouldShowReviewCTA(orderStatus: string, existingReview?: any | null): boolean {
  const isDelivered = orderStatus === 'delivered' || orderStatus === 'Доставлен';
  if (!isDelivered) return false;
  return !existingReview;
}

export function getReviewStatusBadgeText(existingReview?: any | null): string | null {
  if (!existingReview) return null;
  return REVIEW_STATUS_LABELS[existingReview.status] || existingReview.status;
}

export function validateReviewForm(rating: number, text: string): { isValid: boolean; error?: string } {
  if (rating < 1 || rating > 5) {
    return { isValid: false, error: 'Пожалуйста, выберите оценку от 1 до 5' };
  }
  if (text.length > 1000) {
    return { isValid: false, error: 'Текст отзыва слишком длинный (максимум 1000 символов)' };
  }
  return { isValid: true };
}

export function buildReviewPayload(orderItemId: string, rating: number, title?: string, comment?: string): CustomerCreateReviewPayload {
  return {
    orderItemId,
    rating,
    title: title?.trim() || undefined,
    text: comment?.trim() || undefined,
    comment: comment?.trim() || undefined,
  };
}



export function getItemCountWord(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 19) return 'товаров';
  if (mod10 === 1) return 'товар';
  if (mod10 >= 2 && mod10 <= 4) return 'товара';
  return 'товаров';
}

export function getOrderStatusStyle(rawStatus?: string): string {
  switch (rawStatus) {
    case 'delivered':
      return 'text-emerald-700 bg-emerald-50 dark:text-emerald-300 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/40';
    case 'shipped':
    case 'assembling':
    case 'packed':
      return 'text-blue-700 bg-blue-50 dark:text-blue-300 dark:bg-blue-950/40 border border-blue-200 dark:border-blue-800/40';
    case 'paid':
      return 'text-graphite dark:text-white/80 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20';
    case 'awaiting_payment':
      return 'text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-950/40 border border-amber-200 dark:border-amber-800/40';
    case 'returned':
    case 'refunded':
      return 'text-purple-700 bg-purple-50 dark:text-purple-300 dark:bg-purple-950/40 border border-purple-200 dark:border-purple-800/40';
    case 'cancelled':
      return 'text-rose-700 bg-rose-50 dark:text-rose-300 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800/40';
    default:
      return 'text-graphite dark:text-white/70 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20';
  }
}


