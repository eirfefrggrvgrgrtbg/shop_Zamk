/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { ProductDetail } from './ProductDetail';
import * as publicCatalog from '../api/publicCatalog';
import { ApiError } from '@zamk/api-client/src/errors';
import type { Product } from '../types/catalog';

// Mock contexts
const mockAddItem = vi.fn().mockResolvedValue(undefined);
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
    isFavorite: () => false,
    toggleFavorite: mockToggleFavorite,
  }),
}));

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 'user-1' },
    isAuthenticated: true,
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

// Multi-variant product (COLOR_AND_SIZE)
// Black: S (inStock), M (sold out), L (inStock)
// White: M (inStock), L (inStock) [S is NOT OFFERED]
const mockColorAndSizeProduct: Product = {
  id: 'prod-dress-100',
  name: 'Шёлковое вечернее платье',
  brand: 'ZAMK Couture',
  brandId: 'brand-couture',
  sellerId: 'seller-couture',
  sellerName: 'ZAMK Flagship Store',
  sellerSlug: 'zamk-flagship',
  price: 45000,
  oldPrice: 50000,
  discountPrice: 45000,
  image: 'https://example.com/dress-general.jpg',
  images: [
    { url: 'https://example.com/dress-general.jpg' }, // 0: general
    { url: 'https://example.com/dress-black-1.jpg', colorId: 'color-black' }, // 1: black
    { url: 'https://example.com/dress-black-2.jpg', colorId: 'color-black' }, // 2: black
    { url: 'https://example.com/dress-white-1.jpg', colorId: 'color-white' }, // 3: white
  ],
  category: 'Вечерние платья',
  description: 'Шёлковое платье.',
  variants: [
    {
      id: 'var-black-s',
      size: 'S',
      color: 'Чёрный',
      colorName: 'Чёрный',
      colorHex: '#000000',
      colorId: 'color-black',
      inStock: true,
      isActive: true,
      priceCents: 4500000,
      sellerSku: 'DRS-BLK-S',
    },
    {
      id: 'var-black-m',
      size: 'M',
      color: 'Чёрный',
      colorName: 'Чёрный',
      colorHex: '#000000',
      colorId: 'color-black',
      inStock: false,
      isActive: true,
      priceCents: 4500000,
      sellerSku: 'DRS-BLK-M',
    },
    {
      id: 'var-black-l',
      size: 'L',
      color: 'Чёрный',
      colorName: 'Чёрный',
      colorHex: '#000000',
      colorId: 'color-black',
      inStock: true,
      isActive: true,
      priceCents: 4500000,
      sellerSku: 'DRS-BLK-L',
    },
    {
      id: 'var-white-m',
      size: 'M',
      color: 'Белый',
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      colorId: 'color-white',
      inStock: true,
      isActive: true,
      priceCents: 4700000,
      sellerSku: 'DRS-WHT-M',
    },
    {
      id: 'var-white-l',
      size: 'L',
      color: 'Белый',
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      colorId: 'color-white',
      inStock: true,
      isActive: true,
      priceCents: 4700000,
      sellerSku: 'DRS-WHT-L',
    },
  ],
};

// COLOR_ONLY Product
const mockColorOnlyProduct: Product = {
  id: 'prod-scarf-200',
  name: 'Шёлковый шарф',
  brand: 'ZAMK Accessories',
  brandId: 'brand-acc',
  sellerId: 'seller-acc',
  sellerName: 'ZAMK Accessories',
  sellerSlug: 'zamk-acc',
  price: 12000,
  image: 'https://example.com/scarf-general.jpg',
  images: [
    { url: 'https://example.com/scarf-general.jpg' },
    { url: 'https://example.com/scarf-red.jpg', colorId: 'color-red' },
    { url: 'https://example.com/scarf-blue.jpg', colorId: 'color-blue' },
  ],
  category: 'Аксессуары',
  description: 'Шарф.',
  variants: [
    {
      id: 'var-scarf-red',
      color: 'Красный',
      colorName: 'Красный',
      colorHex: '#FF0000',
      colorId: 'color-red',
      inStock: true,
      isActive: true,
      priceCents: 1200000,
      sellerSku: 'SCF-RED',
    },
    {
      id: 'var-scarf-blue',
      color: 'Синий',
      colorName: 'Синий',
      colorHex: '#0000FF',
      colorId: 'color-blue',
      inStock: true,
      isActive: true,
      priceCents: 1200000,
      sellerSku: 'SCF-BLU',
    },
  ],
};

