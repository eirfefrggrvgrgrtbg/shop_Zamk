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

vi.mock('../api/publicCatalog', () => ({
  fetchProductById: vi.fn(),
  fetchProductReviews: vi.fn().mockResolvedValue([]),
  fetchProductPreviewByToken: vi.fn(),
}));

const mockApparelProduct: Product = {
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
  image: 'https://example.com/dress-main.jpg',
  images: [
    { url: 'https://example.com/dress-main.jpg', colorId: 'color-black' },
    { url: 'https://example.com/dress-back.jpg', colorId: 'color-black' },
    { url: 'https://example.com/dress-detail.jpg', colorId: 'color-black' },
  ],
  category: 'Вечерние платья',
  description: 'Элегантное шёлковое платье приталенного кроя с открытой спиной.',
  materials: '100% натуральный шёлк mulberry',
  materialComposition: [{ materialName: 'Шёлк Mulberry', percentage: 100 }],
  careInstructions: 'Сухая чистка, гладить при температуре до 110°C.',
  rating: 5.0,
  reviewsCount: 3,
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
      inStock: true,
      isActive: true,
      priceCents: 4500000,
      sellerSku: 'DRS-BLK-M',
    },
  ],
  sizeChart: {
    rows: [
      {
        size: 'S',
        measurements: {
          CHEST: 86,
          WAIST: 66,
          HIPS: 94,
          LENGTH: 120,
        },
      },
      {
        size: 'M',
        measurements: {
          CHEST: 90,
          WAIST: 70,
          HIPS: 98,
          LENGTH: 122,
        },
      },
    ],
  },
};

