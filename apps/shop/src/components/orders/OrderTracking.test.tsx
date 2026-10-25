/**
 * @vitest-environment jsdom
 */
// SHOP.ORDER.TRACKING.1 — the timeline shows only what the backend really reports.
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import type { CustomerFulfillment } from '@zamk/api-client/src/types';
import { buildTrackingSteps } from './trackingSteps';
import { OrderTracking } from './OrderTracking';

const ful = (over: Partial<CustomerFulfillment>): CustomerFulfillment => ({
  id: 'f1', orderId: 'o1', sellerId: 's1', status: 'shipped', createdAt: '2026-10-01T10:00:00Z', updatedAt: '2026-10-01T10:00:00Z', items: [], ...over,
});

const states = (status: string, extra: Partial<Parameters<typeof buildTrackingSteps>[0]> = {}) =>
  buildTrackingSteps({ status, createdAt: '2026-10-01T10:00:00Z', ...extra }).map((s) => `${s.key}:${s.state}`);

describe('buildTrackingSteps', () => {
  it('awaiting payment: only the order is done, payment is the current step', () => {
    expect(states('awaiting_payment')).toEqual(['created:done', 'paid:current', 'assembling:pending', 'shipped:pending', 'delivered:pending']);
    expect(buildTrackingSteps({ status: 'awaiting_payment' })[1].note).toBe('Ожидает оплаты');
  });

  it('paid / assembling / packed: assembling is current with its own note', () => {
    expect(states('paid')).toEqual(['created:done', 'paid:done', 'assembling:current', 'shipped:pending', 'delivered:pending']);
    expect(buildTrackingSteps({ status: 'paid' })[2].note).toBe('Ожидает сборки');
    expect(buildTrackingSteps({ status: 'assembling' })[2].note).toBe('Собирается на складе');
    expect(buildTrackingSteps({ status: 'packed' })[2].note).toMatch(/Упакован/);
  });

  it('shipped: handed over with the real shipped date, on its way; no delivery date', () => {
    const steps = buildTrackingSteps({ status: 'shipped', createdAt: 'c', fulfillments: [ful({ shippedAt: '2026-10-03T09:00:00Z' })] });
    expect(steps.map((s) => `${s.key}:${s.state}`)).toEqual(['created:done', 'paid:done', 'assembling:done', 'shipped:done', 'delivered:current']);
    expect(steps[3].date).toBe('2026-10-03T09:00:00Z');
    expect(steps[4].note).toBe('В пути');
    expect(steps[4].date).toBeUndefined();
  });

  it('no date is invented when the backend reports none', () => {
    const steps = buildTrackingSteps({ status: 'shipped', fulfillments: [ful({ shippedAt: null })] });
    expect(steps[3].date).toBeUndefined();
    expect(steps[0].date).toBeUndefined();
  });

  it('delivered: every step done, delivery date is the confirmed one', () => {
    const steps = buildTrackingSteps({ status: 'delivered', fulfillments: [ful({ status: 'delivered', shippedAt: '2026-10-03T09:00:00Z', deliveredAt: '2026-10-05T12:00:00Z' })] });
    expect(steps.every((s) => s.state === 'done')).toBe(true);
    expect(steps[4].date).toBe('2026-10-05T12:00:00Z');
  });

  it('delivered without a delivery date from the backend shows none', () => {
    const steps = buildTrackingSteps({ status: 'delivered', fulfillments: [ful({ status: 'delivered', deliveredAt: null })] });
    expect(steps[4].date).toBeUndefined();
  });

  it('two parcels, one delivered: the order is not delivered and has no delivery date', () => {
    const steps = buildTrackingSteps({
      status: 'shipped',
      fulfillments: [ful({ id: 'a', status: 'delivered', shippedAt: '2026-10-03T09:00:00Z', deliveredAt: '2026-10-05T12:00:00Z' }), ful({ id: 'b', status: 'shipped', shippedAt: '2026-10-04T09:00:00Z' })],
    });
    expect(steps[4].state).toBe('current');
    expect(steps[4].date).toBeUndefined();
  });

  it('partial shipment while the order is still packed: shipped step shows how many parcels left', () => {
    const steps = buildTrackingSteps({
      status: 'packed',
      fulfillments: [ful({ id: 'a', status: 'shipped', shippedAt: '2026-10-03T09:00:00Z' }), ful({ id: 'b', status: 'packed' })],
    });
    expect(steps[3].state).toBe('current');
    expect(steps[3].note).toBe('Передано посылок: 1 из 2');
    expect(steps[4].state).toBe('pending');
  });

  it('cancelled: only the order and the cancellation, no invented progress', () => {
    const steps = buildTrackingSteps({ status: 'cancelled', createdAt: 'c', cancelledAt: '2026-10-02T08:00:00Z', fulfillments: [ful({ status: 'cancelled' })] });
    expect(steps.map((s) => `${s.key}:${s.state}`)).toEqual(['created:done', 'cancelled:cancelled']);
    expect(steps[1].date).toBe('2026-10-02T08:00:00Z');
  });

  it('returned / refunded: all steps done and a final step; an unknown status shows only the order', () => {
    expect(states('returned').slice(-1)).toEqual(['returned:done']);
    expect(states('refunded').slice(-1)).toEqual(['refunded:done']);
    expect(states('mystery')).toEqual(['created:done']);
  });
});

