import {
  type ClientSafeEventType,
  type BehavioralIngestionEvent,
  type AttributionMetadata,
  ingestBehavioralEvents,
} from '@zamk/api-client/src/behavior';
import { ApiError } from '@zamk/api-client/src/errors';
import { getOrCreateVisitorId } from './visitorId';
import { shouldSuppressEvent, recordEventDeduped } from './semanticDedupe';
import {
  getOrRenewSession,
  isSessionStartedEmitted,
  markSessionStartedEmitted,
} from './session';

export interface BehaviorClientConfig {
  flushThreshold: number;
  flushIntervalMs: number;
  maxQueueSize: number;
  maxServerBatchSize: number;
  maxEventRetries: number;
}

export const DEFAULT_BEHAVIOR_CONFIG: BehaviorClientConfig = {
  flushThreshold: 20,
  flushIntervalMs: 2000,
  maxQueueSize: 100,
  maxServerBatchSize: 50,
  maxEventRetries: 3,
};

interface QueuedEvent {
  event: BehavioralIngestionEvent;
  retries: number;
}

export interface EmitEventOptions {
  productId?: string;
  variantId?: string;
  quantity?: number;
  placement?: string;
  route?: string;
  metadata?: AttributionMetadata;
}

export class BehaviorEmitter {
  private queue: QueuedEvent[] = [];
  private isFlushing = false;
  private timer: ReturnType<typeof setInterval> | null = null;
  private config: BehaviorClientConfig;

  constructor(config: Partial<BehaviorClientConfig> = {}) {
    this.config = { ...DEFAULT_BEHAVIOR_CONFIG, ...config };
  }

  public start(): void {
    if (this.timer) {
      return;
    }
    this.timer = setInterval(() => {
      this.flush().catch(() => {
        // Telemetry errors are isolated and never thrown
      });
    }, this.config.flushIntervalMs);
  }

  public stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  public getQueueLength(): number {
    return this.queue.length;
  }

  public getQueue(): BehavioralIngestionEvent[] {
    return this.queue.map((item) => item.event);
  }

  public resetForTesting(): void {
    this.queue = [];
    this.isFlushing = false;
    this.stop();
  }

  private cleanRoute(route?: string): string | undefined {
    let r = route;
    if (!r && typeof window !== 'undefined' && window.location) {
      r = window.location.pathname;
    }
    if (!r) {
      return undefined;
    }
    // Strip query string and fragment
    const queryIdx = r.indexOf('?');
    if (queryIdx !== -1) {
      r = r.substring(0, queryIdx);
    }
    const hashIdx = r.indexOf('#');
    if (hashIdx !== -1) {
      r = r.substring(0, hashIdx);
    }
    if (r.length > 255) {
      r = r.substring(0, 255);
    }
    return r.length > 0 ? r : undefined;
  }

  private cleanPlacement(placement?: string): string | undefined {
    if (!placement) return undefined;
    const clean = placement.trim();
    return clean.length > 100 ? clean.substring(0, 100) : clean;
  }

