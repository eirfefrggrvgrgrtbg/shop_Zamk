/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ProductDetail } from './ProductDetail';
import * as publicCatalog from '../api/publicCatalog';
import { ApiError } from '@zamk/api-client/src/errors';
import type { Product } from '../types/catalog';

// Mock contexts
const mockAddItem = vi.fn().mockResolvedValue(undefined);
const authState = { isAuthenticated: true };
const mockToggleFavorite = vi.fn();
const mockShowToast = vi.fn();

vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({
    addItem: mockAddItem,
    items: [],
    removeItem: vi.fn(),
    updateQuantity: vi.fn(),
    clearCart: vi.fn(),
    totalItems: 0,
    totalPrice: 0,
    isLoadingCart: false,
  }),
}));

vi.mock('../contexts/FavoritesContext', () => ({
  useFavorites: () => ({
    isFavorite: (id: string) => id === 'fav-prod',
    toggleFavorite: mockToggleFavorite,
  }),
}));

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 'user-1' },
    isAuthenticated: authState.isAuthenticated,
    openAuthModal: vi.fn(),
    isInitializing: false,
  }),
}));

vi.mock('../contexts/ToastContext', () => ({
  useToast: () => ({
    showToast: mockShowToast,
  }),
}));

vi.mock('@zamk/api-client/src/customer', () => ({
  recordProductView: vi.fn().mockResolvedValue({ status: 'ok' }),
}));

vi.mock('../components/product/SimilarProductsBlock', () => ({
  SimilarProductsBlock: () => <div data-testid="similar-products-mock" />,
}));

vi.mock('../api/publicCatalog', () => ({
  fetchProductById: vi.fn(),
  fetchProductReviews: vi.fn().mockResolvedValue([]),
  fetchProductPreviewByToken: vi.fn(),
}));


const v = (id: string, size: string | undefined, over: Record<string, unknown> = {}) => ({
  id,
  size,
  inStock: true,
  isActive: true,
  priceCents: 1000000,
  sellerSku: `SKU-${id}`,
  ...over,
});

function makeProduct(variants: Product['variants']): Product {
  return {
    id: 'prod-1',
    name: 'Тестовое пальто',
    brand: 'ZAMK',
    brandId: 'brand-1',
    price: 10000,
    image: 'https://example.com/a.jpg',
    category: 'Пальто',
    variants,
  };
}

const sizeSet = () =>
  makeProduct([
    v('var-s', 'S'),
    v('var-m', 'M'),
    v('var-l', 'L', { inStock: false }),
    v('var-xl', 'XL', { isActive: false, inStock: true }),
    v('var-xxl', 'XXL', { inStock: undefined }),
  ]);

function renderPdp() {
  return render(
    <MemoryRouter initialEntries={['/product/prod-1']}>
      <Routes>
        <Route path="/product/:id" element={<ProductDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

async function openPdp(product: Product) {
  vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(product);
  renderPdp();
  await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: product.name })).toBeTruthy());
}

const size = (label: string) => screen.getByTestId(`size-button-${label}`);
const addBtn = () => screen.getByTestId('add-to-cart-button') as HTMLButtonElement;

