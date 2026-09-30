// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ProductDetail } from '../pages/ProductDetail';
import * as publicCatalog from '../api/publicCatalog';
import type { Product } from '../types/catalog';

// Mock scrollTo
window.scrollTo = vi.fn();

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
    isFavorite: (id: string) => id === 'fav-prod',
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

// Mock publicCatalog API calls
vi.mock('../api/publicCatalog', async () => {
  const actual = await vi.importActual('../api/publicCatalog');
  return {
    ...actual,
    fetchProductById: vi.fn(),
    fetchSimilarProducts: vi.fn().mockResolvedValue([]),
    fetchProductReviews: vi.fn().mockResolvedValue([]),
    fetchProductRatingSummary: vi.fn().mockResolvedValue({ average: 0, count: 0, distribution: {} }),
  };
});

// Mock analytics/telemetry
vi.mock('../lib/analytics', () => ({
  trackProductView: vi.fn(),
  trackVariantSelected: vi.fn(),
}));

// Colorway galleries product (mode COLORWAY_GALLERIES)
const mockBaseProduct: Product = {
  id: 'prod-variants-3c',
  name: 'Кожаная куртка оверсайз',
  brand: 'ZAMK Studio',
  brandId: 'brand-1',
  category: 'Куртки',
  price: 15000,
  image: 'https://example.com/white-front.jpg',
  images: [
    { url: 'https://example.com/general-front.jpg' }, // Generic 0 (must be excluded in COLORWAY_GALLERIES)
    { url: 'https://example.com/white-front.jpg', colorId: 'color-white' }, // White 1
    { url: 'https://example.com/white-back.jpg', colorId: 'color-white' }, // White 2
    { url: 'https://example.com/black-front.jpg', colorId: 'color-black' }, // Black 1
    { url: 'https://example.com/black-back.jpg', colorId: 'color-black' }, // Black 2
    { url: 'https://example.com/black-detail.jpg', colorId: 'color-black' }, // Black 3
    { url: 'https://example.com/red-front.jpg', colorId: 'color-red' }, // Red 1
    { url: 'https://example.com/red-back.jpg', colorId: 'color-red' }, // Red 2
  ],
  variants: [
    {
      id: 'var-wht-48',
      size: '48',
      color: 'Белый',
      colorName: 'Белый',
      colorHex: '#ffffff',
      colorId: 'color-white',
      sizeValueId: 'sz-48',
      inStock: true,
      isActive: true,
      priceCents: 1500000,
    },
    {
      id: 'var-wht-50',
      size: '50',
      color: 'Белый',
      colorName: 'Белый',
      colorHex: '#ffffff',
      colorId: 'color-white',
      sizeValueId: 'sz-50',
      inStock: true,
      isActive: true,
      priceCents: 1500000,
    },
    {
      id: 'var-blk-48',
      size: '48',
      color: 'Чёрный',
      colorName: 'Чёрный',
      colorHex: '#000000',
      colorId: 'color-black',
      sizeValueId: 'sz-48',
      inStock: true,
      isActive: true,
      priceCents: 1500000,
    },
    {
      id: 'var-blk-52',
      size: '52',
      color: 'Чёрный',
      colorName: 'Чёрный',
      colorHex: '#000000',
      colorId: 'color-black',
      sizeValueId: 'sz-52',
      inStock: true,
      isActive: true,
      priceCents: 1500000,
    },
    {
      id: 'var-red-48',
      size: '48',
      color: 'Красный',
      colorName: 'Красный',
      colorHex: '#ff0000',
      colorId: 'color-red',
      sizeValueId: 'sz-48',
      inStock: true,
      isActive: true,
      priceCents: 1500000,
    },
    {
      id: 'var-grn-48',
      size: '48',
      color: 'Зелёный',
      colorName: 'Зелёный',
      colorHex: '#00aa00',
      colorId: 'color-green', // No images of its own
      sizeValueId: 'sz-48',
      inStock: true,
      isActive: true,
      priceCents: 1500000,
    },
  ],
};