// SIZE_ONLY Product
const mockSizeOnlyProduct: Product = {
  id: 'prod-shoes-300',
  name: 'Кожаные лоферы',
  brand: 'ZAMK Shoes',
  brandId: 'brand-shoes',
  sellerId: 'seller-shoes',
  sellerName: 'ZAMK Shoes',
  sellerSlug: 'zamk-shoes',
  price: 25000,
  image: 'https://example.com/shoes-main.jpg',
  images: [{ url: 'https://example.com/shoes-main.jpg' }],
  category: 'Обувь',
  description: 'Лоферы.',
  variants: [
    {
      id: 'var-shoes-39',
      size: '39',
      inStock: true,
      isActive: true,
      priceCents: 2500000,
      sellerSku: 'SH-39',
    },
    {
      id: 'var-shoes-40',
      size: '40',
      inStock: true,
      isActive: true,
      priceCents: 2500000,
      sellerSku: 'SH-40',
    },
    {
      id: 'var-shoes-41',
      size: '41',
      inStock: false,
      isActive: true,
      priceCents: 2500000,
      sellerSku: 'SH-41',
    },
  ],
};

// Helper component to track URL
let latestLocation: any = null;
let navHelper: any = null;

function LocationTracker() {
  latestLocation = useLocation();
  const navigate = useNavigate();
  navHelper = navigate;
  return (
    <div data-testid="location-debug" data-search={latestLocation.search}>
      {latestLocation.pathname + latestLocation.search}
    </div>
  );
}

function renderProductDetail(initialPath: string) {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route
          path="/product/:id"
          element={
            <>
              <ProductDetail />
              <LocationTracker />
            </>
          }
        />
      </Routes>
    </MemoryRouter>
  );
}

