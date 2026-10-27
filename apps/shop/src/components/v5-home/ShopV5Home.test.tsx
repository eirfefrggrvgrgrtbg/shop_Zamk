/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ShopV5Home } from './ShopV5Home';
import * as api from '../../api/publicCatalog';
import * as authContext from '../../contexts/AuthContext';
import type { Brand, Product } from '../../types/catalog';

const toggleFavorite = vi.fn();
const addItem = vi.fn();

vi.mock('../../api/publicCatalog');
vi.mock('../../contexts/AuthContext', () => ({ useAuth: vi.fn() }));
vi.mock('../../contexts/CartContext', () => ({ useCart: () => ({ totalItems: 0, addItem }) }));
vi.mock('../../contexts/FavoritesContext', () => ({ useFavorites: () => ({ favoriteIds: [], isFavorite: () => false, toggleFavorite }) }));
vi.mock('../../contexts/SearchContext', () => ({ useSearch: () => ({ openSearch: vi.fn() }) }));
vi.mock('../../lib/behavior', () => ({ useCatalogImpression: () => ({ current: null }) }));

const prod = (id: string): Product => ({ id, name: `Товар ${id}`, brand: 'Бренд', brandId: 'b1', price: 1000, image: `https://img.test/${id}.jpg`, category: 'c1' });
const brand = (id: string): Brand => ({ id, name: `Бренд ${id}`, slug: `slug-${id}`, description: '', country: '', image: '' });

function setup(auth: { isAuthenticated: boolean; role?: string }) {
  vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: auth.isAuthenticated, user: auth.isAuthenticated ? { role: auth.role ?? 'customer' } : null, openAuthModal: vi.fn() } as unknown as ReturnType<typeof authContext.useAuth>);
  return render(<MemoryRouter><ShopV5Home /></MemoryRouter>);
}

beforeEach(() => {
  vi.mocked(api.fetchProducts).mockResolvedValue({ items: [prod('1'), prod('2'), prod('3'), prod('4')], totalCount: 4 });
  vi.mocked(api.fetchCategories).mockResolvedValue([{ id: 'c1', slug: 'c', name: 'Верхняя одежда', icon: '' }]);
  vi.mocked(api.fetchBrands).mockResolvedValue([brand('a'), brand('b')]);
  vi.mocked(api.fetchPopularProducts).mockResolvedValue({ items: [prod('5'), prod('6')], totalCount: 2 });
  vi.mocked(api.fetchHomeRecommendations).mockResolvedValue({ blocks: [{ type: 'for_you', title: 't', items: [prod('7')] }] });
});
afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe('ShopV5Home', () => {
  it('guest: three real arrivals + catalog CTA, honest «Популярное», real links', async () => {
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-card-1')).toBeTruthy());
    expect(screen.queryByTestId('v5-card-4')).toBeNull(); // only 3 in arrivals; the 4th goes to the selection pool
    expect(screen.getByTestId('v5-arrivals-catalog-cta').getAttribute('href')).toBe('/catalog');
    await waitFor(() => expect(screen.getByTestId('v5-for-you').getAttribute('data-kind')).toBe('popular'));
    expect(screen.getByRole('heading', { name: 'Популярное' })).toBeTruthy();
    expect(api.fetchHomeRecommendations).not.toHaveBeenCalled();
    expect(screen.getByTestId('v5-card-1').querySelector('a')?.getAttribute('href')).toBe('/product/1');
    expect(screen.getByTestId('v5-brand-a').getAttribute('href')).toBe('/brand/slug-a');
  });

  it('customer: uses backend recommendations as «Для вас»', async () => {
    setup({ isAuthenticated: true });
    await waitFor(() => expect(screen.getByTestId('v5-for-you').getAttribute('data-kind')).toBe('for_you'));
    expect(api.fetchHomeRecommendations).toHaveBeenCalled();
    expect(api.fetchPopularProducts).not.toHaveBeenCalled();
    expect(screen.getByTestId('v5-card-7')).toBeTruthy();
  });

  it('favorite uses the shared favorites context; no direct add-to-cart exists', async () => {
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-card-1')).toBeTruthy());
    fireEvent.click(screen.getAllByRole('button', { name: 'Добавить в избранное' })[0]);
    expect(toggleFavorite).toHaveBeenCalledWith('1');
    expect(addItem).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /корзин/i })).toBeNull();
  });

  it('empty storefront: empty states, no invented brands, no for-you section', async () => {
    vi.mocked(api.fetchProducts).mockResolvedValue({ items: [], totalCount: 0 });
    vi.mocked(api.fetchBrands).mockResolvedValue([]);
    vi.mocked(api.fetchPopularProducts).mockResolvedValue({ items: [], totalCount: 0 });
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-arrivals-empty')).toBeTruthy());
    expect(screen.getByTestId('v5-brands-empty')).toBeTruthy();
    expect(screen.queryByTestId('v5-for-you')).toBeNull();
  });

  it('errors are shown with retry', async () => {
    vi.mocked(api.fetchProducts).mockRejectedValueOnce(new Error('x'));
    vi.mocked(api.fetchBrands).mockRejectedValueOnce(new Error('x'));
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-arrivals-error')).toBeTruthy());
    expect(screen.getByTestId('v5-brands-error')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Повторить' })[0]);
    await waitFor(() => expect(screen.getByTestId('v5-card-1')).toBeTruthy());
  });

  it('for-you rail marks the first card as lead and reports the real item count', async () => {
    vi.mocked(api.fetchPopularProducts).mockResolvedValue({ items: [prod('5'), prod('6'), prod('8')], totalCount: 3 });
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-for-you-rail')).toBeTruthy());
    const rail = screen.getByTestId('v5-for-you-rail');
    expect(rail.getAttribute('data-count')).toBe('3');
    expect(rail.querySelectorAll('.v5-card--lead')).toHaveLength(1);
    expect(rail.firstElementChild?.classList.contains('v5-card--lead')).toBe(true);
  });

  it('for-you handles a single item', async () => {
    vi.mocked(api.fetchPopularProducts).mockResolvedValue({ items: [prod('5')], totalCount: 1 });
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-for-you-rail').getAttribute('data-count')).toBe('1'));
  });

  it('brands: lead brand follows the newest arrival, others are a compact list with real links', async () => {
    vi.mocked(api.fetchProducts).mockResolvedValue({ items: [{ ...prod('1'), brandId: 'b' }, prod('2'), prod('3')], totalCount: 3 });
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-brands-index')).toBeTruthy());
    const index = screen.getByTestId('v5-brands-index');
    expect(index.querySelector('.v5-bx__lead')?.getAttribute('href')).toBe('/brand/slug-b');
    expect(screen.getByTestId('v5-brand-a').getAttribute('href')).toBe('/brand/slug-a');
  });

  it('many brands: limited selection plus a link to all brands', async () => {
    vi.mocked(api.fetchBrands).mockResolvedValue(Array.from({ length: 10 }, (_, i) => brand(`x${i}`)));
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-brands-more')).toBeTruthy());
    expect(screen.getByTestId('v5-brands-more').getAttribute('href')).toBe('/brands');
    expect(document.querySelectorAll('[data-testid^="v5-brand-x"]')).toHaveLength(7);
  });

  it('a single brand renders without filler brands', async () => {
    vi.mocked(api.fetchBrands).mockResolvedValue([brand('a')]);
    setup({ isAuthenticated: false });
    await waitFor(() => expect(screen.getByTestId('v5-brand-a')).toBeTruthy());
    expect(document.querySelectorAll('[data-testid^="v5-brand-"]')).toHaveLength(1);
  });
});
