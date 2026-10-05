// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ReturnQuickView } from '../../components/support/ReturnQuickView';
import * as adminReturnsApi from '../../api/adminReturns';

vi.mock('../../api/adminReturns', () => ({
  getAdminReturn: vi.fn(),
  getReturnStatusLabel: vi.fn((s: string) => (s === 'approved' ? 'Возврат одобрен' : s)),
  getReturnReasonLabel: vi.fn((r?: string) => (r === 'wrong_item' ? 'Получен не тот товар' : r || '—')),
}));

const mockReturn: adminReturnsApi.AdminReturn = {
  id: 'ret-uuid-200',
  orderId: 'order-uuid-100',
  orderNumber: 'ZMK-100481',
  status: 'approved',
  reason: 'wrong_item',
  customerEmail: 'anna@example.com',
  customerName: 'Анна Иванова',
  customerPhone: '+7 999 123-45-67',
  comment: 'Прислали другой цвет',
  items: [
    {
      id: 'item-1',
      returnId: 'ret-uuid-200',
      orderItemId: 'oi-1',
      quantity: 1,
      productTitle: 'Кроссовки Urban Runner',
      priceCents: 450000,
      subtotalPriceCents: 450000,
    },
  ],
};

describe('ReturnQuickView Component Contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state initially while fetching return', async () => {
    (adminReturnsApi.getAdminReturn as any).mockReturnValue(new Promise(() => {}));

    render(
      <MemoryRouter>
        <ReturnQuickView returnId="ret-uuid-200" onClose={vi.fn()} />
      </MemoryRouter>
    );

    expect(screen.getByText('Загрузка данных возврата...')).toBeDefined();
  });

  it('renders error state when return fetch fails', async () => {
    (adminReturnsApi.getAdminReturn as any).mockRejectedValue(new Error('Сбой сети при загрузке возврата'));

    render(
      <MemoryRouter>
        <ReturnQuickView returnId="ret-uuid-200" onClose={vi.fn()} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Сбой сети при загрузке возврата')).toBeDefined();
    });
  });

  it('renders canonical return details, items and linked order button on success', async () => {
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue(mockReturn);

    render(
      <MemoryRouter>
        <ReturnQuickView returnId="ret-uuid-200" onClose={vi.fn()} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Возврат #ZMK-100481')).toBeDefined();
      expect(screen.getByText('Возврат одобрен')).toBeDefined();
      expect(screen.getByText('Получен не тот товар')).toBeDefined();
      expect(screen.getByText('Анна Иванова')).toBeDefined();
      expect(screen.getByText('anna@example.com')).toBeDefined();
      expect(screen.getByText('Кроссовки Urban Runner')).toBeDefined();
      expect(screen.getByText('1 шт.')).toBeDefined();
      expect(screen.getByText('Прислали другой цвет')).toBeDefined();
      expect(screen.getByText('Заказ ZMK-100481')).toBeDefined();
    });
  });

  it('provides explicit full return link pointing to canonical /returns?id=:id in a new tab', async () => {
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue(mockReturn);

    render(
      <MemoryRouter>
        <ReturnQuickView returnId="ret-uuid-200" onClose={vi.fn()} />
      </MemoryRouter>
    );

    await waitFor(() => {
      const fullLink = screen.getByRole('link', { name: /Открыть полный возврат/i });
      expect(fullLink.getAttribute('href')).toBe('/returns?id=ret-uuid-200');
      expect(fullLink.getAttribute('target')).toBe('_blank');
      expect(fullLink.getAttribute('rel')).toBe('noopener noreferrer');
    });
  });

  it('triggers onBack when back button is clicked', async () => {
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue(mockReturn);
    const onBack = vi.fn();

    render(
      <MemoryRouter>
        <ReturnQuickView returnId="ret-uuid-200" onClose={vi.fn()} onBack={onBack} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Возврат #ZMK-100481')).toBeDefined();
    });

    const backBtn = screen.getByRole('button', { name: 'Назад' });
    fireEvent.click(backBtn);
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('triggers onClose when close button is clicked', async () => {
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue(mockReturn);
    const onClose = vi.fn();

    render(
      <MemoryRouter>
        <ReturnQuickView returnId="ret-uuid-200" onClose={onClose} />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Возврат #ZMK-100481')).toBeDefined();
    });

    const closeBtn = screen.getByRole('button', { name: 'Закрыть' });
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('triggers onContextClick with ORDER when clicking linked order button', async () => {
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue(mockReturn);
    const onContextClick = vi.fn();

    render(
      <MemoryRouter>
        <ReturnQuickView
          returnId="ret-uuid-200"
          onClose={vi.fn()}
          onContextClick={onContextClick}
        />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Заказ ZMK-100481')).toBeDefined();
    });

    const orderBtn = screen.getByRole('button', { name: /Заказ ZMK-100481/i });
    fireEvent.click(orderBtn);
    expect(onContextClick).toHaveBeenCalledWith('ORDER', 'order-uuid-100');
  });
});
