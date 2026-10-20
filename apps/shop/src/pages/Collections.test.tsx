/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { Collections } from './Collections';
import { CollectionDetail } from './CollectionDetail';

const fetchCollections = vi.fn();
const fetchCollectionBySlug = vi.fn();
vi.mock('../api/publicCatalog', () => ({
  fetchCollections: () => fetchCollections(),
  fetchCollectionBySlug: (slug: string) => fetchCollectionBySlug(slug),
}));
vi.mock('../components/product/ProductCard', () => ({
  ProductCard: ({ product }: { product: { id: string; name: string; brand: string; price: number } }) => (
    <a href={`/product/${product.id}`} data-testid='card'>
      {product.name} · {product.brand} · {product.price}
    </a>
  ),
}));

const renderList = () => render(<MemoryRouter><Collections /></MemoryRouter>);
const renderDetail = (slug = 'summer') =>
  render(
    <MemoryRouter initialEntries={[`/collections/${slug}`]}>
      <Routes>
        <Route path='/collections/:slug' element={<CollectionDetail />} />
      </Routes>
    </MemoryRouter>,
  );

describe('Shop editorial collections', () => {
  beforeEach(() => {
    fetchCollections.mockReset();
    fetchCollectionBySlug.mockReset();
  });
  afterEach(() => cleanup());

  it('list: shows published collections as links', async () => {
    fetchCollections.mockResolvedValue([{ id: '1', slug: 'summer', title: 'Лето', description: 'Лёгкие вещи' }]);
    renderList();
    const link = await screen.findByRole('link', { name: /Лето/ });
    expect(link.getAttribute('href')).toBe('/collections/summer');
    expect(screen.getByText('Лёгкие вещи')).toBeDefined();
  });

  it('list: empty state without fake collections', async () => {
    fetchCollections.mockResolvedValue([]);
    renderList();
    expect(await screen.findByText('Подборок пока нет')).toBeDefined();
    expect(screen.queryByText('Смотреть подборку')).toBeNull();
  });

  it('list: API error', async () => {
    fetchCollections.mockRejectedValue(new Error('down'));
    renderList();
    expect((await screen.findByRole('alert')).textContent).toContain('Не удалось загрузить');
  });

  it('detail: title, description and product cards with brand and price', async () => {
    fetchCollectionBySlug.mockResolvedValue({
      id: '1', slug: 'summer', title: 'Лето', description: 'Лёгкие вещи',
      products: [
        { id: 'p1', name: 'Платье', brand: 'Brand A', price: 5000 },
        { id: 'p2', name: 'Рубашка', brand: 'Brand B', price: 3000 },
      ],
    });
    renderDetail();
    expect((await screen.findByRole('heading', { name: 'Лето' })).tagName).toBe('H1');
    expect(screen.getByText('Лёгкие вещи')).toBeDefined();
    const cards = screen.getAllByTestId('card');
    expect(cards.map((c) => c.getAttribute('href'))).toEqual(['/product/p1', '/product/p2']);
    expect(cards[0].textContent).toContain('Brand A');
    expect(fetchCollectionBySlug).toHaveBeenCalledWith('summer');
  });

  it('detail: collection without available products shows the empty state', async () => {
    fetchCollectionBySlug.mockResolvedValue({ id: '1', slug: 'summer', title: 'Лето', products: [] });
    renderDetail();
    expect(await screen.findByText('В подборке пока нет доступных вещей')).toBeDefined();
    expect(screen.queryByTestId('card')).toBeNull();
  });

  it('detail: unpublished or unknown collection is "not found"', async () => {
    fetchCollectionBySlug.mockResolvedValue(null);
    renderDetail('draft-one');
    expect(await screen.findByText('Подборка не найдена')).toBeDefined();
  });

  it('detail: network error', async () => {
    fetchCollectionBySlug.mockRejectedValue(new Error('down'));
    renderDetail();
    expect((await screen.findByRole('alert')).textContent).toContain('Не удалось загрузить');
  });
});
