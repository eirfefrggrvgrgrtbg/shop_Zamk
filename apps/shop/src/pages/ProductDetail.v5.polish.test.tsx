/**
 * @vitest-environment jsdom
 */
/// <reference types="node" />
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ProductDetail } from './ProductDetail';
import { ShopV5ProductDetail } from '../components/product-detail/ShopV5ProductDetail';
import * as publicCatalog from '../api/publicCatalog';
import type { Product } from '../types/catalog';

vi.mock('../contexts/CartContext', () => ({ useCart: () => ({ addItem: vi.fn(), items: [], totalItems: 0 }) }));
vi.mock('../contexts/FavoritesContext', () => ({ useFavorites: () => ({ isFavorite: () => false, toggleFavorite: vi.fn() }) }));
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 'u1' }, isAuthenticated: true, openAuthModal: vi.fn(), isInitializing: false }),
}));
vi.mock('../contexts/ToastContext', () => ({ useToast: () => ({ showToast: vi.fn() }) }));
vi.mock('@zamk/api-client/src/customer', () => ({ recordProductView: vi.fn().mockResolvedValue({ status: 'ok' }) }));
vi.mock('../components/product/SimilarProductsBlock', () => ({ SimilarProductsBlock: () => <div /> }));
vi.mock('../api/publicCatalog', () => ({
  fetchProductById: vi.fn(),
  fetchProductReviews: vi.fn().mockResolvedValue([]),
  fetchProductPreviewByToken: vi.fn(),
}));

// vitest does not expose stylesheet text through ?raw, so the CSS contract is read from disk
const cssPath = ['src', 'apps/shop/src']
  .map((base) => join(process.cwd(), base, 'components/product-detail/v5-pdp.css'))
  .find((candidate) => existsSync(candidate));
const v5Css = readFileSync(cssPath as string, 'utf8');

const SHORT = 'Пальто';
const LONG = 'Пальто оверсайз из плотной итальянской шерсти с запахом, съёмным поясом и асимметричной застёжкой';

const v = (id: string, size: string, colorId: string, colorName: string, sku: string) => ({
  id, size, sizeValueId: `sv-${size}`, colorId, colorName, inStock: true, isActive: true, priceCents: 1000000, sellerSku: sku,
});

function product(name = SHORT): Product {
  return {
    id: '11111111-2222-3333-4444-abcdef012345',
    name, brand: 'ZAMK', brandId: 'b-1', price: 10000, image: 'a.jpg', images: [{ url: 'a.jpg' }, { url: 'b.jpg' }], category: 'Пальто',
    variants: [
      v('v-r-s', 'S', 'c-red', 'Красный', 'sku-red-s'),
      v('v-r-m', 'M', 'c-red', 'Красный', 'sku-red-m'),
      v('v-b-s', 'S', 'c-blue', 'Синий', 'sku-blue-s'),
      v('v-b-m', 'M', 'c-blue', 'Синий', 'sku-blue-m'),
    ],
  } as Product;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/product/:id" element={<ProductDetail />} />
        <Route path="/v5-pdp-preview/:id" element={<ShopV5ProductDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

async function openV5(p: Product) {
  vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(p);
  const r = renderAt('/v5-pdp-preview/x');
  await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: p.name })).toBeTruthy());
  return r;
}

