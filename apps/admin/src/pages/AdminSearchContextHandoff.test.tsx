import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminReturns } from './AdminReturns';
import { AdminInventory } from './AdminInventory';
import { AdminUsers } from './AdminUsers';
import * as adminReturnsApi from '../api/adminReturns';
import * as adminInventoryApi from '../api/adminInventory';
import * as adminUsersApi from '@zamk/api-client/src/admin';
import type { AdminReturn } from '../api/adminReturns';

vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: () => ({
    user: { id: 'admin-1', role: 'admin', email: 'admin@zamk.local' },
    staff: { id: 'staff-1', permissions: ['returns.read', 'orders.read', 'inventory.read'] },
    permissions: ['returns.read', 'orders.read', 'inventory.read'],
    isAuthenticated: true,
    isLoading: false,
    error: null,
    hasPermission: () => true,
    hasAnyPermission: () => true,
    isOwner: () => true,
    isCoOwner: () => false,
  }),
}));

// Stub timeline endpoints so EntityTimeline inside AdminReturns/AdminOrderDetail
// does not make real network calls during these integration tests.
vi.mock('../api/adminTimeline', () => ({
  getAdminOrderTimeline: () => Promise.resolve({ entityType: 'order', entityId: '', canonicalIdentifier: '', events: [] }),
  getAdminReturnTimeline: () => Promise.resolve({ entityType: 'return', entityId: '', canonicalIdentifier: '', events: [] }),
}));


