/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { NotificationBell, formatDaysRussian, formatVariantDisplayLabel } from './NotificationBell';
import { notificationsApi, type Notification } from '../../api/notifications';

vi.mock('../../api/notifications', () => ({
  notificationsApi: {
    getNotifications: vi.fn(),
    getUnreadCount: vi.fn(),
    markRead: vi.fn(),
    markAllRead: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    user: { id: 'seller-1', name: 'Test Seller', role: 'seller' },
  }),
}));

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('NotificationBell - Helper functions', () => {
  it('pluralizes Russian days correctly', () => {
    expect(formatDaysRussian(1)).toBe('1 день');
    expect(formatDaysRussian(21)).toBe('21 день');
    expect(formatDaysRussian(101)).toBe('101 день');

    expect(formatDaysRussian(2)).toBe('2 дня');
    expect(formatDaysRussian(3)).toBe('3 дня');
    expect(formatDaysRussian(4)).toBe('4 дня');
    expect(formatDaysRussian(22)).toBe('22 дня');
    expect(formatDaysRussian(24)).toBe('24 дня');

    expect(formatDaysRussian(0)).toBe('0 дней');
    expect(formatDaysRussian(5)).toBe('5 дней');
    expect(formatDaysRussian(9)).toBe('9 дней');
    expect(formatDaysRussian(11)).toBe('11 дней');
    expect(formatDaysRussian(12)).toBe('12 дней');
    expect(formatDaysRussian(14)).toBe('14 дней');
    expect(formatDaysRussian(19)).toBe('19 дней');
    expect(formatDaysRussian(20)).toBe('20 дней');
  });

  it('formats variant display label according to presence of color and size', () => {
    expect(formatVariantDisplayLabel('Красный', 'L')).toBe('Красный · L');
    expect(formatVariantDisplayLabel('Красный', '')).toBe('Красный');
    expect(formatVariantDisplayLabel('Красный', null)).toBe('Красный');
    expect(formatVariantDisplayLabel('', 'L')).toBe('L');
    expect(formatVariantDisplayLabel(null, 'XL')).toBe('XL');
    expect(formatVariantDisplayLabel('', '')).toBe('');
    expect(formatVariantDisplayLabel(undefined, undefined)).toBe('');
  });
});

