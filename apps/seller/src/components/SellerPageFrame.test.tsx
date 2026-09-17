/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerPageFrame, SellerPageHeader } from './SellerPageFrame';
import { SellerDashboard } from '../pages/SellerDashboard';
import { SellerProducts } from '../pages/SellerProducts';
import { SellerSettings } from '../pages/SellerSettings';
import { SellerOrders } from '../pages/SellerOrders';
import { SellerInventory } from '../pages/SellerInventory';
import { SellerSupplies } from '../pages/SellerSupplies';
import { SellerReturns } from '../pages/SellerReturns';

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
  getSellerProducts: vi.fn().mockResolvedValue([]),
  getSellerOrders: vi.fn().mockResolvedValue([]),
  getSellerOrderSummary: vi.fn().mockResolvedValue({
    totalOrders: 0,
    totalRevenueCents: 0,
    pendingDelivery: 0,
    delivered: 0,
    returnRatePercent: 0,
  }),
  getSellerReturns: vi.fn().mockResolvedValue([]),
  getSellerInventory: vi.fn().mockResolvedValue([]),
  getSellerSupplies: vi.fn().mockResolvedValue([]),
  getSellerBalance: vi.fn().mockResolvedValue({ availableCents: 5000000 }),
  getSellerWarnings: vi.fn().mockResolvedValue([]),
  getSellerViolations: vi.fn().mockResolvedValue([]),
  updateSellerMe: vi.fn(),
  uploadSellerLogo: vi.fn(),
}));

describe('SellerPageFrame & SellerPageHeader — Canonical 1296px Canvas Architecture', () => {
  it('renders children within the frame', () => {
    render(
      <SellerPageFrame>
        <div data-testid="test-child">Child Content</div>
      </SellerPageFrame>
    );
    expect(screen.getByTestId('test-child')).toBeTruthy();
  });

  it('enforces the exact same canonical outer canvas (1296px, mx-auto, gutters, top spacing) across all variants', () => {
    const { rerender } = render(
      <SellerPageFrame variant="summary">
        <div>Summary</div>
      </SellerPageFrame>
    );
    const summaryCanvas = screen.getByTestId('seller-page-frame');
    const expectedClasses = summaryCanvas.className;

    expect(expectedClasses).toContain('max-w-[1296px]');
    expect(expectedClasses).toContain('mx-auto');
    expect(expectedClasses).toContain('px-4 sm:px-6 lg:px-8');
    expect(expectedClasses).toContain('pt-8 pb-12');

    rerender(
      <SellerPageFrame variant="wide">
        <div>Wide</div>
      </SellerPageFrame>
    );
    const wideCanvas = screen.getByTestId('seller-page-frame');
    expect(wideCanvas.className).toBe(expectedClasses);

    rerender(
      <SellerPageFrame variant="form">
        <div>Form</div>
      </SellerPageFrame>
    );
    const formCanvas = screen.getByTestId('seller-page-frame');
    expect(formCanvas.className).toBe(expectedClasses);

    rerender(
      <SellerPageFrame variant="default">
        <div>Default</div>
      </SellerPageFrame>
    );
    const defaultCanvas = screen.getByTestId('seller-page-frame');
    expect(defaultCanvas.className).toBe(expectedClasses);
  });

  it('proves page family no longer changes outer canvas max-width or introduces family-width branching', () => {
    const variants: Array<'summary' | 'wide' | 'form' | 'default'> = ['summary', 'wide', 'form', 'default'];

    variants.forEach((variant) => {
      const { unmount } = render(
        <SellerPageFrame variant={variant}>
          <div>Content</div>
        </SellerPageFrame>
      );
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(frame.className).not.toContain('max-w-5xl');
      expect(frame.className).not.toContain('max-w-[1600px]');
      expect(frame.className).not.toContain('max-w-3xl');
      expect(frame.className).not.toContain('max-w-7xl');
      unmount();
    });
  });

  it('renders SellerPageHeader with eyebrow, title, description, and action', () => {
    render(
      <SellerPageHeader
        eyebrow="Тестовая надпись"
        title="Заголовок страницы"
        description="Описание функционала страницы"
        action={<button data-testid="header-action">Действие</button>}
      />
    );

    expect(screen.getByTestId('seller-page-header-eyebrow').textContent).toBe('Тестовая надпись');
    expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Заголовок страницы');
    expect(screen.getByTestId('seller-page-header-description').textContent).toBe('Описание функционала страницы');
    expect(screen.getByTestId('header-action')).toBeTruthy();
  });
});

