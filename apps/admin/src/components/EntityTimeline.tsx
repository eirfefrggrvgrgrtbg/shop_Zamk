import { useEffect, useState } from 'react';
import {
  CheckCircle2,
  Clock,
  Package,
  Truck,
  CreditCard,
  AlertTriangle,
  RotateCcw,
  Search,
  ShoppingBag,
  MessageSquare,
  Star,
  Archive,
  Loader2,
} from 'lucide-react';
import type { AdminTimelineEvent, AdminTimelineResponse } from '../api/adminTimeline';
import { formatDateTime } from '../utils/orderFormatters';

// ---------------------------------------------------------------------------
// Icon mapping
// ---------------------------------------------------------------------------

type IconName = React.ComponentType<{ className?: string }>;

function resolveIcon(eventType: string): IconName {
  // Order events
  if (eventType === 'order.created') return ShoppingBag;
  if (eventType === 'order.reserved') return Archive;
  if (eventType === 'order.reservation_released') return Archive;
  if (eventType === 'order.paid') return CreditCard;
  if (eventType === 'order.picking_started') return Search;
  if (eventType === 'order.unit_picked') return Package;
  if (eventType === 'order.packed') return Package;
  if (eventType === 'order.shipped') return Truck;
  if (eventType === 'order.delivered') return CheckCircle2;
  if (eventType === 'order.cancelled') return AlertTriangle;

  // Return events
  if (eventType === 'return.requested') return RotateCcw;
  if (eventType === 'return.approved') return CheckCircle2;
  if (eventType === 'return.rejected') return AlertTriangle;
  if (eventType === 'return.info_requested') return MessageSquare;
  if (eventType === 'return.customer_replied') return MessageSquare;
  if (eventType === 'return.logistics_created') return Truck;
  if (eventType === 'return.receiving_started') return Package;
  if (eventType === 'return.unit_scanned') return Package;
  if (eventType === 'return.refunded') return Star;

  // Fallback for unknown future events
  return Clock;
}

function resolveIconColor(eventType: string): string {
  if (eventType === 'order.paid' || eventType === 'order.delivered' || eventType === 'return.approved' || eventType === 'return.refunded') {
    return 'text-emerald-600';
  }
  if (eventType === 'order.cancelled' || eventType === 'return.rejected') {
    return 'text-rose-500';
  }
  if (eventType === 'order.shipped' || eventType === 'return.logistics_created') {
    return 'text-purple-600';
  }
  if (eventType === 'order.packed' || eventType === 'order.unit_picked' || eventType === 'return.unit_scanned' || eventType === 'return.receiving_started') {
    return 'text-blue-600';
  }
  return 'text-indigo-600';
}

function resolveDotColor(eventType: string): string {
  if (eventType === 'order.paid' || eventType === 'order.delivered' || eventType === 'return.approved' || eventType === 'return.refunded') {
    return 'bg-emerald-500';
  }
  if (eventType === 'order.cancelled' || eventType === 'return.rejected') {
    return 'bg-rose-500';
  }
  if (eventType === 'order.shipped' || eventType === 'return.logistics_created') {
    return 'bg-purple-500';
  }
  if (eventType === 'order.packed' || eventType === 'order.unit_picked' || eventType === 'return.unit_scanned' || eventType === 'return.receiving_started') {
    return 'bg-blue-500';
  }
  return 'bg-indigo-500';
}

// ---------------------------------------------------------------------------
// EntityTimeline event list
// ---------------------------------------------------------------------------

interface TimelineEventItemProps {
  event: AdminTimelineEvent;
  isLast: boolean;
}

