import { getAccessToken } from '@zamk/api-client/src/tokenStore';
import { ApiError } from '@zamk/api-client/src/errors';
import { API_URL } from '../lib/api';

/** Shape returned by GET /api/admin/orders/{id}/timeline and returns variant */
export interface AdminTimelineEvent {
  id: string;
  type: string;
  occurredAt: string;
  title: string;
  description: string;
  actorType: string;
  actorLabel: string;
  metadata?: Record<string, unknown>;
}

export interface AdminTimelineResponse {
  entityType: string;
  entityId: string;
  canonicalIdentifier: string;
  events: AdminTimelineEvent[];
}

async function fetchTimeline(url: string): Promise<AdminTimelineResponse> {
  const response = await fetch(url, {
    headers: {
      Authorization: `Bearer ${getAccessToken() || ''}`,
    },
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new ApiError(
      data.error?.message || 'Не удалось загрузить историю',
      data.error?.code,
      response.status,
    );
  }
  return response.json();
}

export const getAdminOrderTimeline = (orderId: string): Promise<AdminTimelineResponse> =>
  fetchTimeline(`${API_URL}/admin/orders/${orderId}/timeline`);

export const getAdminReturnTimeline = (returnId: string): Promise<AdminTimelineResponse> =>
  fetchTimeline(`${API_URL}/admin/returns/${returnId}/timeline`);
