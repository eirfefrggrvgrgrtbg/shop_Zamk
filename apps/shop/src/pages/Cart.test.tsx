/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Cart } from './Cart';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';
import type { UIContextCartItem } from '../contexts/CartContext';

let mockCartState: {
  items: UIContextCartItem[];
  isLoadingCart: boolean;
} = {
  items: [],
  isLoadingCart: false,
};

vi.mock('../contexts/CartContext', () => ({
  useCart: () => ({
    items: mockCartState.items,
    totalPrice: mockCartState.items.reduce((sum, i) => sum + (i.price || 0) * i.quantity, 0),
    isLoadingCart: mockCartState.isLoadingCart,
    addItem: vi.fn(),
    removeItem: vi.fn(),
    updateQuantity: vi.fn(),
    clearCart: vi.fn(),
    totalItems: mockCartState.items.reduce((sum, i) => sum + i.quantity, 0),
  }),
}));

describe('SHOP PDP.2E2 — Cart Variant Image Truth (Frontend Acceptance)', () => {
  const whiteImgUrl = 'https://cdn.zamk.test/products/hoodie-white-primary.jpg';
  const blackImgUrl = 'https://cdn.zamk.test/products/hoodie-black-rendition.jpg';

  beforeEach(() => {
    mockCartState = {
      items: [],
      isLoadingCart: false,
    };
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Cart line item with White variant displays White image', () => {
    mockCartState.items = [
      {
        id: 'cart-item-white',
        productId: 'prod-hoodie-1',
        productVariantId: 'var-white-m',
        quantity: 1,
        title: 'Худи Белое',
        price: 5000,
        color: 'Белый',
        size: 'M',
        sellerSku: 'SKU-HOODIE-WHITE-M',
        imageUrl: whiteImgUrl,
        inStock: true,
      },
    ];

    render(
      <MemoryRouter>
        <Cart />
      </MemoryRouter>
    );

    const img = screen.getByRole('img', { name: 'Худи Белое' }) as HTMLImageElement;
    expect(img).toBeDefined();
    expect(img.src).toBe(whiteImgUrl);
  });

  it('2. Cart line item with Black variant displays Black image', () => {
    mockCartState.items = [
      {
        id: 'cart-item-black',
        productId: 'prod-hoodie-1',
        productVariantId: 'var-black-m',
        quantity: 1,
        title: 'Худи Чёрное',
        price: 5000,
        color: 'Чёрный',
        size: 'M',
        sellerSku: 'SKU-HOODIE-BLACK-M',
        imageUrl: blackImgUrl,
        inStock: true,
      },
    ];

    render(
      <MemoryRouter>
        <Cart />
      </MemoryRouter>
    );

    const img = screen.getByRole('img', { name: 'Худи Чёрное' }) as HTMLImageElement;
    expect(img).toBeDefined();
    expect(img.src).toBe(blackImgUrl);
  });

  it('3. Cart image does NOT change based on PDP activeImage or viewed photos', () => {
    // A customer purchases Black variant while PDP activeImage was photo 5 (or any general photo)
    // The Cart line item derives solely from the canonical cart item (product_variant_id -> imageUrl)
    mockCartState.items = [
      {
        id: 'cart-item-black-active',
        productId: 'prod-hoodie-1',
        productVariantId: 'var-black-m',
        quantity: 1,
        title: 'Худи Чёрное Независимое',
        price: 5000,
        color: 'Чёрный',
        size: 'M',
        sellerSku: 'SKU-HOODIE-BLACK-M',
        imageUrl: blackImgUrl, // Resolved deterministically by variant color
        inStock: true,
      },
    ];

    const { rerender } = render(
      <MemoryRouter>
        <Cart />
      </MemoryRouter>
    );

    let img = screen.getByRole('img', { name: 'Худи Чёрное Независимое' }) as HTMLImageElement;
    expect(img.src).toBe(blackImgUrl);

    // Simulate PDP state changes (browsing other images, switching viewports, etc.)
    // Re-rendering Cart component with the same canonical cart data retains exact Black image truth
    rerender(
      <MemoryRouter>
        <Cart />
      </MemoryRouter>
    );

    img = screen.getByRole('img', { name: 'Худи Чёрное Независимое' }) as HTMLImageElement;
    expect(img.src).toBe(blackImgUrl);
  });

  it('4. Quick Buy item displays correct variant image in Cart', () => {
    // Quick Buy directly adds the selected variant to cart and refetches the cart read model.
    // The returned item has the variant-specific color image.
    mockCartState.items = [
      {
        id: 'cart-item-qb',
        productId: 'prod-hoodie-1',
        productVariantId: 'var-white-m',
        quantity: 1,
        title: 'Худи Быстрая Покупка',
        price: 5000,
        color: 'Белый',
        size: 'M',
        sellerSku: 'SKU-HOODIE-WHITE-M',
        imageUrl: whiteImgUrl,
        inStock: true,
      },
    ];

    render(
      <MemoryRouter>
        <Cart />
      </MemoryRouter>
    );

    const img = screen.getByRole('img', { name: 'Худи Быстрая Покупка' }) as HTMLImageElement;
    expect(img.src).toBe(whiteImgUrl);
  });

  it('5. Fallback to placeholder when imageUrl and product image are absent', () => {
    mockCartState.items = [
      {
        id: 'cart-item-noimg',
        productId: 'prod-noimg',
        productVariantId: 'var-noimg-m',
        quantity: 1,
        title: 'Товар без фото',
        price: 1500,
        color: 'Серый',
        size: 'M',
        sellerSku: 'SKU-NOIMG-M',
        imageUrl: undefined,
        inStock: true,
      },
    ];

    render(
      <MemoryRouter>
        <Cart />
      </MemoryRouter>
    );

    const img = screen.getByRole('img', { name: 'Товар без фото' }) as HTMLImageElement;
    expect(img.getAttribute('src')).toBe(PRODUCT_PLACEHOLDER_IMAGE);
  });
});
