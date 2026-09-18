/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ProductDetail } from './ProductDetail';
import * as publicCatalog from '../api/publicCatalog';
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

    // Before size selection, button indicates size selection is required and is disabled
    const disabledBtn = screen.getByRole('button', { name: 'Выберите размер' });
    expect(disabledBtn.hasAttribute('disabled')).toBe(true);

    // Select size S
    const sizeSBtn = screen.getByRole('button', { name: 'S' });
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

    // Switch color to Красный
    const redColorBtn = screen.getByRole('radio', { name: /Красный/i });
    fireEvent.click(redColorBtn);

    // Active image remains photo 3 (not reset to 0) and total count remains 4
    expect(screen.getByText('3 / 4')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-red-1.jpg');

    // Switch back to Чёрный
    const blackColorBtn = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorBtn);

    // Still photo 3
    expect(screen.getByText('3 / 4')).toBeTruthy();
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

    // Initially color-black is selected by default. Select size M.
    const sizeMBtn = screen.getByRole('button', { name: 'M' });
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

    // In Black color, select size S (in stock)
    const sizeSBtn = screen.getByRole('button', { name: 'S' });
    fireEvent.click(sizeSBtn);
    expect(sizeSBtn.getAttribute('aria-pressed')).toBe('true');

    // Switch to Белый (size S is sold out in White!)
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size S must be cleared, NOT selected
    expect(sizeSBtn.getAttribute('aria-pressed')).toBe('false');

    // No other size must be auto-selected (M must not be pressed)
    const sizeMBtn = screen.getByRole('button', { name: 'M' });
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

    // In Black color, select size L
    const sizeLBtn = screen.getByRole('button', { name: 'L' });
    fireEvent.click(sizeLBtn);
    expect(sizeLBtn.getAttribute('aria-pressed')).toBe('true');

    // Switch to Белый (size L does not exist in White!)
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size must be cleared
    expect(screen.queryByRole('button', { name: 'L' })).toBeNull(); // L doesn't exist for White
    const sizeSBtn = screen.getByRole('button', { name: 'S' });
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
    const sizeSBtn = screen.getByRole('button', { name: 'S' });
    fireEvent.click(sizeSBtn);

    // Switch color to Белый (S is sold out in White)
    const whiteColorRadio = screen.getByRole('radio', { name: /Белый/i });
    fireEvent.click(whiteColorRadio);

    // Size S was cleared with notice, but gallery image MUST remain at 2 / 3
    expect(screen.getByText('2 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-2.jpg');

    // Select size M (buyable)
    const sizeMBtn = screen.getByRole('button', { name: 'M' });
    fireEvent.click(sizeMBtn);

    // Switch back to Чёрный (M is buyable in Black, size retained)
    const blackColorRadio = screen.getByRole('radio', { name: /Чёрный/i });
    fireEvent.click(blackColorRadio);

    // Gallery image still at 2 / 3
    expect(screen.getByText('2 / 3')).toBeTruthy();
    expect((screen.getByAltText('Шёлковое вечернее платье') as HTMLImageElement).src).toContain('dress-2.jpg');
  });
});
