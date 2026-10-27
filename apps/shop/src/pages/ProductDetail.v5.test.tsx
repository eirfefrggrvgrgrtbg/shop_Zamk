/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import appSource from '../App.tsx?raw';
import { ProductDetail } from './ProductDetail';
import { ShopV5ProductDetail } from '../components/product-detail/ShopV5ProductDetail';
import * as publicCatalog from '../api/publicCatalog';
import type { Product } from '../types/catalog';

const mockAddItem = vi.fn().mockResolvedValue(undefined);
const mockToggleFavorite = vi.fn();
const mockShowToast = vi.fn();
const mockOpenAuth = vi.fn();
const authState: { user: { id: string } | null } = { user: { id: 'user-1' } };

vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({ addItem: mockAddItem, items: [], totalItems: 0 }),
}));
vi.mock('../contexts/FavoritesContext', () => ({
  useFavorites: () => ({ isFavorite: () => false, toggleFavorite: mockToggleFavorite }),
}));
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ user: authState.user, isAuthenticated: Boolean(authState.user), openAuthModal: mockOpenAuth, isInitializing: false }),
}));
vi.mock('../contexts/ToastContext', () => ({ useToast: () => ({ showToast: mockShowToast }) }));
vi.mock('@zamk/api-client/src/customer', () => ({ recordProductView: vi.fn().mockResolvedValue({ status: 'ok' }) }));
vi.mock('../components/product/SimilarProductsBlock', () => ({
  SimilarProductsBlock: () => <div data-testid="similar-products-mock" />,
}));
vi.mock('../api/publicCatalog', () => ({
  fetchProductById: vi.fn(),
  fetchProductReviews: vi.fn().mockResolvedValue([]),
  fetchProductPreviewByToken: vi.fn(),
}));

const v = (id: string, size: string, over: Record<string, unknown> = {}) => ({
  id,
  size,
  inStock: true,
  isActive: true,
  priceCents: 1000000,
  sellerSku: `SKU-${id}`,
  ...over,
});

function makeProduct(over: Partial<Product> = {}): Product {
  return {
    id: 'prod-1',
    name: 'Тестовое пальто',
    brand: 'ZAMK',
    brandId: 'brand-1',
    sellerName: 'Test Seller',
    sellerSlug: 'test-seller',
    price: 10000,
    image: 'https://example.com/a.jpg',
    images: [{ url: 'https://example.com/a.jpg' }, { url: 'https://example.com/b.jpg' }],
    category: 'Пальто',
    materialComposition: [{ materialName: 'Шерсть', percentage: 80 }],
    variants: [v('var-s', 'S'), v('var-m', 'M'), v('var-l', 'L', { inStock: false })],
    ...over,
  } as Product;
}

function renderAt(path: '/product/prod-1' | '/v5-pdp-preview/prod-1') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/product/:id" element={<ProductDetail />} />
        <Route path="/v5-pdp-preview/:id" element={<ShopV5ProductDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

async function open(path: '/product/prod-1' | '/v5-pdp-preview/prod-1', product: Product) {
  vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(product);
  const r = renderAt(path);
  await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: product.name })).toBeTruthy());
  return r;
}

