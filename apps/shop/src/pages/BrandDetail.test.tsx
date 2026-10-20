/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { BrandDetail } from './BrandDetail';

const fetchPublicBrand = vi.fn();
vi.mock('../api/publicCatalog', () => ({
  fetchPublicBrand: (idOrSlug: string) => fetchPublicBrand(idOrSlug),
}));

vi.mock('../components/product/ProductCard', () => ({
  ProductCard: ({ product }: { product: { id: string; name: string; brand: string; price: number } }) => (
    <div data-testid={`product-card-${product.id}`}>
      <a href={`/product/${product.id}`}>{product.name}</a>
      <span>{product.price}</span>
    </div>
  ),
}));

const renderBrandDetail = (slugOrId = 'raf-simons') =>
  render(
    <MemoryRouter initialEntries={[`/brand/${slugOrId}`]}>
      <Routes>
        <Route path='/brand/:slugOrId' element={<BrandDetail />} />
      </Routes>
    </MemoryRouter>,
  );

describe('BrandDetail storefront component (MARKETING.M1.8)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Brand with multiple products: displays brand identity and product cards', async () => {
    fetchPublicBrand.mockResolvedValue({
      brand: {
        id: 'b-1',
        name: 'Undercover Archive',
        slug: 'undercover-archive',
        description: 'Iconic silhouettes and runway specimens.',
        logoUrl: 'https://cdn.zamk.test/undercover-logo.png',
        coverUrl: 'https://cdn.zamk.test/undercover-cover.png',
      },
      items: [
        { id: 'p-1', name: 'SS03 Scab Jacket', brand: 'Undercover Archive', price: 120000 },
        { id: 'p-2', name: 'AW02 Witches Hoodie', brand: 'Undercover Archive', price: 95000 },
      ],
      totalCount: 2,
    });

    renderBrandDetail('undercover-archive');

    expect(await screen.findByRole('heading', { level: 1, name: /Undercover Archive/i })).toBeDefined();
    expect(screen.getByText('Iconic silhouettes and runway specimens.')).toBeDefined();
    expect(screen.getByAltText('Undercover Archive logo')).toBeDefined();
    expect(screen.getByAltText('Undercover Archive cover')).toBeDefined();
    expect(screen.getByText('2 вещи')).toBeDefined();

    expect(screen.getByTestId('product-card-p-1')).toBeDefined();
    expect(screen.getByTestId('product-card-p-2')).toBeDefined();
  });

  it('2. Brand with 1 product: displays 1 вещь and single product card', async () => {
    fetchPublicBrand.mockResolvedValue({
      brand: {
        id: 'b-2',
        name: 'Solo Archive',
        slug: 'solo-archive',
      },
      items: [
        { id: 'p-single', name: '1998 Trench Coat', brand: 'Solo Archive', price: 68000 },
      ],
      totalCount: 1,
    });

    renderBrandDetail('solo-archive');

    expect(await screen.findByRole('heading', { level: 1, name: /Solo Archive/i })).toBeDefined();
    expect(screen.getByText('1 вещь')).toBeDefined();
    expect(screen.getByTestId('product-card-p-single')).toBeDefined();
  });

  it('3. Brand without products: shows elegant empty state with archive explanation', async () => {
    fetchPublicBrand.mockResolvedValue({
      brand: {
        id: 'b-3',
        name: 'Future Drop Lab',
        slug: 'future-drop-lab',
      },
      items: [],
      totalCount: 0,
    });

    renderBrandDetail('future-drop-lab');

    expect(await screen.findByRole('heading', { level: 1, name: /Future Drop Lab/i })).toBeDefined();
    expect(screen.getByText('0 вещей')).toBeDefined();
    expect(screen.getByText(/В архиве бренда пока нет доступных вещей/i)).toBeDefined();
    expect(screen.getByRole('link', { name: /В каталог/i })).toBeDefined();
  });

  it('4. Brand without logo, description, cover: renders cleanly without placeholder text', async () => {
    fetchPublicBrand.mockResolvedValue({
      brand: {
        id: 'b-minimal',
        name: 'Minimalist Studio',
        slug: 'minimalist-studio',
      },
      items: [],
      totalCount: 0,
    });

    renderBrandDetail('minimalist-studio');

    expect(await screen.findByRole('heading', { level: 1, name: /Minimalist Studio/i })).toBeDefined();
    // Verify no ugly "Нет лого" or "Описание отсутствует" strings
    expect(screen.queryByText(/нет лого/i)).toBeNull();
    expect(screen.queryByText(/описание отсутствует/i)).toBeNull();
    expect(screen.queryByText(/нет данных/i)).toBeNull();
  });

  it('5. Unknown brand: renders 404 state with navigation back', async () => {
    fetchPublicBrand.mockResolvedValue(null);

    renderBrandDetail('nonexistent-brand');

    expect(await screen.findByRole('heading', { level: 1, name: /Бренд не найден/i })).toBeDefined();
    expect(screen.getByRole('link', { name: /К брендам/i })).toBeDefined();
  });

  it('6. Server error: renders friendly error message', async () => {
    fetchPublicBrand.mockRejectedValue(new Error('Network error'));

    renderBrandDetail('error-brand');

    expect(await screen.findByRole('heading', { level: 1, name: /Ошибка загрузки/i })).toBeDefined();
    expect(screen.getByRole('link', { name: /К брендам/i })).toBeDefined();
  });
});