// General gallery product (mode GENERAL_GALLERY)
const mockGeneralProduct: Product = {
  id: 'prod-general-gallery',
  name: 'Хлопковая футболка',
  brand: 'ZAMK Studio',
  brandId: 'brand-1',
  category: 'Футболки',
  price: 3000,
  image: 'https://example.com/tee-front.jpg',
  images: [
    { url: 'https://example.com/tee-front.jpg' },
    { url: 'https://example.com/tee-back.jpg' },
    { url: 'https://example.com/tee-model.jpg' },
  ],
  variants: [
    {
      id: 'var-gen-wht-s',
      size: 'S',
      color: 'Белый',
      colorName: 'Белый',
      colorHex: '#ffffff',
      colorId: 'color-white',
      sizeValueId: 'sz-s',
      inStock: true,
      isActive: true,
      priceCents: 300000,
    },
    {
      id: 'var-gen-blk-s',
      size: 'S',
      color: 'Чёрный',
      colorName: 'Чёрный',
      colorHex: '#000000',
      colorId: 'color-black',
      sizeValueId: 'sz-s',
      inStock: true,
      isActive: true,
      priceCents: 300000,
    },
  ],
};

function renderPDP(initialUrl = '/product/prod-variants-3c') {
  return render(
    <MemoryRouter initialEntries={[initialUrl]}>
      <Routes>
        <Route path="/product/:id" element={<ProductDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('CATALOG VARIANTS.3C-R1 — Two Media Modes + Stable Fashion Gallery Test Matrix', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValue(mockBaseProduct);
  });

  afterEach(() => {
    cleanup();
  });

  describe('MODE A — GENERAL_GALLERY', () => {
    it('shows common editorial stream and color selection does NOT change gallery', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockGeneralProduct);
      renderPDP('/product/prod-general-gallery');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Хлопковая футболка' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
      });

      const mainImg = screen.getByAltText('Хлопковая футболка') as HTMLImageElement;
      expect(mainImg.src).toContain('tee-front.jpg');

      // Click White swatch
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);

      // Gallery remains the same (3 photos)
      expect(screen.getByText('1 / 3')).toBeTruthy();
      expect(mainImg.src).toContain('tee-front.jpg');

      // Click Black swatch
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);

      // Gallery still remains the same (3 photos)
      expect(screen.getByText('1 / 3')).toBeTruthy();
      expect(mainImg.src).toContain('tee-front.jpg');
    });
  });

  describe('MODE B — COLORWAY_GALLERIES (A, B, C, D)', () => {
    it('A. selected White -> only White images visible (no generic photo mixed in)', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-white');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Белый/i }).getAttribute('aria-checked')).toBe('true');
      });

      const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
      expect(mainImg.src).toContain('white-front.jpg');

      // Next image is White photo 2
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn);
      await waitFor(() => {
        expect(screen.getByText('2 / 2')).toBeTruthy();
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('white-back.jpg');

      // Generic photo is NOT present
      const thumbnails = screen.getAllByTestId(/^pdp-thumbnail-\d+$/);
      expect(thumbnails).toHaveLength(2);
      expect(thumbnails.some((t) => t.querySelector('img')?.src.includes('general'))).toBe(false);
    });

    it('B. selected Black -> only Black images visible', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
      });

      const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
      expect(mainImg.src).toContain('black-front.jpg');

      // Thumbnails show 3 items
      expect(screen.getAllByTestId(/^pdp-thumbnail-\d+$/)).toHaveLength(3);
    });

    it('C. White images disappear after switching to Black', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-white');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Белый/i }).getAttribute('aria-checked')).toBe('true');
      });

      // Click Black color swatch
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);

      // Now on Black (3 photos)
      await waitFor(() => {
        expect(screen.getByText('1 / 3')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
      });
      const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
      expect(mainImg.src).toContain('black-front.jpg');

      // Verify none of the visible thumbnails have white or general images
      const thumbImages = screen.getAllByTestId(/^pdp-thumbnail-\d+$/).map((el) => {
        return (el.querySelector('img') as HTMLImageElement).src;
      });
      expect(thumbImages).toHaveLength(3);
      expect(thumbImages.some((src) => src.includes('white'))).toBe(false);
      expect(thumbImages.some((src) => src.includes('general'))).toBe(false);
      expect(thumbImages.every((src) => src.includes('black'))).toBe(true);
    });

    it('D. Red images never appear in Black gallery', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
      });

      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      // Cycle through all Black images
      fireEvent.click(nextBtn); // 2 / 3
      fireEvent.click(nextBtn); // 3 / 3
      fireEvent.click(nextBtn); // loops back to 1 / 3

      const currentImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
      expect(currentImg.src).not.toContain('red');
      expect(screen.getByText('1 / 3')).toBeTruthy();
    });
  });

  describe('Fallback Hierarchy & Clean Load (E, F, G)', () => {
    it('E. selected color with no own images -> falls back to neutral placeholder, NEVER generic or other color', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      // Green color has 0 images tagged with 'color-green'
      renderPDP('/product/prod-variants-3c?color=color-green');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Зелёный/i }).getAttribute('aria-checked')).toBe('true');
        const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
        expect(mainImg.src).toContain('placehold.co');
        expect(mainImg.src).not.toContain('general-front.jpg');
        expect(mainImg.src).not.toContain('white');
        expect(mainImg.src).not.toContain('black');
      });

      // No counter badge for 1 image
      expect(screen.queryByText(/1 \/ 1/)).toBeNull();
    });

    it('F. clean load without color in COLORWAY_GALLERIES previews default colorway without auto-selecting color', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
      });

      // Previews default colorway (White, matching product.image: 2 photos)
      expect(screen.getByText('1 / 2')).toBeTruthy();
      const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
      expect(mainImg.src).toContain('white-front.jpg');

      // Purchase state remains unselected
      expect(
        screen.getByText(
          (_, el) =>
            el?.tagName.toLowerCase() === 'p' &&
            (el.textContent?.includes('Цвет:') ?? false) &&
            (el.textContent?.includes('Не выбран') ?? false)
        )
      ).toBeTruthy();
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      expect(whiteSwatch.getAttribute('aria-checked')).toBe('false');
    });

    it('G. never leaks another colorway into an empty colorway', async () => {
      const productOnlyColored: Product = {
        ...mockBaseProduct,
        images: [
          { url: 'https://example.com/white-front.jpg', colorId: 'color-white' },
          { url: 'https://example.com/red-front.jpg', colorId: 'color-red' },
        ],
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(productOnlyColored);
      // Black has no images
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
        const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
        expect(mainImg.src).toContain('placehold.co');
      });

      const mainImg = screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement;
      expect(mainImg.src).not.toContain('white');
      expect(mainImg.src).not.toContain('red');
    });
  });

  describe('State & Navigation (H, I, J, K)', () => {
    it('H. changing color resets selected image index to 0', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
      });

      // Move to photo 3 of Black
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn); // 2
      fireEvent.click(nextBtn); // 3
      await waitFor(() => {
        expect(screen.getByText('3 / 3')).toBeTruthy();
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('black-detail.jpg');

      // Switch to White
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);

      // Resets to photo 1 (1 / 2) of White
      await waitFor(() => {
        expect(screen.getByText('1 / 2')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Белый/i }).getAttribute('aria-checked')).toBe('true');
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('white-front.jpg');
    });

    it('I. thumbnail list matches filtered gallery exactly', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-white');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Белый/i }).getAttribute('aria-checked')).toBe('true');
      });

      const thumbnails = screen.getAllByTestId(/^pdp-thumbnail-\d+$/);
      expect(thumbnails).toHaveLength(2);
      expect((thumbnails[0].querySelector('img') as HTMLImageElement).src).toContain('white-front.jpg');
      expect((thumbnails[1].querySelector('img') as HTMLImageElement).src).toContain('white-back.jpg');
    });

    it('J. counter uses filtered image count', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
      });

      // Black has 3 images -> counter is 1 / 3, not 1 / 8
      expect(screen.queryByText(/1 \/ 8/)).toBeNull();
    });

    it('K. navigation stays inside filtered images', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-red');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Красный/i }).getAttribute('aria-checked')).toBe('true');
      });

      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn); // 2 / 2
      await waitFor(() => {
        expect(screen.getByText('2 / 2')).toBeTruthy();
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('red-back.jpg');

      fireEvent.click(nextBtn); // cycles back to 1 / 2
      await waitFor(() => {
        expect(screen.getByText('1 / 2')).toBeTruthy();
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('red-front.jpg');
    });
  });

  describe('Gallery Geometry & Controls (AA, AB, AC)', () => {
    it('AA. vertical rail is NOT rendered on desktop; bottom thumbnail strip is used', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
      });

      // No vertical rail
      expect(screen.queryByTestId('pdp-desktop-vertical-rail')).toBeNull();

      // Horizontal bottom strip exists
      expect(screen.getByTestId('pdp-thumbnails-container')).toBeTruthy();
      expect(screen.getAllByTestId(/^pdp-thumbnail-\d+$/)).toHaveLength(3);
    });

    it('AB. main stage maintains fixed aspect ratio', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
      });

      const stage = screen.getByTestId('pdp-main-stage');
      expect(stage.className).toContain('aspect-[4/5]');
      expect(stage.className).toContain('max-h-[640px]');
      expect(stage.parentElement?.className).toContain('max-w-[520px]');
    });

    it('AC. left/right arrows are discoverable with contrast class', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
      });

      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      const prevBtn = screen.getByRole('button', { name: 'Предыдущее фото' });
      const nextSpan = nextBtn.querySelector('span');
      const prevSpan = prevBtn.querySelector('span');

      expect(nextSpan?.className).toContain('opacity-75');
      expect(nextSpan?.className).toContain('backdrop-blur-xs');
      expect(prevSpan?.className).toContain('opacity-75');
      expect(prevSpan?.className).toContain('backdrop-blur-xs');
    });
  });

  describe('URL & Deep Linking (L, M, N)', () => {
    it('L. ?color=color-black deep link -> Black gallery rendered immediately', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
      });

      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('black-front.jpg');
    });

    it('N. invalid color -> falls back to deterministic default preview gallery', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=non-existent-color');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
      });

      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('white-front.jpg');
    });
  });

  describe('VARIANTS.3B Regression (O, P, Q, R)', () => {
    it('O. size-first without color -> default preview gallery, no color selected', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      // Size 48 is available in Black, White, Red
      renderPDP('/product/prod-variants-3c?size=sz-48');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
      });

      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('white-front.jpg');
      // No color is selected
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      expect(blackSwatch.getAttribute('aria-checked')).toBe('false');
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      expect(whiteSwatch.getAttribute('aria-checked')).toBe('false');
    });

    it('P. incompatible color click -> size clears and gallery switches to new color', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      // Size 52 is available ONLY in Black (not in White or Red)
      renderPDP('/product/prod-variants-3c?color=color-black&size=sz-52');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
      });

      // Click White swatch (size 52 is incompatible with White)
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);

      // Gallery becomes White (2 photos)
      await waitFor(() => {
        expect(screen.getByText('1 / 2')).toBeTruthy();
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('white-front.jpg');

      // Size 52 is cleared
      const size52Btn = screen.getByRole('button', { name: /^Размер 52/i });
      expect(size52Btn.getAttribute('aria-pressed')).toBe('false');

      // Contextual notice is displayed
      expect(screen.getByRole('status').textContent).toContain('Размер 52 недоступен в белом цвете');
    });

    it('Q. incompatible size click -> color clears and gallery returns to default preview gallery', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      // Select White (which offers 48 and 50)
      renderPDP('/product/prod-variants-3c?color=color-white&size=sz-48');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
      });

      // Click size 52 (not offered in White, but offered in Black)
      const size52Btn = screen.getByRole('button', { name: /^Размер 52/i });
      fireEvent.click(size52Btn);

      // Color is cleared -> returns to default preview gallery (White, 2 photos)
      await waitFor(() => {
        expect(screen.getByText('1 / 2')).toBeTruthy();
      });
      expect((screen.getByAltText('Кожаная куртка оверсайз') as HTMLImageElement).src).toContain('white-front.jpg');

      // Size 52 is selected
      expect(size52Btn.getAttribute('aria-pressed')).toBe('true');
    });

    it('R. Add to Cart semantics unchanged: requires full resolution', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-white');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 2')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Белый/i }).getAttribute('aria-checked')).toBe('true');
      });

      // Only color selected -> CTA disabled
      const ctaBtn = screen.getByRole('button', { name: 'Выберите размер' }) as HTMLButtonElement;
      expect(ctaBtn.disabled).toBe(true);

      // Select size 48 -> resolved -> CTA enabled
      const size48Btn = screen.getByRole('button', { name: /^Размер 48/i });
      fireEvent.click(size48Btn);

      const addBtn = screen.getByRole('button', { name: 'Добавить в корзину' }) as HTMLButtonElement;
      expect(addBtn.disabled).toBe(false);
    });
  });

  describe('Data & Ordering Integrity (S, T, U)', () => {
    it('S, T, U. preserves ordering, canonical main image first, no duplicates', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockBaseProduct);
      renderPDP('/product/prod-variants-3c?color=color-black');

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Кожаная куртка оверсайз' })).toBeTruthy();
        expect(screen.getByText('1 / 3')).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i }).getAttribute('aria-checked')).toBe('true');
      });

      const thumbnails = screen.getAllByTestId(/^pdp-thumbnail-\d+$/);
      const urls = thumbnails.map((t) => (t.querySelector('img') as HTMLImageElement).src);

      // Preserves order: black-front, black-back, black-detail
      expect(urls[0]).toContain('black-front.jpg');
      expect(urls[1]).toContain('black-back.jpg');
      expect(urls[2]).toContain('black-detail.jpg');

      // No duplicates
      const uniqueUrls = new Set(urls);
      expect(uniqueUrls.size).toBe(urls.length);
    });
  });
});