describe('SHOP PDP.VARIANTS.1 — SKU selection and add-to-cart', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAddItem.mockResolvedValue(undefined);
    authState.isAuthenticated = true;
  });
  afterEach(() => cleanup());

  it('renders each size state; inactive SKU is hidden; unknown availability is not addable', async () => {
    await openPdp(sizeSet());
    expect(size('S').getAttribute('data-state')).toBe('AVAILABLE');
    expect(size('M').getAttribute('data-state')).toBe('AVAILABLE');
    expect(size('L').getAttribute('data-state')).toBe('SOLD_OUT');
    expect(screen.queryByTestId('size-button-XL')).toBeNull();
    // unknown (no inStock flag) is never treated as available
    expect(size('XXL').getAttribute('data-state')).not.toBe('AVAILABLE');
    expect(addBtn().disabled).toBe(true);
  });

  it('selecting a size marks it pressed, switching changes the variantId sent to the cart', async () => {
    await openPdp(sizeSet());
    fireEvent.click(size('S'));
    expect(size('S').getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(size('M'));
    expect(size('S').getAttribute('aria-pressed')).toBe('false');
    expect(size('M').getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(addBtn());
    await waitFor(() => expect(mockAddItem).toHaveBeenCalledWith('prod-1', 'var-m', 1));
    await waitFor(() => expect(mockShowToast).toHaveBeenCalledWith('Товар добавлен в корзину'));
    expect(mockAddItem).toHaveBeenCalledTimes(1);
  });

  it('sold-out size cannot be added; inactive-with-stock SKU is not offered', async () => {
    await openPdp(sizeSet());
    const l = size('L') as HTMLButtonElement;
    if (!l.disabled) fireEvent.click(l);
    expect(addBtn().disabled).toBe(true);
    fireEvent.click(addBtn());
    expect(mockAddItem).not.toHaveBeenCalled();
  });

  it('two SKUs of one size: no arbitrary variantId is chosen', async () => {
    await openPdp(makeProduct([v('dup-1', 'M'), v('dup-2', 'M'), v('var-s', 'S')]));
    fireEvent.click(size('M'));
    expect(addBtn().disabled).toBe(true);
    fireEvent.click(addBtn());
    expect(mockAddItem).not.toHaveBeenCalled();
  });

  it('double click sends a single request', async () => {
    let resolve!: () => void;
    mockAddItem.mockImplementationOnce(() => new Promise<void>((r) => { resolve = r; }));
    await openPdp(sizeSet());
    fireEvent.click(size('S'));
    fireEvent.click(addBtn());
    fireEvent.click(addBtn());
    expect(mockAddItem).toHaveBeenCalledTimes(1);
    resolve();
    await waitFor(() => expect(mockShowToast).toHaveBeenCalledWith('Товар добавлен в корзину'));
    expect(mockAddItem).toHaveBeenCalledTimes(1);
  });

  it('409 product_unavailable: refreshes, reports unavailable variant, no success toast', async () => {
    mockAddItem.mockRejectedValueOnce(new ApiError('Этот товар сейчас недоступен для заказа.', 'product_unavailable', 409, {}));
    await openPdp(sizeSet());
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(
      makeProduct([v('var-s', 'S', { inStock: false }), v('var-m', 'M')])
    );
    fireEvent.click(size('S'));
    fireEvent.click(addBtn());
    await waitFor(() => expect(screen.getByTestId('size-selection-notice').textContent).toMatch(/Размер S/));
    expect(mockShowToast).not.toHaveBeenCalledWith('Товар добавлен в корзину');
    expect(size('S').getAttribute('data-state')).toBe('SOLD_OUT');
    expect(size('S').getAttribute('aria-pressed')).toBe('false');
  });

  it('409 when the product is gone: whole product becomes unavailable', async () => {
    mockAddItem.mockRejectedValueOnce(new ApiError('x', 'product_unavailable', 409, {}));
    await openPdp(sizeSet());
    vi.mocked(publicCatalog.fetchProductById).mockRejectedValueOnce(Object.assign(new Error('nf'), { status: 404 }));
    fireEvent.click(size('S'));
    fireEvent.click(addBtn());
    await waitFor(() => expect(mockShowToast).toHaveBeenCalledWith('Товар только что закончился.'));
    expect(addBtn().disabled).toBe(true);
  });

  it('network error: no false confirmation, selection is kept, retry possible', async () => {
    mockAddItem.mockRejectedValueOnce(new ApiError('Не удалось подключиться к серверу. Проверьте, запущен ли backend.', 'NETWORK_ERROR'));
    await openPdp(sizeSet());
    fireEvent.click(size('M'));
    fireEvent.click(addBtn());
    await waitFor(() => expect(mockShowToast).toHaveBeenCalledWith(expect.stringContaining('Проверьте соединение')));
    expect(mockShowToast).not.toHaveBeenCalledWith('Товар добавлен в корзину');
    expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(1);
    expect(size('M').getAttribute('aria-pressed')).toBe('true');
    expect(addBtn().disabled).toBe(false);
    fireEvent.click(addBtn());
    await waitFor(() => expect(mockAddItem).toHaveBeenCalledTimes(2));
    expect(mockAddItem).toHaveBeenLastCalledWith('prod-1', 'var-m', 1);
  });

  it('changing size during a request: result of the old SKU does not disturb the new selection', async () => {
    let reject!: (e: unknown) => void;
    mockAddItem.mockImplementationOnce(() => new Promise<void>((_, r) => { reject = r; }));
    await openPdp(sizeSet());
    fireEvent.click(size('S'));
    fireEvent.click(addBtn());
    fireEvent.click(size('M'));
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(
      makeProduct([v('var-s', 'S', { inStock: false }), v('var-m', 'M')])
    );
    reject(new ApiError('x', 'invalid_item', 400, { error: { message: 'insufficient stock' } }, 'insufficient stock'));
    await waitFor(() => expect(mockShowToast).toHaveBeenCalledWith(expect.stringMatching(/Размер S/)));
    expect(size('M').getAttribute('aria-pressed')).toBe('true');
    expect(screen.queryByTestId('size-selection-notice')).toBeNull();
    expect(addBtn().disabled).toBe(false);
    expect(mockAddItem).toHaveBeenCalledTimes(1);
  });

  it('unauthenticated buyer: opens sign-in via cart context and never sees a success toast', async () => {
    authState.isAuthenticated = false;
    await openPdp(sizeSet());
    fireEvent.click(size('S'));
    fireEvent.click(addBtn());
    await waitFor(() => expect(mockAddItem).toHaveBeenCalledWith('prod-1', 'var-s', 1));
    expect(mockShowToast).not.toHaveBeenCalledWith('Товар добавлен в корзину');
  });
});