describe('Reference Pages Geometry Integration — R1.2A', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('Dashboard, Products, and Settings share the exact same outer canvas geometry', async () => {
    const { unmount: unmountDashboard } = render(
      <MemoryRouter>
        <SellerDashboard />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const dashboardCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountDashboard();

    const { unmount: unmountProducts } = render(
      <MemoryRouter>
        <SellerProducts />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const productsCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountProducts();

    const { unmount: unmountSettings } = render(
      <MemoryRouter>
        <SellerSettings />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const settingsCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountSettings();

    expect(dashboardCanvasClasses).toBe(productsCanvasClasses);
    expect(productsCanvasClasses).toBe(settingsCanvasClasses);
    expect(dashboardCanvasClasses).toContain('max-w-[1296px]');
    expect(dashboardCanvasClasses).toContain('mx-auto');
    expect(dashboardCanvasClasses).toContain('px-4 sm:px-6 lg:px-8');
    expect(dashboardCanvasClasses).toContain('pt-8 pb-12');
  });

  it('Dashboard uses SUMMARY geometry and renders page header correctly', async () => {
    render(
      <MemoryRouter>
        <SellerDashboard />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('summary');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Обзор магазина');
      expect(screen.getByTestId('seller-page-header-eyebrow').textContent).toBe('Панель продавца');
    });
  });

  it('Products uses WIDE geometry, renders title, and preserves Add Product CTA to /products/new', async () => {
    render(
      <MemoryRouter>
        <SellerProducts />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('wide');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Мои товары');
    });

    const addBtns = screen.getAllByRole('link', { name: /добавить товар/i });
    expect(addBtns.length).toBeGreaterThan(0);
    expect(addBtns.every(btn => btn.getAttribute('href') === '/products/new')).toBe(true);
  });

  it('Settings uses FORM geometry with internal 8-9 col grid composition and renders header with status badge', async () => {
    render(
      <MemoryRouter>
        <SellerSettings />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('form');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Профиль магазина');
      expect(screen.getByText('Активен')).toBeTruthy();

      const grid = screen.getByTestId('seller-grid');
      expect(grid.className).toContain('grid-cols-12');
      const formColumn = grid.firstElementChild;
      expect(formColumn?.className).toContain('lg:col-span-8');
      expect(formColumn?.className).toContain('xl:col-span-9');
    });
  });
});

describe('Rollout Pages Geometry Integration — R1.2B1 (Orders, Inventory, Supplies, Returns)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('Orders, Inventory, Supplies, and Returns share the exact same outer canvas geometry', async () => {
    const { unmount: unmountOrders } = render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const ordersCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountOrders();

    const { unmount: unmountInventory } = render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const inventoryCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountInventory();

    const { unmount: unmountSupplies } = render(
      <MemoryRouter>
        <SellerSupplies />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const suppliesCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountSupplies();

    const { unmount: unmountReturns } = render(
      <MemoryRouter>
        <SellerReturns />
      </MemoryRouter>
    );
    await waitFor(() => expect(screen.getByTestId('seller-page-frame')).toBeTruthy());
    const returnsCanvasClasses = screen.getByTestId('seller-page-frame').className;
    unmountReturns();

    expect(ordersCanvasClasses).toBe(inventoryCanvasClasses);
    expect(inventoryCanvasClasses).toBe(suppliesCanvasClasses);
    expect(suppliesCanvasClasses).toBe(returnsCanvasClasses);
    expect(ordersCanvasClasses).toContain('max-w-[1296px]');
    expect(ordersCanvasClasses).toContain('mx-auto');
    expect(ordersCanvasClasses).toContain('px-4 sm:px-6 lg:px-8');
    expect(ordersCanvasClasses).toContain('pt-8 pb-12');
  });

  it('Orders uses summary frame and renders header with correct eyebrow and title', async () => {
    render(
      <MemoryRouter>
        <SellerOrders />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('summary');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-eyebrow').textContent).toBe('Продажи');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Заказы');
    });
  });

  it('Inventory uses wide frame, renders header, and provides Create Supply CTA to /supplies/new', async () => {
    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('wide');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-eyebrow').textContent).toBe('Ассортимент');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Остатки');
      expect(screen.getByRole('button', { name: /создать поставку/i })).toBeTruthy();
    });
  });

  it('Supplies uses wide frame, renders header, and provides Create Supply CTA to /supplies/new', async () => {
    render(
      <MemoryRouter>
        <SellerSupplies />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('wide');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-eyebrow').textContent).toBe('Ассортимент');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Поставки');
    });

    const createLinks = screen.getAllByRole('link', { name: /создать поставку/i });
    expect(createLinks.length).toBeGreaterThan(0);
    expect(createLinks.every((link) => link.getAttribute('href') === '/supplies/new')).toBe(true);
  });

  it('Returns uses summary frame and renders header with correct eyebrow and title', async () => {
    render(
      <MemoryRouter>
        <SellerReturns />
      </MemoryRouter>
    );

    await waitFor(() => {
      const frame = screen.getByTestId('seller-page-frame');
      expect(frame.getAttribute('data-variant')).toBe('summary');
      expect(frame.className).toContain('max-w-[1296px]');
      expect(screen.getByTestId('seller-page-header-eyebrow').textContent).toBe('Продажи');
      expect(screen.getByTestId('seller-page-header-title').textContent).toBe('Возвраты');
    });
  });
});