  private enqueueEvent(
    eventType: ClientSafeEventType,
    visitorId: string,
    sessionId: string,
    options: EmitEventOptions = {}
  ): void {
    const { productId, variantId, quantity, placement, metadata } = options;

    // Semantic dedupe check (only applies to product_view and catalog_impression)
    if (shouldSuppressEvent(visitorId, eventType, productId, placement)) {
      return;
    }

    const eventId =
      typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
        ? crypto.randomUUID()
        : 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
            const r = (Math.random() * 16) | 0;
            const v = c === 'x' ? r : (r & 0x3) | 0x8;
            return v.toString(16);
          });

    const event: BehavioralIngestionEvent = {
      eventId,
      eventType,
      visitorId,
      sessionId,
      occurredAt: new Date().toISOString(),
    };

    if (productId) event.productId = productId;
    if (variantId) event.variantId = variantId;
    if (typeof quantity === 'number') event.quantity = quantity;
    if (metadata && Object.keys(metadata).length > 0) event.metadata = metadata;

    const safePlacement = this.cleanPlacement(placement);
    if (safePlacement) event.placement = safePlacement;

    const safeRoute = this.cleanRoute(options.route);
    if (safeRoute) event.route = safeRoute;

    // Bound queue size: if maxQueueSize reached, drop oldest non-critical events
    if (this.queue.length >= this.config.maxQueueSize) {
      this.queue.shift();
    }

    // Add to queue
    this.queue.push({ event, retries: 0 });

    // Record semantic dedupe marker only AFTER event is successfully queued
    recordEventDeduped(visitorId, eventType, productId, placement);
  }

  /**
   * Emits a typed client event into the local queue with dedupe checks and batching.
   * Centrally resolves visitor_id and active session_id.
   */
  public emit(eventType: ClientSafeEventType, options: EmitEventOptions = {}): void {
    try {
      const visitorId = getOrCreateVisitorId();
      const { sessionId } = getOrRenewSession(visitorId);

      if (eventType === 'session_started') {
        markSessionStartedEmitted(sessionId);
      }

      this.enqueueEvent(eventType, visitorId, sessionId, options);

      // Flush immediately if threshold reached
      if (this.queue.length >= this.config.flushThreshold) {
        this.flush().catch(() => {
          // Failure handled internally
        });
      }
    } catch {
      // Telemetry must never crash or throw to callers
    }
  }

  /**
   * Flushes currently queued events to the backend in batches.
   */
  public async flush(): Promise<void> {
    if (this.isFlushing || this.queue.length === 0) {
      return;
    }

    this.isFlushing = true;

    try {
      while (this.queue.length > 0) {
        // Take up to maxServerBatchSize items
        const batch = this.queue.splice(0, this.config.maxServerBatchSize);
        if (batch.length === 0) {
          break;
        }

        const payload = {
          events: batch.map((item) => item.event),
        };

        try {
          // Send via canonical api-client transport
          await ingestBehavioralEvents(payload);
          // 200 or 202 Response: batch successfully processed, events done
        } catch (err: unknown) {
          // Handle transport failures
          if (err instanceof ApiError) {
            const status = err.status;
            // Structural errors (400, 413, 422): drop poison payloads, do not retry
            if (status !== undefined && status >= 400 && status < 500 && status !== 429) {
              if (import.meta.env?.DEV) {
                console.warn('[Telemetry] Structural error, dropping batch:', err.message);
              }
              continue;
            }
          }

          // Network error, 5xx server error, or 429 Too Many Requests -> bounded requeue
          const retryable: QueuedEvent[] = [];
          for (const item of batch) {
            if (item.retries < this.config.maxEventRetries) {
              retryable.push({
                event: item.event, // Preserve exact same eventId and occurredAt
                retries: item.retries + 1,
              });
            }
          }

          // Prepend retryable events to front of queue
          if (retryable.length > 0) {
            this.queue = [...retryable, ...this.queue].slice(0, this.config.maxQueueSize);
          }

          // Break loop on failure to back off for next interval
          break;
        }
      }
    } finally {
      this.isFlushing = false;
    }
  }
}

// Global singleton instance
export const behaviorClient = new BehaviorEmitter();

/**
 * Parses safe attribution metadata from query parameters and referrer.
 * Whitelists only known UTM parameters; strips query, fragment, and userinfo credentials from referrer.
 * Same-origin referrers are excluded from non-direct attribution.
 */
