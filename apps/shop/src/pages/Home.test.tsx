/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { Home } from './Home';
import { BrowserRouter } from 'react-router-dom';
import * as authContext from '../contexts/AuthContext';
import * as publicCatalog from '../api/publicCatalog';
import type { Product } from '../types/catalog';

// Mock IntersectionObserver for Framer Motion
class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(window as any).IntersectionObserver = IntersectionObserverMock;

// Mock dependencies
vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn()
}));
vi.mock('../api/publicCatalog');
vi.mock('../components/product/ProductCard', () => ({
  ProductCard: ({ product }: { product: Product }) => (
    <div data-testid={`product-card-${product.id}`}>{product.name}</div>
  )
}));
vi.mock('../components/home/HomeAuctionBlock', () => ({
  HomeAuctionBlock: () => <div data-testid="auction-block">Auction Block</div>
}));
vi.mock('../components/home/HeroSection', () => ({
  HeroSection: () => <div data-testid="hero-section">Hero Section</div>
}));
vi.mock('../components/editorial/StudioKit', () => ({
  SectionHeader: ({ label, title }: { label: string, title: string }) => (
    <div data-testid="section-header">{label} - {title}</div>
  ),
  BrandCard: () => <div data-testid="brand-card" />,
  CategoryCard: () => <div data-testid="category-card" />
}));

const mockProducts: Product[] = [
  { id: '1', name: 'Product A', price: 100, category: 'Cat 1', brand: 'Brand 1', brandId: 'b1', sellerId: 's1', sellerName: 'Seller 1', sellerSlug: 'seller-1', image: 'img1.jpg', images: [], isNew: true },
  { id: '2', name: 'Product B', price: 200, category: 'Cat 1', brand: 'Brand 1', brandId: 'b1', sellerId: 's1', sellerName: 'Seller 1', sellerSlug: 'seller-1', image: 'img2.jpg', images: [], isNew: true },
];

const mockRecentProducts: Product[] = [
  { id: 'r2', name: 'Recent B', price: 200, category: 'Cat 1', brand: 'Brand 1', brandId: 'b1', sellerId: 's1', sellerName: 'Seller 1', sellerSlug: 'seller-1', image: 'img2.jpg', images: [], isNew: true },
  { id: 'r1', name: 'Recent A', price: 100, category: 'Cat 1', brand: 'Brand 1', brandId: 'b1', sellerId: 's1', sellerName: 'Seller 1', sellerSlug: 'seller-1', image: 'img1.jpg', images: [], isNew: true },
];

describe('Home Page - PER.3 Recently Viewed Block', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default mocks
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: mockProducts, totalCount: 2 });
    vi.mocked(publicCatalog.fetchDirectSaleProducts).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(publicCatalog.fetchBrands).mockResolvedValue([]);
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue([]);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false } as any);
  });

  afterEach(() => {
    cleanup();
  });

  const renderHome = () => {
    return render(
      <BrowserRouter>
        <Home />
      </BrowserRouter>
    );
  };

  it('A. authenticated + non-empty response -> "Недавно просмотренные" rendered', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: true } as any);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: mockRecentProducts, totalCount: 2 });

    renderHome();

    await waitFor(() => {
      expect(screen.getByText('История - Недавно просмотренные')).toBeTruthy();
    });

    // B. backend ordering preserved
    // "Recent B" and "Recent A" should be rendered in order, and ProductCard is mocked to output the name
    const recentB = await screen.findByTestId('product-card-r2');
    const recentA = await screen.findByTestId('product-card-r1');
    expect(recentB).toBeTruthy();
    expect(recentA).toBeTruthy();
    // In DOM, recentB should appear before recentA
    expect(recentB.compareDocumentPosition(recentA)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('D. anonymous -> no recently-viewed request -> block absent', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false } as any);

    renderHome();

    await waitFor(() => {
      expect(screen.queryByText('Загрузка витрины...')).toBeNull();
    });

    expect(publicCatalog.fetchRecentlyViewed).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
    });
  });

  it('E. authenticated + [] -> block absent', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: true } as any);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: [], totalCount: 0 });

    renderHome();

    await waitFor(() => {
      expect(screen.queryByText('Загрузка витрины...')).toBeNull();
    });

    expect(publicCatalog.fetchRecentlyViewed).toHaveBeenCalled();
    // Wait for the block to not be there
    await waitFor(() => {
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
    });
  });

  it('F. API error -> homepage still renders -> recently viewed block hidden', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: true } as any);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockRejectedValue(new Error('Network error'));

    renderHome();

    await waitFor(() => {
      expect(screen.queryByText('Загрузка витрины...')).toBeNull();
    });

    // Because fetchRecentlyViewed catches errors in our implementation and returns {items: []}
    // we just check if it degrades silently
    await waitFor(() => {
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
    });

    // G. existing homepage content remains present
    expect(screen.getByTestId('hero-section')).toBeTruthy();
    expect(screen.getByTestId('auction-block')).toBeTruthy();
    expect(screen.getByText('Новинки - Свежие поступления')).toBeTruthy();
  });
});
