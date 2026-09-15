import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { AdminPackingQueue } from './AdminPackingQueue';
import { MemoryRouter } from 'react-router-dom';
import * as adminPickingApi from '../api/adminPicking';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('../api/adminPicking', () => ({
  getAdminPackingQueue: vi.fn(),
}));

const sampleQueue: adminPickingApi.PackingQueueItem[] = [
  {
    fulfillmentId: 'fulf-1111-2222',
    orderId: 'ord-111',
    orderNumber: '1001',
    status: 'assembling',
    orderStatus: 'assembling',
    itemsCount: 2,
    totalQuantity: 3,
    pickedQuantity: 3,
    createdAt: '2026-09-15T09:00:00Z',
    pickingCompletedAt: '2026-09-15T09:30:00Z',
  },
  {
    fulfillmentId: 'fulf-3333-4444',
    orderId: 'ord-222',
    orderNumber: '1002',
    status: 'assembling',
    orderStatus: 'assembling',
    itemsCount: 1,
    totalQuantity: 1,
    pickedQuantity: 1,
    createdAt: '2026-09-15T10:00:00Z',
  },
];

describe('AdminPackingQueue Page Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders packing queue items with operational fields and KPIs', async () => {
    vi.mocked(adminPickingApi.getAdminPackingQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminPackingQueue />
      </MemoryRouter>
    );

    expect(await screen.findByTestId('packing-queue-page')).toBeDefined();
    expect(screen.getByText('Очередь упаковки')).toBeDefined();

    // KPIs
    expect(screen.getByTestId('packing-queue-count').textContent).toBe('2');
    expect(screen.getByText('4 шт.')).toBeDefined();

    // Orders
    expect(screen.getByText(/1001/)).toBeDefined();
    expect(screen.getByText(/1002/)).toBeDefined();
    expect(screen.getAllByText('Готов к упаковке')).toHaveLength(2);
    expect(screen.getByText('2 поз. · 3 шт.')).toBeDefined();
    expect(screen.getByText('1 поз. · 1 шт.')).toBeDefined();

    // No financial fields or customer PII in DOM
    expect(screen.queryByText(/₽|руб|phone|address|email/i)).toBeNull();
  });

  it('navigates to /fulfillment/packing/:id when clicking action button', async () => {
    vi.mocked(adminPickingApi.getAdminPackingQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminPackingQueue />
      </MemoryRouter>
    );

    const packButtons = await screen.findAllByRole('button', { name: /Упаковать/i });
    expect(packButtons).toHaveLength(2);

    fireEvent.click(packButtons[0]);
    expect(mockNavigate).toHaveBeenCalledWith('/fulfillment/packing/fulf-1111-2222');
  });

  it('navigates to /fulfillment/packing/:id when clicking the queue item row', async () => {
    vi.mocked(adminPickingApi.getAdminPackingQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminPackingQueue />
      </MemoryRouter>
    );

    await screen.findByTestId('packing-queue-page');
    const orderTitle = screen.getByText(/1002/);
    fireEvent.click(orderTitle);

    expect(mockNavigate).toHaveBeenCalledWith('/fulfillment/packing/fulf-3333-4444');
  });

  it('renders empty state when there are no packable fulfillments', async () => {
    vi.mocked(adminPickingApi.getAdminPackingQueue).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <AdminPackingQueue />
      </MemoryRouter>
    );

    expect(await screen.findByText('Нет заказов, ожидающих упаковки')).toBeDefined();
    expect(screen.getByTestId('packing-queue-count').textContent).toBe('0');
  });

  it('renders error state and supports retry', async () => {
    vi.mocked(adminPickingApi.getAdminPackingQueue)
      .mockRejectedValueOnce(new Error('Сетевой сбой'))
      .mockResolvedValueOnce(sampleQueue);

    render(
      <MemoryRouter>
        <AdminPackingQueue />
      </MemoryRouter>
    );

    expect(await screen.findByText('Сетевой сбой')).toBeDefined();

    const retryBtn = screen.getByRole('button', { name: /Повторить/i });
    fireEvent.click(retryBtn);

    expect(await screen.findByText(/1001/)).toBeDefined();
    expect(adminPickingApi.getAdminPackingQueue).toHaveBeenCalledTimes(2);
  });
});
