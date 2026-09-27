import {
  type ClientSafeEventType,
  type BehavioralIngestionEvent,
  ingestBehavioralEvents,
} from '@zamk/api-client/src/behavior';
import { ApiError } from '@zamk/api-client/src/errors';
import { getOrCreateVisitorId } from './visitorId';
import { shouldSuppressEvent, recordEventDeduped } from './semanticDedupe';

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

  /**
   * Emits a typed client event into the local queue with dedupe checks and batching.
   */
  public emit(eventType: ClientSafeEventType, options: EmitEventOptions = {}): void {
    try {
      const visitorId = getOrCreateVisitorId();
      const { productId, variantId, quantity, placement } = options;

      // 1. Semantic dedupe check
      if (shouldSuppressEvent(visitorId, eventType, productId, placement)) {
        return;
      }

      // 2. Build stable event payload
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
        occurredAt: new Date().toISOString(),
      };

      if (productId) event.productId = productId;
      if (variantId) event.variantId = variantId;
      if (typeof quantity === 'number') event.quantity = quantity;

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

      // 3. Flush immediately if threshold reached
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

// Convenience tracking functions
export const trackProductView = (
  productId: string,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('product_view', { productId, ...options });
};

export const trackCatalogImpression = (
  productId: string,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('catalog_impression', { productId, ...options });
};

export const trackVariantSelected = (
  productId: string,
  variantId: string,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('product_variant_selected', { productId, variantId, ...options });
};

export const trackFavoriteAdded = (
  productId: string,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('favorite_added', { productId, ...options });
};

export const trackFavoriteRemoved = (
  productId: string,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('favorite_removed', { productId, ...options });
};

export const trackAddToCart = (
  productId: string,
  variantId: string,
  quantity: number,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('add_to_cart', { productId, variantId, quantity, ...options });
};

export const trackRemoveFromCart = (
  productId: string,
  variantId: string,
  quantity: number,
  options?: { placement?: string; route?: string }
): void => {
  behaviorClient.emit('remove_from_cart', { productId, variantId, quantity, ...options });
};

export const trackCheckoutStarted = (options?: { placement?: string; route?: string }): void => {
  behaviorClient.emit('checkout_started', options);
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
