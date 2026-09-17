/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Package } from 'lucide-react';
import {
  SellerSurface,
  SellerKpiCard,
  SellerTableShell,
  SellerFilterBar,
} from './SellerSurface';
import { SellerDashboard } from '../pages/SellerDashboard';
import { SellerOrders } from '../pages/SellerOrders';
import { SellerInventory } from '../pages/SellerInventory';
import type { SellerOrder, SellerInventoryItem } from '@zamk/api-client/src/types';

const { mockOrders, mockInventory } = vi.hoisted(() => {
  const mockOrders: SellerOrder[] = [
    {
      id: 'ord-12345-abc',
      orderNumber: '12345',
      createdAt: '2026-09-15T10:00:00Z',
      commercialStatus: 'paid',
      deliveryStatus: 'shipped',
      sellerUnits: 2,
      sellerItemCount: 1,
      sellerGrossAmount: 500000,
      sellerRefundAmount: 0,
      sellerNetAmount: 450000,
      items: [
        {
          id: 'item-1',
          orderId: 'ord-12345-abc',
          productId: 'prod-1',
          productVariantId: 'var-1',
          sellerId: 's1',
          title: 'Шелковое платье',
          variantSize: 'M',
          variantColor: 'Черный',
          sku: 'SKU-001',
          quantity: 2,
          priceCents: 250000,
          subtotalPriceCents: 500000,
          imageUrl: '',
          status: 'paid',
          commissionRate: 0.1,
          commissionCents: 50000,
          sellerPayoutCents: 450000,
        } as any,
      ],
    },
  ];

  const mockInventory: SellerInventoryItem[] = [
    {
      variantId: 'var-1',
      productId: 'prod-1',
      productTitle: 'Шелковое платье',
      sku: 'SKU-001',
      onHand: 10,
      reserved: 2,
      available: 8,
      inbound: 5,
      availabilityStatus: 'В наличии',
      optionValues: { Цвет: 'Черный', Размер: 'M' },
      forecast: { state: 'healthy', daysOfCover: 20, calculatedAt: '2026-09-15T10:00:00Z' },
    },
    {
      variantId: 'var-2',
      productId: 'prod-2',
      productTitle: 'Худи оверсайз',
      sku: 'SKU-002',
      onHand: 0,
      reserved: 0,
      available: 0,
      inbound: 0,
      availabilityStatus: 'Нет в наличии',
      optionValues: { Цвет: 'Белый', Размер: 'L' },
    },
  ];

  return { mockOrders, mockInventory };
});

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerMe: vi.fn().mockResolvedValue({
    seller: {
      id: 's1',
      brandName: 'Тестовый Бренд',
      slug: 'test-brand',
      status: 'active',
      description: 'Описание бренда',
      contactEmail: 'brand@test.com',
      contactPhone: '+79990000000',
      logoUrl: '',
    },
  }),
  getSellerProducts: vi.fn().mockResolvedValue([
    { id: 'p1', title: 'Шелковое платье' },
  ]),
  getSellerOrders: vi.fn().mockResolvedValue({ items: mockOrders }),
  getSellerOrderSummary: vi.fn().mockResolvedValue({
    todayUnits: 2,
    todayOrders: 1,
    last7dGross: 500000,
    last30dGross: 1200000,
    returnsAmount: 0,
    returnsCount: 0,
  }),
  getSellerReturns: vi.fn().mockResolvedValue([]),
  getSellerInventory: vi.fn().mockResolvedValue({ items: mockInventory }),
  getSellerSupplies: vi.fn().mockResolvedValue([]),
  getSellerReviews: vi.fn().mockResolvedValue([]),
  getSellerLedger: vi.fn().mockResolvedValue([]),
  getSellerPayouts: vi.fn().mockResolvedValue([]),
  getSellerBalance: vi.fn().mockResolvedValue({ availableCents: 5000000 }),
  getSellerWarnings: vi.fn().mockResolvedValue([]),
  getSellerViolations: vi.fn().mockResolvedValue([]),
}));

