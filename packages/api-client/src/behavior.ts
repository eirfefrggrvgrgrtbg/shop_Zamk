import { request, type RequestOptions } from './client';

export type ClientSafeEventType =
  | 'catalog_impression'
  | 'product_view'
  | 'product_variant_selected'
  | 'favorite_added'
  | 'favorite_removed'
  | 'add_to_cart'
  | 'remove_from_cart'
  | 'checkout_started';

export interface BehavioralIngestionEvent {
  eventId: string;
  eventType: ClientSafeEventType;
  visitorId: string;
  occurredAt: string;
  productId?: string;
  variantId?: string;
  quantity?: number;
  placement?: string;
  route?: string;
  metadata?: Record<string, never>;
}

export interface IngestEventsRequest {
  events: BehavioralIngestionEvent[];
}

export interface RejectedEvent {
  eventId: string;
  code: string;
}

export interface IngestEventsResponse {
  accepted: number;
  duplicates: number;
  rejected: RejectedEvent[];
}

export const ingestBehavioralEvents = async (
  payload: IngestEventsRequest,
  options?: RequestOptions
): Promise<IngestEventsResponse> => {
  return request<IngestEventsResponse>('POST', '/behavior/events', {
    body: payload,
    ...options,
  });
};
