/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { SimilarProductsBlock } from './SimilarProductsBlock';
import * as publicCatalog from '../../api/publicCatalog';
import type { Product } from '../../types/catalog';

// Mock IntersectionObserver
class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(window as any).IntersectionObserver = IntersectionObserverMock;

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: null, isAuthenticated: false, openAuthModal: vi.fn() }),
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
  fetchSimilarProducts: vi.fn(),
}));

const sourceProductID = 'source-prod-123';

const mockCandidate1: Product = {
  id: 'cand-1',
  name: 'Candidate Jacket 1',
  price: 1500,
  category: 'cat-outerwear',
  brand: 'Brand Alpha',
  brandId: 'b-1',
  sellerId: 's-1',
  sellerName: 'Seller 1',
  sellerSlug: 'seller-1',
  image: 'img1.jpg',
  images: [],
  isNew: false,
};

const mockCandidate2: Product = {
  id: 'cand-2',
  name: 'Candidate Coat 2',
  price: 2000,
  category: 'cat-outerwear',
  brand: 'Brand Beta',
  brandId: 'b-2',
  sellerId: 's-1',
  sellerName: 'Seller 1',
  sellerSlug: 'seller-1',
  image: 'img2.jpg',
  images: [],
  isNew: false,
};

describe('SimilarProductsBlock - PER.4 Acceptance', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  const renderBlock = (productId = sourceProductID) => {
    return render(
      <BrowserRouter>
        <SimilarProductsBlock productId={productId} />
      </BrowserRouter>
    );
  };

  it('A. PDP + non-empty response -> "Похожие товары" renders', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [mockCandidate1, mockCandidate2],
      totalCount: 2,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Похожие товары')).toBeTruthy();
    });

    expect(screen.getByText('Candidate Jacket 1')).toBeTruthy();
    expect(screen.getByText('Candidate Coat 2')).toBeTruthy();
  });

  it('B. backend ordering preserved', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [mockCandidate1, mockCandidate2],
      totalCount: 2,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Candidate Jacket 1')).toBeTruthy();
    });

    const card1 = screen.getByText('Candidate Jacket 1');
    const card2 = screen.getByText('Candidate Coat 2');
    expect(card1.compareDocumentPosition(card2)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('C. source product itself is not rendered (frontend guard)', async () => {
    const candidateSourceDuplicate: Product = {
      ...mockCandidate1,
      id: sourceProductID,
      name: 'Accidental Source Duplicate',
    };

    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [candidateSourceDuplicate, mockCandidate2],
      totalCount: 2,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Candidate Coat 2')).toBeTruthy();
    });

    expect(screen.queryByText('Accidental Source Duplicate')).toBeNull();
  });

  it('D. clicking recommendation opens canonical PDP link', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [mockCandidate1],
      totalCount: 1,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Candidate Jacket 1')).toBeTruthy();
    });

    const links = screen.getAllByRole('link');
    const hasCanonicalHref = links.some((l) => l.getAttribute('href') === `/product/${mockCandidate1.id}`);
    expect(hasCanonicalHref).toBe(true);
  });

  it('E & F. renders recommendation without requiring user auth', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [mockCandidate1],
      totalCount: 1,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Похожие товары')).toBeTruthy();
    });

    expect(publicCatalog.fetchSimilarProducts).toHaveBeenCalledWith(sourceProductID, 8);
  });

  it('G. [] response -> block hidden', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [],
      totalCount: 0,
    });

    renderBlock();

    await waitFor(() => {
      expect(publicCatalog.fetchSimilarProducts).toHaveBeenCalled();
    });

    expect(screen.queryByText('Похожие товары')).toBeNull();
    expect(screen.queryByTestId('similar-products-block')).toBeNull();
  });

  it('H. API failure -> PDP remains usable, block hidden', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockRejectedValue(new Error('Network error'));

    renderBlock();

    await waitFor(() => {
      expect(publicCatalog.fetchSimilarProducts).toHaveBeenCalled();
    });

    expect(screen.queryByText('Похожие товары')).toBeNull();
    expect(screen.queryByTestId('similar-products-block')).toBeNull();
  });

  it('J. canonical ProductCard is reused in grid', async () => {
    vi.mocked(publicCatalog.fetchSimilarProducts).mockResolvedValue({
      items: [mockCandidate1],
      totalCount: 1,
    });

    renderBlock();

    await waitFor(() => {
      expect(screen.getByText('Candidate Jacket 1')).toBeTruthy();
    });

    // ProductCard renders seller and price
    expect(screen.getByText('Seller 1')).toBeTruthy();
    expect(screen.getByText(/1\s*500/)).toBeTruthy();
  });
});