describe('SELLER R1.4A — Surface System Reference', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('1. Shared Surface Primitives Contract', () => {
    it('SellerSurface renders canonical container with rounded-xl border and no heavy shadow', () => {
      render(
        <SellerSurface as="section" data-testid="test-surface" className="p-6">
          <span>Surface Content</span>
        </SellerSurface>
      );

      const surface = screen.getByTestId('test-surface');
      expect(surface.tagName).toBe('SECTION');
      expect(surface.className).toContain('rounded-xl');
      expect(surface.className).toContain('border');
      expect(surface.className).toContain('bg-white');
      expect(surface.className).not.toContain('shadow-xl');
      expect(surface.className).not.toContain('shadow-2xl');
      expect(surface.textContent).toContain('Surface Content');
    });

    it('SellerKpiCard renders label, value, supporting text, low-emphasis icon, and semantic accents', () => {
      const { rerender } = render(
        <MemoryRouter>
          <SellerKpiCard
            label="Активные заказы"
            value="12"
            supportText="В работе"
            icon={Package}
            accent="positive"
            to="/orders"
          />
        </MemoryRouter>
      );

      const kpiCard = screen.getByTestId('seller-kpi-card');
      expect(kpiCard.tagName).toBe('A');
      expect(kpiCard.getAttribute('href')).toBe('/orders');
      expect(kpiCard.className).toContain('rounded-xl');
      expect(kpiCard.className).toContain('border');

      expect(screen.getByTestId('seller-kpi-label').textContent).toBe('Активные заказы');
      expect(screen.getByTestId('seller-kpi-value').textContent).toBe('12');
      expect(screen.getByTestId('seller-kpi-value').className).toContain('text-emerald-600');
      expect(screen.getByTestId('seller-kpi-support').textContent).toBe('В работе');
      expect(screen.getByTestId('seller-kpi-icon')).toBeTruthy();

      rerender(
        <MemoryRouter>
          <SellerKpiCard
            label="Предупреждения"
            value="3"
            accent="danger"
          />
        </MemoryRouter>
      );

      expect(screen.getByTestId('seller-kpi-value').className).toContain('text-red-600');
    });

    it('SellerTableShell renders white surface with rounded-xl and overflow-hidden', () => {
      render(
        <SellerTableShell data-testid="custom-table-shell">
          <table><tbody><tr><td>Row</td></tr></tbody></table>
        </SellerTableShell>
      );

      const tableShell = screen.getByTestId('custom-table-shell');
      expect(tableShell.className).toContain('rounded-xl');
      expect(tableShell.className).toContain('border');
      expect(tableShell.className).toContain('overflow-hidden');
    });

    it('SellerFilterBar renders compact integrated controls bar', () => {
      render(
        <SellerFilterBar data-testid="custom-filter-bar">
          <div>Filters</div>
        </SellerFilterBar>
      );

      const filterBar = screen.getByTestId('custom-filter-bar');
      expect(filterBar.className).toContain('border-b');
      expect(filterBar.className).toContain('p-3');
    });
  });

  describe('2. Dashboard Reference Surface Vocabulary', () => {
    it('Dashboard adopts SellerSurface for attention section and SellerKpiCard for metrics', async () => {
      render(
        <MemoryRouter>
          <SellerDashboard />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Обзор магазина')).toBeTruthy();
      });

      // KPI tiles adopted
      const kpis = screen.getAllByTestId('seller-kpi-card');
      expect(kpis.length).toBeGreaterThanOrEqual(4);

      // Labels and values
      expect(screen.getByText('Товары')).toBeTruthy();
      expect(screen.getByText('Активные заказы')).toBeTruthy();
      expect(screen.getByText('Возвраты')).toBeTruthy();
      expect(screen.getByText('Остатки')).toBeTruthy();

      // Attention section uses SellerSurface
      const surfaces = screen.getAllByTestId('seller-surface');
      expect(surfaces.length).toBeGreaterThanOrEqual(1);
    });
  });

  describe('3. Orders Reference Surface Vocabulary', () => {
    it('Orders adopts SellerKpiCard for summary metrics and SellerTableShell for orders table', async () => {
      render(
        <MemoryRouter>
          <SellerOrders />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Заказы')).toBeTruthy();
      });

      // 4 summary KPI cards
      const kpiLabels = screen.getAllByTestId('seller-kpi-label').map(el => el.textContent);
      expect(kpiLabels).toContain('За сегодня');
      expect(kpiLabels).toContain('За 7 дней');
      expect(kpiLabels).toContain('За 30 дней');
      expect(kpiLabels).toContain('Возвраты (всего)');

      // Table shell exists
      expect(screen.getByTestId('seller-table-shell')).toBeTruthy();

      // Order rows and status
      expect(screen.getByText('#12345')).toBeTruthy();
      expect(screen.getByText('Оплачен')).toBeTruthy();
      expect(screen.getByText('🚚 Передан в доставку')).toBeTruthy();

      // Row expansion interaction
      fireEvent.click(screen.getByText('#12345'));
      expect(screen.getByText('Ваши товары в заказе #12345')).toBeTruthy();
      expect(screen.getByText('Шелковое платье')).toBeTruthy();
      expect(screen.getByText('Размер: M')).toBeTruthy();
    });
  });

  describe('4. Inventory Reference Surface Vocabulary', () => {
    it('Inventory adopts SellerKpiCard, SellerFilterBar, and SellerTableShell with working filters and search', async () => {
      render(
        <MemoryRouter>
          <SellerInventory />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Остатки')).toBeTruthy();
      });

      // 5 KPI cards
      const kpiLabels = screen.getAllByTestId('seller-kpi-label').map(el => el.textContent);
      expect(kpiLabels).toContain('На складе');
      expect(kpiLabels).toContain('Доступно');
      expect(kpiLabels).toContain('В резерве');
      expect(kpiLabels).toContain('В пути');
      expect(kpiLabels).toContain('Без остатка');

      // Table shell and filter bar exist
      expect(screen.getByTestId('seller-table-shell')).toBeTruthy();
      expect(screen.getByTestId('seller-filter-bar')).toBeTruthy();

      // Both items initially present
      expect(screen.getByText('Шелковое платье')).toBeTruthy();
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();

      // Filter by "Нет в наличии"
      const outOfStockButton = screen.getByRole('button', { name: 'Нет в наличии' });
      fireEvent.click(outOfStockButton);

      expect(screen.queryByText('Шелковое платье')).toBeNull();
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();

      // Filter back to "Все"
      fireEvent.click(screen.getByRole('button', { name: 'Все' }));
      expect(screen.getByText('Шелковое платье')).toBeTruthy();

      // Search interaction
      const searchInput = screen.getByPlaceholderText('Поиск по SKU или названию...');
      fireEvent.change(searchInput, { target: { value: 'Худи' } });

      expect(screen.queryByText('Шелковое платье')).toBeNull();
      expect(screen.getByText('Худи оверсайз')).toBeTruthy();

      // Create Supply CTA is present and links to /supplies/new
      const createSupplyBtn = screen.getByRole('button', { name: /Создать поставку/i });
      expect(createSupplyBtn).toBeTruthy();
    });
  });

  describe('5. Canvas Geometry Preservation', () => {
    it('Dashboard, Orders, and Inventory strictly preserve canonical 1296px canvas rhythm', async () => {
      const { unmount: unmountDashboard } = render(
        <MemoryRouter>
          <SellerDashboard />
        </MemoryRouter>
      );
      await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
      const dashboardCanvas = screen.getByTestId('seller-page-frame').className;
      unmountDashboard();

      const { unmount: unmountOrders } = render(
        <MemoryRouter>
          <SellerOrders />
        </MemoryRouter>
      );
      await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
      const ordersCanvas = screen.getByTestId('seller-page-frame').className;
      unmountOrders();

      const { unmount: unmountInventory } = render(
        <MemoryRouter>
          <SellerInventory />
        </MemoryRouter>
      );
      await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
      const inventoryCanvas = screen.getByTestId('seller-page-frame').className;
      unmountInventory();

      expect(dashboardCanvas).toContain('max-w-[1296px]');
      expect(dashboardCanvas).toContain('pt-6 pb-10');
      expect(ordersCanvas).toBe(dashboardCanvas);
      expect(inventoryCanvas).toBe(dashboardCanvas);
    });
  });
});