export const parseAttributionMetadata = (
  customSearch?: string,
  customReferrer?: string
): AttributionMetadata => {
  const metadata: AttributionMetadata = {};

  try {
    const search =
      customSearch ??
      (typeof window !== 'undefined' && window.location ? window.location.search : '');

    if (search) {
      const params = new URLSearchParams(search);
      const utms: (keyof AttributionMetadata)[] = [
        'utm_source',
        'utm_medium',
        'utm_campaign',
        'utm_term',
        'utm_content',
      ];
      for (const utm of utms) {
        const val = params.get(utm);
        if (val) {
          metadata[utm] = val.substring(0, 255);
        }
      }
      const rawToken = params.get('zamk_token');
      if (rawToken && rawToken.length >= 8 && rawToken.length <= 128 && /^[A-Za-z0-9_-]+$/.test(rawToken)) {
        metadata.zamk_token = rawToken;
      }
    }

    const rawReferrer =
      customReferrer ?? (typeof document !== 'undefined' ? document.referrer : '');

    if (rawReferrer) {
      try {
        const refUrl = new URL(rawReferrer);
        const currentHostname =
          typeof window !== 'undefined' && window.location
            ? window.location.hostname.toLowerCase()
            : '';

        // Same-origin check: do not attribute if same hostname / origin
        const isSameOrigin =
          currentHostname !== '' &&
          (refUrl.hostname.toLowerCase() === currentHostname ||
            (window.location.origin && refUrl.origin === window.location.origin));

        if (!isSameOrigin) {
          // Normalize: protocol + host + pathname (strips search, hash, and user credentials)
          const cleanRef = `${refUrl.protocol}//${refUrl.host}${refUrl.pathname}`;
          metadata.referrer = cleanRef.substring(0, 1024);

          // External referral classification if not already specified by UTMs
          if (!metadata.utm_source && !metadata.utm_medium) {
            metadata.source = refUrl.hostname.toLowerCase().substring(0, 255);
            metadata.medium = 'referral';
          }
        }
      } catch {
        // Ignore malformed referrer URLs
      }
    }

    if (typeof window !== 'undefined' && window.location && window.location.pathname) {
      metadata.landing_path = window.location.pathname.substring(0, 1024);
    }
  } catch {
    // Best effort parsing
  }

  return metadata;
};

/**
 * Checks if a navigation URL contains new non-direct UTM parameters or tracking token.
 * Used during an active session to pass new attribution metadata on page_view.
 */
export const getNavigationAttributionMetadata = (
  searchStr?: string
): AttributionMetadata | undefined => {
  try {
    const search =
      searchStr ??
      (typeof window !== 'undefined' && window.location ? window.location.search : '');

    if (!search) return undefined;

    const params = new URLSearchParams(search);
    const utmSource = params.get('utm_source');
    const rawToken = params.get('zamk_token');
    if (!utmSource && !rawToken) return undefined;

    const metadata: AttributionMetadata = {};
    const utms: (keyof AttributionMetadata)[] = [
      'utm_source',
      'utm_medium',
      'utm_campaign',
      'utm_term',
      'utm_content',
    ];
    for (const utm of utms) {
      const val = params.get(utm);
      if (val) {
        metadata[utm] = val.substring(0, 255);
      }
    }
    if (rawToken && rawToken.length >= 8 && rawToken.length <= 128 && /^[A-Za-z0-9_-]+$/.test(rawToken)) {
      metadata.zamk_token = rawToken;
    }

    if (typeof window !== 'undefined' && window.location && window.location.pathname) {
      metadata.landing_path = window.location.pathname.substring(0, 1024);
    }

    return metadata;
  } catch {
    return undefined;
  }
};

export const trackSessionStarted = (metadata?: AttributionMetadata): void => {
  try {
    behaviorClient.emit('session_started', { metadata });
  } catch {
    // Failure isolation
  }
};

export const trackPageView = (
  route?: string,
  options?: { metadata?: AttributionMetadata }
): void => {
  try {
    behaviorClient.emit('page_view', { route, metadata: options?.metadata });
  } catch {
    // Failure isolation
  }
};

// Convenience tracking functions
export const trackProductView = (
  productId: string,
  placementOrOptions?: string | { placement?: string; route?: string }
): void => {
  try {
    const opts =
      typeof placementOrOptions === 'string'
        ? { placement: placementOrOptions }
        : placementOrOptions;
    behaviorClient.emit('product_view', { productId, ...opts });
  } catch {
    // Failure isolation
  }
};

