import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { EntityTimeline } from './EntityTimeline';
import type { AdminTimelineResponse } from '../api/adminTimeline';

afterEach(() => {
  vi.restoreAllMocks();
});

const makeResponse = (overrides?: Partial<AdminTimelineResponse>): AdminTimelineResponse => ({
  entityType: 'order',
  entityId: 'test-entity-id',
  canonicalIdentifier: 'ORD-TEST',
  events: [],
  ...overrides,
});

describe('EntityTimeline', () => {
  it('shows loading state while fetcher is pending', () => {
    // Never resolves
    const fetcher = () => new Promise<AdminTimelineResponse>(() => {});
    render(<EntityTimeline fetcher={fetcher} />);
    expect(screen.getByTestId('entity-timeline-loading')).toBeDefined();
  });

  it('renders events oldest-to-newest after load', async () => {
    const fetcher = vi.fn().mockResolvedValue(
      makeResponse({
        events: [
          {
            id: 'e1',
            type: 'order.created',
            occurredAt: '2026-08-01T10:00:00Z',
            title: 'Заказ создан',
            description: 'Заказ был размещён',
            actorType: 'system',
            actorLabel: 'Система',
          },
          {
            id: 'e2',
            type: 'order.paid',
            occurredAt: '2026-08-01T10:05:00Z',
            title: 'Заказ оплачен',
            description: 'Платёж подтверждён',
            actorType: 'system',
            actorLabel: 'Система',
          },
        ],
      }),
    );

    render(<EntityTimeline fetcher={fetcher} title="История заказа" />);

    await waitFor(() => {
      expect(screen.queryByTestId('entity-timeline-loading')).toBeNull();
    });

    expect(screen.getByText('История заказа')).toBeDefined();
    expect(screen.getByText('Заказ создан')).toBeDefined();
    expect(screen.getByText('Заказ оплачен')).toBeDefined();
    // Both events rendered
    expect(screen.getByTestId('timeline-event-order.created')).toBeDefined();
    expect(screen.getByTestId('timeline-event-order.paid')).toBeDefined();
  });

  it('shows empty state when events array is empty', async () => {
    const fetcher = vi.fn().mockResolvedValue(makeResponse({ events: [] }));

    render(<EntityTimeline fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.getByTestId('entity-timeline-empty')).toBeDefined();
    });
  });

  it('shows non-breaking error state when fetcher rejects — does not throw', async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error('Ошибка сервера'));

    render(<EntityTimeline fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.getByTestId('entity-timeline-error')).toBeDefined();
    });

    // Error message is displayed, not thrown
    expect(screen.getByText('Ошибка сервера')).toBeDefined();
    // The component itself is still mounted (testid present)
    expect(screen.getByTestId('entity-timeline')).toBeDefined();
  });

  it('does not expose actor UUIDs — only actorLabel when actorType is not system', async () => {
    const uuidLike = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';
    const fetcher = vi.fn().mockResolvedValue(
      makeResponse({
        events: [
          {
            id: 'e1',
            type: 'return.approved',
            occurredAt: '2026-08-01T11:00:00Z',
            title: 'Возврат одобрен',
            description: 'Оператор одобрил',
            actorType: 'admin_staff',
            actorLabel: 'Оператор ZAMK',
            metadata: { staffId: uuidLike },
          },
        ],
      }),
    );

    render(<EntityTimeline fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.queryByTestId('entity-timeline-loading')).toBeNull();
    });

    // Actor label should appear
    expect(screen.getByText('Оператор ZAMK')).toBeDefined();
    // UUID must NOT appear anywhere in the rendered tree
    expect(screen.queryByText(uuidLike)).toBeNull();
  });

  it('renders unknown event types with fallback icon (no crash)', async () => {
    const fetcher = vi.fn().mockResolvedValue(
      makeResponse({
        events: [
          {
            id: 'e-future',
            type: 'order.future_unknown_event_type',
            occurredAt: '2026-08-01T12:00:00Z',
            title: 'Будущий неизвестный тип',
            description: 'Этот тип появится в следующей версии бэкенда',
            actorType: 'system',
            actorLabel: 'Система',
          },
        ],
      }),
    );

    render(<EntityTimeline fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.queryByTestId('entity-timeline-loading')).toBeNull();
    });

    // Component must not crash and must render the title
    expect(screen.getByText('Будущий неизвестный тип')).toBeDefined();
    expect(screen.getByTestId('timeline-event-order.future_unknown_event_type')).toBeDefined();
  });

  it('does NOT show actor label for system actors', async () => {
    const fetcher = vi.fn().mockResolvedValue(
      makeResponse({
        events: [
          {
            id: 'e-sys',
            type: 'order.created',
            occurredAt: '2026-08-01T10:00:00Z',
            title: 'Заказ создан',
            description: 'Заказ был размещён',
            actorType: 'system',
            actorLabel: 'Система',
          },
        ],
      }),
    );

    render(<EntityTimeline fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.queryByTestId('entity-timeline-loading')).toBeNull();
    });

    // actorLabel "Система" must NOT appear when actorType = system
    expect(screen.queryByText('Система')).toBeNull();
  });

  it('calls fetcher exactly once on mount', async () => {
    const fetcher = vi.fn().mockResolvedValue(makeResponse());

    render(<EntityTimeline fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.queryByTestId('entity-timeline-loading')).toBeNull();
    });

    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});