describe('NotificationBell - NTF.3C1 Forecast Risk Alerts Component Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const openBell = async (notificationsList: Notification[], unreadCount = 1) => {
    vi.mocked(notificationsApi.getUnreadCount).mockResolvedValue(unreadCount);
    vi.mocked(notificationsApi.getNotifications).mockResolvedValue({
      items: notificationsList,
      totalCount: notificationsList.length,
    });

    render(
      <MemoryRouter>
        <NotificationBell />
      </MemoryRouter>
    );

    const bellBtn = screen.getByRole('button', { name: /уведомления/i });
    fireEvent.click(bellBtn);

    await waitFor(() => {
      expect(notificationsApi.getNotifications).toHaveBeenCalled();
    });
  };

  // A. warning forecast alert renders warning treatment
  it('A. warning forecast alert renders warning treatment', async () => {
    const notif: Notification = {
      id: 'notif-w-1',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'warning',
      status: 'active',
      title: 'Запас товара скоро закончится',
      body: 'Красный / L — примерно на 10 дней.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        worstDaysOfCover: 10,
        variantCount: 1,
        variants: [
          {
            color: 'Красный',
            size: 'L',
            daysOfCover: 10,
            severity: 'warning',
          },
        ],
      },
    };

    await openBell([notif]);
    expect(screen.getByText('Запас товара скоро закончится')).toBeTruthy();
    expect(screen.getByText('Активно')).toBeTruthy();
    expect(screen.getByText('Красный · L')).toBeTruthy();
    expect(screen.getByText('≈ 10 дней запаса')).toBeTruthy();
  });

  // B. critical forecast alert renders critical treatment
  it('B. critical forecast alert renders critical treatment', async () => {
    const notif: Notification = {
      id: 'notif-c-1',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'critical',
      status: 'active',
      title: 'Товар может закончиться в ближайшие дни',
      body: 'Красный / L — примерно на 4 дня.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        worstDaysOfCover: 4.2,
        variantCount: 1,
        variants: [
          {
            color: 'Красный',
            size: 'L',
            daysOfCover: 4.2,
            severity: 'critical',
          },
        ],
      },
    };

    await openBell([notif]);
    expect(screen.getByText('Товар может закончиться в ближайшие дни')).toBeTruthy();
    expect(screen.getByText('Активно')).toBeTruthy();
  });

  // C. Red/L at 4.2 days renders: Красный · L ≈ 4 дня запаса
  it('C. Red/L at 4.2 days renders Красный · L and ≈ 4 дня запаса', async () => {
    const notif: Notification = {
      id: 'notif-c-2',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'critical',
      status: 'active',
      title: 'Товар может закончиться в ближайшие дни',
      body: 'Красный / L — примерно на 4 дня.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        worstDaysOfCover: 4.2,
        variantCount: 1,
        variants: [
          {
            color: 'Красный',
            size: 'L',
            daysOfCover: 4.2,
            severity: 'critical',
          },
        ],
      },
    };

    await openBell([notif]);
    expect(screen.getByText('Красный · L')).toBeTruthy();
    expect(screen.getByText('≈ 4 дня запаса')).toBeTruthy();
    // Verify no UUIDs, no DSV, no raw decimals exposed in diagnostic card
    expect(screen.queryByText(/4\.2/)).toBeNull();
    expect(screen.queryByText(/dsv/i)).toBeNull();
  });

  // D. 9.1 days uses correct Russian pluralization (≈ 9 дней запаса)
  it('D. 9.1 days uses correct Russian pluralization: ≈ 9 дней запаса', async () => {
    const notif: Notification = {
      id: 'notif-w-2',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'warning',
      status: 'active',
      title: 'Запас товара скоро закончится',
      body: 'Белый / M — примерно на 9 дней.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        worstDaysOfCover: 9.1,
        variantCount: 1,
        variants: [
          {
            color: 'Белый',
            size: 'M',
            daysOfCover: 9.1,
            severity: 'warning',
          },
        ],
      },
    };

    await openBell([notif]);
    expect(screen.getByText('Белый · M')).toBeTruthy();
    expect(screen.getByText('≈ 9 дней запаса')).toBeTruthy();
  });

  // E. color-only and size-only variants render correctly
  it('E. color-only and size-only variants render correctly without empty dots', async () => {
    const notif: Notification = {
      id: 'notif-multi',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'warning',
      status: 'active',
      title: 'Запас товара скоро закончится',
      body: 'Синий — примерно на 3 дня; XL — примерно на 5 дней.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Платье',
        worstDaysOfCover: 3,
        variantCount: 2,
        variants: [
          {
            color: 'Синий',
            size: '',
            daysOfCover: 3,
            severity: 'critical',
          },
          {
            color: '',
            size: 'XL',
            daysOfCover: 5.2,
            severity: 'critical',
          },
        ],
      },
    };

    await openBell([notif]);
    expect(screen.getByText('Синий')).toBeTruthy();
    expect(screen.getByText('≈ 3 дня запаса')).toBeTruthy();
    expect(screen.getByText('XL')).toBeTruthy();
    expect(screen.getByText('≈ 5 дней запаса')).toBeTruthy();
  });

  // F. forecast alert with malformed/missing metadata falls back safely
  it('F. forecast alert with malformed/missing metadata falls back to generic body safely without crashing', async () => {
    const malformedNotif: Notification = {
      id: 'notif-malformed',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'warning',
      status: 'active',
      title: 'Запас товара скоро закончится',
      body: 'Fallback body text explaining risk.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: null, // null metadata
    };

    await openBell([malformedNotif]);
    expect(screen.getByText('Запас товара скоро закончится')).toBeTruthy();
    expect(screen.getByText('Fallback body text explaining risk.')).toBeTruthy();
  });

  it('F2. forecast alert with corrupt variants array falls back gracefully', async () => {
    const corruptNotif: Notification = {
      id: 'notif-corrupt',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'warning',
      status: 'active',
      title: 'Запас товара скоро закончится',
      body: 'Corrupt fallback body text.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: { variants: "invalid_not_array" } as any,
    };

    await openBell([corruptNotif]);
    expect(screen.getByText('Запас товара скоро закончится')).toBeTruthy();
    expect(screen.getByText('Corrupt fallback body text.')).toBeTruthy();
  });

  // G. active forecast remains Active after mark-read
  it('G. active forecast remains Active after mark-read', async () => {
    const notif: Notification = {
      id: 'notif-read-test',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'critical',
      status: 'active',
      title: 'Товар может закончиться в ближайшие дни',
      body: 'Красный / L — примерно на 4 дня.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        variants: [{ color: 'Красный', size: 'L', daysOfCover: 4 }],
      },
    };

    vi.mocked(notificationsApi.markRead).mockResolvedValue(undefined as any);

    await openBell([notif]);
    expect(screen.getByText('Активно')).toBeTruthy();

    const markReadBtn = screen.getByTitle('Отметить как прочитанное');
    fireEvent.click(markReadBtn);

    await waitFor(() => {
      expect(notificationsApi.markRead).toHaveBeenCalledWith('notif-read-test');
    });

    // Still shows "Активно" because READ != RESOLVED
    expect(screen.getByText('Активно')).toBeTruthy();
  });

  // H. resolved forecast renders Решено and "Было: ≈ 4 дня запаса" with last-active metadata
  it('H. resolved forecast renders Решено badge and "Было: ≈ 4 дня запаса"', async () => {
    const notif: Notification = {
      id: 'notif-res-1',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'critical',
      status: 'resolved',
      title: 'Товар может закончиться в ближайшие дни',
      body: 'Красный / L — примерно на 4 дня.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: '2026-09-11T11:00:00Z',
      resolvedAt: '2026-09-11T11:05:00Z',
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        worstDaysOfCover: 4.2,
        variantCount: 1,
        variants: [
          { color: 'Красный', size: 'L', daysOfCover: 4.2 },
        ],
      },
    };

    await openBell([notif], 0);
    expect(screen.getByText('Решено')).toBeTruthy();
    expect(screen.getByText('Красный · L')).toBeTruthy();
    expect(screen.getByText('Было: ≈ 4 дня запаса')).toBeTruthy();
    expect(screen.queryByText(/^≈ 4 дня запаса$/)).toBeNull();
  });

  it('H2. active forecast with the same 4.2 days metadata renders without "Было:"', async () => {
    const notif: Notification = {
      id: 'notif-active-1',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'critical',
      status: 'active',
      title: 'Товар может закончиться в ближайшие дни',
      body: 'Красный / L — примерно на 4 дня.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        productId: 'prod-1',
        productTitle: 'Худи Оверсайз',
        worstDaysOfCover: 4.2,
        variantCount: 1,
        variants: [
          { color: 'Красный', size: 'L', daysOfCover: 4.2 },
        ],
      },
    };

    await openBell([notif], 1);
    expect(screen.getByText('Активно')).toBeTruthy();
    expect(screen.getByText('Красный · L')).toBeTruthy();
    expect(screen.getByText('≈ 4 дня запаса')).toBeTruthy();
    expect(screen.queryByText(/Было:/)).toBeNull();
  });

  // I. actionUrl navigates to /supplies/new
  it('I. actionUrl navigates to /supplies/new on click', async () => {
    const notif: Notification = {
      id: 'notif-act-1',
      type: 'stock_forecast_risk',
      kind: 'alert',
      severity: 'critical',
      status: 'active',
      title: 'Товар может закончиться в ближайшие дни',
      body: 'Красный / L — примерно на 4 дня.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
      metadata: {
        variants: [{ color: 'Красный', size: 'L', daysOfCover: 4 }],
      },
    };

    await openBell([notif]);
    const card = screen.getByText('Товар может закончиться в ближайшие дни').closest('.cursor-pointer');
    expect(card).toBeTruthy();

    fireEvent.click(card!);
    expect(mockNavigate).toHaveBeenCalledWith('/supplies/new');
  });

  // J. ordinary non-forecast notification rendering remains unchanged
  it('J. ordinary non-forecast notification rendering remains unchanged', async () => {
    const regularEvent: Notification = {
      id: 'notif-event-1',
      type: 'fulfillment_paid',
      kind: 'event',
      severity: 'info',
      title: 'Заказ оплачен',
      body: 'Покупатель оплатил заказ ORD-100205.',
      entityType: 'order',
      entityId: 'ord-100205',
      actionUrl: '/orders/ord-100205',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
    };

    const ntf2Alert: Notification = {
      id: 'notif-ntf2-1',
      type: 'stock_critical_hidden',
      kind: 'alert',
      severity: 'critical',
      status: 'active',
      title: 'Карточка скрыта из магазина',
      body: 'Товар "Худи" скрыт из витрины, так как свободный остаток меньше 2 шт.',
      entityType: 'product',
      entityId: 'prod-1',
      actionUrl: '/supplies/new',
      readAt: null,
      createdAt: '2026-09-11T10:00:00Z',
    };

    await openBell([regularEvent, ntf2Alert]);

    expect(screen.getByText('Заказ оплачен')).toBeTruthy();
    expect(screen.getByText('Покупатель оплатил заказ ORD-100205.')).toBeTruthy();

    expect(screen.getByText('Карточка скрыта из магазина')).toBeTruthy();
    expect(screen.getByText('Товар "Худи" скрыт из витрины, так как свободный остаток меньше 2 шт.')).toBeTruthy();
    expect(screen.getByText('Активно')).toBeTruthy();
  });
});
