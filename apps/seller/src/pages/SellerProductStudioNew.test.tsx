/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import fs from 'fs';
import path from 'path';
import * as sellerApi from '@zamk/api-client/src/seller';
import * as authApi from '@zamk/api-client/src/auth';
import { SellerProductStudioNew, INITIAL_EMPTY_STUDIO_DRAFT } from './SellerProductStudioNew';

vi.mock('@zamk/api-client/src/seller', async (importOriginal) => {
  const actual = await importOriginal<typeof sellerApi>();
  return {
    ...actual,
    getSellerMe: vi.fn().mockResolvedValue({
      user: { id: 'usr-1', name: 'Seller User', email: 'seller@example.com', role: 'seller', status: 'active' },
      sellerUser: { id: 'su-1', sellerId: 'sel-1', userId: 'usr-1', role: 'owner' },
      seller: { id: 'sel-1', brandName: 'Brand Test', slug: 'brand-test', status: 'active' },
    }),
    getSellerBalance: vi.fn().mockResolvedValue({ availableCents: 150000 }),
    getSellerProducts: vi.fn().mockResolvedValue([]),
    createSellerProduct: vi.fn().mockResolvedValue({ id: 'prod-new' }),
    updateSellerProduct: vi.fn().mockResolvedValue({ id: 'prod-updated' }),
  };
});