describe('SHOP.V5.PDP.VISUAL.POLISH.1', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(cleanup);

  describe('mobile top: title and breadcrumbs', () => {
    it('short title: brand, full title, price and rating stay in the purchase header', async () => {
      const { container } = await openV5({ ...product(SHORT), rating: 4.5, reviewsCount: 3 } as Product);
      expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(SHORT);
      expect(container.querySelector('.zv5-brand')?.textContent).toBe('ZAMK');
      expect(container.querySelector('.zv5-price')?.textContent).toMatch(/10\s?000/);
      expect(container.querySelector('.zv5-rating')?.textContent).toContain('4,5');
    });

    it('long title: the whole name is rendered as the heading, never truncated by markup', async () => {
      await openV5(product(LONG));
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1.textContent).toBe(LONG);
      expect(h1.className).not.toMatch(/line-clamp|truncate/);
    });

    it('mobile CSS compacts the title (<= 34px, serif kept, wraps long words) inside the <960px block', () => {
      const mobile = v5Css.slice(v5Css.indexOf('@media (max-width: 959px)'));
      expect(mobile).toMatch(/\.zv5-title \{[^}]*font-size: clamp\(26px, 7vw, 34px\)/);
      expect(mobile).toMatch(/\.zv5-title \{[^}]*hyphens: auto/);
      expect(v5Css).toMatch(/\.zv5-title \{[^}]*font-family: var\(--font-serif\)/);
      expect(v5Css).toMatch(/\.zv5-title \{[^}]*overflow-wrap: anywhere/);
    });

    it('breadcrumb: the duplicate current-title is visually hidden on mobile but stays for screen readers', async () => {
      const { container } = await openV5(product(LONG));
      const current = container.querySelector('.zv5-crumb-current') as HTMLElement;
      expect(current.textContent).toBe(LONG);
      expect(current.getAttribute('aria-current')).toBe('page');
      expect(v5Css).toMatch(/@media \(max-width: 959px\)[\s\S]*\.zv5-crumb-current \{[^}]*clip: rect\(0 0 0 0\)/);
      // on desktop (>=960px) there is no hiding rule: the approved composition keeps the visible name
      const desktopCss = v5Css.slice(0, v5Css.indexOf('@media (max-width: 959px)'));
      expect(desktopCss).not.toMatch(/\.zv5-crumb-current/);
    });

    it('breadcrumb navigation to home and catalog is kept; share keeps its accessible name', async () => {
      await openV5(product(SHORT));
      const crumbs = screen.getByRole('navigation', { name: 'Навигационная цепочка' });
      expect(within(crumbs).getByRole('link', { name: 'Главная' }).getAttribute('href')).toBe('/');
      expect(within(crumbs).getByRole('link', { name: 'Каталог' }).getAttribute('href')).toBe('/catalog');
      expect(screen.getByRole('button', { name: 'Поделиться товаром' })).toBeTruthy();
      expect(v5Css).toMatch(/button\[aria-label='Поделиться товаром'\] \{[^}]*min-height: 44px/);
    });

    it('production PDP keeps the plain breadcrumb markup', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(product(SHORT));
      const { container } = renderAt('/product/x');
      await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: SHORT })).toBeTruthy());
      expect(container.innerHTML).not.toContain('zv5-');
      expect(container.querySelector('nav[aria-label="Навигационная цепочка"]')).toBeNull();
    });
  });

  describe('passport article labelling', () => {
    it('before a variant is resolved: "Код товара" only, no SKU', async () => {
      await openV5(product());
      const passport = screen.getByTestId('v5-passport');
      expect(within(passport).getByTestId('v5-passport-Код товара').textContent).toContain('012345');
      expect(passport.textContent).not.toContain('SKU-');
      expect(within(passport).queryByTestId('v5-passport-Артикул варианта')).toBeNull();
    });

    it('after choosing colour and size: "Артикул варианта" of exactly that variant; changing colour or size never leaves a stale SKU', async () => {
      await openV5(product());
      fireEvent.click(screen.getByTestId('size-button-S'));
      // only the size is chosen: the variant is still ambiguous, so no SKU yet
      expect(within(screen.getByTestId('v5-passport')).queryByTestId('v5-passport-Артикул варианта')).toBeNull();
      fireEvent.click(screen.getByTestId('color-swatch-c-red'));
      const sku = () => within(screen.getByTestId('v5-passport')).getByTestId('v5-passport-Артикул варианта').textContent ?? '';
      expect(sku()).toContain('SKU-RED-S');
      fireEvent.click(screen.getByTestId('color-swatch-c-blue'));
      expect(sku()).toContain('SKU-BLUE-S');
      expect(sku()).not.toContain('RED');
      fireEvent.click(screen.getByTestId('size-button-M'));
      expect(sku()).toContain('SKU-BLUE-M');
      expect(within(screen.getByTestId('v5-passport')).queryByTestId('v5-passport-Код товара')).toBeNull();
    });
  });

  describe('loading', () => {
    it('V5 shows a skeleton layout (gallery, title/price, purchase) with no fake data and a status announcement', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockReturnValueOnce(new Promise(() => {}));
      const { container } = renderAt('/v5-pdp-preview/x');
      const sk = screen.getByTestId('v5-pdp-loading');
      expect(sk.getAttribute('role')).toBe('status');
      expect(sk.getAttribute('aria-busy')).toBe('true');
      expect(within(sk).getByText('Загрузка товара')).toBeTruthy();
      for (const part of ['.zv5-skel-head', '.zv5-skel-gallery', '.zv5-skel-buy', '.zv5-skel-stage']) {
        expect(container.querySelector(part), part).toBeTruthy();
      }
      expect(sk.textContent).not.toMatch(/₽|\bS\b|\bM\b|ZAMK|Размер|Цвет/);
      expect(container.querySelector('.animate-spin')).toBeNull();
    });

    it('skeleton uses the page grid and respects prefers-reduced-motion', () => {
      expect(v5Css).toMatch(/\.zv5-skel-grid \{[^}]*grid-template-columns: minmax\(0, 1fr\)/);
      expect(v5Css).toMatch(/grid-template-columns: minmax\(0, 1\.2fr\) minmax\(0, 1fr\)/);
      expect(v5Css).toMatch(/@media \(prefers-reduced-motion: reduce\) \{ \.zv5-skel \{ animation: none; \} \}/);
      expect(v5Css).not.toMatch(/backdrop-filter|blur\(/);
    });

    it('standard production loading is unchanged: spinner, no skeleton', () => {
      vi.mocked(publicCatalog.fetchProductById).mockReturnValueOnce(new Promise(() => {}));
      const { container } = renderAt('/product/x');
      expect(container.querySelector('.animate-spin')).toBeTruthy();
      expect(container.innerHTML).not.toContain('zv5-');
      expect(screen.queryByTestId('v5-pdp-loading')).toBeNull();
    });
  });

  describe('production bundle isolation', () => {
    it('ProductDetail does not import V5-only components: a plain v5 page without the dev wrapper slots renders no passport, archive label or skeleton', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(product(SHORT));
      const { container } = render(
        <MemoryRouter initialEntries={['/x/1']}>
          <Routes>
            <Route path="/x/:id" element={<ProductDetail presentation="v5" />} />
          </Routes>
        </MemoryRouter>
      );
      await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: SHORT })).toBeTruthy());
      expect(container.querySelector('.zv5-passport')).toBeNull();
      expect(screen.queryByTestId('v5-archive-label')).toBeNull();
      expect(screen.queryByTestId('v5-pdp-loading')).toBeNull();
    });

    it('the V5 components are imported only by the dev wrapper, never by ProductDetail or App', () => {
      const read = (rel: string) => readFileSync(join(process.cwd(), process.cwd().endsWith('apps/shop') ? '' : 'apps/shop', rel), 'utf8');
      const pd = read('src/pages/ProductDetail.tsx');
      expect(pd).not.toMatch(/V5ProductPassport|V5PdpSkeleton|v5PassportRows|v5-pdp\.css/);
      expect(read('src/App.tsx')).not.toMatch(/V5ProductPassport|V5PdpSkeleton/);
      const wrapper = read('src/components/product-detail/ShopV5ProductDetail.tsx');
      expect(wrapper).toMatch(/V5ProductPassport/);
      expect(wrapper).toMatch(/V5PdpSkeleton/);
    });
  });

  describe('calm background', () => {
    it('adds a local, light surface layer under the V5 page only (no blur, Shop background untouched)', () => {
      expect(v5Css).toMatch(/\.zv5-pdp::before \{[^}]*position: absolute;[^}]*z-index: -1;[^}]*pointer-events: none/);
      expect(v5Css).toMatch(/--zp-surface: rgba\(251, 252, 254, 0\.9\)/);
      expect(v5Css).toMatch(/\.dark \.zv5-pdp \{ --zp-surface: rgba\(11, 12, 14, 0\.88\)/);
      // nothing in the sheet reaches the global background or other pages
      expect(v5Css).not.toMatch(/IcyBackground|body\s*\{|^\s*html\b/m);
    });
  });
});
