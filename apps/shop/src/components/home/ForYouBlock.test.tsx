/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { ForYouBlock } from './ForYouBlock';
import * as publicCatalog from '../../api/publicCatalog';
import * as authContext from '../../contexts/AuthContext';
import type { Product } from '../../types/catalog';

// Mock IntersectionObserver
class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(window as any).IntersectionObserver = IntersectionObserverMock;

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}));

vi.mock('../../contexts/FavoritesContext', () => ({
  useFavorites: () => ({ isFavorite: () => false, toggleFavorite: vi.fn() }),
}));

vi.mock('../../contexts/ToastContext', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));

vi.mock('../../contexts/CartContext', () => ({
  useCart: () => ({ addItem: vi.fn() }),
}));

vi.mock('../../api/publicCatalog', () => ({
  fetchForYouProducts: vi.fn(),
}));

const mockProductB: Product = {
  id: 'prod-b',
  name: 'Product B',
  price: 2500,
  category: 'cat-outerwear',
  brand: 'Brand Beta',
  brandId: 'b-2',
  sellerId: 's-1',
  sellerName: 'Seller Beta',
  sellerSlug: 'seller-beta',
  image: 'img-b.jpg',
  images: [],
  isNew: false,
};

const mockProductA: Product = {
  id: 'prod-a',
  name: 'Product A',
  price: 1800,
  category: 'cat-outerwear',
  brand: 'Brand Alpha',
  brandId: 'b-1',
  sellerId: 's-1',
  sellerName: 'Seller Alpha',
  sellerSlug: 'seller-alpha',
  image: 'img-a.jpg',
  images: [],
  isNew: false,
};

describe('ForYouBlock - PER.5B2 Unit & Acceptance', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  const renderBlock = () => {
    return render(
      <BrowserRouter>
        <ForYouBlock />
      </BrowserRouter>
    );
  };

  it('A. Authenticated customer + non-empty response -> "Для вас" renders', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [mockProductB, mockProductA],
      totalCount: 2,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Для вас')).toBeTruthy();
    });

    expect(screen.getByText('Product B')).toBeTruthy();
    expect(screen.getByText('Product A')).toBeTruthy();
  });

  it('B. Backend response order is preserved (B then A)', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [mockProductB, mockProductA],
      totalCount: 2,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Product B')).toBeTruthy();
    });

    const cardB = screen.getByText('Product B');
    const cardA = screen.getByText('Product A');
    expect(cardB.compareDocumentPosition(cardA)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('C. Canonical ProductCard is reused with brand, seller, and price', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [mockProductB],
      totalCount: 1,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Product B')).toBeTruthy();
    });

    // ProductCard renders seller and formatted price
    expect(screen.getByText('Seller Beta')).toBeTruthy();
    expect(screen.getByText(/2\s*500/)).toBeTruthy();
  });

  it('D. Clicking a recommendation opens canonical PDP link', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [mockProductB],
      totalCount: 1,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Product B')).toBeTruthy();
    });

    const links = screen.getAllByRole('link');
    const hasCanonicalHref = links.some((l) => l.getAttribute('href') === `/product/${mockProductB.id}`);
    expect(hasCanonicalHref).toBe(true);
  });

  it('E. Anonymous visitor -> no /for-you request -> no "Для вас" block', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    renderBlock();

    expect(publicCatalog.fetchForYouProducts).not.toHaveBeenCalled();
    expect(screen.queryByText('Для вас')).toBeNull();
    expect(screen.queryByTestId('for-you-block')).toBeNull();
  });

  it('E2. Seller or Admin role -> no /for-you request -> no "Для вас" block', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'admin-1', email: 'admin@test.com', role: 'admin' } as any,
    } as any);

    renderBlock();

    expect(publicCatalog.fetchForYouProducts).not.toHaveBeenCalled();
    expect(screen.queryByText('Для вас')).toBeNull();
    expect(screen.queryByTestId('for-you-block')).toBeNull();
  });

  it('F. Authenticated customer + items: [] -> block hidden', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [],
      totalCount: 0,
    });

    renderBlock();

    await waitFor(() => {
      expect(publicCatalog.fetchForYouProducts).toHaveBeenCalled();
    });

    expect(screen.queryByText('Для вас')).toBeNull();
    expect(screen.queryByTestId('for-you-block')).toBeNull();
  });

  it('G. API failure -> block hidden gracefully', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockRejectedValue(new Error('Network error'));

    renderBlock();

    await waitFor(() => {
      expect(publicCatalog.fetchForYouProducts).toHaveBeenCalled();
    });

    expect(screen.queryByText('Для вас')).toBeNull();
    expect(screen.queryByTestId('for-you-block')).toBeNull();
  });

  it('H. Logout / unauthenticated state -> block absent', async () => {
    const authMock = vi.mocked(authContext.useAuth);
    authMock.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [mockProductB],
      totalCount: 1,
    });

    const { rerender } = renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Для вас')).toBeTruthy();
    });

    // Simulate logout
    authMock.mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    rerender(
      <BrowserRouter>
        <ForYouBlock />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.queryByText('Для вас')).toBeNull();
    });
  });

  it('J. No duplicate request caused by ordinary rerenders', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'cust-1', email: 'cust@test.com', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({
      items: [mockProductB],
      totalCount: 1,
    });

    const { rerender } = renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Для вас')).toBeTruthy();
    });

    expect(publicCatalog.fetchForYouProducts).toHaveBeenCalledTimes(1);

    // Rerender with same auth state
    rerender(
      <BrowserRouter>
        <ForYouBlock />
      </BrowserRouter>
    );

    expect(publicCatalog.fetchForYouProducts).toHaveBeenCalledTimes(1);
  });
});