vi.mock('@zamk/api-client/src/auth', async (importOriginal) => {
  const actual = await importOriginal<typeof authApi>();
  return {
    ...actual,
    me: vi.fn().mockResolvedValue({
      user: { id: 'usr-1', name: 'Seller User', email: 'seller@example.com', role: 'seller', status: 'active' },
    }),
    refresh: vi.fn().mockResolvedValue({
      user: { id: 'usr-1', name: 'Seller User', email: 'seller@example.com', role: 'seller', status: 'active' },
    }),
  };
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

beforeEach(() => {
  vi.clearAllMocks();
});

describe('PS.R4A — Seller Product Studio Create Route Integration', () => {
  // 1. /products/new resolves to Seller Product Studio page in App.tsx
  it('1. /products/new resolves to SellerProductStudioNew in App.tsx', () => {
    const appPath = path.resolve(__dirname, '../App.tsx');
    const appContent = fs.readFileSync(appPath, 'utf-8');

    expect(appContent).toContain("import SellerProductStudioNew from './pages/SellerProductStudioNew';");
    expect(appContent).toContain('path="/products/new" element={<SellerProtectedRoute><SellerLayout><SellerProductStudioNew /></SellerLayout></SellerProtectedRoute>}');
  });

  // 2. ProductStudioProvider receives entryMode="create" and canonical empty initial draft
  it('2. mounts with entryMode="create" and canonical empty initial draft', () => {
    expect(INITIAL_EMPTY_STUDIO_DRAFT).toEqual({
      title: '',
      description: '',
      categoryId: '',
      images: [],
      variants: [],
    });

    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    expect(screen.getByTestId('product-studio-root')).toBeTruthy();
    expect(screen.getByTestId('studio-entry-badge').textContent).toBe('Черновик');
    expect(screen.getByTestId('studio-product-title').textContent).toBe('Новый товар');
  });

  // 3 & 4. Visual is initial mode and shared Visual workspace renders
  it('3 & 4. defaults to Visual mode with shared presentation workspace and neutral empty draft placeholders', () => {
    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();
    expect(screen.queryByTestId('studio-form-workspace')).toBeNull();

    // Check neutral empty placeholders in Visual mode
    const visual = screen.getByTestId('studio-visual-workspace');
    expect(within(visual).getByRole('heading', { level: 1 }).textContent).toBe('Название товара *');
    expect(within(visual).getByText(/Добавить фото/i)).toBeTruthy();
    expect(within(visual).getByTestId('add-to-cart-button')).toBeTruthy();

    // CTA must be disabled / non-actionable for empty 0-variants draft
    const cta = within(visual).getByTestId('add-to-cart-button') as HTMLButtonElement;
    expect(cta.disabled).toBe(true);
    expect(cta.textContent).toBe('Добавить в корзину');
  });

  // 5 & 6. Form toggle works and Form -> Visual preserves draft values
  it('5 & 6. toggles between Visual and Form mode preserving draft values bidirectionally', () => {
    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    // Switch to Form mode
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    expect(screen.getByTestId('studio-form-workspace')).toBeTruthy();
    expect(screen.queryByTestId('studio-visual-workspace')).toBeNull();

    // Type in Form mode
    const titleInput = screen.getByTestId('form-product-title-input') as HTMLInputElement;
    fireEvent.change(titleInput, { target: { value: 'Новая блузка из льна' } });
    expect(titleInput.value).toBe('Новая блузка из льна');

    // Switch to Pricing and set price
    fireEvent.click(screen.getByTestId('studio-section-btn-pricing'));
    const priceInput = screen.getByTestId('form-product-price-input') as HTMLInputElement;
    fireEvent.change(priceInput, { target: { value: '8900' } });

    // Switch back to Visual mode
    fireEvent.click(screen.getByTestId('studio-view-toggle-visual'));
    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();

    const visual = screen.getByTestId('studio-visual-workspace');
    expect(within(visual).getByRole('heading', { level: 1 }).textContent).toBe('Новая блузка из льна');
    expect(within(visual).getByText(/8[\s\u00a0]900/)).toBeTruthy();
  });

  // 7. create route does NOT call create-product API on mount
  it('7. mounting create route makes NO create/persist product API calls', async () => {
    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    expect(sellerApi.createSellerProduct).not.toHaveBeenCalled();
    expect(sellerApi.updateSellerProduct).not.toHaveBeenCalled();
  });

  // 8. create route does NOT auto-save on change
  it('8. changing draft in create route does NOT trigger auto-save or server mutations', () => {
    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    const titleInput = screen.getByTestId('form-product-title-input');
    fireEvent.change(titleInput, { target: { value: 'Черновик без сохранения' } });

    expect(sellerApi.createSellerProduct).not.toHaveBeenCalled();
    expect(sellerApi.updateSellerProduct).not.toHaveBeenCalled();
  });

  // 9 & 10. edit route remains legacy existing component and /products unchanged
  it('9 & 10. edit route and products list route remain legacy in App.tsx', () => {
    const appPath = path.resolve(__dirname, '../App.tsx');
    const appContent = fs.readFileSync(appPath, 'utf-8');

    expect(appContent).toContain("import SellerProductEdit from './pages/SellerProductEdit';");
    expect(appContent).toContain('path="/products/:id/edit" element={<SellerProtectedRoute><SellerLayout><SellerProductEdit /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products" element={<SellerProtectedRoute><SellerLayout><SellerProducts /></SellerLayout></SellerProtectedRoute>}');
  });

  // 11. Seller protection / auth wrapper unchanged
  it('11. SellerProtectedRoute and SellerLayout protect the new Studio route', () => {
    const appPath = path.resolve(__dirname, '../App.tsx');
    const appContent = fs.readFileSync(appPath, 'utf-8');

    const studioRoute = appContent
      .split('\n')
      .find((line) => line.includes('/products/new'));

    expect(studioRoute).toContain('SellerProtectedRoute');
    expect(studioRoute).toContain('SellerLayout');
    expect(studioRoute).toContain('SellerProductStudioNew');
  });

  // 12. no hard navigation API used in SellerProductStudioNew or ProductStudioHeader
  it('12. uses React Router Link for back navigation without hard location reload', () => {
    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    const backLink = screen.getByTestId('studio-back-to-products');
    expect(backLink.getAttribute('href')).toBe('/products');
    expect(backLink.tagName.toLowerCase()).toBe('a');
  });

  // 13 & 14. Full-width studio canvas inside SellerLayout
  it('13 & 14. Studio renders with appropriate max-width canvas without compressing PDP column geometry', () => {
    render(
      <MemoryRouter>
        <SellerProductStudioNew />
      </MemoryRouter>
    );

    const visualWorkspace = screen.getByTestId('studio-visual-workspace');
    expect(visualWorkspace.firstElementChild?.className).toContain('max-w-[1360px]');

    const header = screen.getByTestId('product-studio-header');
    expect(header.firstElementChild?.className).toContain('max-w-[1360px]');
  });
});
