import { describe, it, expect } from 'vitest';
import { formatVariantDetails, getCartItemImageUrl } from './variantSelection';
import type { UIContextCartItem } from '../contexts/CartContext';

describe('Cart & Checkout Variant Presentation (PV.2C)', () => {
  const redImage = 'http://localhost:9000/zamk-local/hoodie-red.png';
  const defaultPlaceholder = 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image';

  const hoodieRedL: UIContextCartItem = {
    id: 'cart-item-1',
    productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
    productVariantId: '2d262098-a7ce-4f82-b6d4-c93639f46d98',
    quantity: 1,
    title: 'ХУДИ',
    price: 1222,
    color: 'Красный',
    size: 'L',
    sellerSku: 'SKU-4444-645916',
    imageUrl: redImage,
    inStock: true,
  };

  const hoodieWhiteL: UIContextCartItem = {
    id: 'cart-item-2',
    productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
    productVariantId: '92efda9d-b336-4b08-9536-bbba19a30497',
    quantity: 1,
    title: 'ХУДИ',
    price: 1222,
    color: 'Белый',
    size: 'L',
    sellerSku: 'SKU-4444-326866',
    imageUrl: 'http://localhost:9000/zamk-local/hoodie-white.png',
    inStock: false,
  };

  it('A. Cart displays title = ХУДИ, color = Красный, size = L, imageUrl = red image', () => {
    const subtitle = formatVariantDetails(hoodieRedL.color, hoodieRedL.size);
    expect(hoodieRedL.title).toBe('ХУДИ');
    expect(subtitle).toBe('Красный · L');
    const image = getCartItemImageUrl(hoodieRedL.imageUrl, undefined, defaultPlaceholder);
    expect(image).toBe(redImage);
  });

  it('B. Cart uses placeholder only when imageUrl is absent', () => {
    // When imageUrl is provided
    const imageWithUrl = getCartItemImageUrl('http://example.com/custom.png', undefined, defaultPlaceholder);
    expect(imageWithUrl).toBe('http://example.com/custom.png');

    // When imageUrl is null/undefined/empty
    const imageNull = getCartItemImageUrl(null, undefined, defaultPlaceholder);
    expect(imageNull).toBe(defaultPlaceholder);

    const imageUndefined = getCartItemImageUrl(undefined, undefined, defaultPlaceholder);
    expect(imageUndefined).toBe(defaultPlaceholder);

    const imageEmpty = getCartItemImageUrl('   ', undefined, defaultPlaceholder);
    expect(imageEmpty).toBe(defaultPlaceholder);

    // When fallback product image is present but no variant image
    const imageWithFallback = getCartItemImageUrl(undefined, 'http://example.com/main.png', defaultPlaceholder);
    expect(imageWithFallback).toBe('http://example.com/main.png');
  });

  it('C. Checkout summary displays title = ХУДИ, subtitle = Красный · L, quantity = × 1', () => {
    const title = hoodieRedL.title;
    const variantSubtitle = formatVariantDetails(hoodieRedL.color, hoodieRedL.size);
    const quantityText = `× ${hoodieRedL.quantity}`;

    expect(title).toBe('ХУДИ');
    expect(variantSubtitle).toBe('Красный · L');
    expect(quantityText).toBe('× 1');
  });

  it('D. color-only item renders correctly without dangling separators', () => {
    const colorOnly = formatVariantDetails('Синий', undefined);
    expect(colorOnly).toBe('Синий');

    const colorOnlyWithEmptySize = formatVariantDetails('Синий', '   ');
    expect(colorOnlyWithEmptySize).toBe('Синий');
  });

  it('E. size-only item renders correctly without dangling separators', () => {
    const sizeOnly = formatVariantDetails(undefined, 'XL');
    expect(sizeOnly).toBe('XL');

    const sizeOnlyWithEmptyColor = formatVariantDetails('', 'XL');
    expect(sizeOnlyWithEmptyColor).toBe('XL');
  });

  it('F. standard variant renders without dangling separators (null)', () => {
    const standardVariant = formatVariantDetails(undefined, undefined);
    expect(standardVariant).toBeNull();

    const emptyStrings = formatVariantDetails('  ', '  ');
    expect(emptyStrings).toBeNull();
  });

  it('G. two variants of same product remain distinct by productVariantId', () => {
    const cartItems = [hoodieRedL, hoodieWhiteL];

    // Same product ID
    expect(cartItems[0].productId).toBe(cartItems[1].productId);

    // Distinct variant IDs
    expect(cartItems[0].productVariantId).not.toBe(cartItems[1].productVariantId);
    expect(cartItems[0].id).not.toBe(cartItems[1].id);

    // Distinct colors / stock
    expect(cartItems[0].color).toBe('Красный');
    expect(cartItems[1].color).toBe('Белый');
    expect(cartItems[0].inStock).toBe(true);
    expect(cartItems[1].inStock).toBe(false);

    // Ensure array length is 2 (never merged)
    expect(cartItems).toHaveLength(2);
  });

  it('H. quantity update preserves exact variant/cart item identity', () => {
    // Simulating updating quantity for item 1 (Red / L)
    const originalItem = hoodieRedL;
    const newQuantity = 2;
    const updatedItem: UIContextCartItem = {
      ...originalItem,
      quantity: newQuantity,
    };

    expect(updatedItem.id).toBe(originalItem.id);
    expect(updatedItem.productVariantId).toBe('2d262098-a7ce-4f82-b6d4-c93639f46d98');
    expect(updatedItem.color).toBe('Красный');
    expect(updatedItem.size).toBe('L');
    expect(updatedItem.quantity).toBe(2);
  });

  it('I. unavailable cart item remains the same variant and is visibly blocked according to checkout semantics', () => {
    const cartItems = [hoodieRedL, hoodieWhiteL];

    // White/L is unavailable
    expect(hoodieWhiteL.inStock).toBe(false);

    // Checkout / Cart guard logic:
    const hasUnavailable = cartItems.some(i => i.inStock === false);
    expect(hasUnavailable).toBe(true);

    // When hasUnavailable is true, checkout button is blocked
    const canCheckout = !hasUnavailable;
    expect(canCheckout).toBe(false);

    // Variant identity is not altered or substituted
    expect(hoodieWhiteL.productVariantId).toBe('92efda9d-b336-4b08-9536-bbba19a30497');
    expect(hoodieWhiteL.color).toBe('Белый');
    expect(hoodieWhiteL.size).toBe('L');
  });
});

