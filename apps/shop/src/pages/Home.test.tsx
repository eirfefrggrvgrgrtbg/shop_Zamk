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
  useAuth: vi.fn(),
}));

vi.mock('../api/publicCatalog');

vi.mock('../components/product/ProductCard', () => ({
  ProductCard: ({ product }: { product: Product }) => (
    <div data-testid={`product-card-${product.id}`}>
      <a href={`/product/${product.id}`}>{product.name}</a>
    </div>
  ),
}));

vi.mock('../components/home/HomeAuctionBlock', () => ({
  HomeAuctionBlock: () => <div data-testid="auction-block">Auction Block</div>,
}));

vi.mock('../components/home/HeroSection', () => ({
  HeroSection: () => <div data-testid="hero-section">Hero Section</div>,
}));

vi.mock('../components/editorial/StudioKit', () => ({
  SectionHeader: ({ label, title }: { label: string; title: string }) => (
    <div data-testid="section-header">
      {label} - {title}
    </div>
  ),
  BrandCard: () => <div data-testid="brand-card" />,
  CategoryCard: () => <div data-testid="category-card" />,
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
    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({ blocks: [] });
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
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: mockRecentProducts, totalCount: 2 });

    renderHome();

    await waitFor(() => {
      expect(screen.getByText('История - Недавно просмотренные')).toBeTruthy();
    });

    const recentB = await screen.findByTestId('product-card-r2');
    const recentA = await screen.findByTestId('product-card-r1');
    expect(recentB).toBeTruthy();
    expect(recentA).toBeTruthy();
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
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: [], totalCount: 0 });

    renderHome();

    await waitFor(() => {
      expect(screen.queryByText('Загрузка витрины...')).toBeNull();
    });

    expect(publicCatalog.fetchRecentlyViewed).toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
    });
  });

  it('F. API error -> homepage still renders -> recently viewed block hidden', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false } as any);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockRejectedValue(new Error('Network error'));

    renderHome();

    await waitFor(() => {
      expect(screen.queryByText('Загрузка витрины...')).toBeNull();
    });

    await waitFor(() => {
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
    });

    expect(screen.getByTestId('hero-section')).toBeTruthy();
    expect(screen.getByTestId('auction-block')).toBeTruthy();
    expect(screen.getByText('Новинки - Свежие поступления')).toBeTruthy();
  });
});