describe('Shop V5 PDP preview', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    authState.user = { id: 'user-1' };
    mockAddItem.mockResolvedValue(undefined);
  });
  afterEach(cleanup);

  it('is a dev-only route: guarded by import.meta.env.DEV and absent from the production route list', () => {
    const app: string = appSource;
    expect(app).toMatch(/const ShopV5ProductDetail = import\.meta\.env\.DEV/);
    expect(app).toMatch(/ShopV5ProductDetail && \(\s*<Route\s+path="\/v5-pdp-preview\/:id"/);
    // the production route is still the plain ProductDetail
    expect(app).toContain('<Route path="/product/:id" element={<ProductDetail />} />');
  });

  it('production /product/:id keeps the current design: no V5 hooks, no passport, original service rows', async () => {
    const { container } = await open('/product/prod-1', makeProduct());
    expect(container.innerHTML).not.toContain('zv5-');
    expect(screen.queryByTestId('v5-passport')).toBeNull();
    expect(screen.getByText('Доставка: по России от 2 дней')).toBeTruthy();
    expect(screen.getByText('Возврат: в течение 14 дней')).toBeTruthy();
  });

  it('V5 route renders the V5 presentation with the same data flow', async () => {
    const { container } = await open('/v5-pdp-preview/prod-1', makeProduct());
    expect(screen.getByTestId('v5-pdp')).toBeTruthy();
    expect(screen.getByTestId('v5-archive-label').textContent).toBe('ZAMK ARCHIVE');
    expect(container.querySelector('.zv5-grid')).toBeTruthy();
    expect(container.querySelector('.zv5-shell')).toBeTruthy();
    // no big white box around the page
    expect(container.innerHTML).not.toContain('rounded-2xl p-5');
    expect(publicCatalog.fetchProductById).toHaveBeenCalledWith('prod-1');
  });

  it('removes the duplicated delivery/returns copy and does not pick a delivery duration', async () => {
    await open('/v5-pdp-preview/prod-1', makeProduct());
    expect(screen.queryByText('Доставка: по России от 2 дней')).toBeNull();
    expect(screen.queryByText('Возврат: в течение 14 дней')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Доставка и возврат' }));
    expect(screen.queryByText(/2–7 дней/)).toBeNull();
    expect(screen.getByText(/Бесплатная доставка от 10 000/)).toBeTruthy();
    expect(screen.getByText(/возврат в течение 14 дней/)).toBeTruthy();
    expect(screen.getByRole('link', { name: /Сроки и стоимость доставки/ }).getAttribute('href')).toBe('/delivery');
    // the seller row stays
    expect(screen.getByText('Test Seller')).toBeTruthy();
  });

  it('keeps exact SKU selection: variantId sent to the cart is the selected size, with no guessing', async () => {
    await open('/v5-pdp-preview/prod-1', makeProduct());
    const add = screen.getByTestId('add-to-cart-button') as HTMLButtonElement;
    expect(add.disabled).toBe(true);
    expect(screen.getByTestId('size-button-L').getAttribute('data-state')).toBe('SOLD_OUT');
    fireEvent.click(screen.getByTestId('size-button-M'));
    expect(screen.getByTestId('size-button-M').getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(add);
    await waitFor(() => expect(mockAddItem).toHaveBeenCalledTimes(1));
    expect(mockAddItem).toHaveBeenCalledWith('prod-1', 'var-m', 1);
    fireEvent.click(screen.getByTestId('size-button-S'));
    expect(screen.getByTestId('size-button-M').getAttribute('aria-pressed')).toBe('false');
  });

  it('shows the passport from real fields only (brand, SKU article, composition)', async () => {
    await open('/v5-pdp-preview/prod-1', makeProduct());
    const passport = screen.getByTestId('v5-passport');
    expect(within(passport).getByTestId('v5-passport-Бренд').textContent).toContain('ZAMK');
    expect(within(passport).getByTestId('v5-passport-Состав').textContent).toContain('Шерсть 80%');
    // before a size is chosen: the technical product code; afterwards the variant's own seller SKU
    expect(within(passport).getByTestId('v5-passport-Код товара').textContent).toContain('1');
    expect(within(passport).queryByTestId('v5-passport-Артикул варианта')).toBeNull();
    fireEvent.click(screen.getByTestId('size-button-S'));
    const after = screen.getByTestId('v5-passport');
    expect(within(after).getByTestId('v5-passport-Артикул варианта').textContent).toContain('SKU-VAR-S');
    expect(within(after).queryByTestId('v5-passport-Код товара')).toBeNull();
    expect(after.textContent).not.toMatch(/принят|поступил|склад|резерв|проверк/i);
    expect(after.querySelector('[data-decorative="true"]')?.getAttribute('aria-hidden')).toBe('true');
  });

  it('omits the composition row when the product has no composition', async () => {
    await open('/v5-pdp-preview/prod-1', makeProduct({ materialComposition: undefined, materials: undefined }));
    const passport = screen.getByTestId('v5-passport');
    expect(within(passport).queryByTestId('v5-passport-Состав')).toBeNull();
    expect(within(passport).queryByTestId('v5-passport-Материал')).toBeNull();
    expect(screen.getByText('Информация о составе не указана.')).toBeTruthy();
  });

  it('guest favorite: login prompt, no toggle, no success message', async () => {
    authState.user = null;
    await open('/v5-pdp-preview/prod-1', makeProduct());
    fireEvent.click(screen.getByRole('button', { name: 'Добавить в избранное' }));
    expect(mockToggleFavorite).not.toHaveBeenCalled();
    expect(mockShowToast).toHaveBeenCalledWith('Войдите, чтобы добавить товар в избранное', 'error');
  });

  it('authenticated favorite uses the existing context', async () => {
    await open('/v5-pdp-preview/prod-1', makeProduct());
    fireEvent.click(screen.getByRole('button', { name: 'Добавить в избранное' }));
    expect(mockToggleFavorite).toHaveBeenCalledWith('prod-1');
  });

  it('gallery: several photos show the counter and thumbnails, one photo shows neither', async () => {
    const first = await open('/v5-pdp-preview/prod-1', makeProduct());
    expect(screen.getByTestId('pdp-thumbnails-container')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Следующее фото' })).toBeTruthy();
    first.unmount();
    await open('/v5-pdp-preview/prod-1', makeProduct({ images: [{ url: 'https://example.com/a.jpg' }] }));
    expect(screen.queryByTestId('pdp-thumbnails-container')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Следующее фото' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Открыть изображение на полный экран' })).toBeTruthy();
  });

  it('long title still renders as the page heading', async () => {
    const name = 'Пальто оверсайз из плотной итальянской шерсти с запахом, съёмным поясом и асимметричной застёжкой';
    await open('/v5-pdp-preview/prod-1', makeProduct({ name }));
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(name);
  });

  it('error state keeps the V5 frame and the way back to the catalog', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockRejectedValueOnce(new Error('boom'));
    renderAt('/v5-pdp-preview/prod-1');
    await waitFor(() => expect(screen.getByTestId('v5-pdp-error')).toBeTruthy());
    expect(screen.getByText('Не удалось загрузить товар')).toBeTruthy();
    expect(screen.getByRole('link', { name: /Вернуться в каталог/ }).getAttribute('href')).toBe('/catalog');
  });
});
