/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { FavoriteBrands } from './FavoriteBrands';

const ctx = vi.hoisted(() => ({
  value: {} as Record<string, unknown>,
}));
vi.mock('../contexts/FavoriteBrandsContext', () => ({ useFavoriteBrands: () => ctx.value }));
vi.mock('../components/account/AccountLayout', () => ({
  AccountLayout: ({ children, title }: { children: React.ReactNode; title: string }) => (
    <div><h1>{title}</h1>{children}</div>
  ),
}));

const fetchFeed = vi.hoisted(() => vi.fn());
vi.mock('../api/publicCatalog', () => ({ fetchFavoriteBrandProducts: fetchFeed }));
vi.mock('../components/product/ProductCard', () => ({
  ProductCard: ({ product }: { product: { id: string; name: string; brand: string; brandId: string } }) => (
    <div data-testid={`feed-card-${product.id}`}>
      <a href={`/product/${product.id}`}>{product.name}</a>
      <a href={`/brand/${product.brandId}`}>{product.brand}</a>
    </div>
  ),
}));

const toggle = vi.fn();
const reload = vi.fn();
const base = { isLoading: false, error: null, pendingIds: [], toggleFavoriteBrand: toggle, reloadFavoriteBrands: reload };
const brand = { id: 'b-1', name: 'Acne Studios', slug: 'acne-studios', description: 'Stockholm', addedAt: '2026-01-01T00:00:00Z' };

const renderPage = () => render(<MemoryRouter><FavoriteBrands /></MemoryRouter>);

describe('FavoriteBrands account page (MARKETING.M1.11)', () => {
  beforeEach(() => { vi.clearAllMocks(); toggle.mockResolvedValue(true); fetchFeed.mockResolvedValue({ items: [], totalCount: 0 }); });
  afterEach(cleanup);

  it('lists saved brands with a storefront link', () => {
    ctx.value = { ...base, brands: [brand] };
    renderPage();
    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.getByRole('link', { name: /Acne Studios/ }).getAttribute('href')).toBe('/brand/acne-studios');
  });

  it('removes a brand through the context', async () => {
    ctx.value = { ...base, brands: [brand] };
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /Удалить Acne Studios/ }));
    await waitFor(() => expect(toggle).toHaveBeenCalledWith('b-1'));
  });

  it('shows an alert if removal fails', async () => {
    toggle.mockResolvedValue(false);
    ctx.value = { ...base, brands: [brand] };
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /Удалить Acne Studios/ }));
    expect((await screen.findByRole('alert')).textContent).toMatch(/Не удалось удалить/);
  });

  it('renders an empty state with a link to brands', () => {
    ctx.value = { ...base, brands: [] };
    renderPage();
    expect(screen.getByText('Любимых брендов пока нет')).toBeDefined();
    expect(screen.getByRole('link', { name: /Смотреть бренды/ }).getAttribute('href')).toBe('/brands');
  });

  it('renders a server error state with retry', () => {
    ctx.value = { ...base, brands: [], error: 'Не удалось загрузить любимые бренды.' };
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    expect(reload).toHaveBeenCalled();
  });

  it('does not render internal seller data', () => {
    ctx.value = { ...base, brands: [{ ...brand, sellerId: 'secret-seller', contactEmail: 'x@y.z' }] };
    const { container } = renderPage();
    expect(container.textContent).not.toMatch(/secret-seller|x@y\.z/);
  });

  describe('feed "Вещи любимых брендов"', () => {
    const prod = (n: number) => ({ id: `p-${n}`, name: `Item ${n}`, brand: 'Acne Studios', brandId: 'b-1' });

    it('renders the feed with product and brand links', async () => {
      ctx.value = { ...base, brands: [brand] };
      fetchFeed.mockResolvedValue({ items: [prod(1), prod(2)], totalCount: 2 });
      renderPage();
      expect(await screen.findByTestId('feed-card-p-1')).toBeDefined();
      expect(screen.getByRole('heading', { name: 'Вещи любимых брендов' })).toBeDefined();
      expect(screen.getByRole('link', { name: 'Item 1' }).getAttribute('href')).toBe('/product/p-1');
      expect(screen.queryByRole('button', { name: 'Показать ещё' })).toBeNull();
    });

    it('shows a neutral empty state when no items are available', async () => {
      ctx.value = { ...base, brands: [brand] };
      renderPage();
      expect(await screen.findByText(/нет доступных вещей/)).toBeDefined();
    });

    it('shows error with retry', async () => {
      ctx.value = { ...base, brands: [brand] };
      fetchFeed.mockRejectedValueOnce(new Error('x')).mockResolvedValueOnce({ items: [prod(1)], totalCount: 1 });
      renderPage();
      expect((await screen.findAllByRole('alert'))[0].textContent).toMatch(/Не удалось загрузить вещи/);
      fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
      expect(await screen.findByTestId('feed-card-p-1')).toBeDefined();
    });

    it('"Показать ещё" appends the next page without duplicates', async () => {
      ctx.value = { ...base, brands: [brand] };
      fetchFeed
        .mockResolvedValueOnce({ items: [prod(1), prod(2)], totalCount: 3 })
        .mockResolvedValueOnce({ items: [prod(2), prod(3)], totalCount: 3 });
      renderPage();
      fireEvent.click(await screen.findByRole('button', { name: 'Показать ещё' }));
      expect(await screen.findByTestId('feed-card-p-3')).toBeDefined();
      expect(screen.getAllByTestId(/feed-card-/)).toHaveLength(3);
      expect(fetchFeed).toHaveBeenLastCalledWith(12, 2);
      expect(screen.queryByRole('button', { name: 'Показать ещё' })).toBeNull();
    });

    it('refetches the feed when the set of favorite brands changes', async () => {
      const brand2 = { ...brand, id: 'b-2', name: 'Other', slug: 'other' };
      ctx.value = { ...base, brands: [brand, brand2] };
      fetchFeed.mockResolvedValue({ items: [prod(1), { ...prod(9), brandId: 'b-2' }], totalCount: 2 });
      const { rerender } = renderPage();
      expect(await screen.findByTestId('feed-card-p-9')).toBeDefined();
      ctx.value = { ...base, brands: [brand] };
      fetchFeed.mockResolvedValue({ items: [prod(1)], totalCount: 1 });
      rerender(<MemoryRouter><FavoriteBrands /></MemoryRouter>);
      await waitFor(() => expect(screen.queryByTestId('feed-card-p-9')).toBeNull());
      expect(fetchFeed).toHaveBeenCalledTimes(2);
    });
  });
});
