import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { AdminDispatchQueue } from './AdminDispatchQueue';
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
  getAdminDispatchQueue: vi.fn(),
}));

const sampleQueue: adminPickingApi.DispatchQueueItem[] = [
  {
    fulfillmentId: 'fulf-1111-2222',
    orderId: 'ord-111',
    orderNumber: '1001',
    status: 'packed',
    orderStatus: 'packed',
    itemsCount: 2,
    totalQuantity: 3,
    deliveryMethodName: 'СДЭК Курьер',
    carrier: 'СДЭК',
    createdAt: '2026-09-15T09:00:00Z',
    packedAt: '2026-09-15T09:30:00Z',
  },
  {
    fulfillmentId: 'fulf-3333-4444',
    orderId: 'ord-222',
    orderNumber: '1002',
    status: 'packed',
    orderStatus: 'assembling',
    itemsCount: 1,
    totalQuantity: 1,
    createdAt: '2026-09-15T10:00:00Z',
    packedAt: '2026-09-15T10:15:00Z',
  },
];

describe('AdminDispatchQueue Page Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders dispatch queue items with operational fields and KPIs', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminDispatchQueue />
      </MemoryRouter>
    );

    expect(await screen.findByTestId('dispatch-queue-page')).toBeDefined();
    expect(screen.getByText('Отгрузка')).toBeDefined();
    expect(screen.getByText('Заказы, упакованные и готовые к передаче в доставку')).toBeDefined();

    // KPIs
    expect(screen.getByTestId('dispatch-queue-count').textContent).toBe('2');
    expect(screen.getByText('4 шт.')).toBeDefined();

    // Orders
    expect(screen.getByText(/1001/)).toBeDefined();
    expect(screen.getByText(/1002/)).toBeDefined();
    expect(screen.getAllByText('Собран')).toHaveLength(2);
    expect(screen.getByText('2 поз. · 3 ед.')).toBeDefined();
    expect(screen.getByText('1 поз. · 1 ед.')).toBeDefined();
    expect(screen.getByText(/СДЭК Курьер/)).toBeDefined();

    // No financial fields or customer PII in DOM
    expect(screen.queryByText(/₽|руб|totalCents|priceCents|phone|address|email/i)).toBeNull();
  });

  it('renders tabs for packed queue and shipped archive', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminDispatchQueue />
      </MemoryRouter>
    );

    expect(await screen.findByRole('button', { name: /К отгрузке/i })).toBeDefined();
    expect(screen.getByRole('button', { name: /Отгружено/i })).toBeDefined();
  });

  it('navigates to /fulfillment/dispatch/:id when clicking action button', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminDispatchQueue />
      </MemoryRouter>
    );

    const openButtons = await screen.findAllByRole('button', { name: /Открыть/i });
    expect(openButtons).toHaveLength(2);

    fireEvent.click(openButtons[0]);
    expect(mockNavigate).toHaveBeenCalledWith('/fulfillment/dispatch/fulf-1111-2222');
  });

  it('navigates to /fulfillment/dispatch/:id when clicking the queue item row', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminDispatchQueue />
      </MemoryRouter>
    );

    const orderRow = await screen.findByText(/1002/);
    fireEvent.click(orderRow);
    expect(mockNavigate).toHaveBeenCalledWith('/fulfillment/dispatch/fulf-3333-4444');
  });

  it('renders empty state when queue is empty', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <AdminDispatchQueue />
      </MemoryRouter>
    );

    expect(await screen.findByText('Нет заказов, ожидающих отгрузки')).toBeDefined();
    expect(screen.getByTestId('dispatch-queue-count').textContent).toBe('0');
  });

  it('handles error state and provides retry action', async () => {
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockRejectedValueOnce(
      new Error('Сетевая ошибка при загрузке очереди')
    );

    render(
      <MemoryRouter>
        <AdminDispatchQueue />
      </MemoryRouter>
    );

    expect(await screen.findByText('Сетевая ошибка при загрузке очереди')).toBeDefined();

    // Retry
    vi.mocked(adminPickingApi.getAdminDispatchQueue).mockResolvedValueOnce(sampleQueue);
    const retryBtn = screen.getByRole('button', { name: /Повторить/i });
    fireEvent.click(retryBtn);

    expect(await screen.findByText(/1001/)).toBeDefined();
  });
});
