/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerPromotions } from './SellerPromotions';
import { usageShare, formatStatsMoney } from '../components/PromoStatsModal';
import {
  getSellerPromotions,
  getSellerPromotionsStats,
  getSellerPromotionStats,
  getSellerProducts,
  getSellerProductsPaginated,
  getSellerCategories,
} from '@zamk/api-client/src/seller';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerPromotions: vi.fn(),
  getSellerPromotionsStats: vi.fn(),
  getSellerPromotionStats: vi.fn(),
  createSellerPromotion: vi.fn(),
  updateSellerPromotion: vi.fn(),
  getSellerProducts: vi.fn(),
  getSellerProductsPaginated: vi.fn(),
  getSellerCategories: vi.fn(),
}));

const promo = (over: Record<string, unknown> = {}) => ({
  id: 'promo-1', code: 'SUMMER10', discountType: 'percent', discountValueBps: 1000, discountValueFixedCents: 0,
  minOrderSubtotalCents: 0, audienceType: 'ALL_CUSTOMERS', productScope: 'ENTIRE_STORE', isActive: true,
  startsAt: null, endsAt: null, globalUsageLimit: 10, perCustomerUsageLimit: 1, consumedUsageCount: 3,
  reservedUsageCount: 1, campaignId: 'c1', fundingMode: 'seller', status: 'active',
  createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', ...over,
});
const stats = (over: Record<string, unknown> = {}) => ({
  promoCodeId: 'promo-1', reservedOrders: 1, paidOrders: 2, cancelledAfterPaymentOrders: 1, releasedOrders: 1,
  returnedOrders: 1, salesCents: 180000, sellerDiscountCents: 20000, ...over,
});

const renderPage = () => render(<MemoryRouter><SellerPromotions /></MemoryRouter>);

describe('Seller promo code statistics', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getSellerProducts).mockResolvedValue([]);
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [], totalCount: 0 } as never);
    vi.mocked(getSellerCategories).mockResolvedValue([]);
    vi.mocked(getSellerPromotions).mockResolvedValue({ items: [promo()], count: 1 } as never);
  });
  afterEach(() => cleanup());

  it('helpers: usage share and money formatting', () => {
    expect(usageShare(3, 1, 10)).toBe(40);
    expect(usageShare(12, 0, 10)).toBe(100);
    expect(usageShare(3, 1, null)).toBeNull();
    expect(formatStatsMoney(180000).replace(/\s/g, ' ')).toContain('1');
  });

  it('shows paid orders, sales and seller-funded discount next to each code', async () => {
    vi.mocked(getSellerPromotionsStats).mockResolvedValue({ items: [stats()] } as never);
    renderPage();
    const paid = await screen.findByTestId('promo-paid-SUMMER10');
    await waitFor(() => expect(paid.textContent).toContain('2'));
    expect(paid.textContent).toContain('возвраты: 1');
    const sales = screen.getByTestId('promo-sales-SUMMER10');
    expect(sales.textContent).toContain('скидка за ваш счёт');
    expect(sales.textContent).toMatch(/1\s?800/);
  });

  it('empty state: a code with no orders shows zeros, not dashes', async () => {
    vi.mocked(getSellerPromotionsStats).mockResolvedValue({
      items: [stats({ reservedOrders: 0, paidOrders: 0, cancelledAfterPaymentOrders: 0, releasedOrders: 0, returnedOrders: 0, salesCents: 0, sellerDiscountCents: 0 })],
    } as never);
    renderPage();
    await waitFor(() => expect(screen.getByTestId('promo-paid-SUMMER10').textContent).toBe('0'));
    expect(screen.getByTestId('promo-sales-SUMMER10').textContent).toMatch(/0/);
  });

  it('the list still works when statistics fail to load', async () => {
    vi.mocked(getSellerPromotionsStats).mockRejectedValue(new Error('down'));
    renderPage();
    expect(await screen.findByTestId('promo-row-SUMMER10')).toBeDefined();
    expect(screen.getByTestId('promo-paid-SUMMER10').textContent).toBe('—');
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('detail modal separates reserved, paid, cancelled and released and never calls unpaid orders sales', async () => {
    vi.mocked(getSellerPromotionsStats).mockResolvedValue({ items: [stats()] } as never);
    vi.mocked(getSellerPromotionStats).mockResolvedValue({
      ...stats(),
      recentOrders: [
        { orderId: 'o1aaaaaaaa', orderNumber: 'ZM-1', outcome: 'paid', at: '2026-06-01T10:00:00Z', salesCents: 90000, sellerDiscountCents: 10000, returned: true },
        { orderId: 'o2aaaaaaaa', outcome: 'reserved', at: '2026-06-02T10:00:00Z', salesCents: 0, sellerDiscountCents: 0, returned: false },
        { orderId: 'o3aaaaaaaa', outcome: 'cancelled_after_payment', at: '2026-06-03T10:00:00Z', salesCents: 0, sellerDiscountCents: 0, returned: false },
        { orderId: 'o4aaaaaaaa', outcome: 'released', at: '2026-06-04T10:00:00Z', salesCents: 0, sellerDiscountCents: 0, returned: false },
      ],
    } as never);
    renderPage();
    fireEvent.click(await screen.findByTestId('promo-stats-button-SUMMER10'));
    const modal = await screen.findByTestId('promo-stats-modal');
    await within(modal).findByTestId('stat-paid');
    expect(getSellerPromotionStats).toHaveBeenCalledWith('promo-1');
    expect(within(modal).getByTestId('stat-paid').textContent).toBe('2');
    expect(within(modal).getByTestId('stat-reserved').textContent).toBe('1');
    expect(within(modal).getByTestId('stat-cancelled').textContent).toBe('1');
    expect(within(modal).getByTestId('stat-released').textContent).toBe('1');
    expect(within(modal).getByTestId('stat-returned').textContent).toBe('1');
    expect(within(modal).getByTestId('stat-limit').textContent).toContain('4 из 10');

    const rows = within(modal).getByTestId('promo-stats-orders');
    expect(within(rows).getByTestId('promo-stats-order-reserved').textContent).toContain('—');
    expect(within(rows).getByTestId('promo-stats-order-cancelled_after_payment').textContent).toContain('Отменён после оплаты');
    expect(within(rows).getByTestId('promo-stats-order-paid').textContent).toContain('есть возврат');
    expect(modal.textContent).toContain('Это не дополнительная выручка');
    expect(modal.textContent).not.toMatch(/ROAS/i);
  });

  it('detail modal: empty state and load error', async () => {
    vi.mocked(getSellerPromotionsStats).mockResolvedValue({ items: [] } as never);
    vi.mocked(getSellerPromotionStats).mockResolvedValueOnce({
      ...stats({ paidOrders: 0, salesCents: 0, sellerDiscountCents: 0, reservedOrders: 0, releasedOrders: 0, cancelledAfterPaymentOrders: 0, returnedOrders: 0 }),
      recentOrders: [],
    } as never);
    renderPage();
    fireEvent.click(await screen.findByTestId('promo-stats-button-SUMMER10'));
    expect((await screen.findByTestId('promo-stats-empty')).textContent).toContain('Пока никто');

    cleanup();
    vi.mocked(getSellerPromotionStats).mockRejectedValueOnce(new Error('boom'));
    renderPage();
    fireEvent.click(await screen.findByTestId('promo-stats-button-SUMMER10'));
    expect((await screen.findByRole('alert')).textContent).toContain('Не удалось загрузить статистику');
  });
});
