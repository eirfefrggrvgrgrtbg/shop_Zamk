import {
  getAdminReview as apiGetAdminReview,
  getAdminReviews as apiGetAdminReviews,
  moderateAdminReview as apiModerateAdminReview,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type { AdminReview } from '@zamk/api-client/src/types';

type ListResponse<T> = T[] | { items?: T[]; totalCount?: number };
export type ReviewAction = 'approve' | 'reject' | 'hide' | 'block';

export interface AdminReviewView {
  id: string;
  productId: string;
  productTitle?: string;
  sellerId?: string;
  sellerName?: string;
  rating: number;
  title?: string;
  comment?: string;
  status: string;
  statusLabel: string;
  createdAt?: string;
  publishedAt?: string;
  rejectedAt?: string;
  moderationComment?: string;
}

const labels: Record<string, string> = {
  pending_moderation: 'Pending moderation',
  published: 'Published',
  rejected: 'Rejected',
  hidden: 'Hidden',
  blocked: 'Blocked',
};

const unwrapItems = <T>(response: ListResponse<T>): T[] => Array.isArray(response) ? response : response.items ?? [];

export const mapAdminReview = (review: AdminReview): AdminReviewView => ({
  id: review.id,
  productId: review.productId,
  productTitle: review.productTitle,
  sellerId: review.sellerId,
  sellerName: review.sellerName,
  rating: review.rating,
  title: review.title,
  comment: review.comment,
  status: review.status,
  statusLabel: labels[review.status] ?? review.status,
  createdAt: review.createdAt,
  publishedAt: review.publishedAt,
  rejectedAt: review.rejectedAt,
  moderationComment: review.moderationComment,
});

export const getAdminReviews = async (): Promise<AdminReviewView[]> => {
  const response = await apiGetAdminReviews() as unknown as ListResponse<AdminReview>;
  return unwrapItems(response).map(mapAdminReview);
};

export const getAdminReview = async (id: string): Promise<AdminReviewView> => {
  return mapAdminReview(await apiGetAdminReview(id));
};

export const moderateAdminReview = async (id: string, action: ReviewAction, comment?: string): Promise<void> => {
  await apiModerateAdminReview(id, action, comment);
};

export const getAdminReviewErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'You do not have permission to moderate reviews.';
    if (error.status === 400) return 'Review action was rejected by backend rules.';
    if (error.status === 404) return 'Review was not found.';
    if (error.code === 'NETWORK_ERROR') return 'Network error. Check that the backend API is running and try again.';
  }
  return fallback;
};