describe('Admin Global Search Context Handoff (M6.1C Tests)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  // 1. Returns Context Tests
  describe('AdminReturns context handoff', () => {
    const mockReturnList: AdminReturn[] = [
      {
        id: 'ret-100193-id',
        orderId: 'ord-100193-id',
        orderNumber: 'ORD-100193',
        status: 'approved',
        reason: 'damaged',
        customerName: 'Nikita Osipov',
        customerEmail: 'customer@zamk.local',
        createdAt: '2026-08-30T12:00:00Z',
        items: [],
      },
    ];

    const mockReturnDetail: AdminReturn = {
      id: 'ret-100193-id',
      orderId: 'ord-100193-id',
      orderNumber: 'ORD-100193',
      status: 'approved',
      reason: 'damaged',
      comment: 'Torn seam on right sleeve',
      customerName: 'Nikita Osipov',
      customerEmail: 'customer@zamk.local',
      createdAt: '2026-08-30T12:00:00Z',
      items: [
        {
          id: 'item-1',
          orderItemId: 'oi-1',
          productTitle: 'Dev Wool Coat',
          sku: 'SKU-COAT-M',
          quantity: 1,
          priceCents: 1500000,
          subtotalPriceCents: 1500000,
        },
      ],
    };

    it('opens exact Return dossier when navigated with ?id=...&orderNumber=...', async () => {
      const getReturnsSpy = vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue(mockReturnList);
      const getReturnSpy = vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockReturnDetail);

      render(
        <MemoryRouter initialEntries={['/returns?id=ret-100193-id&orderNumber=ORD-100193']}>
          <AdminReturns />
        </MemoryRouter>
      );

      // Verify list and detail were fetched
      await waitFor(() => {
        expect(getReturnsSpy).toHaveBeenCalled();
        expect(getReturnSpy).toHaveBeenCalledWith('ret-100193-id');
      });

      // Verify dossier view is rendered directly without needing manual clicks
      await waitFor(() => {
        expect(screen.getByText('Заявка на возврат')).toBeDefined();
        expect(screen.getAllByText('ORD-100193').length).toBeGreaterThan(0);
        expect(screen.getByText('Товары к возврату')).toBeDefined();
        expect(screen.getByText('Dev Wool Coat')).toBeDefined();
        expect(screen.getByText('Претензия покупателя')).toBeDefined();
      });

      // Back button returns to list view and does not get stuck in a loop
      const backButton = screen.getByText('Назад к списку');
      fireEvent.click(backButton);

      await waitFor(() => {
        expect(screen.getByText('Возвраты')).toBeDefined();
        expect(screen.queryByText('Претензия покупателя')).toBeNull();
      });
    });

    it('falls back gracefully to normal Returns list when ?id is invalid or 404', async () => {
      vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue(mockReturnList);
      vi.spyOn(adminReturnsApi, 'getAdminReturn').mockRejectedValue({
        status: 404,
        code: 'not_found',
        message: 'Return not found',
      });

      render(
        <MemoryRouter initialEntries={['/returns?id=invalid-id']}>
          <AdminReturns />
        </MemoryRouter>
      );

      // Verifies it falls back gracefully to the list view and shows error
      await waitFor(() => {
        expect(screen.getByText('Возвраты')).toBeDefined();
        expect(screen.queryByText('Претензия покупателя')).toBeNull();
      });
    });

    it('renders normal Returns list without dossier on direct visit without query params', async () => {
      vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue(mockReturnList);
      const getReturnSpy = vi.spyOn(adminReturnsApi, 'getAdminReturn');

      render(
        <MemoryRouter initialEntries={['/returns']}>
          <AdminReturns />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Возвраты')).toBeDefined();
        expect(getReturnSpy).not.toHaveBeenCalled();
      });
    });
  });

  // 2. Inventory Context Tests
  describe('AdminInventory context handoff', () => {
    it('initializes search input, renders ZMU context card and owning variant with ?q=ZMU-...', async () => {
      const getInventorySpy = vi.spyOn(adminInventoryApi, 'getAdminInventory').mockResolvedValue({
        items: [
          {
            id: 'inv-1',
            productId: 'prod-1',
            productTitle: 'Dev Wool Coat',
            productVariantId: 'pv-1',
            variant: 'M / Black',
            sku: 'SKU-COAT-M',
            source: 'auction_direct_sale',
            totalStock: 25,
            reservedStock: 0,
            availableStock: 25,
            updatedAt: '2026-08-30T12:00:00Z',
          },
        ],
        totalCount: 1,
        unitContext: {
          unitCode: 'ZMU-C5MXPTQ7WH8WZYQP',
          status: 'shipped',
          statusLabel: 'Отгружен',
          productTitle: 'Dev Wool Coat',
          variant: 'SKU-COAT-M / M / Black',
          sku: 'SKU-COAT-M',
          productId: 'prod-1',
          variantId: 'pv-1',
        },
      });

      render(
        <MemoryRouter initialEntries={['/inventory?q=ZMU-C5MXPTQ7WH8WZYQP']}>
          <AdminInventory />
        </MemoryRouter>
      );

      // Verifies search input is populated with ZMU code
      const input = screen.getByPlaceholderText('Поиск по названию, SKU или ZMU...') as HTMLInputElement;
      expect(input.value).toBe('ZMU-C5MXPTQ7WH8WZYQP');

      // Verifies API was called with the ZMU query param
      await waitFor(() => {
        expect(getInventorySpy).toHaveBeenCalledWith(
          expect.objectContaining({
            q: 'ZMU-C5MXPTQ7WH8WZYQP',
          })
        );
      });

      // Verifies physical ZMU context banner is rendered
      await waitFor(() => {
        expect(screen.getByTestId('admin-inventory-zmu-context')).toBeDefined();
        expect(screen.getAllByText('ZMU-C5MXPTQ7WH8WZYQP').length).toBeGreaterThan(0);
        expect(screen.getByText('Отгружен')).toBeDefined();
      });

      // Verifies owning variant aggregate row is rendered in the table
      await waitFor(() => {
        expect(screen.getAllByText('Dev Wool Coat').length).toBeGreaterThan(0);
        expect(screen.queryByText('Нет данных')).toBeNull();
      });
    });

    it('renders ZMU context when unit exists even without aggregate inventory row', async () => {
      vi.spyOn(adminInventoryApi, 'getAdminInventory').mockResolvedValue({
        items: [],
        totalCount: 0,
        unitContext: {
          unitCode: 'ZMU-C5MXPTQ7WH8WZYQP',
          status: 'damaged',
          statusLabel: 'Поврежден',
          productTitle: 'Dev Wool Coat',
          variant: 'SKU-COAT-M',
          sku: 'SKU-COAT-M',
          productId: 'prod-1',
          variantId: 'pv-1',
        },
      });

      render(
        <MemoryRouter initialEntries={['/inventory?q=ZMU-C5MXPTQ7WH8WZYQP']}>
          <AdminInventory />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('admin-inventory-zmu-context')).toBeDefined();
        expect(screen.getByText('Поврежден')).toBeDefined();
        expect(screen.getByText('Нет агрегированных остатков')).toBeDefined();
      });
    });

    it('renders inventory without query on direct visit without query params', async () => {
      const getInventorySpy = vi.spyOn(adminInventoryApi, 'getAdminInventory').mockResolvedValue({
        items: [],
        totalCount: 0,
      });

      render(
        <MemoryRouter initialEntries={['/inventory']}>
          <AdminInventory />
        </MemoryRouter>
      );

      const input = screen.getByPlaceholderText('Поиск по названию, SKU или ZMU...') as HTMLInputElement;
      expect(input.value).toBe('');

      await waitFor(() => {
        expect(getInventorySpy).toHaveBeenCalledWith(
          expect.objectContaining({
            q: '',
          })
        );
      });
    });
  });

  // 3. Users Context Tests
  describe('AdminUsers context handoff', () => {
    it('initializes search input and filters users with ?q=customer@zamk.local', async () => {
      const getUsersSpy = vi.spyOn(adminUsersApi, 'getAdminUsers').mockResolvedValue({
        items: [
          {
            id: 'user-1',
            name: 'Nikita Osipov',
            email: 'customer@zamk.local',
            role: 'customer',
            status: 'active',
            createdAt: '2026-08-30T12:00:00Z',
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
      });

      render(
        <MemoryRouter initialEntries={['/users?q=customer@zamk.local']}>
          <AdminUsers />
        </MemoryRouter>
      );

      // Verifies search input has customer email
      const input = screen.getByPlaceholderText('Поиск по имени или email...') as HTMLInputElement;
      expect(input.value).toBe('customer@zamk.local');

      // Verifies API was called with customer email
      await waitFor(() => {
        expect(getUsersSpy).toHaveBeenCalledWith(
          expect.objectContaining({
            q: 'customer@zamk.local',
          })
        );
      });

      // Verifies matching user appears in table
      await waitFor(() => {
        expect(screen.getByText('customer@zamk.local')).toBeDefined();
        expect(screen.getByText('Nikita Osipov')).toBeDefined();
      });
    });

    it('renders users list without query on direct visit without query params', async () => {
      const getUsersSpy = vi.spyOn(adminUsersApi, 'getAdminUsers').mockResolvedValue({
        items: [],
        total: 0,
        limit: 20,
        offset: 0,
      });

      render(
        <MemoryRouter initialEntries={['/users']}>
          <AdminUsers />
        </MemoryRouter>
      );

      const input = screen.getByPlaceholderText('Поиск по имени или email...') as HTMLInputElement;
      expect(input.value).toBe('');

      await waitFor(() => {
        expect(getUsersSpy).toHaveBeenCalledWith(
          expect.objectContaining({
            q: '',
          })
        );
      });
    });
  });
});
