/* @vitest-environment jsdom */
// CATALOG.C5.5 — approved characteristics in the PDP «Характеристики» section.
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { ProductPresentationCore } from '@zamk/shared';
import { mapPublicCharacteristics } from '../../api/publicCatalog';

const baseProps = (product: any) => ({
  product,
  visibleImages: [{ url: 'https://example.com/img1.jpg' }],
  activeImage: 0,
  onActiveImageChange: vi.fn(),
  displayPrice: 7200,
  colors: [],
  sizes: [],
  selectedColorId: null,
  selectedSizeId: null,
  selectedColor: null,
  selectedSize: null,
  selectedVariant: null,
  isResolved: false,
  canAddToCart: false,
  requiresColor: false,
  requiresSize: false,
  isAddingToCart: false,
  isProductUnavailable: false,
  ctaText: 'Добавить в корзину',
  sizeSelectionNotice: null,
  refreshErrorNotice: null,
  sizeError: '',
  reviewsCountText: '',
  onColorChange: vi.fn(),
  onSizeChange: vi.fn(),
  onAddToCart: vi.fn(),
  isFavorite: false,
  onToggleFavorite: vi.fn(),
  onScrollToReviews: vi.fn(),
});

const renderCore = (product: any) =>
  render(
    <MemoryRouter>
      <ProductPresentationCore {...(baseProps(product) as any)} />
    </MemoryRouter>,
  );

afterEach(() => cleanup());

describe('mapPublicCharacteristics', () => {
  it('maps the public API list to readable spec rows', () => {
    expect(mapPublicCharacteristics([{ name: 'Сезон', value: 'Зима' }, { name: 'Посадка', value: 'Свободная' }])).toEqual([
      { label: 'Сезон', value: 'Зима' },
      { label: 'Посадка', value: 'Свободная' },
    ]);
  });

  it('drops empty, malformed and technical (UUID) values; tolerates a missing field', () => {
    expect(mapPublicCharacteristics(undefined)).toEqual([]);
    expect(mapPublicCharacteristics({ Сезон: 'Зима' })).toEqual([]);
    expect(
      mapPublicCharacteristics([
        { name: 'Сезон', value: '  ' },
        { name: '', value: 'Зима' },
        { name: 'Ткань', value: '3f2c1a9e-4b7d-4e2a-9c1f-0a1b2c3d4e5f' },
        { name: 'Сезон', value: 1 },
        null,
        { name: 'Посадка', value: 'Прямая' },
      ]),
    ).toEqual([{ label: 'Посадка', value: 'Прямая' }]);
  });
});

describe('PDP «Характеристики»', () => {
  it('shows approved characteristics in the existing section, composition stays in «Состав и уход» only', () => {
    renderCore({
      id: 'C54-DRESS',
      name: 'Платье',
      price: 7200,
      brand: 'Nord Atelier',
      materialComposition: [{ materialName: 'Хлопок', percentage: 60 }, { materialName: 'Полиэстер', percentage: 40 }],
      attributes: [{ label: 'Сезон', value: 'Зима' }],
    });
    fireEvent.click(screen.getByText('Характеристики'));
    expect(screen.getByText('Сезон')).toBeTruthy();
    expect(screen.getByText('Зима')).toBeTruthy();
    expect(screen.getAllByText(/Хлопок — 60%/)).toHaveLength(1);
    const body = document.body.textContent || '';
    expect(body).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}/i);
  });

  it('no characteristics and no other specs: no empty section', () => {
    renderCore({ name: 'Платье', price: 7200, attributes: [] });
    expect(screen.queryByText('Характеристики')).toBeNull();
  });
});