describe('Home Page - PERS.2C3B Home Recommendation Composer Integration', () => {
  const mockFYProduct: Product = {
    id: 'fy-1',
    name: 'For You Jacket',
    price: 300,
    category: 'Cat 1',
    brand: 'Brand 1',
    brandId: 'b1',
    sellerId: 's1',
    sellerName: 'Seller 1',
    sellerSlug: 'seller-1',
    image: 'img1.jpg',
    images: [],
    isNew: false,
  };

  const mockPopularProduct: Product = {
    id: 'pop-1',
    name: 'Popular Hoodie',
    price: 250,
    category: 'Cat 1',
    brand: 'Brand 2',
    brandId: 'b2',
    sellerId: 's2',
    sellerName: 'Seller 2',
    sellerSlug: 'seller-2',
    image: 'img2.jpg',
    images: [],
    isNew: false,
  };

  const mockNewProduct: Product = {
    id: 'new-1',
    name: 'New Arrivals Trench',
    price: 500,
    category: 'Cat 2',
    brand: 'Brand 3',
    brandId: 'b3',
    sellerId: 's3',
    sellerName: 'Seller 3',
    sellerSlug: 'seller-3',
    image: 'img3.jpg',
    images: [],
    isNew: true,
  };

  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: mockProducts, totalCount: 2 });
    vi.mocked(publicCatalog.fetchDirectSaleProducts).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(publicCatalog.fetchBrands).mockResolvedValue([]);
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue([]);
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(publicCatalog.fetchForYouProducts).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({ blocks: [] });
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

  it('A. authenticated customer: composer blocks render in backend order', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [
        { type: 'for_you', title: 'Для вас', items: [mockFYProduct] },
        { type: 'popular', title: 'Популярное', items: [mockPopularProduct] },
        { type: 'new', title: 'Новинки', items: [mockNewProduct] },
      ],
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('recommendation-block-for_you')).toBeTruthy();
      expect(screen.getByTestId('recommendation-block-popular')).toBeTruthy();
      expect(screen.getByTestId('recommendation-block-new')).toBeTruthy();
    });

    const blockFY = screen.getByTestId('recommendation-block-for_you');
    const blockPop = screen.getByTestId('recommendation-block-popular');
    const blockNew = screen.getByTestId('recommendation-block-new');

    expect(blockFY.compareDocumentPosition(blockPop)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(blockPop.compareDocumentPosition(blockNew)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('B. for_you/popular/new render correct titles and items', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [
        { type: 'for_you', title: 'Для вас', items: [mockFYProduct] },
        { type: 'popular', title: 'Популярное', items: [mockPopularProduct] },
        { type: 'new', title: 'Новинки', items: [mockNewProduct] },
      ],
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByText('Рекомендации - Для вас')).toBeTruthy();
      expect(screen.getByText('Популярное - Популярное')).toBeTruthy();
      expect(screen.getByText('Новинки - Новинки')).toBeTruthy();
    });

    expect(screen.getByTestId('product-card-fy-1')).toBeTruthy();
    expect(screen.getByTestId('product-card-pop-1')).toBeTruthy();
    expect(screen.getByTestId('product-card-new-1')).toBeTruthy();
  });

  it('C. missing/empty block is not rendered', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    // Only popular returned, for_you and new are missing
    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [{ type: 'popular', title: 'Популярное', items: [mockPopularProduct] }],
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('recommendation-block-popular')).toBeTruthy();
    });

    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    expect(screen.queryByTestId('recommendation-block-new')).toBeNull();
  });

  it('D. blocks [] does not break Home', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({ blocks: [] });

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('hero-section')).toBeTruthy();
      expect(screen.getByTestId('auction-block')).toBeTruthy();
    });

    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    expect(screen.queryByTestId('recommendation-block-popular')).toBeNull();
    expect(screen.queryByTestId('recommendation-block-new')).toBeNull();
  });

  it('E. composer error does not break other Home sections', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockRejectedValue(new Error('Composer 500 error'));

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('hero-section')).toBeTruthy();
      expect(screen.getByTestId('auction-block')).toBeTruthy();
    });

    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    expect(screen.queryByTestId('recommendation-block-popular')).toBeNull();
    expect(screen.queryByTestId('recommendation-block-new')).toBeNull();
  });

  it('F. authenticated Home no longer performs legacy separate For You fetch', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [{ type: 'for_you', title: 'Для вас', items: [mockFYProduct] }],
    });

    renderHome();

    await waitFor(() => {
      expect(publicCatalog.fetchHomeRecommendations).toHaveBeenCalledTimes(1);
    });

    expect(publicCatalog.fetchForYouProducts).not.toHaveBeenCalled();
  });

  it('G. authenticated Home no longer performs legacy UUID-sorted New logic', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [{ type: 'new', title: 'Новинки', items: [mockNewProduct] }],
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('recommendation-block-new')).toBeTruthy();
    });

    // The legacy section "Новинки - Свежие поступления" is NOT rendered for authenticated customer
    expect(screen.queryByText('Новинки - Свежие поступления')).toBeNull();
  });

  it('H. Recently Viewed remains independent', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [{ type: 'for_you', title: 'Для вас', items: [mockFYProduct] }],
    });
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({
      items: [mockPopularProduct],
      totalCount: 1,
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByText('Рекомендации - Для вас')).toBeTruthy();
      expect(screen.getByText('История - Недавно просмотренные')).toBeTruthy();
    });

    expect(publicCatalog.fetchRecentlyViewed).toHaveBeenCalledTimes(1);
  });

  it('I. same product may exist in Recently Viewed + discovery without frontend removal', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    // Product mockFYProduct is in both For You discovery and Recently Viewed
    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [{ type: 'for_you', title: 'Для вас', items: [mockFYProduct] }],
    });
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({
      items: [mockFYProduct],
      totalCount: 1,
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByText('Рекомендации - Для вас')).toBeTruthy();
      expect(screen.getByText('История - Недавно просмотренные')).toBeTruthy();
      expect(screen.getAllByTestId(`product-card-${mockFYProduct.id}`)).toHaveLength(2);
    });
  });

  it('J. anonymous visitor does not call customer composer', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('hero-section')).toBeTruthy();
    });

    expect(publicCatalog.fetchHomeRecommendations).not.toHaveBeenCalled();
    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
  });

  it('K. anonymous existing Home behavior remains intact', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: false,
      user: null,
    } as any);

    renderHome();

    await waitFor(() => {
      expect(screen.getByText('Новинки - Свежие поступления')).toBeTruthy();
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
      expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    });
  });

  it('L. clicking product from composer block opens canonical PDP link', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 'c1', role: 'customer' } as any,
    } as any);

    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [{ type: 'popular', title: 'Популярное', items: [mockPopularProduct] }],
    });

    renderHome();

    await waitFor(() => {
      expect(screen.getByTestId('recommendation-block-popular')).toBeTruthy();
    });

    const link = screen.getByRole('link', { name: mockPopularProduct.name });
    expect(link.getAttribute('href')).toBe(`/product/${mockPopularProduct.id}`);
  });
});
