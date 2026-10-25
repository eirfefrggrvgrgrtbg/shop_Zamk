import type { CustomerFulfillment } from '@zamk/api-client/src/types';

// SHOP.ORDER.TRACKING.1: the customer timeline is derived only from what the backend really reports: the order
// status, createdAt / cancelledAt of the order and shippedAt / deliveredAt of its fulfillments. A date is shown only
// where the backend gives one; no step is invented for a status the order never had.

export type TrackingStepState = 'done' | 'current' | 'pending' | 'cancelled';

export interface TrackingStep {
  key: string;
  label: string;
  state: TrackingStepState;
  /** ISO timestamp from the backend, or undefined when the backend does not report one. */
  date?: string;
  note?: string;
}

const STEP_LABELS = ['Оформлен', 'Оплачен', 'Собирается', 'Передан в доставку', 'Доставлен'] as const;
const STEP_KEYS = ['created', 'paid', 'assembling', 'shipped', 'delivered'] as const;

/** Index of the step an order is currently at (0-based), or 5 when every step is done. */
const CURRENT_STEP: Record<string, number> = {
  awaiting_payment: 1,
  paid: 2,
  assembling: 2,
  packed: 2,
  shipped: 4,
  delivered: 5,
  returned: 5,
  refunded: 5,
};

const CURRENT_NOTE: Record<string, string> = {
  awaiting_payment: 'Ожидает оплаты',
  paid: 'Ожидает сборки',
  assembling: 'Собирается на складе',
  packed: 'Упакован, готовится к передаче в доставку',
  shipped: 'В пути',
};

function earliest(values: Array<string | null | undefined>): string | undefined {
  const dates = values.filter((v): v is string => Boolean(v)).sort();
  return dates[0];
}

function latest(values: Array<string | null | undefined>): string | undefined {
  const dates = values.filter((v): v is string => Boolean(v)).sort();
  return dates[dates.length - 1];
}

export function buildTrackingSteps(input: {
  status: string;
  createdAt?: string;
  cancelledAt?: string | null;
  fulfillments?: CustomerFulfillment[] | null;
}): TrackingStep[] {
  const { status, createdAt, cancelledAt } = input;
  const fulfillments = input.fulfillments ?? [];

  if (status === 'cancelled') {
    return [
      { key: 'created', label: STEP_LABELS[0], state: 'done', date: createdAt },
      { key: 'cancelled', label: 'Отменён', state: 'cancelled', date: cancelledAt ?? undefined },
    ];
  }

  const current = CURRENT_STEP[status];
  if (current === undefined) {
    // an unknown status: show only what is certain
    return [{ key: 'created', label: STEP_LABELS[0], state: 'done', date: createdAt }];
  }

  const shippedAt = earliest(fulfillments.map((f) => f.shippedAt));
  const parcelsTotal = fulfillments.length;
  const parcelsDelivered = fulfillments.filter((f) => f.status === 'delivered').length;
  const delivered = status === 'delivered' || status === 'returned' || status === 'refunded';
  const deliveredAt = delivered && parcelsTotal > 0 && parcelsDelivered === parcelsTotal
    ? latest(fulfillments.map((f) => f.deliveredAt))
    : undefined;

  const steps: TrackingStep[] = STEP_KEYS.map((key, idx) => {
    let state: TrackingStepState = idx < current ? 'done' : idx === current ? 'current' : 'pending';
    let date: string | undefined;
    let note: string | undefined;
    if (idx === 0) date = createdAt;
    if (idx === 3 && idx < current) date = shippedAt;
    if (idx === 4 && delivered) date = deliveredAt;
    if (state === 'current') note = CURRENT_NOTE[status];
    if (idx === 3 && state !== 'done' && parcelsTotal > 1) {
      const shipped = fulfillments.filter((f) => f.status === 'shipped' || f.status === 'delivered').length;
      if (shipped > 0) {
        state = 'current';
        note = `Передано посылок: ${shipped} из ${parcelsTotal}`;
        date = shippedAt;
      }
    }
    return { key, label: STEP_LABELS[idx], state, date, note };
  });

  if (status === 'returned' || status === 'refunded') {
    steps.push({ key: status, label: status === 'returned' ? 'Возврат' : 'Возмещён', state: 'done' });
  }
  return steps;
}

const PARCEL_STATUS_LABELS: Record<string, string> = {
  awaiting_payment: 'Ожидает оплаты',
  paid: 'Ожидает сборки',
  assembling: 'Собирается',
  packed: 'Упакована',
  shipped: 'Передана в доставку',
  delivered: 'Доставлена',
  cancelled: 'Отменена',
  returned: 'Возврат',
  refunded: 'Возмещена',
};

export function parcelStatusLabel(status: string): string {
  return PARCEL_STATUS_LABELS[status] ?? status;
}