describe('OrderTracking', () => {
  afterEach(() => cleanup());

  it('renders Russian labels, dates and the current note', () => {
    render(<OrderTracking status="shipped" createdAt="2026-10-01T10:00:00Z" fulfillments={[ful({ shippedAt: '2026-10-03T09:00:00Z' })]} />);
    expect(screen.getByText('Оформлен')).toBeDefined();
    expect(screen.getByText('Оплачен')).toBeDefined();
    expect(screen.getByText('Собирается')).toBeDefined();
    expect(screen.getByText('Передан в доставку')).toBeDefined();
    expect(screen.getByText('Доставлен')).toBeDefined();
    expect(screen.getByTestId('tracking-step-delivered').textContent).toContain('В пути');
    expect(screen.getByTestId('tracking-step-shipped').textContent).toContain('2026');
    expect(screen.queryByText('Отправлен')).toBeNull();
  });

  it('multiple parcels get their own status lines', () => {
    render(<OrderTracking status="shipped" fulfillments={[ful({ id: 'a', status: 'delivered' }), ful({ id: 'b', status: 'shipped' })]} />);
    const list = screen.getByTestId('tracking-parcels');
    expect(list.textContent).toContain('Посылка №1');
    expect(list.textContent).toContain('Доставлена');
    expect(list.textContent).toContain('Посылка №2');
    expect(list.textContent).toContain('Передана в доставку');
  });

  it('cancelled order shows the cancellation and no later steps or parcels', () => {
    render(<OrderTracking status="cancelled" createdAt="2026-10-01T10:00:00Z" cancelledAt="2026-10-02T08:00:00Z" fulfillments={[ful({ id: 'a' }), ful({ id: 'b' })]} />);
    expect(screen.getByText('Отменён')).toBeDefined();
    expect(screen.queryByText('Доставлен')).toBeNull();
    expect(screen.queryByTestId('tracking-parcels')).toBeNull();
  });

  it('a failed parcel request keeps the timeline and shows a neutral note', () => {
    render(<OrderTracking status="shipped" failed fulfillments={null} />);
    expect(screen.getByTestId('order-tracking')).toBeDefined();
    expect(screen.getByText('Сведения о посылках временно недоступны')).toBeDefined();
  });

  it('while parcels load, no parcel list is shown', () => {
    render(<OrderTracking status="shipped" loading fulfillments={undefined} />);
    expect(screen.queryByTestId('tracking-parcels')).toBeNull();
  });
});