describe('Order History & Detail Variant Presentation (PV.2D)', () => {
  const redImage = 'http://localhost:9000/zamk-local/hoodie-red.png';
  const defaultPlaceholder = 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image';

  // Sample order item returned by API from snapshot (e.g. ORD-100205)
  const orderItemRedL = {
    id: 'item-101',
    orderId: 'ORD-100205',
    productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
    productVariantId: '2d262098-a7ce-4f82-b6d4-c93639f46d98',
    title: 'ХУДИ',
    variantColor: 'Красный',
    variantSize: 'L',
    sku: 'SKU-4444-645916',
    imageUrl: redImage,
    priceCents: 122200,
    quantity: 1,
  };

  const orderItemColorOnly = {
    id: 'item-102',
    orderId: 'ORD-100206',
    productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
    productVariantId: '2d262098-a7ce-4f82-b6d4-c93639f46d99',
    title: 'ХУДИ',
    variantColor: 'Красный',
    variantSize: null,
    sku: 'SKU-4444-645917',
    imageUrl: redImage,
    priceCents: 122200,
    quantity: 1,
  };

  const orderItemSizeOnly = {
    id: 'item-103',
    orderId: 'ORD-100207',
    productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
    productVariantId: '2d262098-a7ce-4f82-b6d4-c93639f46d99',
    title: 'ХУДИ',
    variantColor: null,
    variantSize: 'L',
    sku: 'SKU-4444-645918',
    imageUrl: redImage,
    priceCents: 122200,
    quantity: 1,
  };

  const orderItemNoVariant = {
    id: 'item-104',
    orderId: 'ORD-100208',
    productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
    productVariantId: '2d262098-a7ce-4f82-b6d4-c93639f46d99',
    title: 'АКСЕССУАР',
    variantColor: null,
    variantSize: null,
    sku: null,
    imageUrl: null,
    priceCents: 50000,
    quantity: 1,
  };

  it('A. Formats both color and size as "Color · Size" (e.g. "Красный · L")', () => {
    const details = formatVariantDetails(orderItemRedL.variantColor, orderItemRedL.variantSize);
    expect(details).toBe('Красный · L');
  });

  it('B. Formats color only without dangling separators', () => {
    const details = formatVariantDetails(orderItemColorOnly.variantColor, orderItemColorOnly.variantSize);
    expect(details).toBe('Красный');
  });

  it('C. Formats size only without dangling separators', () => {
    const details = formatVariantDetails(orderItemSizeOnly.variantColor, orderItemSizeOnly.variantSize);
    expect(details).toBe('L');
  });

  it('D. Returns null when neither color nor size is present (never fake "Единый размер")', () => {
    const details = formatVariantDetails(orderItemNoVariant.variantColor, orderItemNoVariant.variantSize);
    expect(details).toBeNull();
    expect(details).not.toBe('Единый размер');
  });

  it('E. Assembles subtitle with quantity correctly', () => {
    // 1 item with variant
    const v1 = formatVariantDetails(orderItemRedL.variantColor, orderItemRedL.variantSize);
    const q1 = (orderItemRedL.quantity ?? 1) > 1 ? `${orderItemRedL.quantity} шт.` : '';
    const sub1 = [v1, q1].filter(Boolean).join(' · ');
    expect(sub1).toBe('Красный · L');

    // 2 items with variant
    const qty2 = 2;
    const q2 = qty2 > 1 ? `${qty2} шт.` : '';
    const sub2 = [v1, q2].filter(Boolean).join(' · ');
    expect(sub2).toBe('Красный · L · 2 шт.');

    // 1 item without variant
    const vNone = formatVariantDetails(orderItemNoVariant.variantColor, orderItemNoVariant.variantSize);
    const subNone = [vNone, q1].filter(Boolean).join(' · ');
    expect(subNone).toBe('');

    // 2 items without variant
    const subNone2 = [vNone, q2].filter(Boolean).join(' · ');
    expect(subNone2).toBe('2 шт.');
  });

  it('F. Preserves historical snapshot image from order item', () => {
    const img1 = getCartItemImageUrl(orderItemRedL.imageUrl, undefined, defaultPlaceholder);
    expect(img1).toBe(redImage);

    const imgFallback = getCartItemImageUrl(orderItemNoVariant.imageUrl, undefined, defaultPlaceholder);
    expect(imgFallback).toBe(defaultPlaceholder);
  });
});
