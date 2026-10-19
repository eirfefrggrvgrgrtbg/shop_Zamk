/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react';
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

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}));

vi.mock('../api/publicCatalog');

vi.mock('../components/product/ProductCard', () => ({
  ProductCard: ({ product, placement }: { product: Product; placement?: string }) => (
    <div data-testid={`product-card-${product.id}`} data-placement={placement}>
      <a href={`/product/${product.id}`}>{product.name}</a>
    </div>
  ),
}));

vi.mock('../components/home/HomeAuctionBlock', () => ({
  HomeAuctionBlock: () => <div data-testid="auction-block">Auction Block</div>,
}));

vi.mock('../components/home/HeroSection', () => ({
  HeroSection: ({ products }: { products?: Product[] }) => (
    <div data-testid="hero-section">Hero Section {products?.length ?? 0}</div>
  ),
}));

vi.mock('../components/editorial/StudioKit', () => ({
  SectionHeader: ({ label, title, action }: { label: string; title: string; action?: React.ReactNode }) => (
    <div data-testid="section-header">
      <span>
        {label} - {title}
      </span>
      {action}
    </div>
  ),
  BrandCard: () => <div data-testid="brand-card" />,
}));

function makeProduct(id: string, overrides: Partial<Product> = {}): Product {
  return {
    id,
    name: `Product ${id}`,
    price: 100,
    category: 'cat-1',
    brand: 'Brand 1',
    brandId: 'b1',
    sellerId: 's1',
    sellerName: 'Seller 1',
    sellerSlug: 'seller-1',
    image: `https://cdn.example/${id}.jpg`,
    images: [],
    ...overrides,
  };
}

const newest = [makeProduct('n1'), makeProduct('n2'), makeProduct('n3')];

const guest = () =>
  vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false, user: null } as any);
const customer = () =>
  vi.mocked(authContext.useAuth).mockReturnValue({
    isAuthenticated: true,
    user: { id: 'c1', role: 'customer' } as any,
  } as any);

const renderHome = () =>
  render(
    <BrowserRouter>
      <Home />
    </BrowserRouter>,
  );

async function waitForLoaded() {
  await waitFor(() => {
    expect(screen.queryByText('Загрузка витрины...')).toBeNull();
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: newest, totalCount: newest.length });
  vi.mocked(publicCatalog.fetchDirectSaleProducts).mockResolvedValue({ items: [], totalCount: 0 });
  vi.mocked(publicCatalog.fetchBrands).mockResolvedValue([]);
  vi.mocked(publicCatalog.fetchCategories).mockResolvedValue([]);
  vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({ items: [], totalCount: 0 });
  vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({ blocks: [] });
  vi.mocked(publicCatalog.fetchPopularProducts).mockResolvedValue({ items: [], totalCount: 0 });
  guest();
});

afterEach(() => {
  cleanup();
});