describe('SHOP PDP.1 Canonical Geometry & Information Hierarchy', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('1. renders product identity, canonical brand, title, and compact rating', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Canonical brand in identity position
    const brandLink = screen.getByTestId('product-brand-link');
    expect(brandLink.textContent).toBe('ZAMK Couture');
    expect(brandLink.getAttribute('href')).toBe('/brand/brand-couture');

    // Title
    expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();

    // Compact rating
    expect(screen.getByText('5,0')).toBeTruthy();
    expect(screen.getByText(/3 отзыва/)).toBeTruthy();

    // Seller is separate in service rows below, NOT faked as brand
    expect(screen.getByText('ZAMK Flagship Store')).toBeTruthy();
    const sellerLink = screen.getByRole('link', { name: 'В магазин →' });
    expect(sellerLink.getAttribute('href')).toBe('/seller/zamk-flagship');
  });

  it('2. hides brand line and does NOT fake it with seller name when brand is absent', async () => {
    const productWithoutBrand: Product = {
      ...mockApparelProduct,
      brand: '',
      brandId: '',
    };
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(productWithoutBrand);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Brand link/text must NOT exist
    expect(screen.queryByTestId('product-brand-link')).toBeNull();
    expect(screen.queryByTestId('product-brand-text')).toBeNull();

    // Seller is still present in the service row below
    expect(screen.getByText('ZAMK Flagship Store')).toBeTruthy();
  });

  it('3. thumbnail rail allows switching the main active image', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Initial image
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-main.jpg');

    // Click on photo 2 thumbnail
    const thumb2Buttons = screen.getAllByLabelText('Фото 2');
    fireEvent.click(thumb2Buttons[0]);

    // Re-query image because key={currentImageUrl} creates a fresh element
    const updatedImg = screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement;
    expect(updatedImg.src).toContain('dress-back.jpg');
  });

  it('4. variant selection enforces size requirement and adds quantity 1 to cart', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Before color/size selection, button indicates color selection is required and is disabled
    const disabledBtn = screen.getByRole('button', { name: 'Выберите цвет' });
    expect(disabledBtn.hasAttribute('disabled')).toBe(true);

    // Select size S is disabled before color selection
    const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
    expect(sizeSBtn.hasAttribute('disabled')).toBe(true);

    // Select color Чёрный
    const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackSwatch);

    // Now button indicates size selection is required
    const sizeRequiredBtn = screen.getByRole('button', { name: 'Выберите размер' });
    expect(sizeRequiredBtn.hasAttribute('disabled')).toBe(true);

    // Select size S
    fireEvent.click(sizeSBtn);

    // Now CTA becomes active "Добавить в корзину"
    const addBtn = screen.getByRole('button', { name: 'Добавить в корзину' });
    expect(addBtn.hasAttribute('disabled')).toBe(false);

    // Click Add to Cart
    fireEvent.click(addBtn);

    // AddItem called with exactly quantity 1
    await waitFor(() => {
      expect(mockAddItem).toHaveBeenCalledWith('prod-dress-100', 'var-black-s', 1);
    });
    expect(mockShowToast).toHaveBeenCalledWith('Товар добавлен в корзину');
  });

  it('5. favorite button toggles favorite state', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    const favBtn = screen.getByLabelText('Добавить в избранное');
    fireEvent.click(favBtn);

    expect(mockToggleFavorite).toHaveBeenCalledWith('prod-dress-100');
  });

  it('6. renders description and characteristics in the lower full-width section', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // О вещи section
    expect(screen.getByRole('heading', { level: 2, name: 'О вещи' })).toBeTruthy();
    expect(screen.getByText('Элегантное шёлковое платье приталенного кроя с открытой спиной.')).toBeTruthy();

    // Состав и уход section
    expect(screen.getByRole('heading', { level: 2, name: 'Состав и уход' })).toBeTruthy();
    expect(screen.getByText(/Шёлк Mulberry — 100%/)).toBeTruthy();
    expect(screen.getByText(/Сухая чистка/)).toBeTruthy();

    // Accordions
    expect(screen.getByText('Характеристики')).toBeTruthy();
    expect(screen.getByText('Размер и посадка')).toBeTruthy();
    expect(screen.getByText('Доставка и возврат')).toBeTruthy();
  });

  it('7. large left and right click zones allow cycling through images and update the counter badge', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Counter badge shows 1 / 3 initially
    expect(screen.getByText('1 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-main.jpg');

    // Click large Right Click Zone
    const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
    fireEvent.click(nextBtn);

    expect(screen.getByText('2 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-back.jpg');

    // Click large Left Click Zone
    const prevBtn = screen.getByRole('button', { name: 'Предыдущее фото' });
    fireEvent.click(prevBtn);

    expect(screen.getByText('1 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-main.jpg');

    // Wrap around to last image
    fireEvent.click(prevBtn);
    expect(screen.getByText('3 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-detail.jpg');
  });

  it('8. opens lightbox on center zone click and allows large-zone navigation, keyboard handling, and closing', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    const { container } = render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Lightbox modal not open initially
    expect(screen.queryByRole('dialog', { name: 'Просмотр фотографии на весь экран' })).toBeNull();

    // Click central zoom zone on main image stage to open lightbox
    const triggerBtn = screen.getByRole('button', { name: 'Открыть изображение на полный экран' });
    fireEvent.click(triggerBtn);

    // Modal dialog is rendered
    const dialog = screen.getByRole('dialog', { name: 'Просмотр фотографии на весь экран' });
    expect(dialog).toBeTruthy();

    // PROVE TRUE PORTAL: Dialog is direct child of document.body and NOT inside PDP container
    expect(dialog.parentElement).toBe(document.body);
    expect(container.contains(dialog)).toBe(false);

    // Body overflow is hidden
    expect(document.body.style.overflow).toBe('hidden');

    // Lightbox has large side navigation zones
    const lightboxNextBtn = screen.getAllByRole('button', { name: 'Следующее фото' })[1];
    fireEvent.click(lightboxNextBtn);

    const counters = screen.getAllByText('2 / 3');
    expect(counters.length).toBeGreaterThan(0);

    // Keyboard navigation in lightbox: ArrowRight
    fireEvent.keyDown(window, { key: 'ArrowRight' });
    const counters3 = screen.getAllByText('3 / 3');
    expect(counters3.length).toBeGreaterThan(0);

    // Close via Escape key
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog', { name: 'Просмотр фотографии на весь экран' })).toBeNull();
    expect(document.body.style.overflow).toBe('');

    // Active image in PDP is retained at index 2 (dress-detail)
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-detail.jpg');
  });

  it('9. single-image product does NOT render thumbnail rail, counter badge, or arrows', async () => {
    const singleImgProduct: Product = {
      ...mockApparelProduct,
      images: [{ url: 'https://example.com/single-dress.jpg' }],
    };
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(singleImgProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Single image rendered
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('single-dress.jpg');

    // No thumbnails
    expect(screen.queryByLabelText('Фото 1')).toBeNull();

    // No arrows or click zones
    expect(screen.queryByRole('button', { name: 'Следующее фото' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Предыдущее фото' })).toBeNull();

    // No counter badge
    expect(screen.queryByText(/1 \//)).toBeNull();

    // Whole image is zoom trigger
    const singleTrigger = screen.getByRole('button', { name: 'Открыть изображение на полный экран' });
    expect(singleTrigger).toBeTruthy();
  });

  it('10. gallery media belongs to the product as a whole and survives color changes without resetting index', async () => {
    const multiColorProduct: Product = {
      ...mockApparelProduct,
      images: [
        { url: 'https://example.com/dress-black-1.jpg', colorId: 'color-black' },
        { url: 'https://example.com/dress-black-2.jpg', colorId: 'color-black' },
        { url: 'https://example.com/dress-red-1.jpg', colorId: 'color-red' },
        { url: 'https://example.com/dress-red-2.jpg', colorId: 'color-red' },
      ],
      variants: [
        {
          id: 'var-blk-s',
          size: 'S',
          color: 'Чёрный',
          colorName: 'Чёрный',
          colorHex: '#000000',
          colorId: 'color-black',
          inStock: true,
          isActive: true,
          priceCents: 4500000,
        },
        {
          id: 'var-red-s',
          size: 'S',
          color: 'Красный',
          colorName: 'Красный',
          colorHex: '#ff0000',
          colorId: 'color-red',
          inStock: true,
          isActive: true,
          priceCents: 4500000,
        },
      ],
    };

    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiColorProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // All 4 photos are present in one product-level stream
    expect(screen.getByText('1 / 4')).toBeTruthy();

    // Navigate to photo 3
    const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
    fireEvent.click(nextBtn);
    fireEvent.click(nextBtn);
    expect(screen.getByText('3 / 4')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-red-1.jpg');

    // Switch color to Красный: focuses first red photo (photo 3)
    const redColorBtn = screen.getByRole('radio', { name: /Красный/i });
    fireEvent.click(redColorBtn);

    // Active image focuses first Красный photo (photo 3) and total count remains 4
    expect(screen.getByText('3 / 4')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-red-1.jpg');

    // Switch back to Чёрный: focuses first black photo (photo 1)
    const blackColorBtn = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorBtn);

    // Active image focuses first Чёрный photo (photo 1) while total count remains 4
    expect(screen.getByText('1 / 4')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-black-1.jpg');
  });

  it('11. lightbox real zoom toggles on image click, allows dragging, and resets on photo navigation', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Open lightbox
    const triggerBtn = screen.getByRole('button', { name: 'Открыть изображение на полный экран' });
    fireEvent.click(triggerBtn);

    const dialog = screen.getByRole('dialog', { name: 'Просмотр фотографии на весь экран' });
    expect(dialog).toBeTruthy();

    // The lightbox image
    const lbImg = dialog.querySelector('img') as HTMLImageElement;
    expect(lbImg).toBeTruthy();

    // Initially fitted: scale(1) and cursor-zoom-in
    expect(lbImg.style.transform).toContain('scale(1)');
    expect(lbImg.className).toContain('cursor-zoom-in');

    // Click image: enters zoomed state scale(2) and cursor-grab
    fireEvent.click(lbImg);
    expect(lbImg.style.transform).toContain('scale(2)');
    expect(lbImg.className).toContain('cursor-grab');

    // Drag / Pan simulation
    fireEvent.pointerDown(lbImg, { clientX: 100, clientY: 100, pointerId: 1 });
    fireEvent.pointerMove(lbImg, { clientX: 50, clientY: 40, pointerId: 1 });
    fireEvent.pointerUp(lbImg, { clientX: 50, clientY: 40, pointerId: 1 });

    // Panning offset is applied in transform
    expect(lbImg.style.transform).toContain('scale(2)');
    expect(lbImg.style.transform).toContain('translate3d(-50px, -60px, 0px)');

    // Navigate to next photo: resets zoom back to scale(1) and center pan
    const lightboxNextBtn = screen.getAllByRole('button', { name: 'Следующее фото' })[1];
    fireEvent.click(lightboxNextBtn);

    expect(lbImg.style.transform).toBe('translate3d(0px, 0px, 0px) scale(1)');
    expect(lbImg.className).toContain('cursor-zoom-in');
  });
});

describe('SHOP PDP.2B Variant Selection State Hardening', () => {
  const mockMultiColorSizeProduct: Product = {
    ...mockApparelProduct,
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
      },
      {
        id: 'var-black-m',
        size: 'M',
        color: 'Чёрный',
        colorName: 'Чёрный',
        colorHex: '#000000',
        colorId: 'color-black',
        inStock: true,
        isActive: true,
        priceCents: 4500000,
      },
      {
        id: 'var-white-s',
        size: 'S',
        color: 'Белый',
        colorName: 'Белый',
        colorHex: '#ffffff',
        colorId: 'color-white',
        inStock: false, // SOLD OUT in White!
        isActive: true,
        priceCents: 4800000,
      },
      {
        id: 'var-white-m',
        size: 'M',
        color: 'Белый',
        colorName: 'Белый',
        colorHex: '#ffffff',
        colorId: 'color-white',
        inStock: true, // Buyable in White!
        isActive: true,
        priceCents: 4800000,
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('preserves selected size across color switch when size is buyable in the new color, updates variant and adds exact variant to cart', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockMultiColorSizeProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Explicitly select Чёрный first
    const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorRadio);

    // Select size M.
    const sizeMBtn = screen.getByRole('button', { name: /^Размер M/i });
    fireEvent.click(sizeMBtn);

    // Switch to Белый
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size M should still be selected
    expect(sizeMBtn.getAttribute('aria-pressed')).toBe('true');

    // No warning notice should be displayed
    expect(screen.queryByRole('status')).toBeNull();

    // CTA should be ready
    const addBtn = screen.getByRole('button', { name: 'Добавить в корзину' });
    expect(addBtn.hasAttribute('disabled')).toBe(false);

    // Clicking Add to Cart must send the WHITE M variant id ('var-white-m')
    fireEvent.click(addBtn);
    await waitFor(() => {
      expect(mockAddItem).toHaveBeenCalledWith('prod-dress-100', 'var-white-m', 1);
    });
  });

  it('clears size selection, shows contextual notice, and does NOT auto-select when size is sold out in new color', async () => {
    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockMultiColorSizeProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Explicitly select Чёрный first
    const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorRadio);

    // In Black color, select size S (in stock)
    const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
    fireEvent.click(sizeSBtn);
    expect(sizeSBtn.getAttribute('aria-pressed')).toBe('true');

    // Switch to Белый (size S is sold out in White!)
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size S must be cleared, NOT selected, but still visible and disabled
    expect(sizeSBtn.getAttribute('aria-pressed')).toBe('false');
    expect(sizeSBtn.hasAttribute('disabled')).toBe(true);

    // No other size must be auto-selected (M must not be pressed)
    const sizeMBtn = screen.getByRole('button', { name: /^Размер M/i });
    expect(sizeMBtn.getAttribute('aria-pressed')).toBe('false');

    // Contextual notice must be visible
    const notice = screen.getByRole('status');
    expect(notice).toBeTruthy();
    expect(notice.textContent).toBe('Размер S недоступен в белом цвете');

    // CTA must be disabled ("Выберите размер")
    const disabledBtn = screen.getByRole('button', { name: 'Выберите размер' });
    expect(disabledBtn.hasAttribute('disabled')).toBe(true);

    // Clicking an available size (M) clears notice and enables CTA
    fireEvent.click(sizeMBtn);
    expect(screen.queryByRole('status')).toBeNull();
    const activeCta = screen.getByRole('button', { name: 'Добавить в корзину' });
    expect(activeCta.hasAttribute('disabled')).toBe(false);
  });

  it('clears size selection and shows contextual notice when combination does not exist in new color', async () => {
    const productWithMissingCombo: Product = {
      ...mockApparelProduct,
      variants: [
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
        },
        {
          id: 'var-white-s',
          size: 'S',
          color: 'Белый',
          colorName: 'Белый',
          colorHex: '#ffffff',
          colorId: 'color-white',
          inStock: true,
          isActive: true,
          priceCents: 4800000,
        },
      ],
    };

    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(productWithMissingCombo);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Explicitly select Чёрный first
    const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorRadio);

    // In Black color, select size L
    const sizeLBtn = screen.getByRole('button', { name: /^Размер L/i });
    fireEvent.click(sizeLBtn);
    expect(sizeLBtn.getAttribute('aria-pressed')).toBe('true');

    // Switch to Белый (size L does not exist in White!)
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size L remains visible in the stable matrix, but disabled as NOT_OFFERED
    expect(sizeLBtn.getAttribute('aria-pressed')).toBe('false');
    expect(sizeLBtn.hasAttribute('disabled')).toBe(true);
    expect(sizeLBtn.getAttribute('data-state')).toBe('NOT_OFFERED');

    const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
    expect(sizeSBtn.getAttribute('aria-pressed')).toBe('false'); // S must NOT be auto-selected

    // Notice shown
    const notice = screen.getByRole('status');
    expect(notice.textContent).toBe('Размер L недоступен в белом цвете');
  });

  it('switching colors while retaining or clearing size preserves gallery media stream and active image', async () => {
    const multiMediaProduct: Product = {
      ...mockMultiColorSizeProduct,
      images: [
        { url: 'https://example.com/dress-1.jpg', colorId: 'color-black' },
        { url: 'https://example.com/dress-2.jpg', colorId: 'color-black' },
        { url: 'https://example.com/dress-3.jpg', colorId: 'color-white' },
      ],
    };

    vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaProduct);

    render(
      <MemoryRouter initialEntries={['/product/prod-dress-100']}>
        <Routes>
          <Route path="/product/:id" element={<ProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
    });

    // Advance gallery to photo 2
    const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
    fireEvent.click(nextBtn);
    expect(screen.getByText('2 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-2.jpg');

    // Select size S
    const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
    fireEvent.click(sizeSBtn);

    // Switch color to Белый (S is sold out in White; focuses White photo at 3 / 3)
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size S was cleared with notice, and gallery image focuses White photo at 3 / 3
    expect(screen.getByText('3 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-3.jpg');

    // Select size M (buyable; size selection does not affect active image)
    const sizeMBtn = screen.getByRole('button', { name: /^Размер M/i });
    fireEvent.click(sizeMBtn);
    expect(screen.getByText('3 / 3')).toBeTruthy();

    // Switch back to Чёрный (M is buyable in Black, size retained; focuses first Black photo at 1 / 3)
    const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorRadio);

    // Gallery image focuses first Black photo at 1 / 3 while size M remains selected
    expect(screen.getByText('1 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-1.jpg');
  });

  describe('SHOP PDP.2D1 — Stale Stock Recovery After Add-to-Cart Failure', () => {
    it('1. Successful Add: item added, no stale refetch, no stale warning', async () => {
      mockAddItem.mockResolvedValueOnce(undefined);
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      // Select size S
      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      // Click Add to Cart
      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(mockAddItem).toHaveBeenCalledTimes(1);
      });

      // Exactly 1 loadProduct on mount, NO refetch on successful add
      expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(1);
      expect(mockShowToast).toHaveBeenCalledWith('Товар добавлен в корзину');
      expect(screen.queryByRole('status')).toBeNull();
    });

    it('2 & 3. Insufficient stock recognized structurally: triggers 1 refresh, clears size, keeps in stable matrix as SOLD_OUT, shows notice, no auto-select', async () => {
      const insufficientStockErr = new ApiError(
        'Недостаточно товара на складе',
        'invalid_item',
        400,
        { error: { code: 'invalid_item', message: 'insufficient stock' } },
        'insufficient stock'
      );
      mockAddItem.mockRejectedValueOnce(insufficientStockErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      // Refreshed product where S is now SOLD_OUT (inStock: false)
      const refreshedProduct: Product = {
        ...mockApparelProduct,
        variants: mockApparelProduct.variants?.map((v) =>
          v.id === 'var-black-s' ? { ...v, inStock: false } : v
        ),
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(refreshedProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      // Select size S
      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);
      expect(sizeSBtn.getAttribute('aria-pressed')).toBe('true');

      // Click Add to Cart
      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(mockAddItem).toHaveBeenCalledTimes(1);
        // Product was refetched (1 on mount + 1 stale recovery)
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Selected size S must be cleared
      await waitFor(() => {
        expect(sizeSBtn.getAttribute('aria-pressed')).toBe('false');
      });

      // Size S remains visible in the stable matrix, but transitioned in-place to SOLD_OUT
      expect(sizeSBtn.hasAttribute('disabled')).toBe(true);
      expect(sizeSBtn.getAttribute('data-state')).toBe('SOLD_OUT');
      expect(sizeSBtn.getAttribute('aria-label')).toBe('Размер S, закончился');

      // Other sizes must NOT be auto-selected
      const sizeMBtn = screen.getByRole('button', { name: /^Размер M/i });
      expect(sizeMBtn.getAttribute('aria-pressed')).toBe('false');

      // Contextual message shown
      const notice = screen.getByRole('status');
      expect(notice).toBeTruthy();
      expect(notice.textContent).toBe('Размер S только что закончился. Выберите другой размер.');
      expect(mockShowToast).toHaveBeenCalledWith('Размер S только что закончился. Выберите другой размер.');

      // Can pick another size (M) immediately; notice is cleared and button becomes active
      fireEvent.click(sizeMBtn);
      expect(sizeMBtn.getAttribute('aria-pressed')).toBe('true');
      expect(screen.queryByRole('status')).toBeNull();
    });

    it('4. Refreshed selected size remains buyable: selection is preserved and fresh truth used', async () => {
      const insufficientStockErr = new ApiError(
        'Недостаточно товара на складе',
        'invalid_item',
        400,
        { error: { code: 'invalid_item', message: 'insufficient stock' } },
        'insufficient stock'
      );
      mockAddItem.mockRejectedValueOnce(insufficientStockErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      // Refreshed product where S is still inStock (e.g. price updated to 48000)
      const refreshedProduct: Product = {
        ...mockApparelProduct,
        variants: mockApparelProduct.variants?.map((v) =>
          v.id === 'var-black-s' ? { ...v, inStock: true, priceCents: 4800000 } : v
        ),
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(refreshedProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      // Select size S
      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);
      expect(sizeSBtn.getAttribute('aria-pressed')).toBe('true');

      // Click Add to Cart
      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Since S is still buyable in refreshedProduct, selection remains
      expect(sizeSBtn.getAttribute('aria-pressed')).toBe('true');
      expect(screen.queryByRole('status')).toBeNull();
      // Fresh price reflected
      expect(screen.getByText(/48\s?000/)).toBeTruthy();
    });

    it('5. Product drops below public visibility and refresh returns 404: PDP does not crash, purchasing disabled, product-level notice shown', async () => {
      const insufficientStockErr = new ApiError(
        'Недостаточно товара на складе',
        'invalid_item',
        400,
        { error: { code: 'invalid_item', message: 'insufficient stock' } },
        'insufficient stock'
      );
      mockAddItem.mockRejectedValueOnce(insufficientStockErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      // Refresh throws 404 Not Found (product fell below free stock threshold and became hidden)
      const notFoundErr = new ApiError('Product not found', 'not_found', 404);
      vi.mocked(publicCatalog.fetchProductById).mockRejectedValueOnce(notFoundErr);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Page must NOT crash or show generic 404 page
      expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();

      // Purchasing disabled
      const cta = screen.getByRole('button', { name: /Товар закончился/i });
      expect(cta.hasAttribute('disabled')).toBe(true);

      // Product-level notice shown
      const notice = screen.getByRole('status');
      expect(notice.textContent).toBe('Товар только что закончился.');
      expect(mockShowToast).toHaveBeenCalledWith('Товар только что закончился.');

      // Clicking old controls does NOT clear product-level unavailable state
      fireEvent.click(sizeSBtn);
      expect(screen.getByRole('status').textContent).toBe('Товар только что закончился.');
    });

    it('6. Network/server refresh failure: not falsely labeled sold out, restrained message shown', async () => {
      const insufficientStockErr = new ApiError(
        'Недостаточно товара на складе',
        'invalid_item',
        400,
        { error: { code: 'invalid_item', message: 'insufficient stock' } },
        'insufficient stock'
      );
      mockAddItem.mockRejectedValueOnce(insufficientStockErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      // Refresh throws network error
      const netErr = new ApiError('Network error', 'NETWORK_ERROR', 0);
      vi.mocked(publicCatalog.fetchProductById).mockRejectedValueOnce(netErr);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Not labeled sold out!
      expect(screen.queryByText('Товар только что закончился.')).toBeNull();

      // Restrained message asking user to retry/refresh
      expect(mockShowToast).toHaveBeenCalledWith('Не удалось обновить данные о наличии. Попробуйте обновить страницу.');
    });

    it('7. Unrelated Add-to-Cart error: does NOT trigger stale-stock refresh', async () => {
      const serverErr = new ApiError('Internal server error', 'internal_error', 500);
      mockAddItem.mockRejectedValueOnce(serverErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(mockAddItem).toHaveBeenCalledTimes(1);
      });

      // Only initial mount fetch, NO stale refetch
      expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(1);
      expect(mockShowToast).toHaveBeenCalledWith('Internal server error');
    });

    it('8. Exact gallery index is unchanged throughout stale recovery', async () => {
      const insufficientStockErr = new ApiError(
        'Недостаточно товара на складе',
        'invalid_item',
        400,
        { error: { code: 'invalid_item', message: 'insufficient stock' } },
        'insufficient stock'
      );
      mockAddItem.mockRejectedValueOnce(insufficientStockErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(mockApparelProduct);

      const refreshedProduct: Product = {
        ...mockApparelProduct,
        variants: mockApparelProduct.variants?.map((v) =>
          v.id === 'var-black-s' ? { ...v, inStock: false } : v
        ),
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(refreshedProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-100']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Explicitly select Чёрный first
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      // Navigate to photo 2
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn);
      expect(screen.getByText('2 / 3')).toBeTruthy();
      expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-back.jpg');

      // Select size S and click Add to Cart
      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Gallery index must remain EXACTLY at photo 2 (black photo)
      expect(screen.getByText('2 / 3')).toBeTruthy();
      expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-back.jpg');
    });

    it('9 & 11. COLOR_ONLY product stale recovery: no size axis, stays on color, CTA becomes unavailable, strict inStock truth enforced', async () => {
      const colorOnlyProduct: Product = {
        id: 'prod-color-only',
        name: 'Шарф кашемировый',
        brand: 'ZAMK Couture',
        brandId: 'brand-couture',
        category: 'Шарфы',
        price: 15000,
        image: 'https://example.com/scarf.jpg',
        variants: [
          {
            id: 'var-scarf-black',
            colorId: 'c-black',
            colorName: 'Чёрный',
            inStock: true,
            isActive: true,
            priceCents: 1500000,
          },
        ],
      };

      const insufficientStockErr = new ApiError(
        'Недостаточно товара на складе',
        'invalid_item',
        400,
        { error: { code: 'invalid_item', message: 'insufficient stock' } },
        'insufficient stock'
      );
      mockAddItem.mockRejectedValueOnce(insufficientStockErr);

      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(colorOnlyProduct);

      const refreshedScarf: Product = {
        ...colorOnlyProduct,
        variants: [
          {
            ...colorOnlyProduct.variants![0],
            inStock: false,
          },
        ],
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(refreshedScarf);

      render(
        <MemoryRouter initialEntries={['/product/prod-color-only']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шарф кашемировый' })).toBeTruthy();
      });

      // Explicitly select Чёрный color
      const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackColorRadio);

      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Notice shown for no size axis
      expect(mockShowToast).toHaveBeenCalledWith('Этот вариант только что закончился.');
      const notice = screen.getByRole('status');
      expect(notice.textContent).toBe('Этот вариант только что закончился.');

      // CTA becomes "Нет в наличии"
      const cta = screen.getByRole('button', { name: /Нет в наличии/i });
      expect(cta.hasAttribute('disabled')).toBe(true);
    });
  });

  describe('SHOP PDP.2E1 — Color-Aware Media Focus', () => {
    const multiMediaColorProduct: Product = {
      ...mockApparelProduct,
      id: 'prod-dress-media-focus',
      name: 'Платье с цветной галереей',
      images: [
        { url: 'https://example.com/general-hero.jpg' }, // 0: general hero
        { url: 'https://example.com/black-1.jpg', colorId: 'color-black' }, // 1: black #1
        { url: 'https://example.com/white-1.jpg', colorId: 'color-white' }, // 2: white #1
        { url: 'https://example.com/black-2.jpg', colorId: 'color-black' }, // 3: black #2
        { url: 'https://example.com/general-fabric.jpg' }, // 4: general detail
      ],
      variants: [
        {
          id: 'var-blk-s',
          size: 'S',
          color: 'Чёрный',
          colorName: 'Чёрный',
          colorHex: '#000000',
          colorId: 'color-black',
          inStock: true,
          isActive: true,
          priceCents: 4500000,
        },
        {
          id: 'var-blk-m',
          size: 'M',
          color: 'Чёрный',
          colorName: 'Чёрный',
          colorHex: '#000000',
          colorId: 'color-black',
          inStock: true,
          isActive: true,
          priceCents: 4500000,
        },
        {
          id: 'var-wht-s',
          size: 'S',
          color: 'Белый',
          colorName: 'Белый',
          colorHex: '#ffffff',
          colorId: 'color-white',
          inStock: false, // sold out in white for retention test
          isActive: true,
          priceCents: 4500000,
        },
        {
          id: 'var-wht-m',
          size: 'M',
          color: 'Белый',
          colorName: 'Белый',
          colorHex: '#ffffff',
          colorId: 'color-white',
          inStock: true,
          isActive: true,
          priceCents: 4500000,
        },
        {
          id: 'var-grn-s',
          size: 'S',
          color: 'Зелёный',
          colorName: 'Зелёный',
          colorHex: '#00aa00',
          colorId: 'color-green',
          inStock: true,
          isActive: true,
          priceCents: 4500000,
        },
      ],
    };

    it('1. Initial load: no color selected on clean load and GENERAL image remains active', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // On clean load, no color swatch is selected
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      expect(blackSwatch.getAttribute('aria-checked')).toBe('false');
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      expect(whiteSwatch.getAttribute('aria-checked')).toBe('false');
      const unselectedLabels = screen.getAllByText('Не выбран');
      expect(unselectedLabels.length).toBeGreaterThan(0);

      // Active image remains the first GENERAL hero photo (index 0, '1 / 5')
      expect(screen.getByText('1 / 5')).toBeTruthy();
      const mainImg = screen.getByAltText('Платье с цветной галереей') as HTMLImageElement;
      expect(mainImg.src).toContain('general-hero.jpg');
    });

    it('2. Explicit BLACK click: focuses first BLACK-tagged image', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Initially on photo 1 (general hero)
      expect(screen.getByText('1 / 5')).toBeTruthy();

      // Explicitly click Чёрный swatch
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);

      // Jumps to photo 2 (black-1.jpg)
      expect(screen.getByText('2 / 5')).toBeTruthy();
      const mainImg = screen.getByAltText('Платье с цветной галереей') as HTMLImageElement;
      expect(mainImg.src).toContain('black-1.jpg');
    });

    it('3. Explicit WHITE click: focuses first WHITE-tagged image', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Explicitly click Белый swatch
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);

      // Jumps to photo 3 (white-1.jpg)
      expect(screen.getByText('3 / 5')).toBeTruthy();
      const mainImg = screen.getByAltText('Платье с цветной галереей') as HTMLImageElement;
      expect(mainImg.src).toContain('white-1.jpg');
    });

    it('4. Gallery content count and order does not change after color selection', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Select White: active jumps to photo 3
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);
      expect(screen.getByText('3 / 5')).toBeTruthy();

      // Total count remains 5 in thumbnail strip and counter badge
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn);
      expect(screen.getByText('4 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('black-2.jpg');

      fireEvent.click(nextBtn);
      expect(screen.getByText('5 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('general-fabric.jpg');

      fireEvent.click(nextBtn);
      expect(screen.getByText('1 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('general-hero.jpg');
    });

    it('5. After BLACK focus, user can manually navigate to WHITE/general images', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Focus Black (photo 2)
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);
      expect(screen.getByText('2 / 5')).toBeTruthy();

      // Browse to White photo (photo 3)
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn);
      expect(screen.getByText('3 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('white-1.jpg');
    });

    it('6. Re-click current BLACK: returns to first BLACK-tagged image', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);
      expect(screen.getByText('2 / 5')).toBeTruthy();

      // Browse forward to photo 5 (general fabric)
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn); // 3
      fireEvent.click(nextBtn); // 4
      fireEvent.click(nextBtn); // 5
      expect(screen.getByText('5 / 5')).toBeTruthy();

      // Re-click current Black swatch
      fireEvent.click(blackSwatch);

      // Focus returns to first Black image (photo 2, 2 / 5)
      expect(screen.getByText('2 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('black-1.jpg');
    });

    it('7. Color without tagged image: falls back to GENERAL image', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // User first moves to photo 3 (White)
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);
      expect(screen.getByText('3 / 5')).toBeTruthy();

      // User selects Зелёный (has no tagged images, but general images exist)
      const greenSwatch = screen.getByRole('radio', { name: /Зелёный/i });
      fireEvent.click(greenSwatch);

      // Focuses first GENERAL image (photo 1, general-hero.jpg)
      expect(screen.getByText('1 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('general-hero.jpg');
    });

    it('8. Size selection: does not affect active image', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Go to photo 3 (White)
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);
      expect(screen.getByText('3 / 5')).toBeTruthy();

      // Select size M
      const sizeMBtn = screen.getByRole('button', { name: /^Размер M/i });
      fireEvent.click(sizeMBtn);

      // Still photo 3
      expect(screen.getByText('3 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('white-1.jpg');
    });

    it('9. PDP.2B size-retention color logic still works while media focus also occurs', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Select Black color first
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);

      // Select Size S in Black
      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      // Switch color to Белый: S is sold out in White
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);

      // Media focused to White photo (photo 3)
      expect(screen.getByText('3 / 5')).toBeTruthy();
      // Size S was cleared with contextual notice
      expect(screen.getByRole('status').textContent).toBe('Размер S недоступен в белом цвете');

      // Now select Size M (available in White and Black)
      const sizeMBtn = screen.getByRole('button', { name: /^Размер M/i });
      fireEvent.click(sizeMBtn);

      // Switch back to Чёрный: M is available in Black -> size retained
      const blackSwatchAgain = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatchAgain);

      // Media focused to first Black photo (photo 2)
      expect(screen.getByText('2 / 5')).toBeTruthy();
      // Size M is retained
      const sizeMBtnAfter = screen.getByRole('button', { name: /^Размер M/i });
      expect(sizeMBtnAfter.getAttribute('aria-pressed')).toBe('true');
      expect(screen.queryByRole('status')).toBeNull();
    });

    it('10. Stale-stock refresh: does NOT trigger color-media refocus', async () => {
      mockAddItem.mockRejectedValueOnce(
        new ApiError('Недостаточно товара на складе', 'insufficient_stock', 409)
      );
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      const refreshedProduct: Product = {
        ...multiMediaColorProduct,
        variants: multiMediaColorProduct.variants?.map((v) =>
          v.id === 'var-blk-s' ? { ...v, inStock: false } : v
        ),
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(refreshedProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // Select Black color first
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);
      expect(screen.getByText('2 / 5')).toBeTruthy();

      // Move to photo 4 (black-2.jpg)
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn); // 3
      fireEvent.click(nextBtn); // 4
      expect(screen.getByText('4 / 5')).toBeTruthy();

      // Select size S
      const sizeSBtn = screen.getByRole('button', { name: /^Размер S/i });
      fireEvent.click(sizeSBtn);

      // Still photo 4
      expect(screen.getByText('4 / 5')).toBeTruthy();

      // Click Add to Cart -> triggers stale stock recovery
      const addBtn = screen.getByRole('button', { name: /Добавить в корзину/i });
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(publicCatalog.fetchProductById).toHaveBeenCalledTimes(2);
      });

      // Active image MUST still be photo 4 (4 / 5) - not refocused to photo 1 or photo 2!
      expect(screen.getByText('4 / 5')).toBeTruthy();
      expect(((screen.getByAltText('Платье с цветной галереей') as HTMLImageElement).src)).toContain('black-2.jpg');
    });

    it('11. Thumbnail active state and counter follow focused image', async () => {
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(multiMediaColorProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-dress-media-focus']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Платье с цветной галереей' })).toBeTruthy();
      });

      // On initial load: thumbnail 1 is active
      const thumb1 = screen.getAllByRole('button', { name: 'Фото 1' })[0];
      expect(thumb1.className).toContain('border-graphite');

      // Click White: focuses photo 3
      const whiteSwatch = screen.getByRole('radio', { name: /Белый/i });
      fireEvent.click(whiteSwatch);

      expect(screen.getByText('3 / 5')).toBeTruthy();
      const thumb3 = screen.getAllByRole('button', { name: 'Фото 3' })[0];
      expect(thumb3.className).toContain('border-graphite');
      expect(thumb1.className).not.toContain('border-graphite');
    });

    it('12. Single-image product remains correct', async () => {
      const singleImgProduct: Product = {
        ...mockApparelProduct,
        id: 'prod-single',
        images: [{ url: 'https://example.com/single-hero.jpg' }],
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(singleImgProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-single']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // No thumbnails or counter badge
      expect(screen.queryByText(/1 \/ 1/)).toBeNull();
      expect(screen.queryByRole('button', { name: 'Фото 1' })).toBeNull();
      expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('single-hero.jpg');
    });

    it('13. Product with no color-tagged images behaves exactly like normal general gallery', async () => {
      const allGeneralProduct: Product = {
        ...mockApparelProduct,
        id: 'prod-all-general',
        images: [
          { url: 'https://example.com/gen-1.jpg' },
          { url: 'https://example.com/gen-2.jpg' },
          { url: 'https://example.com/gen-3.jpg' },
        ],
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(allGeneralProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-all-general']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      expect(screen.getByText('1 / 3')).toBeTruthy();

      // Click color swatch -> falls back to first general image (1 / 3)
      const blackSwatch = screen.getByRole('radio', { name: /Чёрный/i });
      fireEvent.click(blackSwatch);
      expect(screen.getByText('1 / 3')).toBeTruthy();

      // User navigates normally
      const nextBtn = screen.getByRole('button', { name: 'Следующее фото' });
      fireEvent.click(nextBtn);
      expect(screen.getByText('2 / 3')).toBeTruthy();
      expect(((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src)).toContain('gen-2.jpg');
    });

    it('14. Last-resort fallback: all photos are color-tagged and selected color has no match', async () => {
      const allColoredProduct: Product = {
        ...mockApparelProduct,
        id: 'prod-all-colored',
        images: [
          { url: 'https://example.com/red-1.jpg', colorId: 'color-red' },
          { url: 'https://example.com/blue-1.jpg', colorId: 'color-blue' },
        ],
        variants: [
          {
            id: 'var-red',
            color: 'Красный',
            colorName: 'Красный',
            colorHex: '#ff0000',
            colorId: 'color-red',
            inStock: true,
            isActive: true,
          },
          {
            id: 'var-blue',
            color: 'Синий',
            colorName: 'Синий',
            colorHex: '#0000ff',
            colorId: 'color-blue',
            inStock: true,
            isActive: true,
          },
          {
            id: 'var-yellow',
            color: 'Жёлтый',
            colorName: 'Жёлтый',
            colorHex: '#ffff00',
            colorId: 'color-yellow',
            inStock: true,
            isActive: true,
          },
        ],
      };
      vi.mocked(publicCatalog.fetchProductById).mockResolvedValueOnce(allColoredProduct);

      render(
        <MemoryRouter initialEntries={['/product/prod-all-colored']}>
          <Routes>
            <Route path="/product/:id" element={<ProductDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1, name: 'Шёлковое вечернее платье' })).toBeTruthy();
      });

      // Move to blue photo (photo 2)
      const blueSwatch = screen.getByRole('radio', { name: /Синий/i });
      fireEvent.click(blueSwatch);
      expect(screen.getByText('2 / 2')).toBeTruthy();

      // Click Жёлтый (no yellow photos and no uncolored photos) -> falls back to photo 1 (index 0)
      const yellowSwatch = screen.getByRole('radio', { name: /Жёлтый/i });
      fireEvent.click(yellowSwatch);

      expect(screen.getByText('1 / 2')).toBeTruthy();
      expect(((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src)).toContain('red-1.jpg');
    });
  });
});