describe('SHOP PDP.2D2 — Variant Deep-Link URL State Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    latestLocation = null;
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Clean initial load (/product/id): no query params are added and main image is active', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // URL should remain completely clean
    expect(latestLocation.search).toBe('');

    // Active image remains index 0 (general photo)
    const mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/dress-general.jpg');
  });

  it('2. Explicit color click: pushes ?color=<COLOR_ID> and focuses matching media', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Click White swatch
    const whiteSwatch = screen.getByTestId('color-swatch-color-white');
    fireEvent.click(whiteSwatch);

    // URL should now have ?color=color-white
    expect(latestLocation.search).toBe('?color=color-white');

    // Media should focus the first image matching white
    const mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/dress-white-1.jpg');
  });

  it('3. Explicit size click: pushes ?color=<COLOR_ID>&size=<SIZE_ID>', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Click Black first to give explicit color intent
    const blackSwatch = screen.getByTestId('color-swatch-color-black');
    fireEvent.click(blackSwatch);
    expect(latestLocation.search).toBe('?color=color-black');

    // Click size S
    const sizeBtnS = screen.getByTestId('size-button-S');
    fireEvent.click(sizeBtnS);

    // URL should now have both color and size
    expect(latestLocation.search).toBe('?color=color-black&size=S');
  });

  it('4. Color switch with buyable size: preserves size in a single navigation entry', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    // Load with Black + L (L is buyable in both Black and White)
    renderProductDetail('/product/prod-dress-100?color=color-black&size=L');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      const sizeBtnL = screen.getByTestId('size-button-L');
      expect(sizeBtnL.getAttribute('aria-pressed')).toBe('true');
    });

    expect(latestLocation.search).toBe('?color=color-black&size=L');

    // Switch to White
    const whiteSwatch = screen.getByTestId('color-swatch-color-white');
    fireEvent.click(whiteSwatch);

    // Size L is available in White, so size is retained atomically
    expect(latestLocation.search).toBe('?color=color-white&size=L');
  });

  it('5. Color switch with unavailable size: removes size in a single navigation entry and displays notice', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    // Load with Black + S (S is NOT offered in White)
    renderProductDetail('/product/prod-dress-100?color=color-black&size=S');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      const sizeBtnS = screen.getByTestId('size-button-S');
      expect(sizeBtnS.getAttribute('aria-pressed')).toBe('true');
    });

    expect(latestLocation.search).toBe('?color=color-black&size=S');

    // Switch to White
    const whiteSwatch = screen.getByTestId('color-swatch-color-white');
    fireEvent.click(whiteSwatch);

    // Size S is not offered in White -> size param removed atomically and notice displayed
    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-white');
      expect(screen.getByTestId('size-selection-notice')).toBeTruthy();
    });
  });

  it('6. Shared URL restoration: restores exact variant, price, CTA, and color media focus', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100?color=color-black&size=S');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      const sizeBtnS = screen.getByTestId('size-button-S');
      expect(sizeBtnS.getAttribute('aria-pressed')).toBe('true');
    });

    // Variant var-black-s is selected
    const addToCartBtn = screen.getByTestId('add-to-cart-button');
    expect(addToCartBtn.textContent).toMatch(/в корзину/i);
    expect(addToCartBtn.hasAttribute('disabled')).toBe(false);

    // Color media focus is set to first black image
    const mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/dress-black-1.jpg');
  });

  it('7. Free gallery browsing after restoration: manual thumb clicks are not overridden', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100?color=color-black&size=S');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/dress-black-1.jpg');
    });

    // Customer clicks thumbnail index 0 (general photo)
    const thumb0 = screen.getByTestId('pdp-thumbnail-0');
    fireEvent.click(thumb0);

    let mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/dress-general.jpg');

    // Customer clicks thumbnail index 2 (dress-black-2)
    const thumb2 = screen.getByTestId('pdp-thumbnail-2');
    fireEvent.click(thumb2);

    mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/dress-black-2.jpg');
  });

  it('8. Invalid color in URL: replaced to clean URL without crashing', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100?color=non-existent-color');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Sanitized: search params stripped via replace
    await waitFor(() => {
      expect(latestLocation.search).toBe('');
    });
  });

  it('9. Sold-out size in URL: stripped via replace, displays notice and does not select variant', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    // Black + M is SOLD_OUT
    renderProductDetail('/product/prod-dress-100?color=color-black&size=M');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Sanitized: size is removed, color remains
    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-black');
    });

    // Notice is displayed
    await waitFor(() => {
      expect(screen.getByTestId('size-selection-notice')).toBeTruthy();
      expect(screen.getByTestId('size-selection-notice').textContent).toContain('закончился');
    });

    // CTA indicates size must be chosen
    const addToCartBtn = screen.getByTestId('add-to-cart-button');
    expect(addToCartBtn.textContent).toContain('Выберите размер');
  });

  it('10. Not-offered size in URL: stripped via replace, displays notice and does not select variant', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    // White does not offer size S
    renderProductDetail('/product/prod-dress-100?color=color-white&size=S');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Sanitized: size is removed, color remains
    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-white');
    });

    // Notice displayed
    await waitFor(() => {
      expect(screen.getByTestId('size-selection-notice')).toBeTruthy();
    });
  });

  it('11. Standalone size without color on COLOR_AND_SIZE product: stripped via replace', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100?size=S');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Size stripped without guessing color
    await waitFor(() => {
      expect(latestLocation.search).toBe('');
    });
  });

  it('12. COLOR_ONLY product: handles ?color=... and sanitizes unexpected ?size=...', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockColorOnlyProduct);

    renderProductDetail('/product/prod-scarf-200?color=color-blue&size=XL');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковый шарф' })).toBeTruthy();
    });

    // Sanitized: size stripped, color retained
    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-blue');
    });

    // Media focused on blue scarf
    const mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/scarf-blue.jpg');
  });

  it('13. SIZE_ONLY product: handles ?size=... and sanitizes unexpected ?color=...', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockSizeOnlyProduct);

    renderProductDetail('/product/prod-shoes-300?size=39&color=color-random');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Кожаные лоферы' })).toBeTruthy();
    });

    // Sanitized: color stripped, size retained
    await waitFor(() => {
      expect(latestLocation.search).toBe('?size=39');
    });

    // Variant 39 is selected and ready to buy
    const addToCartBtn = screen.getByTestId('add-to-cart-button');
    expect(addToCartBtn.textContent).toMatch(/в корзину/i);
  });

  it('14. Stale stock recovery after Add-to-Cart failure removes ?size=... via replace', async () => {
    // Initial fetch: M is in stock
    const initialProduct: Product = {
      ...mockColorAndSizeProduct,
      variants: mockColorAndSizeProduct.variants!.map(v =>
        v.id === 'var-black-m' ? { ...v, inStock: true } : v
      ),
    };

    // Fresh fetch after failure: M is now sold out
    const freshProduct: Product = {
      ...mockColorAndSizeProduct,
      variants: mockColorAndSizeProduct.variants!.map(v =>
        v.id === 'var-black-m' ? { ...v, inStock: false } : v
      ),
    };

    vi.mocked(publicCatalog.fetchProductById)
      .mockResolvedValueOnce(initialProduct)
      .mockResolvedValueOnce(freshProduct);

    // Simulate 409 Insufficient stock error
    mockAddItem.mockRejectedValueOnce(
      new ApiError('Insufficient stock', 'insufficient_stock', 409, {
        available_qty: 0,
        requested_qty: 1,
      })
    );

    renderProductDetail('/product/prod-dress-100?color=color-black&size=M');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      const sizeBtnM = screen.getByTestId('size-button-M');
      expect(sizeBtnM.getAttribute('aria-pressed')).toBe('true');
    });

    expect(latestLocation.search).toBe('?color=color-black&size=M');

    const addToCartBtn = screen.getByTestId('add-to-cart-button');
    fireEvent.click(addToCartBtn);

    await waitFor(() => {
      // URL should have ?size= stripped
      expect(latestLocation.search).toBe('?color=color-black');
    });

    // Notice displayed to customer
    expect(screen.getByTestId('size-selection-notice')).toBeTruthy();
  });

  it('15. Browser Back/Forward navigation restores variant selection and media focus without loops', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValue(mockColorAndSizeProduct);

    renderProductDetail('/product/prod-dress-100');

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    expect(latestLocation.search).toBe('');

    // Step 1: Click White swatch -> pushes ?color=color-white
    const whiteSwatch = screen.getByTestId('color-swatch-color-white');
    fireEvent.click(whiteSwatch);

    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-white');
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/dress-white-1.jpg');
    });

    // Step 2: Click size M -> pushes ?color=color-white&size=M
    const sizeBtnM = screen.getByTestId('size-button-M');
    fireEvent.click(sizeBtnM);

    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-white&size=M');
    });

    // Step 3: Simulate Browser Back -> URL returns to ?color=color-white
    navHelper(-1);

    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-white');
      // Size M is no longer selected
      const sizeBtnMAfterBack = screen.getByTestId('size-button-M');
      expect(sizeBtnMAfterBack.getAttribute('aria-pressed')).toBe('false');
      // Media focus remains white
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/dress-white-1.jpg');
    });

    // Step 4: Simulate Browser Back again -> URL returns to clean URL
    navHelper(-1);

    await waitFor(() => {
      expect(latestLocation.search).toBe('');
      // Main image returns to index 0 (general)
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/dress-general.jpg');
    });

    // Step 5: Simulate Browser Forward -> URL returns to ?color=color-white
    navHelper(1);

    await waitFor(() => {
      expect(latestLocation.search).toBe('?color=color-white');
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/dress-white-1.jpg');
    });
  });
});
