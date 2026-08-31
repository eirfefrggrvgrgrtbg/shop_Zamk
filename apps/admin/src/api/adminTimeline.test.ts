import { describe, it, expect, vi, afterEach } from 'vitest';

// We test the pure type contract and transform logic of the adminTimeline module.
// No real network calls are made.

import type { AdminTimelineEvent, AdminTimelineResponse } from './adminTimeline';

vi.mock('@zamk/api-client/src/tokenStore', () => ({
  getAccessToken: () => 'test-token',
}));

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Type contract tests — ensure the shape is correct
// ---------------------------------------------------------------------------

describe('AdminTimelineEvent shape', () => {
  it('has all required fields', () => {
    const event: AdminTimelineEvent = {
      id: 'evt-1',
      type: 'order.created',
      occurredAt: '2026-08-01T10:00:00Z',
      title: 'Заказ создан',
      description: 'Заказ был размещён покупателем',
      actorType: 'system',
      actorLabel: 'Система',
    };
    expect(event.id).toBe('evt-1');
    expect(event.occurredAt).toBe('2026-08-01T10:00:00Z');
  });

  it('accepts optional metadata field', () => {
    const event: AdminTimelineEvent = {
      id: 'evt-2',
      type: 'order.unit_picked',
      occurredAt: '2026-08-01T11:00:00Z',
      title: 'Единица собрана',
      description: 'ZMU-ABC',
      actorType: 'admin_staff',
      actorLabel: 'Оператор',
      metadata: { unitCode: 'ZMU-ABC' },
    };
    expect(event.metadata?.unitCode).toBe('ZMU-ABC');
  });
});

describe('AdminTimelineResponse shape', () => {
  it('contains entityType, entityId, canonicalIdentifier and events', () => {
    const resp: AdminTimelineResponse = {
      entityType: 'order',
      entityId: 'ord-uuid',
      canonicalIdentifier: 'ORD-100193',
      events: [],
    };
    expect(resp.canonicalIdentifier).toBe('ORD-100193');
    expect(Array.isArray(resp.events)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Fetch behavior tests — mock global fetch
// ---------------------------------------------------------------------------

describe('getAdminOrderTimeline', () => {
  it('calls correct URL and returns parsed response on 200', async () => {
    const mockResp: AdminTimelineResponse = {
      entityType: 'order',
      entityId: 'e94d9db8-60b1-4e06-851a-e96d3490174e',
      canonicalIdentifier: 'ORD-100193',
      events: [
        {
          id: 'evt-1',
          type: 'order.created',
          occurredAt: '2026-08-01T10:00:00Z',
          title: 'Заказ создан',
          description: 'Заказ размещён',
          actorType: 'system',
          actorLabel: 'Система',
        },
      ],
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResp),
    } as Response);

    const { getAdminOrderTimeline } = await import('./adminTimeline');
    const result = await getAdminOrderTimeline('e94d9db8-60b1-4e06-851a-e96d3490174e');

    expect(global.fetch).toHaveBeenCalledTimes(1);
    const [calledUrl] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(calledUrl).toContain('/admin/orders/e94d9db8-60b1-4e06-851a-e96d3490174e/timeline');

    expect(result.canonicalIdentifier).toBe('ORD-100193');
    expect(result.events).toHaveLength(1);
    expect(result.events[0].type).toBe('order.created');
  });

  it('throws ApiError on non-ok response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: { message: 'Forbidden', code: 'forbidden' } }),
    } as Response);

    const { getAdminOrderTimeline } = await import('./adminTimeline');

    await expect(getAdminOrderTimeline('some-id')).rejects.toThrow('Forbidden');
  });
});

describe('getAdminReturnTimeline', () => {
  it('calls correct URL for return timeline', async () => {
    const mockResp: AdminTimelineResponse = {
      entityType: 'return',
      entityId: '583fb821-2b10-4966-aacd-e8d24a215842',
      canonicalIdentifier: 'ORD-100193',
      events: [
        {
          id: 'ret-evt-1',
          type: 'return.requested',
          occurredAt: '2026-08-10T09:00:00Z',
          title: 'Возврат запрошен',
          description: 'Покупатель подал заявку на возврат',
          actorType: 'customer',
          actorLabel: 'Никита Осипов',
        },
        {
          id: 'ret-evt-2',
          type: 'return.approved',
          occurredAt: '2026-08-10T10:00:00Z',
          title: 'Возврат одобрен',
          description: 'Оператор одобрил заявку',
          actorType: 'admin_staff',
          actorLabel: 'Оператор ZAMK',
        },
      ],
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResp),
    } as Response);

    const { getAdminReturnTimeline } = await import('./adminTimeline');
    const result = await getAdminReturnTimeline('583fb821-2b10-4966-aacd-e8d24a215842');

    const [calledUrl] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(calledUrl).toContain('/admin/returns/583fb821-2b10-4966-aacd-e8d24a215842/timeline');

    expect(result.entityType).toBe('return');
    expect(result.canonicalIdentifier).toBe('ORD-100193');
    expect(result.events).toHaveLength(2);
    expect(result.events[0].type).toBe('return.requested');
    expect(result.events[1].type).toBe('return.approved');
  });
});