function TimelineEventItem({ event, isLast }: TimelineEventItemProps) {
  const Icon = resolveIcon(event.type);
  const iconColor = resolveIconColor(event.type);
  const dotColor = resolveDotColor(event.type);

  return (
    <li data-testid={`timeline-event-${event.type}`} className="flex gap-4 relative">
      {/* Vertical connector line */}
      {!isLast && (
        <div className="absolute left-[18px] top-9 bottom-0 w-px bg-gray-200" />
      )}

      {/* Icon circle */}
      <div className={`shrink-0 w-9 h-9 rounded-full border-2 border-white ring-1 ring-gray-200 bg-white flex items-center justify-center z-10 mt-0.5`}>
        <Icon className={`h-4 w-4 ${iconColor}`} />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 pb-6">
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-0.5">
          <span className="font-semibold text-sm text-gray-900">{event.title}</span>
          <span className="text-xs text-gray-400 whitespace-nowrap shrink-0">
            {formatDateTime(event.occurredAt)}
          </span>
        </div>

        {event.description && (
          <p className="text-xs text-gray-600 mt-0.5 leading-relaxed">{event.description}</p>
        )}

        {event.actorLabel && event.actorType !== 'system' && (
          <p className="text-xs text-gray-400 mt-0.5">
            <span className={`inline-block w-1.5 h-1.5 rounded-full ${dotColor} mr-1 align-middle`} />
            {event.actorLabel}
          </p>
        )}
      </div>
    </li>
  );
}

// ---------------------------------------------------------------------------
// Loading state
// ---------------------------------------------------------------------------

function TimelineLoading() {
  return (
    <div
      data-testid="entity-timeline-loading"
      className="flex items-center justify-center gap-2 py-8 text-sm text-gray-400"
    >
      <Loader2 className="h-4 w-4 animate-spin" />
      <span>Загрузка истории...</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Error state (non-breaking — timeline failure must not break dossier)
// ---------------------------------------------------------------------------

function TimelineError({ message }: { message: string }) {
  return (
    <div
      data-testid="entity-timeline-error"
      className="flex items-center gap-2 py-6 text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-lg px-4"
    >
      <AlertTriangle className="h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------

function TimelineEmpty() {
  return (
    <div
      data-testid="entity-timeline-empty"
      className="text-center py-8 text-sm text-gray-400"
    >
      События не найдены.
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public component
// ---------------------------------------------------------------------------

export type TimelineFetcher = () => Promise<AdminTimelineResponse>;

interface EntityTimelineProps {
  /** Called on mount (and optionally on manual refresh). Must return the timeline. */
  fetcher: TimelineFetcher;
  /** Label shown above the list, e.g. "История заказа" */
  title?: string;
  /** Additional className for the outer wrapper */
  className?: string;
}

export function EntityTimeline({ fetcher, title, className = '' }: EntityTimelineProps) {
  const [state, setState] = useState<'loading' | 'loaded' | 'empty' | 'error'>('loading');
  const [events, setEvents] = useState<AdminTimelineEvent[]>([]);
  const [errorMessage, setErrorMessage] = useState('');

  useEffect(() => {
    let cancelled = false;
    setState('loading');
    fetcher()
      .then((resp) => {
        if (cancelled) return;
        const evts = resp.events ?? [];
        setEvents(evts);
        setState(evts.length === 0 ? 'empty' : 'loaded');
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : 'Не удалось загрузить историю.';
        setErrorMessage(msg);
        setState('error');
      });
    return () => { cancelled = true; };
  // fetcher identity must be stable between renders — callers should useCallback or pass arrow in render body
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetcher]);

  return (
    <div data-testid="entity-timeline" className={`space-y-4 ${className}`}>
      {title && (
        <h3 className="text-base font-bold text-gray-900 flex items-center gap-2 border-b border-gray-100 pb-3">
          <Clock className="h-4 w-4 text-indigo-600" />
          <span>{title}</span>
        </h3>
      )}

      {state === 'loading' && <TimelineLoading />}
      {state === 'error' && <TimelineError message={errorMessage} />}
      {state === 'empty' && <TimelineEmpty />}
      {state === 'loaded' && (
        <ul className="space-y-0" role="list">
          {events.map((event, idx) => (
            <TimelineEventItem key={event.id} event={event} isLast={idx === events.length - 1} />
          ))}
        </ul>
      )}
    </div>
  );
}