describe('Home — guest', () => {
  it('shows hero with real products and new arrivals linking to the PDP and /new', async () => {
    renderHome();
    await waitForLoaded();

    expect(screen.getByTestId('hero-section').textContent).toContain('3');
    const section = screen.getByTestId('new-arrivals-section');
    expect(within(section).getByText('Новинки - Новые поступления')).toBeTruthy();
    const link = within(section).getByRole('link', { name: 'Product n1' });
    expect(link.getAttribute('href')).toBe('/product/n1');
    expect(within(section).getByRole('link', { name: /Все новинки/ }).getAttribute('href')).toBe('/new');
  });

  it('requests newest products and the public popular selection, never customer endpoints', async () => {
    renderHome();
    await waitForLoaded();

    expect(publicCatalog.fetchProducts).toHaveBeenCalledWith({ sort: 'newest' });
    expect(publicCatalog.fetchPopularProducts).toHaveBeenCalled();
    expect(publicCatalog.fetchHomeRecommendations).not.toHaveBeenCalled();
    expect(publicCatalog.fetchRecentlyViewed).not.toHaveBeenCalled();
    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    expect(screen.queryByText('Рекомендации - Для вас')).toBeNull();
  });

  it('general fallback: popular products appear as «Популярное», without duplicating new arrivals', async () => {
    vi.mocked(publicCatalog.fetchPopularProducts).mockResolvedValue({
      items: [makeProduct('n1'), makeProduct('p1')],
      totalCount: 2,
    });

    renderHome();

    const block = await screen.findByTestId('recommendation-block-popular');
    expect(within(block).getByText('Популярное - Популярное')).toBeTruthy();
    expect(within(block).queryByText('Product n1')).toBeNull();
    expect(within(block).getByText('Product p1')).toBeTruthy();
  });

  it('popular endpoint failure / empty popular does not break Home and hides the section', async () => {
    vi.mocked(publicCatalog.fetchPopularProducts).mockRejectedValue(new Error('boom'));

    renderHome();
    await waitForLoaded();

    expect(screen.queryByTestId('recommendation-block-popular')).toBeNull();
    expect(screen.getByTestId('new-arrivals-section')).toBeTruthy();
    expect(screen.getByTestId('auction-block')).toBeTruthy();
  });

  it('links categories that have products and skips empty ones', async () => {
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue([
      { id: 'cat-1', slug: 'jackets', name: 'Куртки', icon: '✦' },
      { id: 'cat-empty', slug: 'empty', name: 'Пустая категория', icon: '✦' },
    ]);

    renderHome();

    const strip = await screen.findByTestId('home-category-strip');
    expect(within(strip).getByRole('link', { name: 'Куртки' }).getAttribute('href')).toBe('/catalog?categoryId=cat-1');
    expect(within(strip).queryByText('Пустая категория')).toBeNull();
    expect(within(strip).getByRole('link', { name: 'Весь каталог' }).getAttribute('href')).toBe('/catalog');
  });

  it('empty catalog: renders an empty state without errors and no category strip', async () => {
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [], totalCount: 0 });

    renderHome();
    await waitForLoaded();

    expect(screen.getByText(/Товары пока не добавлены/)).toBeTruthy();
    expect(screen.queryByTestId('home-category-strip')).toBeNull();
    expect(screen.queryByTestId('new-arrivals-section')).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('API failure: shows an error with retry, retry reloads the storefront', async () => {
    vi.mocked(publicCatalog.fetchProducts).mockRejectedValueOnce(new Error('down'));

    renderHome();

    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain('Не удалось загрузить витрину');
    expect(screen.getByTestId('hero-section')).toBeTruthy();

    fireEvent.click(within(alert).getByRole('button', { name: 'Повторить' }));

    await waitFor(() => {
      expect(screen.getByTestId('new-arrivals-section')).toBeTruthy();
    });
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('Home — authenticated customer', () => {
  it('renders «Для вас» from backend composer and removes those products from new arrivals', async () => {
    customer();
    vi.mocked(publicCatalog.fetchHomeRecommendations).mockResolvedValue({
      blocks: [
        { type: 'for_you', title: 'Для вас', items: [makeProduct('n1'), makeProduct('fy1')] },
        { type: 'popular', title: 'Популярное', items: [makeProduct('pop1')] },
        { type: 'new', title: 'Новинки', items: [makeProduct('ignored-new')] },
      ],
    });

    renderHome();

    const forYou = await screen.findByTestId('recommendation-block-for_you');
    expect(within(forYou).getByText('Рекомендации - Для вас')).toBeTruthy();
    expect(within(forYou).getByText('Product fy1')).toBeTruthy();

    const arrivals = screen.getByTestId('new-arrivals-section');
    expect(within(arrivals).queryByText('Product n1')).toBeNull();
    expect(within(arrivals).getByText('Product n2')).toBeTruthy();

    expect(screen.getByTestId('recommendation-block-popular')).toBeTruthy();
    expect(screen.queryByText('Product ignored-new')).toBeNull();

    // for_you is rendered before new arrivals
    expect(forYou.compareDocumentPosition(arrivals) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(publicCatalog.fetchPopularProducts).not.toHaveBeenCalled();
  });

  it('new customer without signals (no blocks) gets the general selection and no «Для вас»', async () => {
    customer();

    renderHome();
    await waitForLoaded();

    expect(publicCatalog.fetchHomeRecommendations).toHaveBeenCalled();
    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    expect(screen.getByTestId('new-arrivals-section')).toBeTruthy();
  });

  it('composer failure keeps the rest of Home intact', async () => {
    customer();
    vi.mocked(publicCatalog.fetchHomeRecommendations).mockRejectedValue(new Error('boom'));

    renderHome();
    await waitForLoaded();

    expect(screen.queryByTestId('recommendation-block-for_you')).toBeNull();
    expect(screen.getByTestId('new-arrivals-section')).toBeTruthy();
    expect(screen.getByTestId('auction-block')).toBeTruthy();
  });

  it('non-customer roles (seller/admin) do not call customer endpoints', async () => {
    vi.mocked(authContext.useAuth).mockReturnValue({
      isAuthenticated: true,
      user: { id: 's1', role: 'seller' } as any,
    } as any);

    renderHome();
    await waitForLoaded();

    expect(publicCatalog.fetchHomeRecommendations).not.toHaveBeenCalled();
  });

  it('renders recently viewed in history order when present', async () => {
    customer();
    vi.mocked(publicCatalog.fetchRecentlyViewed).mockResolvedValue({
      items: [makeProduct('r2'), makeProduct('r1')],
      totalCount: 2,
    });

    renderHome();

    expect(await screen.findByText('История - Недавно просмотренные')).toBeTruthy();
    const r2 = await screen.findByTestId('product-card-r2');
    const r1 = await screen.findByTestId('product-card-r1');
    expect(r2.compareDocumentPosition(r1)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('hides recently viewed when history is empty', async () => {
    customer();

    renderHome();
    await waitForLoaded();

    expect(publicCatalog.fetchRecentlyViewed).toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText('История - Недавно просмотренные')).toBeNull();
    });
  });
});
