import { request, type RequestOptions } from './client';

export type ClientSafeEventType =
  | 'session_started'
  | 'page_view'
  | 'catalog_impression'
  | 'product_view'
  | 'product_variant_selected'
  | 'favorite_added'
  | 'favorite_removed'
  | 'add_to_cart'
  | 'remove_from_cart'
  | 'checkout_started';

export interface AttributionMetadata {
  referrer?: string;
  landing_path?: string;
  source?: string;
  medium?: string;
  utm_source?: string;
  utm_medium?: string;
  utm_campaign?: string;
  utm_term?: string;
  utm_content?: string;
  zamk_token?: string;
}

export interface BehavioralIngestionEvent {
  eventId: string;
  eventType: ClientSafeEventType;
  visitorId: string;
  sessionId?: string;
  occurredAt: string;
  productId?: string;
  variantId?: string;
  quantity?: number;
  placement?: string;
  route?: string;
  metadata?: AttributionMetadata;
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
