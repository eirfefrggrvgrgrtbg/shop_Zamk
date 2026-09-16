/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Catalog } from './Catalog';
import * as authContext from '../contexts/AuthContext';
import * as publicCatalog from '../api/publicCatalog';
import type { Product } from '../types/catalog';

// Mock IntersectionObserver for UI components
class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(window as any).IntersectionObserver = IntersectionObserverMock;

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}));

vi.mock('../contexts/FavoritesContext', () => ({
  useFavorites: () => ({ isFavorite: () => false, toggleFavorite: vi.fn() }),
}));

vi.mock('../contexts/ToastContext', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));

vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({ addItem: vi.fn() }),
}));

vi.mock('../api/publicCatalog', () => ({
  fetchProducts: vi.fn(),
  fetchCategories: vi.fn().mockResolvedValue([]),
  fetchBrands: vi.fn().mockResolvedValue([]),
}));

const mockProductAlpha: Product = {
  id: 'prod-alpha',
  name: 'Alpha Product',
  price: 1500,
  category: 'cat-1',
  brand: 'Brand A',
  brandId: 'b-1',
  sellerId: 's-1',
  sellerName: 'Seller 1',
  sellerSlug: 'seller-1',
  image: 'alpha.jpg',
  images: [],
  isNew: false,
};

const mockProductBeta: Product = {
  id: 'prod-beta',
  name: 'Beta Product',
  price: 2500,
  category: 'cat-1',
  brand: 'Brand B',
  brandId: 'b-2',
  sellerId: 's-1',
  sellerName: 'Seller 1',
  sellerSlug: 'seller-1',
  image: 'beta.jpg',
  images: [],
  isNew: false,
};

describe('Catalog Page - PER.6B2 Shop Personalized Catalog Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue([]);
    vi.mocked(publicCatalog.fetchBrands).mockResolvedValue([]);
  });

  afterEach(() => {
    cleanup();
  });

  const renderCatalog = (initialEntries = ['/catalog']) => {
    return render(
      <MemoryRouter initialEntries={initialEntries}>
        <Catalog />
      </MemoryRouter>
    );
  };

  it('1. Authenticated Customer + Default sort -> requests products with isCustomer: true', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductBeta, mockProductAlpha],
      totalCount: 2,
    });

    renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 24 }),
        { isCustomer: true }
      );
    });

    expect(screen.getByText('Beta Product')).toBeTruthy();
    expect(screen.getByText('Alpha Product')).toBeTruthy();
  });

  it('2. Anonymous visitor + Default sort -> requests products with isCustomer: false', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductAlpha],
      totalCount: 1,
    });

    renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 24 }),
        { isCustomer: false }
      );
    });
  });

  it('3. Authenticated user with missing / undefined role -> fail-closed with isCustomer: false', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-no-role', email: 'norole@test.com', role: undefined } as any,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductAlpha],
      totalCount: 1,
    });

    renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 24 }),
        { isCustomer: false }
      );
    });
  });

  it('4. Non-customer role (seller / admin) -> requests products with isCustomer: false', async () => {
    const authMock = vi.mocked(authContext.useAuth);
    authMock.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'seller-1', email: 'seller@test.com', role: 'seller' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductAlpha],
      totalCount: 1,
    });

    const { rerender } = renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 24 }),
        { isCustomer: false }
      );
    });

    // Test admin
    authMock.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'admin-1', email: 'admin@test.com', role: 'admin' } as any,
    } as any);

    rerender(
      <MemoryRouter initialEntries={['/catalog']}>
        <Catalog />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 24 }),
        { isCustomer: false }
      );
    });
  });

  it('5. Authenticated Customer + Explicit sort (e.g. price_asc) -> passes sort param', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductAlpha, mockProductBeta],
      totalCount: 2,
    });

    renderCatalog(['/catalog?sort=price_asc']);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 24, sort: 'price_asc' }),
        { isCustomer: true }
      );
    });
  });

  it('6. Backend response order is preserved in DOM without client-side re-sorting', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    // Beta first, then Alpha
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductBeta, mockProductAlpha],
      totalCount: 2,
    });

    renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(screen.getByText('Beta Product')).toBeTruthy();
    });

    const cardBeta = screen.getByText('Beta Product');
    const cardAlpha = screen.getByText('Alpha Product');
    expect(cardBeta.compareDocumentPosition(cardAlpha)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('7. Complete auth transition: anonymous -> Customer login -> logout -> public', async () => {
    const authMock = vi.mocked(authContext.useAuth);
    authMock.mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductAlpha],
      totalCount: 1,
    });

    const { rerender } = renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenLastCalledWith(
        expect.anything(),
        { isCustomer: false }
      );
    });

    // Step 2: Login as Customer
    authMock.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    rerender(
      <MemoryRouter initialEntries={['/catalog']}>
        <Catalog />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenLastCalledWith(
        expect.anything(),
        { isCustomer: true }
      );
    });

    // Step 3: Logout
    authMock.mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    rerender(
      <MemoryRouter initialEntries={['/catalog']}>
        <Catalog />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenLastCalledWith(
        expect.anything(),
        { isCustomer: false }
      );
    });
  });

  it('8. Sort switching: DEFAULT -> price_asc -> DEFAULT restores clean state and routing', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductBeta, mockProductAlpha],
      totalCount: 2,
    });

    renderCatalog(['/catalog']);

    // Initial load: DEFAULT sort
    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenLastCalledWith(
        expect.not.objectContaining({ sort: expect.anything() }),
        { isCustomer: true }
      );
    });

    // Open sort dropdown and click "Цена по возрастанию"
    const dropdownBtn = screen.getByText('По умолчанию');
    fireEvent.click(dropdownBtn);

    const priceAscOption = screen.getByText('Цена по возрастанию');
    fireEvent.click(priceAscOption);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenLastCalledWith(
        expect.objectContaining({ sort: 'price_asc' }),
        { isCustomer: true }
      );
    });

    // Open sort dropdown and click "По умолчанию"
    const currentSortBtn = screen.getByText('Цена по возрастанию');
    fireEvent.click(currentSortBtn);

    const defaultOption = screen.getByText('По умолчанию');
    fireEvent.click(defaultOption);

    await waitFor(() => {
      expect(publicCatalog.fetchProducts).toHaveBeenLastCalledWith(
        expect.not.objectContaining({ sort: expect.anything() }),
        { isCustomer: true }
      );
    });
  });

  it('9. No personalization badges or cues rendered on product cards or catalog page', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({
      items: [mockProductBeta],
      totalCount: 1,
    });

    renderCatalog(['/catalog']);

    await waitFor(() => {
      expect(screen.getByText('Beta Product')).toBeTruthy();
    });

    expect(screen.queryByText(/подобрано для вас/i)).toBeNull();
    expect(screen.queryByText(/персонализир/i)).toBeNull();
  });
});