export const trackCatalogImpression = (
  productId: string,
  placementOrOptions?: string | { placement?: string; route?: string }
): void => {
  try {
    const opts =
      typeof placementOrOptions === 'string'
        ? { placement: placementOrOptions }
        : placementOrOptions;
    behaviorClient.emit('catalog_impression', { productId, ...opts });
  } catch {
    // Failure isolation
  }
};

export const trackVariantSelected = (
  productId: string,
  variantId: string,
  placementOrOptions?: string | { placement?: string; route?: string }
): void => {
  try {
    const opts =
      typeof placementOrOptions === 'string'
        ? { placement: placementOrOptions }
        : placementOrOptions;
    behaviorClient.emit('product_variant_selected', { productId, variantId, ...opts });
  } catch {
    // Failure isolation
  }
};

export const trackFavoriteAdded = (
  productId: string,
  placementOrOptions?: string | { placement?: string; route?: string }
): void => {
  try {
    const opts =
      typeof placementOrOptions === 'string'
        ? { placement: placementOrOptions }
        : placementOrOptions;
    behaviorClient.emit('favorite_added', { productId, ...opts });
  } catch {
    // Failure isolation
  }
};

export const trackFavoriteRemoved = (
  productId: string,
  placementOrOptions?: string | { placement?: string; route?: string }
): void => {
  try {
    const opts =
      typeof placementOrOptions === 'string'
        ? { placement: placementOrOptions }
        : placementOrOptions;
    behaviorClient.emit('favorite_removed', { productId, ...opts });
  } catch {
    // Failure isolation
  }
};

export interface TrackCartOptions {
  productId: string;
  variantId: string;
  quantity: number;
  placement?: string;
  route?: string;
}

export function trackAddToCart(options: TrackCartOptions): void;
export function trackAddToCart(
  productId: string,
  variantId: string,
  quantity: number,
  options?: { placement?: string; route?: string }
): void;
export function trackAddToCart(
  productIdOrOpts: string | TrackCartOptions,
  variantId?: string,
  quantity?: number,
  options?: { placement?: string; route?: string }
): void {
  try {
    if (typeof productIdOrOpts === 'object') {
      behaviorClient.emit('add_to_cart', productIdOrOpts);
    } else if (variantId && typeof quantity === 'number') {
      behaviorClient.emit('add_to_cart', {
        productId: productIdOrOpts,
        variantId,
        quantity,
        ...options,
      });
    }
  } catch {
    // Failure isolation
  }
}

export function trackRemoveFromCart(options: TrackCartOptions): void;
export function trackRemoveFromCart(
  productId: string,
  variantId: string,
  quantity: number,
  options?: { placement?: string; route?: string }
): void;
export function trackRemoveFromCart(
  productIdOrOpts: string | TrackCartOptions,
  variantId?: string,
  quantity?: number,
  options?: { placement?: string; route?: string }
): void {
  try {
    if (typeof productIdOrOpts === 'object') {
      behaviorClient.emit('remove_from_cart', productIdOrOpts);
    } else if (variantId && typeof quantity === 'number') {
      behaviorClient.emit('remove_from_cart', {
        productId: productIdOrOpts,
        variantId,
        quantity,
        ...options,
      });
    }
  } catch {
    // Failure isolation
  }
}

export const trackCheckoutStarted = (options?: {
  placement?: string;
  route?: string;
}): void => {
  try {
    behaviorClient.emit('checkout_started', options);
  } catch {
    // Failure isolation
  }
};

export const flushBehaviorEvents = (): Promise<void> => {
  return behaviorClient.flush();
};

export const startBehaviorTracking = (): void => {
  behaviorClient.start();
};

export const stopBehaviorTracking = (): void => {
  behaviorClient.stop();
};
