/* @vitest-environment jsdom */
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductPresentationCore } from "@zamk/shared";

const mockProduct = {
  id: 'product-1',
  name: 'Test Product',
  price: 500000, // 5000 rubles
  brand: 'Test Brand',
  rating: 4.5,
  sizeChart: { rows: [] }
};

const mockVisibleImages = [
  { url: 'https://example.com/img1.jpg', colorId: undefined },
  { url: 'https://example.com/img2.jpg', colorId: 'color-1' }
];

const defaultProps = {
  product: mockProduct as any,
  visibleImages: mockVisibleImages,
  activeImage: 0,
  onActiveImageChange: vi.fn(),
  displayPrice: 5000,
  colors: [
    { id: 'color-1', name: 'Black', hex: '#000000', hasInStock: true },
    { id: 'color-2', name: 'White', hex: '#FFFFFF', hasInStock: false }
  ],
  sizes: [
    { id: 'size-1', label: 'S', state: 'AVAILABLE' as const },
    { id: 'size-2', label: 'M', state: 'SOLD_OUT' as const },
    { id: 'size-3', label: 'L', state: 'NOT_OFFERED' as const }
  ],
  selectedColorId: null,
  selectedSizeId: null,
  selectedColor: null,
  selectedSize: null,
  selectedVariant: null,
  isResolved: false,
  canAddToCart: false,
  requiresColor: true,
  requiresSize: true,
  isAddingToCart: false,
  isProductUnavailable: false,
  ctaText: 'Добавить в корзину',
  sizeSelectionNotice: null,
  refreshErrorNotice: null,
  sizeError: '',
  reviewsCountText: '5 отзывов',
  onColorChange: vi.fn(),
  onSizeChange: vi.fn(),
  onAddToCart: vi.fn(),
  isFavorite: false,
  onToggleFavorite: vi.fn(),
  onScrollToReviews: vi.fn()
};

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('ProductPresentationCore', () => {
  afterEach(() => {
    cleanup();
  });
  it('renders identity hierarchy (brand, title, rating)', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    expect(screen.queryByText('Test Brand')).not.toBeNull();
    expect(screen.queryByText('Test Product')).not.toBeNull();
    expect(screen.queryByText('4,5')).not.toBeNull();
    expect(screen.queryByText('5 отзывов')).not.toBeNull();
  });

  it('shows supplied active image', () => {
    const { rerender } = renderWithRouter(<ProductPresentationCore {...defaultProps} activeImage={1} />);
    const mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/img2.jpg');
  });

  it('thumbnail click emits correct image index', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    const thumb1 = screen.getByTestId('pdp-thumbnail-1');
    fireEvent.click(thumb1);
    expect(defaultProps.onActiveImageChange).toHaveBeenCalledWith(1);
  });

  it('gallery previous/next emits index updates', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    const nextBtn = screen.getByLabelText('Следующее фото');
    fireEvent.click(nextBtn);
    expect(defaultProps.onActiveImageChange).toHaveBeenCalledWith(1);
  });

  it('color click emits canonical color ID', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    const swatch = screen.getByTestId('color-swatch-color-1');
    fireEvent.click(swatch);
    expect(defaultProps.onColorChange).toHaveBeenCalledWith('color-1');
  });

  it('size click emits canonical size ID', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    const sizeBtn = screen.getByTestId('size-button-S');
    fireEvent.click(sizeBtn);
    expect(defaultProps.onSizeChange).toHaveBeenCalledWith('size-1');
  });

  it('renders AVAILABLE / SOLD_OUT / NOT_OFFERED states correctly', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    expect(screen.getByTestId('size-button-S').getAttribute('data-state')).toBe('AVAILABLE');
    expect(screen.getByTestId('size-button-M').getAttribute('data-state')).toBe('SOLD_OUT');
    expect(screen.getByTestId('size-button-L').getAttribute('data-state')).toBe('NOT_OFFERED');
  });

  it('CTA invokes callback only', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} isResolved={true} canAddToCart={true} />);
    const cta = screen.getByTestId('add-to-cart-button');
    fireEvent.click(cta);
    expect(defaultProps.onAddToCart).toHaveBeenCalled();
  });

  it('lower product information renders', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} product={{...mockProduct, description: 'Test Description', materials: 'Cotton'} as any} />);
    expect(screen.queryByText('Test Description')).not.toBeNull();
    expect(screen.queryByText('Cotton')).not.toBeNull();
  });

  it('lightbox opens correctly', () => {
    renderWithRouter(<ProductPresentationCore {...defaultProps} />);
    const trigger = screen.getAllByLabelText('Открыть изображение на полный экран')[0];
    fireEvent.click(trigger);
    expect(screen.queryByRole('dialog', { name: 'Просмотр фотографии на весь экран' })).not.toBeNull();
  });

  describe('size chart (CATALOG.C1)', () => {
    const chart = {
      fields: [
        { code: 'CHEST', name: 'Обхват груди', unit: 'см' },
        { code: 'LENGTH', name: 'Длина изделия', unit: 'см' },
      ],
      rows: [
        { sizeValueId: 'size-1', sizeValueName: 'S', measurements: { CHEST: 100, LENGTH: 68 } },
        { sizeValueId: 'size-2', sizeValueName: 'M', measurements: { CHEST: 104.5 } },
      ],
    };

    it('shows no size-chart action when the product has no measurements', () => {
      renderWithRouter(<ProductPresentationCore {...defaultProps} />);
      expect(screen.queryByRole('button', { name: 'Таблица размеров' })).toBeNull();
    });

    it('opens the table with garment measurements, units and the explanation', () => {
      renderWithRouter(<ProductPresentationCore {...defaultProps} product={{ ...mockProduct, sizeChart: chart } as any} />);
      fireEvent.click(screen.getByRole('button', { name: 'Таблица размеров' }));
      expect(screen.getByText('Обхват груди, см')).not.toBeNull();
      expect(screen.getByText('104.5')).not.toBeNull();
      expect(screen.getAllByText('—').length).toBe(1); // M has no LENGTH: no fake value
      expect(screen.getByText(/не параметры тела/)).not.toBeNull();
    });

    it('highlights only the row of the selected size', () => {
      renderWithRouter(
        <ProductPresentationCore
          {...defaultProps}
          product={{ ...mockProduct, sizeChart: chart } as any}
          selectedSizeId="size-2"
          selectedSize={{ id: 'size-2', label: 'M', state: 'SOLD_OUT' } as any}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: 'Таблица размеров' }));
      const rows = screen.getAllByTestId('size-chart-row');
      expect(rows.map((r) => r.getAttribute('data-selected'))).toEqual([null, 'true']);
    });

    it('highlights nothing when no size is selected', () => {
      renderWithRouter(<ProductPresentationCore {...defaultProps} product={{ ...mockProduct, sizeChart: chart } as any} />);
      fireEvent.click(screen.getByRole('button', { name: 'Таблица размеров' }));
      expect(screen.getAllByTestId('size-chart-row').every((r) => r.getAttribute('data-selected') === null)).toBe(true);
    });
  });
});
