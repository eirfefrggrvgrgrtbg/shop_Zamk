/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, cleanup, fireEvent, screen } from '@testing-library/react';
import { ProductPresentationCore, type ProductPresentationCoreProps } from '@zamk/shared';

const props: ProductPresentationCoreProps = {
  product: { id: 'p-1', name: 'Coat', brand: 'ZAMK', brandId: 'b-1', price: 100, sellerName: 'Seller', sellerSlug: 'seller' },
  visibleImages: [{ url: 'a.jpg' }, { url: 'b.jpg' }],
  activeImage: 0,
  onActiveImageChange: vi.fn(),
  displayPrice: 100,
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
  ctaText: 'Add',
  sizeSelectionNotice: null,
  refreshErrorNotice: null,
  sizeError: '',
  onColorChange: vi.fn(),
  onSizeChange: vi.fn(),
  onAddToCart: vi.fn(),
  isFavorite: false,
  onToggleFavorite: vi.fn(),
  onScrollToReviews: vi.fn(),
};

describe('ProductPresentationCore: V5 variant is opt-in (Seller Studio preview unchanged)', () => {
  afterEach(cleanup);

  it('omitting `presentation` renders exactly the same markup as the explicit default, with no V5 hooks', () => {
    const a = render(<ProductPresentationCore {...props} />).container.innerHTML;
    cleanup();
    const b = render(<ProductPresentationCore {...props} presentation="default" />).container.innerHTML;
    expect(a).toBe(b);
    expect(a).not.toContain('zv5-');
    expect(a).toContain('Доставка: по России от 2 дней');
    fireEvent.click(screen.getByRole('button', { name: 'Доставка и возврат' }));
    expect(document.body.innerHTML).toContain('2–7 дней');
  });

  it('archive/passport slots are ignored unless provided, and rendered when provided', () => {
    const { container, getByText } = render(
      <ProductPresentationCore {...props} presentation="v5" archiveSlot={<i>ARCH</i>} passportSlot={<b>PASS</b>} />
    );
    expect(getByText('ARCH')).toBeTruthy();
    expect(getByText('PASS')).toBeTruthy();
    for (const hook of ['zv5-grid', 'zv5-gallery', 'zv5-stage', 'zv5-thumbs', 'zv5-buy', 'zv5-brand', 'zv5-title', 'zv5-price', 'zv5-service', 'zv5-info']) {
      expect(container.querySelector('.' + hook), hook).toBeTruthy();
    }
  });
});
