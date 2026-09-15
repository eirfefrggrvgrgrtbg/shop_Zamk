import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AdminReturnReceivingQueue } from './AdminReturnReceivingQueue';
import { MemoryRouter } from 'react-router-dom';
import * as adminReturnsApi from '../api/adminReturns';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('../api/adminReturns', () => ({
  getAdminReturnReceivingQueue: vi.fn(),
  getAdminReturnErrorMessage: vi.fn((_err, fallback) => fallback),
}));

const sampleQueue: adminReturnsApi.AdminReturnReceivingQueueItem[] = [
  {
    returnId: '11111111-2222-3333-4444-555555555555',
    orderId: 'aaaa1111-2222-3333-4444-555555555555',
    orderNumber: 'ORD-100204',
    returnStatus: 'approved',
    shipmentStatus: 'arrived_at_zamk',
    trackingNumber: 'TRK-987654',
    expectedUnitsCount: 2,
    receivedUnitsCount: 0,
    remainingUnitsCount: 2,
    arrivedAt: '2026-09-15T09:14:00Z',
    sellerName: 'Acme Fashion',
    productSummary: 'Платье шелковое',
    createdAt: '2026-09-14T10:00:00Z',
  },
  {
    returnId: '22222222-3333-4444-5555-666666666666',
    orderId: 'bbbb2222-3333-4444-5555-666666666666',
    orderNumber: 'ORD-100203',
    returnStatus: 'receiving',
    shipmentStatus: 'arrived_at_zamk',
    trackingNumber: 'TRK-123456',
    expectedUnitsCount: 2,
    receivedUnitsCount: 1,
    remainingUnitsCount: 1,
    arrivedAt: '2026-09-15T08:30:00Z',
    receivingStartedAt: '2026-09-15T09:00:00Z',
    sellerName: 'ZAMK Studio',
    productSummary: 'Куртка кожаная',
    createdAt: '2026-09-14T09:00:00Z',
  },
];

describe('AdminReturnReceivingQueue Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders queue items with operational metadata and status badges', async () => {
    vi.mocked(adminReturnsApi.getAdminReturnReceivingQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminReturnReceivingQueue />
      </MemoryRouter>
    );

    // Page title and subtitle
    expect(await screen.findByText('Приёмка возвратов')).toBeDefined();
    expect(
      screen.getByText('Возвраты, которые прибыли на склад и требуют физической приёмки')
    ).toBeDefined();

    // Summary chips
    expect(screen.getByText(/Ожидают начала 1/)).toBeDefined();
    expect(screen.getByText(/В процессе 1/)).toBeDefined();
    expect(screen.getByText(/Осталось принять 3 шт\./)).toBeDefined();

    // Items
    expect(screen.getByText('RET-11111111')).toBeDefined();
    expect(screen.getByText('RET-22222222')).toBeDefined();

    // Status badges
    expect(screen.getByText('Ожидает приёмки')).toBeDefined();
    expect(screen.getByText('Приёмка начата')).toBeDefined();

    // Orders
    expect(screen.getByText('ORD-100204')).toBeDefined();
    expect(screen.getByText('ORD-100203')).toBeDefined();

    // Units progress
    expect(screen.getByText(/2 единицы/)).toBeDefined();
    expect(screen.getByText(/Принято 1 из 2 · осталось 1/)).toBeDefined();

    // CTAs
    expect(screen.getByText('Начать приёмку')).toBeDefined();
    expect(screen.getByText('Продолжить приёмку')).toBeDefined();

    // No financial fields or customer PII in DOM
    expect(screen.queryByText(/₽|руб|phone|address|email/i)).toBeNull();
  });

  it('navigates to canonical /returns/:id/receiving on click', async () => {
    vi.mocked(adminReturnsApi.getAdminReturnReceivingQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminReturnReceivingQueue />
      </MemoryRouter>
    );

    await screen.findByText('RET-11111111');

    // Click "Начать приёмку"
    fireEvent.click(screen.getByText('Начать приёмку'));
    expect(mockNavigate).toHaveBeenCalledWith('/returns/11111111-2222-3333-4444-555555555555/receiving');

    // Click "Продолжить приёмку"
    fireEvent.click(screen.getByText('Продолжить приёмку'));
    expect(mockNavigate).toHaveBeenCalledWith('/returns/22222222-3333-4444-5555-666666666666/receiving');
  });

  it('filters queue items by search query', async () => {
    vi.mocked(adminReturnsApi.getAdminReturnReceivingQueue).mockResolvedValue(sampleQueue);

    render(
      <MemoryRouter>
        <AdminReturnReceivingQueue />
      </MemoryRouter>
    );

    await screen.findByText('RET-11111111');

    const searchInput = screen.getByPlaceholderText(/Номер возврата, заказа или трек-номер/i);
    fireEvent.change(searchInput, { target: { value: 'ORD-100203' } });

    expect(screen.queryByText('RET-11111111')).toBeNull();
    expect(screen.getByText('RET-22222222')).toBeDefined();
  });

  it('renders empty state when queue is empty', async () => {
    vi.mocked(adminReturnsApi.getAdminReturnReceivingQueue).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <AdminReturnReceivingQueue />
      </MemoryRouter>
    );

    expect(await screen.findByText('Нет возвратов, ожидающих приёмки')).toBeDefined();
  });

  it('renders error state and retries on failure', async () => {
    vi.mocked(adminReturnsApi.getAdminReturnReceivingQueue).mockRejectedValueOnce(
      new Error('Network error')
    );

    render(
      <MemoryRouter>
        <AdminReturnReceivingQueue />
      </MemoryRouter>
    );

    expect(await screen.findByText('Не удалось загрузить очередь приёмки возвратов')).toBeDefined();

    vi.mocked(adminReturnsApi.getAdminReturnReceivingQueue).mockResolvedValueOnce(sampleQueue);
    fireEvent.click(screen.getByText('Повторить'));

    await waitFor(() => {
      expect(screen.getByText('RET-11111111')).toBeDefined();
    });
  });
});
