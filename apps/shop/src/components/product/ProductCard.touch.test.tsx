/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { ProductCard } from './ProductCard';
import * as publicCatalog from '../../api/publicCatalog';
import type { Product } from '../../types/catalog';

class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(window as unknown as { IntersectionObserver: unknown }).IntersectionObserver = IntersectionObserverMock;

const auth = { user: null as null | { id: string }, openAuthModal: vi.fn() };
const favorites = { isFavorite: vi.fn(() => false), toggleFavorite: vi.fn() };
const toast = { showToast: vi.fn() };
const cart = { addItem: vi.fn() };

vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => auth }));
vi.mock('../../contexts/FavoritesContext', () => ({ useFavorites: () => favorites }));
vi.mock('../../contexts/ToastContext', () => ({ useToast: () => toast }));
vi.mock('../../contexts/CartContext', () => ({ useCart: () => cart }));
vi.mock('../../api/publicCatalog', () => ({ fetchProductById: vi.fn() }));

const product: Product = {
  id: 'prod-1',
  name: 'Alpha Coat',
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

function Where() {
  const l = useLocation();
  return <div data-testid="where">{l.pathname}</div>;
}

const renderCard = () =>
  render(
    <MemoryRouter initialEntries={['/catalog']}>
      <ProductCard product={product} />
      <Where />
    </MemoryRouter>
  );

describe('Catalog ProductCard: touch actions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    auth.user = null;
    favorites.isFavorite.mockReturnValue(false);
  });
  afterEach(cleanup);

  it('keeps both actions visible on touch/no-hover devices and reveals them on hover and keyboard focus', () => {
    renderCard();
    for (const name of ['Добавить в избранное', 'Быстрая покупка']) {
      const cls = screen.getByRole('button', { name }).className;
      // hidden by default, revealed by hover, focus (keyboard) and — always — on coarse/no-hover pointers
      expect(cls).toContain('opacity-0');
      expect(cls).toContain('group-hover:opacity-100');
      expect(cls).toContain('focus-visible:opacity-100');
      expect(cls).toContain('group-focus-within:opacity-100');
      expect(cls).toContain('pointer-coarse:opacity-100');
      expect(cls).toContain('[@media(hover:none)]:opacity-100');
      // comfortable touch target
      expect(cls).toContain('pointer-coarse:w-11');
      expect(cls).toContain('pointer-coarse:h-11');
    }
  });

  it('actions are real, tabbable buttons that are not nested inside the PDP link', () => {
    renderCard();
    for (const name of ['Добавить в избранное', 'Быстрая покупка']) {
      const b = screen.getByRole('button', { name });
      expect(b.tagName).toBe('BUTTON');
      expect(b.tabIndex).toBeGreaterThanOrEqual(0);
      expect(b.closest('a')).toBeNull();
      b.focus();
      expect(document.activeElement).toBe(b);
    }
  });

  it('guest favorite: opens the existing login flow, does not toggle, does not announce success, does not open the PDP', () => {
    renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Добавить в избранное' }));
    expect(auth.openAuthModal).toHaveBeenCalledWith('login');
    expect(favorites.toggleFavorite).not.toHaveBeenCalled();
    expect(toast.showToast).toHaveBeenCalledTimes(1);
    expect(toast.showToast).toHaveBeenCalledWith('Войдите, чтобы добавить товар в избранное.', 'error');
    expect(screen.getByTestId('where').textContent).toBe('/catalog');
  });

  it('authenticated favorite: toggles through FavoritesContext and stays on the catalog', () => {
    auth.user = { id: 'u1' };
    renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Добавить в избранное' }));
    expect(favorites.toggleFavorite).toHaveBeenCalledWith('prod-1');
    expect(auth.openAuthModal).not.toHaveBeenCalled();
    expect(screen.getByTestId('where').textContent).toBe('/catalog');
  });

  it('favorite button reflects the current state (aria-pressed + label)', () => {
    auth.user = { id: 'u1' };
    favorites.isFavorite.mockReturnValue(true);
    renderCard();
    const b = screen.getByRole('button', { name: 'Убрать из избранного' });
    expect(b.getAttribute('aria-pressed')).toBe('true');
  });

  it('quick buy: loads the real product and opens the existing variant-selection modal; nothing is added to the cart', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValue({
      ...product,
      variants: [
        { id: 'v-s', size: 'S', inStock: true, isActive: true, sizeValueId: 'sv-s' },
        { id: 'v-m', size: 'M', inStock: true, isActive: true, sizeValueId: 'sv-m' },
      ],
    } as Product);
    renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Быстрая покупка' }));
    expect(publicCatalog.fetchProductById).toHaveBeenCalledWith('prod-1');
    expect(await screen.findByText('Быстрая покупка', { selector: 'h2, h3, div, span' })).toBeTruthy();
    await waitFor(() =>
      expect(screen.getAllByRole('button').filter((b) => b.hasAttribute('data-state')).length).toBe(2)
    );
    // the CTA stays disabled until a size is chosen: no guessed variant
    const cta = screen.getByRole('button', { name: /выберите|в корзину/i });
    expect((cta as HTMLButtonElement).disabled).toBe(true);
    expect(cart.addItem).not.toHaveBeenCalled();
    expect(screen.getByTestId('where').textContent).toBe('/catalog');
  });

  it('quick buy failure shows the existing error toast and no false success', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockRejectedValue(new Error('boom'));
    renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Быстрая покупка' }));
    await waitFor(() => expect(toast.showToast).toHaveBeenCalledWith('Ошибка загрузки вариантов товара', 'error'));
    expect(cart.addItem).not.toHaveBeenCalled();
  });

  it('preserves the paper-tag card: receipt tag, tilt, fields, barcode and article', () => {
    const { container } = renderCard();
    const tag = container.querySelector('.font-mono.rotate-1') as HTMLElement;
    expect(tag).toBeTruthy();
    expect(tag.className).toContain('group-hover:rotate-0');
    expect(tag.className).toContain('bg-[#f4f4f4]');
    expect(tag.textContent).toContain('| ЧЕК');
    expect(tag.textContent).toMatch(/Товар:\s*Alpha Coat/);
    expect(tag.textContent).toMatch(/Цена:/);
    expect(tag.textContent).toContain('Нет отзывов');
    expect(tag.textContent).toContain('Артикул');
    // polaroid frame with rounded photo and the PDP link
    expect(container.querySelector('.rounded-2xl.p-3.pb-8')).toBeTruthy();
    expect(container.querySelector('a[href="/product/prod-1"] img')).toBeTruthy();
  });
});
